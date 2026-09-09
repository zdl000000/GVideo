package service

import (
	"context"

	"gvideo/backend/internal/domain"
)

func (s *Service) Notifications(ctx context.Context, userID int64, page, pageSize int) (domain.NotificationPage, error) {
	if userID <= 0 {
		return domain.NotificationPage{}, domain.ErrInvalidInput
	}
	result, err := s.repo.ListNotifications(ctx, userID, page, pageSize)
	if err != nil {
		return domain.NotificationPage{}, err
	}
	return result, nil
}

func (s *Service) MarkNotificationRead(ctx context.Context, userID, notificationID int64) error {
	if userID <= 0 || notificationID <= 0 {
		return domain.ErrInvalidInput
	}
	return s.repo.MarkNotificationRead(ctx, userID, notificationID)
}

func (s *Service) MarkAllNotificationsRead(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return domain.ErrInvalidInput
	}
	return s.repo.MarkAllNotificationsRead(ctx, userID)
}
