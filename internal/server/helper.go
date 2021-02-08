package server

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io/ioutil"
	"syscall"
	"time"

	"github.com/h44z/wg-portal/internal/common"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func (s *Server) PrepareNewUser() (User, error) {
	device := s.users.GetDevice()

	user := User{}
	user.IsNew = true
	user.AllowedIPsStr = device.AllowedIPsStr
	user.IPs = make([]string, len(device.IPs))
	for i := range device.IPs {
		freeIP, err := s.users.GetAvailableIp(device.IPs[i])
		if err != nil {
			return User{}, err
		}
		user.IPs[i] = freeIP
	}
	user.IPsStr = common.ListToString(user.IPs)
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return User{}, err
	}
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return User{}, err
	}
	user.PresharedKey = psk.String()
	user.PrivateKey = key.String()
	user.PublicKey = key.PublicKey().String()
	user.UID = fmt.Sprintf("u%x", md5.Sum([]byte(user.PublicKey)))

	return user, nil
}

func (s *Server) CreateUserByEmail(email, identifierSuffix string, disabled bool) error {
	ldapUser := s.ldapUsers.GetUserData(s.ldapUsers.GetUserDNByMail(email))
	if ldapUser.DN == "" {
		return errors.New("no user with email " + email + " found")
	}

	device := s.users.GetDevice()
	user := User{}
	user.AllowedIPsStr = device.AllowedIPsStr
	user.IPs = make([]string, len(device.IPs))
	for i := range device.IPs {
		freeIP, err := s.users.GetAvailableIp(device.IPs[i])
		if err != nil {
			return err
		}
		user.IPs[i] = freeIP
	}
	user.IPsStr = common.ListToString(user.IPs)
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return err
	}
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}
	user.PresharedKey = psk.String()
	user.PrivateKey = key.String()
	user.PublicKey = key.PublicKey().String()
	user.UID = fmt.Sprintf("u%x", md5.Sum([]byte(user.PublicKey)))
	user.Email = email
	user.Identifier = fmt.Sprintf("%s %s (%s)", ldapUser.Firstname, ldapUser.Lastname, identifierSuffix)
	now := time.Now()
	if disabled {
		user.DeactivatedAt = &now
	}

	return s.CreateUser(user)
}

func (s *Server) CreateUser(user User) error {

	device := s.users.GetDevice()
	user.AllowedIPsStr = device.AllowedIPsStr
	if len(user.IPs) == 0 {
		for i := range device.IPs {
			freeIP, err := s.users.GetAvailableIp(device.IPs[i])
			if err != nil {
				return err
			}
			user.IPs[i] = freeIP
		}
		user.IPsStr = common.ListToString(user.IPs)
	}
	if user.PrivateKey == "" { // if private key is empty create a new one
		psk, err := wgtypes.GenerateKey()
		if err != nil {
			return err
		}
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		user.PresharedKey = psk.String()
		user.PrivateKey = key.String()
		user.PublicKey = key.PublicKey().String()
	}
	user.UID = fmt.Sprintf("u%x", md5.Sum([]byte(user.PublicKey)))

	// Create WireGuard interface
	if user.DeactivatedAt == nil {
		if err := s.wg.AddPeer(user.GetPeerConfig()); err != nil {
			return err
		}
	}

	// Create in database
	if err := s.users.CreateUser(user); err != nil {
		return err
	}

	return s.WriteWireGuardConfigFile()
}

func (s *Server) UpdateUser(user User, updateTime time.Time) error {
	currentUser := s.users.GetUserByKey(user.PublicKey)

	// Update WireGuard device
	var err error
	switch {
	case user.DeactivatedAt == &updateTime:
		err = s.wg.RemovePeer(user.PublicKey)
	case user.DeactivatedAt == nil && currentUser.Peer != nil:
		err = s.wg.UpdatePeer(user.GetPeerConfig())
	case user.DeactivatedAt == nil && currentUser.Peer == nil:
		err = s.wg.AddPeer(user.GetPeerConfig())
	}
	if err != nil {
		return err
	}

	// Update in database
	if err := s.users.UpdateUser(user); err != nil {
		return err
	}

	return s.WriteWireGuardConfigFile()
}

func (s *Server) DeleteUser(user User) error {
	// Delete WireGuard peer
	if err := s.wg.RemovePeer(user.PublicKey); err != nil {
		return err
	}

	// Delete in database
	if err := s.users.DeleteUser(user); err != nil {
		return err
	}

	return s.WriteWireGuardConfigFile()
}

func (s *Server) RestoreWireGuardInterface() error {
	activeUsers := s.users.GetActiveUsers()

	for i := range activeUsers {
		if activeUsers[i].Peer == nil {
			if err := s.wg.AddPeer(activeUsers[i].GetPeerConfig()); err != nil {
				return err
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
		return err
	}

	device := s.users.GetDevice()
	cfg, err := device.GetDeviceConfigFile(s.users.GetActiveUsers())
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(s.config.WG.WireGuardConfig, cfg, 0644); err != nil {
		return err
	}
	return nil
}
