package domain

import "time"

type AuditSeverityLevel string

const AuditSeverityLevelLow AuditSeverityLevel = "low"

type AuditEntry struct {
	UniqueId  uint64    `gorm:"primaryKey;autoIncrement:true;column:id"`
	CreatedAt time.Time `gorm:"column:created_at;index:idx_au_created"`

	Severity AuditSeverityLevel `gorm:"column:severity;index:idx_au_severity"`

	Origin string `gorm:"column:origin"` // origin: for example user auth, stats, ...

	Message string `gorm:"column:message"`
}
