package domain

import (
	"time"
)

type BaseModel struct {
	CreatedBy string
	UpdatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PrivateString string

func (PrivateString) MarshalJSON() ([]byte, error) {
	return []byte(`""`), nil
}

func (PrivateString) String() string {
	return ""
}

const (
	DisabledReasonExpired          = "expired"
	DisabledReasonDeleted          = "deleted"
	DisabledReasonUserEdit         = "user edit action"
	DisabledReasonUserCreate       = "user create action"
	DisabledReasonAdminEdit        = "admin edit action"
	DisabledReasonAdminCreate      = "admin create action"
	DisabledReasonApiEdit          = "api edit action"
	DisabledReasonApiCreate        = "api create action"
	DisabledReasonLdapMissing      = "missing in ldap"
	DisabledReasonUserMissing      = "missing user"
	DisabledReasonMigrationDummy   = "migration dummy user"
	DisabledReasonInterfaceMissing = "missing WireGuard interface"
)
