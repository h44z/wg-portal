package domain

import (
	"context"

	"github.com/gin-gonic/gin"
)

const CtxUserInfo = "userInfo"

type ContextUserInfo struct {
	Id      UserIdentifier
	IsAdmin bool
}

func DefaultContextUserInfo() *ContextUserInfo {
	return &ContextUserInfo{
		Id:      "_WG_SYS_UNKNOWN_",
		IsAdmin: false,
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

	ctx := context.WithValue(c.Request.Context(), CtxUserInfo, info)
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
