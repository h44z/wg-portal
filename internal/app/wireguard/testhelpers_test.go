package wireguard

import (
	"context"
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

// noopNotificationRepository is a no-op implementation of NotificationRepository
// for use in tests that construct Manager without a real notifRepo.
// It prevents nil-pointer panics when UpdatePeer or DeletePeer call notifRepo methods.
type noopNotificationRepository struct{}

func (noopNotificationRepository) SaveNotificationRecord(_ context.Context, _ domain.PeerNotificationRecord) error {
	return nil
}

func (noopNotificationRepository) GetNotificationRecords(_ context.Context, _ domain.PeerIdentifier) ([]domain.PeerNotificationRecord, error) {
	return nil, nil
}

func (noopNotificationRepository) DeleteNotificationRecordsForPeer(_ context.Context, _ domain.PeerIdentifier) error {
	return nil
}

func (noopNotificationRepository) DeleteNotificationRecordsBefore(_ context.Context, _ time.Time) error {
	return nil
}
