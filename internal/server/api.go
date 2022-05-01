package server

// go get -u github.com/swaggo/swag/cmd/swag
// run: swag init --parseDependency --parseInternal --generalInfo api.go
// in the internal/server folder
import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/common"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/h44z/wg-portal/internal/wireguard"
)

// @title WireGuard Portal API
// @version 1.0
// @description WireGuard Portal API for managing users and peers.

// @license.name MIT
// @license.url https://github.com/h44z/wg-portal/blob/master/LICENSE.txt

// @contact.name WireGuard Portal Project
// @contact.url https://github.com/h44z/wg-portal

// @securityDefinitions.basic ApiBasicAuth
// @in header
// @name Authorization
// @scope.admin Admin access required

// @securityDefinitions.basic GeneralBasicAuth
// @in header
// @name Authorization
// @scope.user User access required

// @BasePath /api/v1

// ApiServer is a simple wrapper struct so that we can have fresh member function names.
type ApiServer struct {
	s *Server
}

type ApiError struct {
	Message string
}

// GetUsers godoc
// @Tags Users
// @Summary Retrieves all users
// @ID GetUsers
// @Produce json
// @Success 200 {object} []users.User
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /backend/users [get]
// @Security ApiBasicAuth
func (s *ApiServer) GetUsers(c *gin.Context) {
	allUsers := s.s.users.GetUsersUnscoped()

	c.JSON(http.StatusOK, allUsers)
}

// GetUser godoc
// @Tags Users
// @Summary Retrieves user based on given Email
// @ID GetUser
// @Produce json
// @Param Email query string true "User Email"
// @Success 200 {object} users.User
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /backend/user [get]
// @Security ApiBasicAuth
func (s *ApiServer) GetUser(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Query("Email")))
	if email == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "Email parameter must be specified"})
		return
	}

	user := s.s.users.GetUserUnscoped(email)
	if user == nil {
		c.JSON(http.StatusNotFound, ApiError{Message: "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// PostUser godoc
// @Tags Users
// @Summary Creates a new user based on the given user model
// @ID PostUser
// @Accept  json
// @Produce json
// @Param User body users.User true "User Model"
// @Success 200 {object} users.User
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/users [post]
// @Security ApiBasicAuth
func (s *ApiServer) PostUser(c *gin.Context) {
	newUser := users.User{}
	if err := c.ShouldBindJSON(&newUser); err != nil {
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
	c.JSON(http.StatusOK, user)
}

// PutUser godoc
// @Tags Users
// @Summary Updates a user based on the given user model
// @ID PutUser
// @Accept  json
// @Produce json
// @Param Email query string true "User Email"
// @Param User body users.User true "User Model"
// @Success 200 {object} users.User
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/user [put]
// @Security ApiBasicAuth
func (s *ApiServer) PutUser(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Query("Email")))
	if email == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "Email parameter must be specified"})
		return
	}

	updateUser := users.User{}
	if err := c.ShouldBindJSON(&updateUser); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	// Changing email address is not allowed
	if email != updateUser.Email {
		c.JSON(http.StatusBadRequest, ApiError{Message: "Email parameter must match the model email address"})
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
	c.JSON(http.StatusOK, user)
}

// PatchUser godoc
// @Tags Users
// @Summary Updates a user based on the given partial user model
// @ID PatchUser
// @Accept  json
// @Produce json
// @Param Email query string true "User Email"
// @Param User body users.User true "User Model"
// @Success 200 {object} users.User
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/user [patch]
// @Security ApiBasicAuth
func (s *ApiServer) PatchUser(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Query("Email")))
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
	c.JSON(http.StatusOK, user)
}

// DeleteUser godoc
// @Tags Users
// @Summary Deletes the specified user
// @ID DeleteUser
// @Produce json
// @Param Email query string true "User Email"
// @Success 204 "No content"
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/user [delete]
// @Security ApiBasicAuth
func (s *ApiServer) DeleteUser(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Query("Email")))
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

// GetPeers godoc
// @Tags Peers
// @Summary Retrieves all peers for the given interface
// @ID GetPeers
// @Produce json
// @Param DeviceName query string true "Device Name"
// @Success 200 {object} []wireguard.Peer
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /backend/peers [get]
// @Security ApiBasicAuth
func (s *ApiServer) GetPeers(c *gin.Context) {
	deviceName := strings.ToLower(strings.TrimSpace(c.Query("DeviceName")))
	if deviceName == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "DeviceName parameter must be specified"})
		return
	}

	// validate device name
	if !common.ListContains(s.s.config.WG.DeviceNames, deviceName) {
		c.JSON(http.StatusNotFound, ApiError{Message: "unknown device"})
		return
	}

	peers := s.s.peers.GetAllPeers(deviceName)
	c.JSON(http.StatusOK, peers)
}

// GetPeer godoc
// @Tags Peers
// @Summary Retrieves the peer for the given public key
// @ID GetPeer
// @Produce json
// @Param PublicKey query string true "Public Key (Base 64)"
// @Success 200 {object} wireguard.Peer
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /backend/peer [get]
// @Security ApiBasicAuth
func (s *ApiServer) GetPeer(c *gin.Context) {
	pkey := c.Query("PublicKey")
	if pkey == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "PublicKey parameter must be specified"})
		return
	}

	peer := s.s.peers.GetPeerByKey(pkey)
	if !peer.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer does not exist"})
		return
	}
	c.JSON(http.StatusOK, peer)
}

// PostPeer godoc
// @Tags Peers
// @Summary Creates a new peer based on the given peer model
// @ID PostPeer
// @Accept  json
// @Produce json
// @Param DeviceName query string true "Device Name"
// @Param Peer body wireguard.Peer true "Peer Model"
// @Success 200 {object} wireguard.Peer
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/peers [post]
// @Security ApiBasicAuth
func (s *ApiServer) PostPeer(c *gin.Context) {
	deviceName := strings.ToLower(strings.TrimSpace(c.Query("DeviceName")))
	if deviceName == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "DeviceName parameter must be specified"})
		return
	}

	// validate device name
	if !common.ListContains(s.s.config.WG.DeviceNames, deviceName) {
		c.JSON(http.StatusNotFound, ApiError{Message: "unknown device"})
		return
	}

	newPeer := wireguard.Peer{}
	if err := c.ShouldBindJSON(&newPeer); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	if peer := s.s.peers.GetPeerByKey(newPeer.PublicKey); peer.IsValid() {
		c.JSON(http.StatusBadRequest, ApiError{Message: "peer already exists"})
		return
	}

	if err := s.s.CreatePeer(deviceName, newPeer); err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	peer := s.s.peers.GetPeerByKey(newPeer.PublicKey)
	if !peer.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer not found"})
		return
	}
	c.JSON(http.StatusOK, peer)
}

// PutPeer godoc
// @Tags Peers
// @Summary Updates the given peer based on the given peer model
// @ID PutPeer
// @Accept  json
// @Produce json
// @Param PublicKey query string true "Public Key"
// @Param Peer body wireguard.Peer true "Peer Model"
// @Success 200 {object} wireguard.Peer
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/peer [put]
// @Security ApiBasicAuth
func (s *ApiServer) PutPeer(c *gin.Context) {
	updatePeer := wireguard.Peer{}
	if err := c.ShouldBindJSON(&updatePeer); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	pkey := c.Query("PublicKey")
	if pkey == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "PublicKey parameter must be specified"})
		return
	}

	if peer := s.s.peers.GetPeerByKey(pkey); !peer.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer does not exist"})
		return
	}

	// Changing public key is not allowed
	if pkey != updatePeer.PublicKey {
		c.JSON(http.StatusBadRequest, ApiError{Message: "PublicKey parameter must match the model public key"})
		return
	}

	now := time.Now()
	if updatePeer.DeactivatedAt != nil {
		updatePeer.DeactivatedAt = &now
	}
	if err := s.s.UpdatePeer(updatePeer, now); err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	peer := s.s.peers.GetPeerByKey(updatePeer.PublicKey)
	if !peer.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer not found"})
		return
	}
	c.JSON(http.StatusOK, peer)
}

// PatchPeer godoc
// @Tags Peers
// @Summary Updates the given peer based on the given partial peer model
// @ID PatchPeer
// @Accept  json
// @Produce json
// @Param PublicKey query string true "Public Key"
// @Param Peer body wireguard.Peer true "Peer Model"
// @Success 200 {object} wireguard.Peer
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/peer [patch]
// @Security ApiBasicAuth
func (s *ApiServer) PatchPeer(c *gin.Context) {
	patch, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	pkey := c.Query("PublicKey")
	if pkey == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "pkey parameter must be specified"})
		return
	}

	peer := s.s.peers.GetPeerByKey(pkey)
	if !peer.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer does not exist"})
		return
	}

	peerData, err := json.Marshal(peer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	mergedPeerData, err := jsonpatch.MergePatch(peerData, patch)
	var mergedPeer wireguard.Peer
	err = json.Unmarshal(mergedPeerData, &mergedPeer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	if !mergedPeer.IsValid() {
		c.JSON(http.StatusBadRequest, ApiError{Message: "invalid peer model"})
		return
	}

	// Changing public key is not allowed
	if pkey != mergedPeer.PublicKey {
		c.JSON(http.StatusBadRequest, ApiError{Message: "PublicKey parameter must match the model public key"})
		return
	}

	now := time.Now()
	if mergedPeer.DeactivatedAt != nil {
		mergedPeer.DeactivatedAt = &now
	}
	if err := s.s.UpdatePeer(mergedPeer, now); err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	peer = s.s.peers.GetPeerByKey(mergedPeer.PublicKey)
	if !peer.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer not found"})
		return
	}
	c.JSON(http.StatusOK, peer)
}

// DeletePeer godoc
// @Tags Peers
// @Summary Updates the given peer based on the given partial peer model
// @ID DeletePeer
// @Produce json
// @Param PublicKey query string true "Public Key"
// @Success 204 "No Content"
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/peer [delete]
// @Security ApiBasicAuth
func (s *ApiServer) DeletePeer(c *gin.Context) {
	pkey := c.Query("PublicKey")
	if pkey == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "PublicKey parameter must be specified"})
		return
	}

	peer := s.s.peers.GetPeerByKey(pkey)
	if peer.PublicKey == "" {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer does not exist"})
		return
	}

	if err := s.s.DeletePeer(peer); err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetDevices godoc
// @Tags Interface
// @Summary Get all devices
// @ID GetDevices
// @Produce json
// @Success 200 {object} []wireguard.Device
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /backend/devices [get]
// @Security ApiBasicAuth
func (s *ApiServer) GetDevices(c *gin.Context) {
	var devices []wireguard.Device
	for _, deviceName := range s.s.config.WG.DeviceNames {
		device := s.s.peers.GetDevice(deviceName)
		if !device.IsValid() {
			continue
		}
		devices = append(devices, device)
	}

	c.JSON(http.StatusOK, devices)
}

// GetDevice godoc
// @Tags Interface
// @Summary Get the given device
// @ID GetDevice
// @Produce json
// @Param DeviceName query string true "Device Name"
// @Success 200 {object} wireguard.Device
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /backend/device [get]
// @Security ApiBasicAuth
func (s *ApiServer) GetDevice(c *gin.Context) {
	deviceName := strings.ToLower(strings.TrimSpace(c.Query("DeviceName")))
	if deviceName == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "DeviceName parameter must be specified"})
		return
	}

	// validate device name
	if !common.ListContains(s.s.config.WG.DeviceNames, deviceName) {
		c.JSON(http.StatusNotFound, ApiError{Message: "unknown device"})
		return
	}

	device := s.s.peers.GetDevice(deviceName)
	if !device.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "device not found"})
		return
	}

	c.JSON(http.StatusOK, device)
}

// PutDevice godoc
// @Tags Interface
// @Summary Updates the given device based on the given device model (UNIMPLEMENTED)
// @ID PutDevice
// @Accept  json
// @Produce json
// @Param DeviceName query string true "Device Name"
// @Param Device body wireguard.Device true "Device Model"
// @Success 200 {object} wireguard.Device
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/device [put]
// @Security ApiBasicAuth
func (s *ApiServer) PutDevice(c *gin.Context) {
	updateDevice := wireguard.Device{}
	if err := c.ShouldBindJSON(&updateDevice); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	deviceName := strings.ToLower(strings.TrimSpace(c.Query("DeviceName")))
	if deviceName == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "DeviceName parameter must be specified"})
		return
	}

	// validate device name
	if !common.ListContains(s.s.config.WG.DeviceNames, deviceName) {
		c.JSON(http.StatusNotFound, ApiError{Message: "unknown device"})
		return
	}

	device := s.s.peers.GetDevice(deviceName)
	if !device.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer not found"})
		return
	}

	// Changing device name is not allowed
	if deviceName != updateDevice.DeviceName {
		c.JSON(http.StatusBadRequest, ApiError{Message: "DeviceName parameter must match the model device name"})
		return
	}

	// TODO: implement

	c.JSON(http.StatusNotImplemented, device)
}

// PatchDevice godoc
// @Tags Interface
// @Summary Updates the given device based on the given partial device model (UNIMPLEMENTED)
// @ID PatchDevice
// @Accept  json
// @Produce json
// @Param DeviceName query string true "Device Name"
// @Param Device body wireguard.Device true "Device Model"
// @Success 200 {object} wireguard.Device
// @Failure 400 {object} ApiError
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Failure 500 {object} ApiError
// @Router /backend/device [patch]
// @Security ApiBasicAuth
func (s *ApiServer) PatchDevice(c *gin.Context) {
	patch, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	deviceName := strings.ToLower(strings.TrimSpace(c.Query("DeviceName")))
	if deviceName == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "DeviceName parameter must be specified"})
		return
	}

	// validate device name
	if !common.ListContains(s.s.config.WG.DeviceNames, deviceName) {
		c.JSON(http.StatusNotFound, ApiError{Message: "unknown device"})
		return
	}

	device := s.s.peers.GetDevice(deviceName)
	if !device.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer not found"})
		return
	}

	deviceData, err := json.Marshal(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	mergedDeviceData, err := jsonpatch.MergePatch(deviceData, patch)
	var mergedDevice wireguard.Device
	err = json.Unmarshal(mergedDeviceData, &mergedDevice)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	if !mergedDevice.IsValid() {
		c.JSON(http.StatusBadRequest, ApiError{Message: "invalid device model"})
		return
	}

	// Changing device name is not allowed
	if deviceName != mergedDevice.DeviceName {
		c.JSON(http.StatusBadRequest, ApiError{Message: "DeviceName parameter must match the model device name"})
		return
	}

	// TODO: implement

	c.JSON(http.StatusNotImplemented, device)
}

type PeerDeploymentInformation struct {
	PublicKey        string
	Identifier       string
	Device           string
	DeviceIdentifier string
}

// GetPeerDeploymentInformation godoc
// @Tags Provisioning
// @Summary Retrieves all active peers for the given email address
// @ID GetPeerDeploymentInformation
// @Produce json
// @Param Email query string true "Email Address"
// @Success 200 {object} []PeerDeploymentInformation "All active WireGuard peers"
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /provisioning/peers [get]
// @Security GeneralBasicAuth
func (s *ApiServer) GetPeerDeploymentInformation(c *gin.Context) {
	email := c.Query("Email")
	if email == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "Email parameter must be specified"})
		return
	}

	// Get authenticated user to check permissions
	username, _, _ := c.Request.BasicAuth()
	user := s.s.users.GetUser(username)

	if !user.IsAdmin && user.Email != email {
		c.JSON(http.StatusForbidden, ApiError{Message: "not enough permissions to access this resource"})
		return
	}

	peers := s.s.peers.GetPeersByMail(email)
	result := make([]PeerDeploymentInformation, 0, len(peers))
	for i := range peers {
		if peers[i].DeactivatedAt != nil {
			continue // skip deactivated peers
		}

		device := s.s.peers.GetDevice(peers[i].DeviceName)
		if device.Type != wireguard.DeviceTypeServer {
			continue // Skip peers on non-server devices
		}

		result = append(result, PeerDeploymentInformation{
			PublicKey:        peers[i].PublicKey,
			Identifier:       peers[i].Identifier,
			Device:           device.DeviceName,
			DeviceIdentifier: device.DisplayName,
		})
	}

	c.JSON(http.StatusOK, result)
}

// GetPeerDeploymentConfig godoc
// @Tags Provisioning
// @Summary Retrieves the peer config for the given public key
// @ID GetPeerDeploymentConfig
// @Produce plain
// @Param PublicKey query string true "Public Key (Base 64)"
// @Success 200 {object} string "The WireGuard configuration file"
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /provisioning/peer [get]
// @Security GeneralBasicAuth
func (s *ApiServer) GetPeerDeploymentConfig(c *gin.Context) {
	pkey := c.Query("PublicKey")
	if pkey == "" {
		c.JSON(http.StatusBadRequest, ApiError{Message: "PublicKey parameter must be specified"})
		return
	}

	peer := s.s.peers.GetPeerByKey(pkey)
	if !peer.IsValid() {
		c.JSON(http.StatusNotFound, ApiError{Message: "peer does not exist"})
		return
	}

	// Get authenticated user to check permissions
	username, _, _ := c.Request.BasicAuth()
	user := s.s.users.GetUser(username)

	if !user.IsAdmin && user.Email != peer.Email {
		c.JSON(http.StatusForbidden, ApiError{Message: "not enough permissions to access this resource"})
		return
	}

	device := s.s.peers.GetDevice(peer.DeviceName)
	config, err := peer.GetConfigFile(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	c.Data(http.StatusOK, "text/plain", config)
}

type ProvisioningRequest struct {
	// DeviceName is optional, if not specified, the configured default device will be used.
	DeviceName string `json:",omitempty"`
	Identifier string `binding:"required"`
	Email      string `binding:"required"`

	// Client specific and optional settings

	AllowedIPsStr       string `binding:"cidrlist" json:",omitempty"`
	PersistentKeepalive int    `binding:"gte=0" json:",omitempty"`
	DNSStr              string `binding:"iplist" json:",omitempty"`
	Mtu                 int    `binding:"gte=0,lte=1500" json:",omitempty"`
}

// PostPeerDeploymentConfig godoc
// @Tags Provisioning
// @Summary Creates the requested peer config and returns the config file
// @ID PostPeerDeploymentConfig
// @Accept  json
// @Produce plain
// @Param ProvisioningRequest body ProvisioningRequest true "Provisioning Request Model"
// @Success 200 {object} string "The WireGuard configuration file"
// @Failure 401 {object} ApiError
// @Failure 403 {object} ApiError
// @Failure 404 {object} ApiError
// @Router /provisioning/peers [post]
// @Security GeneralBasicAuth
func (s *ApiServer) PostPeerDeploymentConfig(c *gin.Context) {
	req := ProvisioningRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ApiError{Message: err.Error()})
		return
	}

	// Get authenticated user to check permissions
	username, _, _ := c.Request.BasicAuth()
	user := s.s.users.GetUser(username)

	if !user.IsAdmin && !s.s.config.Core.SelfProvisioningAllowed {
		c.JSON(http.StatusForbidden, ApiError{Message: "peer provisioning service disabled"})
		return
	}

	if !user.IsAdmin && user.Email != req.Email {
		c.JSON(http.StatusForbidden, ApiError{Message: "not enough permissions to access this resource"})
		return
	}

	deviceName := req.DeviceName
	if deviceName == "" || !common.ListContains(s.s.config.WG.DeviceNames, deviceName) {
		deviceName = s.s.config.WG.GetDefaultDeviceName()
	}
	device := s.s.peers.GetDevice(deviceName)
	if device.Type != wireguard.DeviceTypeServer {
		c.JSON(http.StatusForbidden, ApiError{Message: "invalid device, provisioning disabled"})
		return
	}

	// check if private/public keys are set, if so check database for existing entries
	peer, err := s.s.PrepareNewPeer(deviceName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}
	peer.Email = req.Email
	peer.Identifier = req.Identifier

	if req.AllowedIPsStr != "" {
		peer.AllowedIPsStr = req.AllowedIPsStr
	}
	if req.PersistentKeepalive != 0 {
		peer.PersistentKeepalive = req.PersistentKeepalive
	}
	if req.DNSStr != "" {
		peer.DNSStr = req.DNSStr
	}
	if req.Mtu != 0 {
		peer.Mtu = req.Mtu
	}

	if err := s.s.CreatePeer(deviceName, peer); err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	config, err := peer.GetConfigFile(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ApiError{Message: err.Error()})
		return
	}

	c.Data(http.StatusOK, "text/plain", config)
}
