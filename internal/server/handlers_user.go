package server

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/users"
	csrf "github.com/utrack/gin-csrf"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func (s *Server) GetAdminUsersIndex(c *gin.Context) {
	currentSession := GetSessionData(c)

	sort := c.Query("sort")
	if sort != "" {
		if currentSession.SortedBy["users"] != sort {
			currentSession.SortedBy["users"] = sort
			currentSession.SortDirection["users"] = "asc"
		} else {
			if currentSession.SortDirection["users"] == "asc" {
				currentSession.SortDirection["users"] = "desc"
			} else {
				currentSession.SortDirection["users"] = "asc"
			}
		}

		if err := UpdateSessionData(c, currentSession); err != nil {
			s.GetHandleError(c, http.StatusInternalServerError, "sort error", "failed to save session")
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/users/")
		return
	}

	search, searching := c.GetQuery("search")
	if searching {
		currentSession.Search["users"] = search

		if err := UpdateSessionData(c, currentSession); err != nil {
			s.GetHandleError(c, http.StatusInternalServerError, "search error", "failed to save session")
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/users/")
		return
	}

	dbUsers := s.users.GetFilteredAndSortedUsersUnscoped(currentSession.SortedBy["users"], currentSession.SortDirection["users"], currentSession.Search["users"])

	c.HTML(http.StatusOK, "admin_user_index.html", gin.H{
		"Route":       c.Request.URL.Path,
		"Alerts":      GetFlashes(c),
		"Session":     currentSession,
		"Static":      s.getStaticData(),
		"Users":       dbUsers,
		"TotalUsers":  len(s.users.GetUsers()),
		"Device":      s.peers.GetDevice(currentSession.DeviceName),
		"DeviceNames": s.GetDeviceNames(),
	})
}

func (s *Server) GetAdminUsersEdit(c *gin.Context) {
	user := s.users.GetUserUnscoped(c.Query("pkey"))

	currentSession, err := s.setFormInSession(c, *user)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_edit_user.html", gin.H{
		"Route":       c.Request.URL.Path,
		"Alerts":      GetFlashes(c),
		"Session":     currentSession,
		"Static":      s.getStaticData(),
		"User":        currentSession.FormData.(users.User),
		"Device":      s.peers.GetDevice(currentSession.DeviceName),
		"DeviceNames": s.GetDeviceNames(),
		"Epoch":       time.Time{},
		"Csrf":        csrf.GetToken(c),
	})
}

func (s *Server) PostAdminUsersEdit(c *gin.Context) {
	currentUser := s.users.GetUserUnscoped(c.Query("pkey"))
	if currentUser == nil {
		SetFlashMessage(c, "invalid user", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/users/")
		return
	}
	urlEncodedKey := url.QueryEscape(c.Query("pkey"))

	currentSession := GetSessionData(c)
	var formUser users.User
	if currentSession.FormData != nil {
		formUser = currentSession.FormData.(users.User)
	}
	if err := c.ShouldBind(&formUser); err != nil {
		_ = s.updateFormInSession(c, formUser)
		SetFlashMessage(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/users/edit?pkey="+urlEncodedKey+"&formerr=bind")
		return
	}

	if formUser.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(formUser.Password), bcrypt.DefaultCost)
		if err != nil {
			_ = s.updateFormInSession(c, formUser)
			SetFlashMessage(c, "failed to hash admin password", "danger")
			c.Redirect(http.StatusSeeOther, "/admin/users/edit?pkey="+urlEncodedKey+"&formerr=bind")
			return
		}
		formUser.Password = string(hashedPassword)
	} else {
		formUser.Password = currentUser.Password
	}

	disabled := c.PostForm("isdisabled") != ""
	if disabled {
		formUser.DeletedAt = gorm.DeletedAt{
			Time:  time.Now(),
			Valid: true,
		}
	} else {
		formUser.DeletedAt = gorm.DeletedAt{}
	}
	formUser.IsAdmin = c.PostForm("isadmin") == "true"

	if err := s.UpdateUser(formUser); err != nil {
		_ = s.updateFormInSession(c, formUser)
		SetFlashMessage(c, "failed to update user: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/users/edit?pkey="+urlEncodedKey+"&formerr=update")
		return
	}

	SetFlashMessage(c, "changes applied successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/users/edit?pkey="+urlEncodedKey)
}

func (s *Server) GetAdminUsersCreate(c *gin.Context) {
	user := users.User{}

	currentSession, err := s.setFormInSession(c, user)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_edit_user.html", gin.H{
		"Route":       c.Request.URL.Path,
		"Alerts":      GetFlashes(c),
		"Session":     currentSession,
		"Static":      s.getStaticData(),
		"User":        currentSession.FormData.(users.User),
		"Device":      s.peers.GetDevice(currentSession.DeviceName),
		"DeviceNames": s.GetDeviceNames(),
		"Epoch":       time.Time{},
		"Csrf":        csrf.GetToken(c),
	})
}

func (s *Server) PostAdminUsersCreate(c *gin.Context) {
	currentSession := GetSessionData(c)
	var formUser users.User
	if currentSession.FormData != nil {
		formUser = currentSession.FormData.(users.User)
	}
	if err := c.ShouldBind(&formUser); err != nil {
		_ = s.updateFormInSession(c, formUser)
		SetFlashMessage(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/users/create?formerr=bind")
		return
	}

	if formUser.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(formUser.Password), bcrypt.DefaultCost)
		if err != nil {
			SetFlashMessage(c, "failed to hash admin password", "danger")
			c.Redirect(http.StatusSeeOther, "/admin/users/create?formerr=bind")
			return
		}
		formUser.Password = string(hashedPassword)
	} else {
		_ = s.updateFormInSession(c, formUser)
		SetFlashMessage(c, "invalid password", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/users/create?formerr=create")
		return
	}

	disabled := c.PostForm("isdisabled") != ""
	if disabled {
		formUser.DeletedAt = gorm.DeletedAt{
			Time:  time.Now(),
			Valid: true,
		}
	} else {
		formUser.DeletedAt = gorm.DeletedAt{}
	}
	formUser.IsAdmin = c.PostForm("isadmin") == "true"
	formUser.Source = users.UserSourceDatabase

	if err := s.CreateUser(formUser, currentSession.DeviceName); err != nil {
		_ = s.updateFormInSession(c, formUser)
		SetFlashMessage(c, "failed to add user: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/users/create?formerr=create")
		return
	}

	SetFlashMessage(c, "user created successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/users/")
}
