package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gvideo/backend/internal/domain"
)

func (s *Service) Register(ctx context.Context, username, password string) (CreatedSession, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) || len(password) < 8 || len(password) > 72 {
		return CreatedSession{}, domain.ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return CreatedSession{}, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.repo.CreateUser(ctx, username, string(hash))
	if err != nil {
		return CreatedSession{}, err
	}
	return s.newSession(ctx, user)
}

func (s *Service) Login(ctx context.Context, username, password string) (CreatedSession, error) {
	user, hash, err := s.repo.UserAuthByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return CreatedSession{}, domain.ErrUnauthorized
		}
		return CreatedSession{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return CreatedSession{}, domain.ErrUnauthorized
	}
	return s.newSession(ctx, user)
}

func (s *Service) newSession(ctx context.Context, user domain.User) (CreatedSession, error) {
	token, err := randomToken(32)
	if err != nil {
		return CreatedSession{}, err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return CreatedSession{}, err
	}
	if err := s.repo.CreateSession(ctx, hashToken(token), user.ID, csrf, time.Now().Add(s.cfg.SessionTTL)); err != nil {
		return CreatedSession{}, err
	}
	user.IsAdmin = s.IsAdmin(user.Username)
	return CreatedSession{Token: token, CSRFToken: csrf, User: user}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (domain.Session, error) {
	if token == "" {
		return domain.Session{}, domain.ErrInvalidSession
	}
	session, err := s.repo.SessionByHash(ctx, hashToken(token))
	if err != nil {
		return domain.Session{}, err
	}
	session.User.IsAdmin = s.IsAdmin(session.User.Username)
	return session, nil
}

func (s *Service) IsAdmin(username string) bool {
	return s.cfg.AdminUsername != "" && strings.EqualFold(strings.TrimSpace(username), strings.TrimSpace(s.cfg.AdminUsername))
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, hashToken(token))
}

func (s *Service) ChangePassword(ctx context.Context, userID int64, currentToken, currentPassword, newPassword string) error {
	if userID <= 0 || currentToken == "" || len(newPassword) < 8 || len(newPassword) > 72 {
		return domain.ErrInvalidInput
	}
	_, hash, err := s.repo.UserAuthByID(ctx, userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)) != nil {
		return domain.ErrUnauthorized
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if err := s.repo.UpdatePasswordHash(ctx, userID, string(newHash)); err != nil {
		return err
	}
	return s.repo.DeleteOtherSessions(ctx, userID, hashToken(currentToken))
}
