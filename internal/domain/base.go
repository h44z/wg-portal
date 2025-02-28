package domain

import (
	"database/sql/driver"
	"errors"
	"time"
)

type BaseModel struct {
	CreatedBy string
	UpdatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PrivateString string

func (ps *PrivateString) MarshalJSON() ([]byte, error) {
	return []byte(`""`), nil
}

func (ps *PrivateString) String() string {
	return ""
}

func (ps *PrivateString) Value() (driver.Value, error) {
	if ps == nil {
		return nil, nil
	}
	if len(*ps) == 0 {
		return nil, nil
	}
	return string(*ps), nil
}

func (ps *PrivateString) Scan(value any) error {
	if value == nil {
		*ps = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		*ps = PrivateString(v)
	case []byte:
		*ps = PrivateString(v)
	default:
		return errors.New("invalid type for PrivateString")
	}
	return nil
}

const (
	DisabledReasonExpired          = "expired"
	DisabledReasonDeleted          = "deleted"
	DisabledReasonUserDisabled     = "user disabled"
	DisabledReasonUserDeleted      = "user deleted"
	DisabledReasonAdmin            = "disabled by admin"
	DisabledReasonApi              = "disabled through api"
	DisabledReasonLdapMissing      = "missing in ldap"
	DisabledReasonMigrationDummy   = "migration dummy user"
	DisabledReasonInterfaceMissing = "missing WireGuard interface"

	LockedReasonAdmin = "locked by admin"
	LockedReasonApi   = "locked by admin"
)
