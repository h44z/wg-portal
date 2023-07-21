package audit

import (
	"context"
	"github.com/h44z/wg-portal/internal/domain"
)

type DatabaseRepo interface {
	SaveAuditEntry(ctx context.Context, entry *domain.AuditEntry) error
}
