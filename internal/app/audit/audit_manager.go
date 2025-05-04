package audit

import (
	"context"
	"fmt"

	"github.com/h44z/wg-portal/internal/domain"
)

type ManagerDatabaseRepo interface {
	// GetAllAuditEntries retrieves all audit entries from the database.
	// The entries are ordered by timestamp, with the newest entries first.
	GetAllAuditEntries(ctx context.Context) ([]domain.AuditEntry, error)
}

type Manager struct {
	db ManagerDatabaseRepo
}

func NewManager(db ManagerDatabaseRepo) *Manager {
	return &Manager{db: db}
}

func (m *Manager) GetAll(ctx context.Context) ([]domain.AuditEntry, error) {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return nil, domain.ErrNoPermission
	}

	entries, err := m.db.GetAllAuditEntries(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit entries: %w", err)
	}

	return entries, nil
}
