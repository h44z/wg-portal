package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(s *Server) {
	// Startpage
	s.server.GET("/", s.GetIndex)

	// Auth routes
	auth := s.server.Group("/auth")
	auth.GET("/login", s.GetLogin)
	auth.POST("/login", s.PostLogin)
	auth.GET("/logout", s.GetLogout)

	// Admin routes
	admin := s.server.Group("/admin")
	admin.Use(s.RequireAuthentication(s.config.AdminLdapGroup))
	admin.GET("/", s.GetAdminIndex)

	// User routes
	user := s.server.Group("/user")
	user.Use(s.RequireAuthentication("")) // empty scope = all logged in users
	user.GET("/qrcode", s.GetUserQRCode)
}

func (s *Server) RequireAuthentication(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := s.getSessionData(c)

		if !session.LoggedIn {
			// Abort the request with the appropriate error code
			c.Abort()
			c.Redirect(http.StatusSeeOther, s.config.AuthRoutePrefix+"/login?err=loginreq")
			return
		}

		if scope != "" && !s.ldapUsers.IsInGroup(session.UserName, s.config.AdminLdapGroup) && // admins always have access
			!s.ldapUsers.IsInGroup(session.UserName, scope) {
			// Abort the request with the appropriate error code
			c.Abort()
			s.HandleError(c, http.StatusUnauthorized, "unauthorized", "not enough permissions")
			return
		}

		// Continue down the chain to handler etc
		c.Next()
	}
}
