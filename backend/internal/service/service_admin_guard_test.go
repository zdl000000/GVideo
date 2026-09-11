package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

func newAdminGuardService(t *testing.T, adminUsername string) (*Service, *repository.Repository, context.Context) {
	t.Helper()
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "admin-guard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	repo := repository.New(db)
	svc := New(repo, config.Config{
		SessionTTL:    time.Hour,
		MediaDir:      filepath.Join(dir, "media"),
		AdminUsername: adminUsername,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return svc, repo, context.Background()
}

// The reserved administrator name is claimable once; afterwards the name is
// locked and case variants are rejected, so nobody can squat it later.
func TestReservedAdminNameRegistration(t *testing.T) {
	svc, _, ctx := newAdminGuardService(t, "site_admin")
	created, err := svc.Register(ctx, "site_admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if !created.User.IsAdmin {
		t.Fatalf("reserved username did not receive admin flag: %#v", created.User)
	}
	session, err := svc.Authenticate(ctx, created.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !session.User.IsAdmin {
		t.Fatal("session did not carry admin flag")
	}
	if _, err := svc.Register(ctx, "Site_Admin", "password123"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("case variant registration error = %v", err)
	}
}

func TestOrdinaryRegistrationIsNotAdmin(t *testing.T) {
	svc, _, ctx := newAdminGuardService(t, "site_admin")
	created, err := svc.Register(ctx, "ordinary_user", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if created.User.IsAdmin {
		t.Fatal("ordinary registration received admin flag")
	}
	session, err := svc.Authenticate(ctx, created.Token)
	if err != nil {
		t.Fatal(err)
	}
	if session.User.IsAdmin {
		t.Fatal("ordinary session carried admin flag")
	}
}

// Regression for the privilege-escalation fix: renaming to the reserved
// administrator name must be rejected, while profile-only edits that keep the
// current name stay allowed.
func TestRenameToReservedAdminNameRejected(t *testing.T) {
	svc, _, ctx := newAdminGuardService(t, "site_admin")
	if _, err := svc.Register(ctx, "site_admin", "password123"); err != nil {
		t.Fatal(err)
	}
	attacker, err := svc.Register(ctx, "attacker_01", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateProfile(ctx, UpdateProfileInput{UserID: attacker.User.ID, Username: "site_admin"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("rename error = %v", err)
	}
	if _, err := svc.UpdateProfile(ctx, UpdateProfileInput{UserID: attacker.User.ID, Username: "SITE_ADMIN"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("case variant rename error = %v", err)
	}
	updated, err := svc.UpdateProfile(ctx, UpdateProfileInput{UserID: attacker.User.ID, Username: "attacker_01", Bio: "updated"})
	if err != nil {
		t.Fatalf("self profile edit error = %v", err)
	}
	if updated.IsAdmin {
		t.Fatal("profile edit granted admin flag")
	}
}

// Once an administrator exists under any username, the reserved name stays
// locked even if the configured administrator name changes.
func TestReservedNameLockedWhenAdminExistsUnderOtherName(t *testing.T) {
	svc, repo, ctx := newAdminGuardService(t, "site_admin")
	ops, err := svc.Register(ctx, "ops_admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetAdminFlag(ctx, ops.User.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Register(ctx, "site_admin", "password123"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("reserved name registration error = %v", err)
	}
	session, err := svc.Authenticate(ctx, ops.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !session.User.IsAdmin {
		t.Fatal("granted administrator lost the flag")
	}
}

// The reserved-name comparison ignores surrounding whitespace in the
// configured value.
func TestAdminNameMatchingIgnoresSurroundingWhitespace(t *testing.T) {
	svc, _, ctx := newAdminGuardService(t, " site_admin ")
	created, err := svc.Register(ctx, "site_admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if !created.User.IsAdmin {
		t.Fatal("padded administrator name did not match the reservation")
	}
}

// Without a configured administrator name nobody is granted automatically.
func TestEmptyAdminNameNeverGrantsAdmin(t *testing.T) {
	svc, _, ctx := newAdminGuardService(t, "")
	created, err := svc.Register(ctx, "site_admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if created.User.IsAdmin {
		t.Fatal("empty administrator name granted the admin flag")
	}
}

// Malformed session cookies are rejected by shape before any database lookup.
func TestAuthenticateRejectsMalformedSessionTokens(t *testing.T) {
	svc, _, ctx := newAdminGuardService(t, "")
	for _, token := range []string{"", "short", "not-a-token", strings.Repeat("Z", 64), strings.Repeat("a", 63), strings.Repeat("a", 65)} {
		if _, err := svc.Authenticate(ctx, token); !errors.Is(err, domain.ErrInvalidSession) {
			t.Fatalf("token %q error = %v, want ErrInvalidSession", token, err)
		}
	}
}
