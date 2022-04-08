package ui

import (
	"net/http"
	"time"

	csrf "github.com/utrack/gin-csrf"

	"github.com/h44z/wg-portal/internal/persistence"

	"github.com/gin-gonic/gin"
)

func (h *handler) handleAdminUserIndexGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)
		users, err := h.backend.GetAllUsers()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to load users")
			return
		}

		c.HTML(http.StatusOK, "admin_user_index.gohtml", gin.H{
			"Route":   c.Request.URL.Path,
			"Alerts":  h.session.GetFlashes(c),
			"Session": currentSession,
			"Static":  h.getStaticData(),
			"Users":   users,
		})
	}
}

func (h *handler) handleAdminUserCreateGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		c.HTML(http.StatusOK, "admin_user_edit.gohtml", gin.H{
			"Route":   c.Request.URL.Path,
			"Alerts":  h.session.GetFlashes(c),
			"Session": currentSession,
			"Static":  h.getStaticData(),
			"Csrf":    csrf.GetToken(c),
			"Epoch":   time.Time{},
			"User":    &persistence.User{},
		})
	}
}

func (h *handler) handleAdminUserCreatePost() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: implement
	}
}

func (h *handler) handleAdminUserEditGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		uid := c.Query("uid")
		user, err := h.backend.GetUser(persistence.UserIdentifier(uid))
		if err != nil {
			h.HandleError(c, http.StatusBadRequest, err, "invalid user")
			return
		}

		c.HTML(http.StatusOK, "admin_user_edit.gohtml", gin.H{
			"Route":   c.Request.URL.Path,
			"Alerts":  h.session.GetFlashes(c),
			"Session": currentSession,
			"Static":  h.getStaticData(),
			"Epoch":   time.Time{},
			"User":    user,
		})
	}
}

func (h *handler) handleAdminUserEditPost() gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: implement
	}
}
