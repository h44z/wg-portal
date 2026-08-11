package domain

import "time"

// PeerNotificationRecord persists the fact that an expiry warning email has been sent
// for a given peer and notification interval. The composite unique index on
// (peer_identifier, interval_seconds) enforces the at-most-once guarantee at the DB level.
type PeerNotificationRecord struct {
	ID                uint            `gorm:"primaryKey;autoIncrement"`
	PeerIdentifier    PeerIdentifier  `gorm:"uniqueIndex:idx_peer_interval;index;column:peer_identifier;not null"`
	IntervalSeconds   int64           `gorm:"uniqueIndex:idx_peer_interval;column:interval_seconds;not null"`
	SentAt            time.Time       `gorm:"column:sent_at;not null"`
	Status            string          `gorm:"column:status;not null;default:sent"`
	StatusDescription string          `gorm:"column:status_description"`
}

// TableName returns the database table name for PeerNotificationRecord.
func (PeerNotificationRecord) TableName() string {
	return "peer_notification_records"
}
