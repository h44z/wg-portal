package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	wgportal "github.com/h44z/wg-portal"
)

func SetupRoutes(s *Server) {
	// Startpage
	s.server.GET("/", s.GetIndex)
	s.server.GET("/favicon.ico", func(c *gin.Context) {
		file, _ := wgportal.Statics.ReadFile("assets/img/favicon.ico")
		c.Data(
			http.StatusOK,
			"image/x-icon",
			file,
		)
	})

	// Auth routes
	auth := s.server.Group("/auth")
	auth.GET("/login", s.GetLogin)
	auth.POST("/login", s.PostLogin)
	auth.GET("/logout", s.GetLogout)

	// Admin routes
	admin := s.server.Group("/admin")
	admin.Use(s.RequireAuthentication("admin"))
	admin.GET("/", s.GetAdminIndex)
	admin.GET("/device/edit", s.GetAdminEditInterface)
	admin.POST("/device/edit", s.PostAdminEditInterface)
	admin.GET("/device/download", s.GetInterfaceConfig)
	admin.GET("/device/write", s.GetSaveConfig)
	admin.GET("/device/applyglobals", s.GetApplyGlobalConfig)
	admin.GET("/peer/edit", s.GetAdminEditPeer)
	admin.POST("/peer/edit", s.PostAdminEditPeer)
	admin.GET("/peer/create", s.GetAdminCreatePeer)
	admin.POST("/peer/create", s.PostAdminCreatePeer)
	admin.GET("/peer/createldap", s.GetAdminCreateLdapPeers)
	admin.POST("/peer/createldap", s.PostAdminCreateLdapPeers)
	admin.GET("/peer/delete", s.GetAdminDeletePeer)
	admin.GET("/peer/download", s.GetPeerConfig)
	admin.GET("/peer/email", s.GetPeerConfigMail)

	admin.GET("/users/", s.GetAdminUsersIndex)
	admin.GET("/users/create", s.GetAdminUsersCreate)
	admin.POST("/users/create", s.PostAdminUsersCreate)
	admin.GET("/users/edit", s.GetAdminUsersEdit)
	admin.POST("/users/edit", s.PostAdminUsersEdit)

	// User routes
	user := s.server.Group("/user")
	user.Use(s.RequireAuthentication("")) // empty scope = all logged in users
	user.GET("/qrcode", s.GetPeerQRCode)
	user.GET("/profile", s.GetUserIndex)
	user.GET("/download", s.GetPeerConfig)
	user.GET("/email", s.GetPeerConfigMail)
	user.GET("/status", s.GetPeerStatus)
}

func (s *Server) RequireAuthentication(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := GetSessionData(c)

		if !session.LoggedIn {
			// Abort the request with the appropriate error code
			c.Abort()
			c.Redirect(http.StatusSeeOther, "/auth/login?err=loginreq")
			return
		}

		if scope == "admin" && !session.IsAdmin {
			// Abort the request with the appropriate error code
			c.Abort()
			s.GetHandleError(c, http.StatusUnauthorized, "unauthorized", "not enough permissions")
			return
		}

		// default case if some randome scope was set...
		if scope != "" && !session.IsAdmin {
			// Abort the request with the appropriate error code
			c.Abort()
			s.GetHandleError(c, http.StatusUnauthorized, "unauthorized", "not enough permissions")
			return
		}

		// Continue down the chain to handler etc
		c.Next()
	}
}
