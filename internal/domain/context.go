package domain

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
)

const CtxUserInfo = "userInfo"

const (
	CtxSystemAdminId    = "_WG_SYS_ADMIN_"
	CtxUnknownUserId    = "_WG_SYS_UNKNOWN_"
	CtxSystemLdapSyncer = "_WG_SYS_LDAP_SYNCER_"
	CtxSystemWgImporter = "_WG_SYS_WG_IMPORTER_"
	CtxSystemV1Migrator = "_WG_SYS_V1_MIGRATOR_"
)

type ContextUserInfo struct {
	Id      UserIdentifier
	IsAdmin bool
}

func (u *ContextUserInfo) String() string {
	return fmt.Sprintf("%s|%t", u.Id, u.IsAdmin)
}

func (u *ContextUserInfo) UserId() string {
	return string(u.Id)
}

// DefaultContextUserInfo returns a default context user info.
func DefaultContextUserInfo() *ContextUserInfo {
	return &ContextUserInfo{
		Id:      CtxUnknownUserId,
		IsAdmin: false,
	}
}

// SystemAdminContextUserInfo returns a context user info for the system admin.
func SystemAdminContextUserInfo() *ContextUserInfo {
	return &ContextUserInfo{
		Id:      CtxSystemAdminId,
		IsAdmin: true,
	}
}

// SetUserInfoFromGin sets the user info from the gin context to the request context.
func SetUserInfoFromGin(c *gin.Context) context.Context {
	ginUserInfo, exists := c.Get(CtxUserInfo)

	info := DefaultContextUserInfo()
	if exists {
		if ginInfo, ok := ginUserInfo.(*ContextUserInfo); ok {
			info = ginInfo
		}
	}

	ctx := SetUserInfo(c.Request.Context(), info)
	return ctx
}

// SetUserInfo sets the user info in the context.
func SetUserInfo(ctx context.Context, info *ContextUserInfo) context.Context {
	ctx = context.WithValue(ctx, CtxUserInfo, info)
	return ctx
}

// GetUserInfo returns the user info from the context.
func GetUserInfo(ctx context.Context) *ContextUserInfo {
	rawInfo := ctx.Value(CtxUserInfo)
	if rawInfo == nil {
		return DefaultContextUserInfo()
	}

	if info, ok := rawInfo.(*ContextUserInfo); ok {
		return info
	}

	return DefaultContextUserInfo()
}

// ValidateUserAccessRights checks if the current user has access rights to the requested user.
// If the user is an admin, access is granted.
func ValidateUserAccessRights(ctx context.Context, requiredUser UserIdentifier) error {
	sessionUser := GetUserInfo(ctx)

	if sessionUser.IsAdmin {
		return nil // Admins can do everything
	}

	if sessionUser.Id == requiredUser {
		return nil // User can access own data
	}

	slog.Warn("insufficient permissions",
		"user", sessionUser.Id,
		"requiredUser", requiredUser,
		"stack", GetStackTrace())
	return ErrNoPermission
}

// ValidateAdminAccessRights checks if the current user has admin access rights.
func ValidateAdminAccessRights(ctx context.Context) error {
	sessionUser := GetUserInfo(ctx)

	if sessionUser.IsAdmin {
		return nil
	}

	slog.Warn("insufficient admin permissions",
		"user", sessionUser.Id,
		"stack", GetStackTrace())
	return ErrNoPermission
}
