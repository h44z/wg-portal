package domain

import (
	"context"
	"time"
)

type AuditSeverityLevel string

const AuditSeverityLevelLow AuditSeverityLevel = "low"
const AuditSeverityLevelHigh AuditSeverityLevel = "high"

type AuditEntry struct {
	UniqueId  uint64    `gorm:"primaryKey;autoIncrement:true;column:id"`
	CreatedAt time.Time `gorm:"column:created_at;index:idx_au_created"`

	ContextUser string `gorm:"column:context_user;index:idx_au_context_user"`

	Severity AuditSeverityLevel `gorm:"column:severity;index:idx_au_severity"`

	Origin string `gorm:"column:origin"` // origin: for example user auth, stats, ...

	Message string `gorm:"column:message"`
}

type AuditEventWrapper[T any] struct {
	Ctx    context.Context
	Source string
	Event  T
}
