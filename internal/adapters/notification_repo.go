package adapters

import (
	"context"
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

// SaveNotificationRecord persists a PeerNotificationRecord to the database.
// The unique index on (peer_identifier, interval_seconds) means a duplicate
// insert will be silently ignored (at-most-once guarantee at the DB level).
func (r *SqlRepo) SaveNotificationRecord(ctx context.Context, rec domain.PeerNotificationRecord) error {
	return r.db.WithContext(ctx).Create(&rec).Error
}

// GetNotificationRecords returns all notification records for the given peer.
func (r *SqlRepo) GetNotificationRecords(
	ctx context.Context,
	peerID domain.PeerIdentifier,
) ([]domain.PeerNotificationRecord, error) {
	var records []domain.PeerNotificationRecord
	err := r.db.WithContext(ctx).
		Where("peer_identifier = ?", peerID).
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}

// DeleteNotificationRecordsForPeer removes all notification records for the given peer.
func (r *SqlRepo) DeleteNotificationRecordsForPeer(ctx context.Context, peerID domain.PeerIdentifier) error {
	return r.db.WithContext(ctx).
		Where("peer_identifier = ?", peerID).
		Delete(&domain.PeerNotificationRecord{}).Error
}

// DeleteNotificationRecordsBefore removes all notification records whose SentAt
// is strictly before the given cutoff time (used for retention pruning).
func (r *SqlRepo) DeleteNotificationRecordsBefore(ctx context.Context, cutoff time.Time) error {
	return r.db.WithContext(ctx).
		Where("sent_at < ?", cutoff).
		Delete(&domain.PeerNotificationRecord{}).Error
}
