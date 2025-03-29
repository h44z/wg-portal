package model

import (
	"github.com/h44z/wg-portal/internal/domain"
)

type AuditEntry struct {
	Id        uint64 `json:"Id"`
	Timestamp string `json:"Timestamp"`

	ContextUser string `json:"ContextUser"`
	Severity    string `json:"Severity"`
	Origin      string `json:"Origin"` // origin: for example user auth, stats, ...
	Message     string `message:"Message"`
}

// NewAuditEntry creates a REST API AuditEntry from a domain AuditEntry.
func NewAuditEntry(src domain.AuditEntry) AuditEntry {
	return AuditEntry{
		Id:          src.UniqueId,
		Timestamp:   src.CreatedAt.Format("2006-01-02 15:04:05"),
		ContextUser: src.ContextUser,
		Severity:    string(src.Severity),
		Origin:      src.Origin,
		Message:     src.Message,
	}
}

// NewAuditEntries creates a slice of REST API AuditEntry from a slice of domain AuditEntry.
func NewAuditEntries(src []domain.AuditEntry) []AuditEntry {
	dst := make([]AuditEntry, 0, len(src))
	for _, entry := range src {
		dst = append(dst, NewAuditEntry(entry))
	}
	return dst
}
