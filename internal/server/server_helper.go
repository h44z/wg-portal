package server

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"syscall"
	"time"

	"github.com/h44z/wg-portal/internal/common"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

func (s *Server) PrepareNewPeer() (Peer, error) {
	device := s.peers.GetDevice()

	peer := Peer{}
	peer.IsNew = true
	peer.AllowedIPsStr = device.AllowedIPsStr
	peer.IPs = make([]string, len(device.IPs))
	for i := range device.IPs {
		freeIP, err := s.peers.GetAvailableIp(device.IPs[i])
		if err != nil {
			return Peer{}, errors.WithMessage(err, "failed to get available IP addresses")
		}
		peer.IPs[i] = freeIP
	}
	peer.IPsStr = common.ListToString(peer.IPs)
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return Peer{}, errors.Wrap(err, "failed to generate key")
	}
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return Peer{}, errors.Wrap(err, "failed to generate private key")
	}
	peer.PresharedKey = psk.String()
	peer.PrivateKey = key.String()
	peer.PublicKey = key.PublicKey().String()
	peer.UID = fmt.Sprintf("u%x", md5.Sum([]byte(peer.PublicKey)))

	return peer, nil
}

func (s *Server) CreatePeerByEmail(email, identifierSuffix string, disabled bool) error {
	user, err := s.users.GetOrCreateUser(email)
	if err != nil {
		return errors.WithMessagef(err, "failed to load/create related user %s", email)
	}

	device := s.peers.GetDevice()
	peer := Peer{}
	peer.User = user
	peer.AllowedIPsStr = device.AllowedIPsStr
	peer.IPs = make([]string, len(device.IPs))
	for i := range device.IPs {
		freeIP, err := s.peers.GetAvailableIp(device.IPs[i])
		if err != nil {
			return errors.WithMessage(err, "failed to get available IP addresses")
		}
		peer.IPs[i] = freeIP
	}
	peer.IPsStr = common.ListToString(peer.IPs)
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return errors.Wrap(err, "failed to generate key")
	}
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return errors.Wrap(err, "failed to generate private key")
	}
	peer.PresharedKey = psk.String()
	peer.PrivateKey = key.String()
	peer.PublicKey = key.PublicKey().String()
	peer.UID = fmt.Sprintf("u%x", md5.Sum([]byte(peer.PublicKey)))
	peer.Email = email
	peer.Identifier = fmt.Sprintf("%s %s (%s)", user.Firstname, user.Lastname, identifierSuffix)
	now := time.Now()
	if disabled {
		peer.DeactivatedAt = &now
	}

	return s.CreatePeer(peer)
}

func (s *Server) CreatePeer(peer Peer) error {
	device := s.peers.GetDevice()
	peer.AllowedIPsStr = device.AllowedIPsStr
	if peer.IPs == nil || len(peer.IPs) == 0 {
		peer.IPs = make([]string, len(device.IPs))
		for i := range device.IPs {
			freeIP, err := s.peers.GetAvailableIp(device.IPs[i])
			if err != nil {
				return errors.WithMessage(err, "failed to get available IP addresses")
			}
			peer.IPs[i] = freeIP
		}
		peer.IPsStr = common.ListToString(peer.IPs)
	}
	if peer.PrivateKey == "" { // if private key is empty create a new one
		psk, err := wgtypes.GenerateKey()
		if err != nil {
			return errors.Wrap(err, "failed to generate key")
		}
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return errors.Wrap(err, "failed to generate private key")
		}
		peer.PresharedKey = psk.String()
		peer.PrivateKey = key.String()
		peer.PublicKey = key.PublicKey().String()
	}
	peer.UID = fmt.Sprintf("u%x", md5.Sum([]byte(peer.PublicKey)))

	// Create WireGuard interface
	if peer.DeactivatedAt == nil {
		if err := s.wg.AddPeer(peer.GetConfig()); err != nil {
			return errors.WithMessage(err, "failed to add WireGuard peer")
		}
	}

	// Create in database
	if err := s.peers.CreatePeer(peer); err != nil {
		return errors.WithMessage(err, "failed to create peer")
	}

	return s.WriteWireGuardConfigFile()
}

func (s *Server) UpdatePeer(peer Peer, updateTime time.Time) error {
	currentPeer := s.peers.GetPeerByKey(peer.PublicKey)

	// Update WireGuard device
	var err error
	switch {
	case peer.DeactivatedAt == &updateTime:
		err = s.wg.RemovePeer(peer.PublicKey)
	case peer.DeactivatedAt == nil && currentPeer.Peer != nil:
		err = s.wg.UpdatePeer(peer.GetConfig())
	case peer.DeactivatedAt == nil && currentPeer.Peer == nil:
		err = s.wg.AddPeer(peer.GetConfig())
	}
	if err != nil {
		return errors.WithMessage(err, "failed to update WireGuard peer")
	}

	// Update in database
	if err := s.peers.UpdatePeer(peer); err != nil {
		return errors.WithMessage(err, "failed to update peer")
	}

	return s.WriteWireGuardConfigFile()
}

func (s *Server) DeletePeer(peer Peer) error {
	// Delete WireGuard peer
	if err := s.wg.RemovePeer(peer.PublicKey); err != nil {
		return errors.WithMessage(err, "failed to remove WireGuard peer")
	}

	// Delete in database
	if err := s.peers.DeletePeer(peer); err != nil {
		return errors.WithMessage(err, "failed to remove peer")
	}

	return s.WriteWireGuardConfigFile()
}

func (s *Server) RestoreWireGuardInterface() error {
	activePeers := s.peers.GetActivePeers()

	for i := range activePeers {
		if activePeers[i].Peer == nil {
			if err := s.wg.AddPeer(activePeers[i].GetConfig()); err != nil {
				return errors.WithMessage(err, "failed to add WireGuard peer")
			}
		}
	}

	return nil
}

func (s *Server) WriteWireGuardConfigFile() error {
	if s.config.WG.WireGuardConfig == "" {
		return nil // writing disabled
	}
	if err := syscall.Access(s.config.WG.WireGuardConfig, syscall.O_RDWR); err != nil {
		return errors.Wrap(err, "failed to check WireGuard config access rights")
	}

	device := s.peers.GetDevice()
	cfg, err := device.GetConfigFile(s.peers.GetActivePeers())
	if err != nil {
		return errors.WithMessage(err, "failed to get config file")
	}
	if err := ioutil.WriteFile(s.config.WG.WireGuardConfig, cfg, 0644); err != nil {
		return errors.Wrap(err, "failed to write WireGuard config file")
	}
	return nil
}

func (s *Server) CreateUser(user users.User) error {
	if user.Email == "" {
		return errors.New("cannot create user with empty email address")
	}

	// Check if user already exists, if so re-enable
	if existingUser := s.users.GetUserUnscoped(user.Email); existingUser != nil {
		user.DeletedAt = gorm.DeletedAt{} // reset deleted flag to enable that user again
		return s.UpdateUser(user)
	}

	// Create user in database
	if err := s.users.CreateUser(&user); err != nil {
		return errors.WithMessage(err, "failed to create user in manager")
	}

	// Check if user already has a peer setup, if not, create one
	return s.CreateUserDefaultPeer(user.Email)
}

func (s *Server) UpdateUser(user users.User) error {
	if user.DeletedAt.Valid {
		return s.DeleteUser(user)
	}

	currentUser := s.users.GetUserUnscoped(user.Email)

	// Update in database
	if err := s.users.UpdateUser(&user); err != nil {
		return errors.WithMessage(err, "failed to update user in manager")
	}

	// If user was deleted (disabled), reactivate it's peers
	if currentUser.DeletedAt.Valid {
		for _, peer := range s.peers.GetPeersByMail(user.Email) {
			now := time.Now()
			peer.DeactivatedAt = nil
			if err := s.UpdatePeer(peer, now); err != nil {
				logrus.Errorf("failed to update (re)activated peer %s for %s: %v", peer.PublicKey, user.Email, err)
			}
		}
	}

	return nil
}

func (s *Server) DeleteUser(user users.User) error {
	currentUser := s.users.GetUserUnscoped(user.Email)

	// Update in database
	if err := s.users.DeleteUser(&user); err != nil {
		return errors.WithMessage(err, "failed to delete user in manager")
	}

	// If user was active, disable it's peers
	if !currentUser.DeletedAt.Valid {
		for _, peer := range s.peers.GetPeersByMail(user.Email) {
			now := time.Now()
			peer.DeactivatedAt = &now
			if err := s.UpdatePeer(peer, now); err != nil {
				logrus.Errorf("failed to update deactivated peer %s for %s: %v", peer.PublicKey, user.Email, err)
			}
		}
	}

	return nil
}

func (s *Server) CreateUserDefaultPeer(email string) error {
	// Check if user is active, if not, quit
	var existingUser *users.User
	if existingUser = s.users.GetUser(email); existingUser == nil {
		return nil
	}

	// Check if user already has a peer setup, if not, create one
	if s.config.Core.CreateDefaultPeer {
		peers := s.peers.GetPeersByMail(email)
		if len(peers) == 0 { // Create default vpn peer
			if err := s.CreatePeer(Peer{
				Identifier: existingUser.Firstname + " " + existingUser.Lastname + " (Default)",
				Email:      existingUser.Email,
				CreatedBy:  existingUser.Email,
				UpdatedBy:  existingUser.Email,
			}); err != nil {
				return errors.WithMessagef(err, "failed to automatically create vpn peer for %s", email)
			}
		}
	}

	return nil
}
