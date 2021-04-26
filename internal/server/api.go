package server

// go get -u github.com/swaggo/swag/cmd/swag
// run: swag init --parseDependency --parseInternal --generalInfo api.go
// in the internal/server folder
import (
	"encoding/json"
	"net/http"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/users"
)

// @title WireGuard Portal API
// @version 1.0
// @description WireGuard Portal API for managing users and peers.

// @license.name MIT
// @license.url https://github.com/h44z/wg-portal/blob/master/LICENSE.txt

// @securityDefinitions.basic ApiBasicAuth
// @in header
// @name Authorization

// @BasePath /api/v1

// ApiServer is a simple wrapper struct so that we can have fresh member function names.
type ApiServer struct {
	s *Server
}

type ApiError struct {
	Message string
}

// GetUsers godoc
// @Summary Retrieves all users
// @Produce json
// @Success 200 {object} []users.User
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /users [get]
// @Security ApiBasicAuth
func (s *ApiServer) GetUsers(c *gin.Context) {
	allUsers := s.s.users.GetUsersUnscoped()
	for i := range allUsers {
		allUsers[i].Password = "" // do not publish password...
	}

	c.JSON(http.StatusOK, allUsers)
}

// GetUser godoc
// @Summary Retrieves user based on given Email
// @Produce json
// @Param email path string true "User Email"
// @Success 200 {object} users.User
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /user/{email} [get]
// @Security ApiBasicAuth
func (s *ApiServer) GetUser(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Param("email")))

	if email == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "email parameter must be specified"})
		return
	}
	user := s.s.users.GetUserUnscoped(c.Param("email"))
	if user == nil {
		c.JSON(http.StatusNotFound, ApiError{Message: "user not found"})
		return
	}
	user.Password = "" // do not send password...
	c.JSON(http.StatusOK, user)
}

// PostUser godoc
// @Summary Creates a new user based on the given user model
// @Produce json
// @Success 200 {object} users.User
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /users [post]
// @Security ApiBasicAuth
func (s *ApiServer) PostUser(c *gin.Context) {
	newUser := users.User{}
	if err := c.BindJSON(&newUser); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	if user := s.s.users.GetUserUnscoped(newUser.Email); user != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: "user already exists"})
		return
	}

	if err := s.s.CreateUser(newUser, s.s.wg.Cfg.GetDefaultDeviceName()); err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	user := s.s.users.GetUserUnscoped(newUser.Email)
	if user == nil {
		c.JSON(http.StatusNotFound, ApiError{Message: "user not found"})
		return
	}
	user.Password = "" // do not send password...
	c.JSON(http.StatusOK, user)
}

// PutUser godoc
// @Summary Updates a user based on the given user model
// @Produce json
// @Param email path string true "User Email"
// @Success 200 {object} users.User
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /user/{email} [put]
// @Security ApiBasicAuth
func (s *ApiServer) PutUser(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Param("email")))
	if email == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "email parameter must be specified"})
		return
	}

	updateUser := users.User{}
	if err := c.BindJSON(&updateUser); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	// Changing email address is not allowed
	if email != updateUser.Email {
		c.JSON(http.StatusBadRequest, ApiError{Message: "email parameter must match the model email address"})
		return
	}

	if user := s.s.users.GetUserUnscoped(email); user == nil {
		c.JSON(http.StatusNotFound, ApiError{Message: "user does not exist"})
		return
	}

	if err := s.s.UpdateUser(updateUser); err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	user := s.s.users.GetUserUnscoped(email)
	if user == nil {
		c.JSON(http.StatusNotFound, ApiError{Message: "user not found"})
		return
	}
	user.Password = "" // do not send password...
	c.JSON(http.StatusOK, user)
}

// PatchUser godoc
// @Summary Updates a user based on the given partial user model
// @Produce json
// @Param email path string true "User Email"
// @Success 200 {object} users.User
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /user/{email} [patch]
// @Security ApiBasicAuth
func (s *ApiServer) PatchUser(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Param("email")))
	if email == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "email parameter must be specified"})
		return
	}

	patch, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	user := s.s.users.GetUserUnscoped(email)
	if user == nil {
		c.JSON(http.StatusNotFound, ApiError{Message: "user does not exist"})
		return
	}
	userData, err := json.Marshal(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	mergedUserData, err := jsonpatch.MergePatch(userData, patch)
	var mergedUser users.User
	err = json.Unmarshal(mergedUserData, &mergedUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	// CHanging email address is not allowed
	if email != mergedUser.Email {
		c.JSON(http.StatusBadRequest, ApiError{Message: "email parameter must match the model email address"})
		return
	}

	if err := s.s.UpdateUser(mergedUser); err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	user = s.s.users.GetUserUnscoped(email)
	if user == nil {
		c.JSON(http.StatusNotFound, ApiError{Message: "user not found"})
		return
	}
	user.Password = "" // do not send password...
	c.JSON(http.StatusOK, user)
}

// DeleteUser godoc
// @Summary Deletes the specified user
// @Produce json
// @Param email path string true "User Email"
// @Success 204 "No content"
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /user/{email} [delete]
// @Security ApiBasicAuth
func (s *ApiServer) DeleteUser(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Param("email")))
	if email == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "email parameter must be specified"})
		return
	}

	var user *users.User
	if user = s.s.users.GetUserUnscoped(email); user == nil {
		c.JSON(http.StatusNotFound, ApiError{Message: "user does not exist"})
		return
	}

	if err := s.s.DeleteUser(*user); err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
