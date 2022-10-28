package server

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"path"
	"syscall"
	"time"

	"github.com/h44z/wg-portal/internal/users"
	"github.com/h44z/wg-portal/internal/wireguard"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

// PrepareNewPeer initiates a new peer for the given WireGuard device.
func (s *Server) PrepareNewPeer(device string) (wireguard.Peer, error) {
	dev := s.peers.GetDevice(device)
	deviceIPs := dev.GetIPAddresses()

	peer := wireguard.Peer{}
	peer.IsNew = true

	switch dev.Type {
	case wireguard.DeviceTypeServer:
		peerIPs := make([]string, len(deviceIPs))
		for i := range deviceIPs {
			freeIP, err := s.peers.GetAvailableIp(device, deviceIPs[i])
			if err != nil {
				return wireguard.Peer{}, errors.WithMessage(err, "failed to get available IP addresses")
			}
			peerIPs[i] = freeIP
		}
		peer.SetIPAddresses(peerIPs...)
		psk, err := wgtypes.GenerateKey()
		if err != nil {
			return wireguard.Peer{}, errors.Wrap(err, "failed to generate key")
		}
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return wireguard.Peer{}, errors.Wrap(err, "failed to generate private key")
		}
		peer.PresharedKey = psk.String()
		peer.PrivateKey = key.String()
		peer.PublicKey = key.PublicKey().String()
		peer.UID = fmt.Sprintf("u%x", md5.Sum([]byte(peer.PublicKey)))
		peer.Endpoint = dev.DefaultEndpoint
		peer.DNSStr = dev.DNSStr
		peer.PersistentKeepalive = dev.DefaultPersistentKeepalive
		peer.AllowedIPsStr = dev.DefaultAllowedIPsStr
		peer.Mtu = dev.Mtu
		peer.DeviceName = device
	case wireguard.DeviceTypeClient:
		peer.UID = "newendpoint"
	}

	return peer, nil
}

// CreatePeerByEmail creates a new peer for the given email.
func (s *Server) CreatePeerByEmail(device, email, identifierSuffix string) error {
	user := s.users.GetUser(email)

	peer, err := s.PrepareNewPeer(device)
	if err != nil {
		return errors.WithMessage(err, "failed to prepare new peer")
	}
	peer.Email = email
	if user != nil {
		peer.Identifier = fmt.Sprintf("%s %s (%s)", user.Firstname, user.Lastname, identifierSuffix)
	} else {
		peer.Identifier = fmt.Sprintf("%s (%s)", email, identifierSuffix)
	}

	return s.CreatePeer(device, peer)
}

// CreatePeer creates the new peer in the database. If the peer has no assigned ip addresses, a new one will be assigned
// automatically. Also, if the private key is empty, a new key-pair will be generated.
// This function also configures the new peer on the physical WireGuard interface if the peer is not deactivated.
func (s *Server) CreatePeer(device string, peer wireguard.Peer) error {
	dev := s.peers.GetDevice(device)
	deviceIPs := dev.GetIPAddresses()
	peerIPs := peer.GetIPAddresses()

	peer.AllowedIPsStr = dev.DefaultAllowedIPsStr
	if len(peerIPs) == 0 && dev.Type == wireguard.DeviceTypeServer {
		peerIPs = make([]string, len(deviceIPs))
		for i := range deviceIPs {
			freeIP, err := s.peers.GetAvailableIp(device, deviceIPs[i])
			if err != nil {
				return errors.WithMessage(err, "failed to get available IP addresses")
			}
			peerIPs[i] = freeIP
		}
		peer.SetIPAddresses(peerIPs...)
	}
	if peer.PresharedKey == "" && dev.Type == wireguard.DeviceTypeServer { // if preshared key is empty create a new one

		psk, err := wgtypes.GenerateKey()
		if err != nil {
			return errors.Wrap(err, "failed to generate key")
		}
		peer.PresharedKey = psk.String()
	}

	if peer.PrivateKey == "" && peer.PublicKey == "" && dev.Type == wireguard.DeviceTypeServer { // if private key is empty create a new one

		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return errors.Wrap(err, "failed to generate private key")
		}
		peer.PrivateKey = key.String()
		peer.PublicKey = key.PublicKey().String()
	}
	peer.DeviceName = dev.DeviceName
	peer.UID = fmt.Sprintf("u%x", md5.Sum([]byte(peer.PublicKey)))

	// Create WireGuard interface
	if peer.DeactivatedAt == nil {
		if err := s.wg.AddPeer(device, peer.GetConfig(&dev)); err != nil {
			return errors.WithMessage(err, "failed to add WireGuard peer")
		}
	}

	// Create in database
	if err := s.peers.CreatePeer(peer); err != nil {
		return errors.WithMessage(err, "failed to create peer")
	}

	return s.WriteWireGuardConfigFile(device)
}

// UpdatePeer updates the physical WireGuard interface and the database.
func (s *Server) UpdatePeer(peer wireguard.Peer, updateTime time.Time) error {
	currentPeer := s.peers.GetPeerByKey(peer.PublicKey)
	dev := s.peers.GetDevice(peer.DeviceName)

	// Update WireGuard device
	var err error
	switch {
	case peer.DeactivatedAt != nil && *peer.DeactivatedAt == updateTime:
		err = s.wg.RemovePeer(peer.DeviceName, peer.PublicKey)
	case peer.DeactivatedAt == nil && currentPeer.Peer != nil:
		err = s.wg.UpdatePeer(peer.DeviceName, peer.GetConfig(&dev))
	case peer.DeactivatedAt == nil && currentPeer.Peer == nil:
		err = s.wg.AddPeer(peer.DeviceName, peer.GetConfig(&dev))
	}
	if err != nil {
		return errors.WithMessage(err, "failed to update WireGuard peer")
	}

	peer.UID = fmt.Sprintf("u%x", md5.Sum([]byte(peer.PublicKey)))

	// Update in database
	if err := s.peers.UpdatePeer(peer); err != nil {
		return errors.WithMessage(err, "failed to update peer")
	}

	return s.WriteWireGuardConfigFile(peer.DeviceName)
}

// DeletePeer removes the peer from the physical WireGuard interface and the database.
func (s *Server) DeletePeer(peer wireguard.Peer) error {
	// Delete WireGuard peer
	if err := s.wg.RemovePeer(peer.DeviceName, peer.PublicKey); err != nil {
		return errors.WithMessage(err, "failed to remove WireGuard peer")
	}

	// Delete in database
	if err := s.peers.DeletePeer(peer); err != nil {
		return errors.WithMessage(err, "failed to remove peer")
	}

	return s.WriteWireGuardConfigFile(peer.DeviceName)
}

// RestoreWireGuardInterface restores the state of the physical WireGuard interface from the database.
func (s *Server) RestoreWireGuardInterface(device string) error {
	activePeers := s.peers.GetActivePeers(device)
	dev := s.peers.GetDevice(device)

	for i := range activePeers {
		if activePeers[i].Peer == nil {
			if err := s.wg.AddPeer(device, activePeers[i].GetConfig(&dev)); err != nil {
				return errors.WithMessage(err, "failed to add WireGuard peer")
			}
		}
	}

	return nil
}

// WriteWireGuardConfigFile writes the configuration file for the physical WireGuard interface.
func (s *Server) WriteWireGuardConfigFile(device string) error {
	if s.config.WG.ConfigDirectoryPath == "" {
		return nil // writing disabled
	}
	if err := syscall.Access(s.config.WG.ConfigDirectoryPath, syscall.O_RDWR); err != nil {
		return errors.Wrap(err, "failed to check WireGuard config access rights")
	}

	dev := s.peers.GetDevice(device)
	cfg, err := dev.GetConfigFile(s.peers.GetActivePeers(device), s.config.Core.WGExoprterFriendlyNames)
	if err != nil {
		return errors.WithMessage(err, "failed to get config file")
	}
	filePath := path.Join(s.config.WG.ConfigDirectoryPath, dev.DeviceName+".conf")
	if err := ioutil.WriteFile(filePath, cfg, 0644); err != nil {
		return errors.Wrap(err, "failed to write WireGuard config file")
	}
	return nil
}

// CreateUser creates the user in the database and optionally adds a default WireGuard peer for the user.
func (s *Server) CreateUser(user users.User, device string) error {
	if user.Email == "" {
		return errors.New("cannot create user with empty email address")
	}

	// Check if user already exists, if so re-enable
	if existingUser := s.users.GetUserUnscoped(user.Email); existingUser != nil {
		user.DeletedAt = gorm.DeletedAt{} // reset deleted flag to enable that user again
		return s.UpdateUser(user)
	}

	// Hash user password (if set)
	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return errors.Wrap(err, "unable to hash password")
		}
		user.Password = users.PrivateString(hashedPassword)
	}

	// Create user in database
	if err := s.users.CreateUser(&user); err != nil {
		return errors.WithMessage(err, "failed to create user in manager")
	}

	// Check if user already has a peer setup, if not, create one
	return s.CreateUserDefaultPeer(user.Email, device)
}

// UpdateUser updates the user in the database. If the user is marked as deleted, it will get remove from the database.
// Also, if the user is re-enabled, all it's linked WireGuard peers will be activated again.
func (s *Server) UpdateUser(user users.User) error {
	currentUser := s.users.GetUserUnscoped(user.Email)

	// Hash user password (if set)
	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return errors.Wrap(err, "unable to hash password")
		}
		user.Password = users.PrivateString(hashedPassword)
	} else {
		user.Password = currentUser.Password // keep current password
	}

	// Update in database
	if err := s.users.UpdateUser(&user); err != nil {
		return errors.WithMessage(err, "failed to update user in manager")
	}

	// Set to deleted (disabled) if user's deletedAt date is not empty
	if user.DeletedAt.Valid {
		return s.DeleteUser(user)
	}

	// Otherwise, if user was deleted (disabled), reactivate it's peers
	if currentUser.DeletedAt.Valid {
		for _, peer := range s.peers.GetPeersByMail(user.Email) {
			now := time.Now()
			peer.DeactivatedAt = nil
			peer.DeactivatedReason = ""
			if err := s.UpdatePeer(peer, now); err != nil {
				logrus.Errorf("failed to update (re)activated peer %s for %s: %v", peer.PublicKey, user.Email, err)
			}
		}
	}

	return nil
}

// DeleteUser soft-deletes the user from the database (disable the user).
// Also, if the user has linked WireGuard peers, they will be deactivated.
func (s *Server) DeleteUser(user users.User) error {
	// Update in database
	if err := s.users.DeleteUser(&user, true); err != nil {
		return errors.WithMessage(err, "failed to disable user in manager")
	}

	// Disable users peers
	for _, peer := range s.peers.GetPeersByMail(user.Email) {
		now := time.Now()
		peer.DeactivatedAt = &now
		peer.DeactivatedReason = "user deleted"
		if err := s.UpdatePeer(peer, now); err != nil {
			logrus.Errorf("failed to update deactivated peer %s for %s: %v", peer.PublicKey, user.Email, err)
		}
	}

	return nil
}

// HardDeleteUser removes the user from the database.
// Also, if the user has linked WireGuard peers, they will be deleted.
func (s *Server) HardDeleteUser(user users.User) error {
	// Update in database
	if err := s.users.DeleteUser(&user, false); err != nil {
		return errors.WithMessage(err, "failed to delete user in manager")
	}

	// remove all linked peers
	for _, peer := range s.peers.GetPeersByMail(user.Email) {
		if err := s.DeletePeer(peer); err != nil {
			logrus.Errorf("failed to delete peer %s for %s: %v", peer.PublicKey, user.Email, err)
		}
	}

	return nil
}

func (s *Server) CreateUserDefaultPeer(email, device string) error {
	// Check if automatic peer creation is enabled
	if !s.config.Core.CreateDefaultPeer {
		return nil
	}

	// Check if user is active, if not, quit
	var existingUser *users.User
	if existingUser = s.users.GetUser(email); existingUser == nil {
		return nil
	}

	// Check if user already has a peer setup, if not, create one
	peers := s.peers.GetPeersByMail(email)
	if len(peers) != 0 {
		return nil
	}

	// Create default vpn peer
	peer, err := s.PrepareNewPeer(device)
	if err != nil {
		return errors.WithMessage(err, "failed to prepare new peer")
	}
	peer.Email = email
	if existingUser.Firstname != "" && existingUser.Lastname != "" {
		peer.Identifier = fmt.Sprintf("%s %s (%s)", existingUser.Firstname, existingUser.Lastname, "Default")
	} else {
		peer.Identifier = fmt.Sprintf("%s (%s)", existingUser.Email, "Default")
	}
	peer.CreatedBy = existingUser.Email
	peer.UpdatedBy = existingUser.Email
	if err := s.CreatePeer(device, peer); err != nil {
		return errors.WithMessagef(err, "failed to automatically create vpn peer for %s", email)
	}

	return nil
}

func (s *Server) GetDeviceNames() map[string]string {
	devNames := make(map[string]string, len(s.wg.Cfg.DeviceNames))

	for _, devName := range s.wg.Cfg.DeviceNames {
		dev := s.peers.GetDevice(devName)
		devNames[devName] = dev.DisplayName
	}

	return devNames
}
