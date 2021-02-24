package server

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"syscall"
	"time"

	"github.com/h44z/wg-portal/internal/common"
	"github.com/pkg/errors"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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
			return Peer{}, err
		}
		peer.IPs[i] = freeIP
	}
	peer.IPsStr = common.ListToString(peer.IPs)
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return Peer{}, err
	}
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return Peer{}, err
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
			return err
		}
		peer.IPs[i] = freeIP
	}
	peer.IPsStr = common.ListToString(peer.IPs)
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return err
	}
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
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
				return err
			}
			peer.IPs[i] = freeIP
		}
		peer.IPsStr = common.ListToString(peer.IPs)
	}
	if peer.PrivateKey == "" { // if private key is empty create a new one
		psk, err := wgtypes.GenerateKey()
		if err != nil {
			return err
		}
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		peer.PresharedKey = psk.String()
		peer.PrivateKey = key.String()
		peer.PublicKey = key.PublicKey().String()
	}
	peer.UID = fmt.Sprintf("u%x", md5.Sum([]byte(peer.PublicKey)))

	// Create WireGuard interface
	if peer.DeactivatedAt == nil {
		if err := s.wg.AddPeer(peer.GetConfig()); err != nil {
			return err
		}
	}

	// Create in database
	if err := s.peers.CreatePeer(peer); err != nil {
		return err
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
		return err
	}

	// Update in database
	if err := s.peers.UpdatePeer(peer); err != nil {
		return err
	}

	return s.WriteWireGuardConfigFile()
}

func (s *Server) DeletePeer(peer Peer) error {
	// Delete WireGuard peer
	if err := s.wg.RemovePeer(peer.PublicKey); err != nil {
		return err
	}

	// Delete in database
	if err := s.peers.DeletePeer(peer); err != nil {
		return err
	}

	return s.WriteWireGuardConfigFile()
}

func (s *Server) RestoreWireGuardInterface() error {
	activePeers := s.peers.GetActivePeers()

	for i := range activePeers {
		if activePeers[i].Peer == nil {
			if err := s.wg.AddPeer(activePeers[i].GetConfig()); err != nil {
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

	device := s.peers.GetDevice()
	cfg, err := device.GetConfigFile(s.peers.GetActivePeers())
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(s.config.WG.WireGuardConfig, cfg, 0644); err != nil {
		return err
	}
	return nil
}
