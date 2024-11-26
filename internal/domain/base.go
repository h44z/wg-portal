package domain

import (
	"errors"
	"time"

	"database/sql/driver"
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

func (ps PrivateString) Value() (driver.Value, error) {
	if len(ps) == 0 {
        return nil, nil
    }
	return string(ps), nil
}

func (ps *PrivateString) Scan(value interface{}) error {
    if value == nil {
        *ps = ""
        return nil
    }
    switch v := value.(type) {
    case string:
        *ps = PrivateString(v)
    case []byte:
        *ps = PrivateString(string(v))
    default:
        return errors.New("invalid type for PrivateString")
    }
    return nil
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
