package domain

import "time"

// ExpiryDisplayLayout is the short, human-readable layout of peer expiry dates used in messages
// (disabled reason, expiry notification emails). It matches the format shown in the web frontend.
// Values must be converted to UTC before formatting.
const ExpiryDisplayLayout = "02.01.2006 15:04 UTC"

// FormatExpiryDate formats the given expiry date using ExpiryDisplayLayout.
func FormatExpiryDate(t time.Time) string {
	return t.UTC().Format(ExpiryDisplayLayout)
}

// PeerNotificationRecord persists the fact that an expiry warning email has been sent
// for a given peer and notification interval. The composite unique index on
// (peer_identifier, interval_seconds) enforces the at-most-once guarantee at the DB level.
type PeerNotificationRecord struct {
	ID                uint           `gorm:"primaryKey;autoIncrement"`
	PeerIdentifier    PeerIdentifier `gorm:"uniqueIndex:idx_peer_interval;index;column:peer_identifier;not null"`
	IntervalSeconds   int64          `gorm:"uniqueIndex:idx_peer_interval;column:interval_seconds;not null"`
	SentAt            time.Time      `gorm:"column:sent_at;not null"`
	Status            string         `gorm:"column:status;not null;default:sent"`
	StatusDescription string         `gorm:"column:status_description"`
}

// TableName returns the database table name for PeerNotificationRecord.
func (PeerNotificationRecord) TableName() string {
	return "peer_notification_records"
}
