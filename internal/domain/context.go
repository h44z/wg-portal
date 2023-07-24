package domain

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
)

const CtxUserInfo = "userInfo"

const (
	CtxSystemAdminId = "_WG_SYS_ADMIN_"
	CtxUnknownUserId = "_WG_SYS_UNKNOWN_"
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

func DefaultContextUserInfo() *ContextUserInfo {
	return &ContextUserInfo{
		Id:      CtxUnknownUserId,
		IsAdmin: false,
	}
}

func SystemAdminContextUserInfo() *ContextUserInfo {
	return &ContextUserInfo{
		Id:      CtxSystemAdminId,
		IsAdmin: true,
	}
}

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

func SetUserInfo(ctx context.Context, info *ContextUserInfo) context.Context {
	ctx = context.WithValue(ctx, CtxUserInfo, info)
	return ctx
}

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
