package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/modules/comments"
	"gvideo/backend/internal/modules/interactions"
	"gvideo/backend/internal/modules/moderation"
	"gvideo/backend/internal/modules/notifications"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
	"gvideo/backend/internal/service"
)

// The preserved「通知编号无效」message is part of the endpoint contract for
// malformed notification IDs and must survive the module migration.
func TestNotificationInvalidIDKeepsDedicatedMessage(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "notif-id.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Config{MediaDir: filepath.Join(dir, "media"), SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(service.New(repository.New(db), cfg, logger),
		moderation.NewService(moderation.NewRepository(db)),
		notifications.NewService(notifications.NewRepository(db)),
		comments.NewService(comments.NewRepository(db), nil, logger),
		interactions.NewService(interactions.NewRepository(db), nil, logger),
		cfg, logger).Routes()
	user := registerTestUser(t, handler, "notif_id_user")

	request := authorizedRequest(http.MethodPatch, "/api/v1/me/notifications/nope/read", nil, user.Cookie, user.CSRFToken)
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusBadRequest || !strings.Contains(result.Body.String(), "通知编号无效") {
		t.Fatalf("invalid notification id status=%d body=%s", result.Code, result.Body.String())
	}
}
