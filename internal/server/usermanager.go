package server

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"net"
	"strings"
	"text/template"
	"time"

	"github.com/h44z/wg-portal/internal/wireguard"

	"github.com/h44z/wg-portal/internal/common"

	"github.com/h44z/wg-portal/internal/ldap"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	Peer   wgtypes.Peer               `gorm:"-"`
	User   *ldap.UserCacheHolderEntry `gorm:"-"` // optional, it is still possible to have users without ldap
	Config string                     `gorm:"-"`

	UID        string // uid for html identification
	IsOnline   bool   `gorm:"-"`
	Identifier string // Identifier AND Email make a WireGuard peer unique
	Email      string `gorm:"index"`

	IgnorePersistentKeepalive bool
	PresharedKey              string
	AllowedIPsStr             string
	IPsStr                    string
	AllowedIPs                []string `gorm:"-"` // IPs that are used in the client config file
	IPs                       []string `gorm:"-"` // The IPs of the client
	PrivateKey                string
	PublicKey                 string `gorm:"primaryKey"`

	DeactivatedAt *time.Time
	CreatedBy     string
	UpdatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (u User) GetPeerConfig() wgtypes.PeerConfig {
	publicKey, _ := wgtypes.ParseKey(u.PublicKey)
	var presharedKey *wgtypes.Key
	if u.PresharedKey != "" {
		presharedKeyTmp, _ := wgtypes.ParseKey(u.PresharedKey)
		presharedKey = &presharedKeyTmp
	}

	cfg := wgtypes.PeerConfig{
		PublicKey:                   publicKey,
		Remove:                      false,
		UpdateOnly:                  false,
		PresharedKey:                presharedKey,
		Endpoint:                    nil,
		PersistentKeepaliveInterval: nil,
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  make([]net.IPNet, len(u.IPs)),
	}
	for i, ip := range u.IPs {
		_, ipNet, err := net.ParseCIDR(ip)
		if err == nil {
			cfg.AllowedIPs[i] = *ipNet
		}
	}

	return cfg
}

func (u User) GetQRCode() ([]byte, error) {
	png, err := qrcode.Encode(u.Config, qrcode.Medium, 250)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to create qrcode")
		return nil, err
	}
	return png, nil
}

type Device struct {
	Interface *wgtypes.Device `gorm:"-"`

	DeviceName          string `gorm:"primaryKey"`
	PrivateKey          string
	PublicKey           string
	PersistentKeepalive int
	ListenPort          int
	Mtu                 int
	Endpoint            string
	AllowedIPsStr       string
	IPsStr              string
	AllowedIPs          []string `gorm:"-"` // IPs that are used in the client config file
	IPs                 []string `gorm:"-"` // The IPs of the client
	DNSStr              string
	DNS                 []string `gorm:"-"` // The DNS servers of the client
	PreUp               string
	PostUp              string
	PreDown             string
	PostDown            string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (d Device) IsValid() bool {
	if len(d.IPs) == 0 {
		return false
	}
	if d.Endpoint == "" {
		return false
	}

	return true
}

type UserManager struct {
	db *gorm.DB
}

func NewUserManager() *UserManager {
	um := &UserManager{}
	var err error
	um.db, err = gorm.Open(sqlite.Open("wg_portal.db"), &gorm.Config{})
	if err != nil {
		log.Errorf("failed to open sqlite database: %v", err)
		return nil
	}

	err = um.db.AutoMigrate(&User{}, &Device{})
	if err != nil {
		log.Errorf("failed to migrate sqlite database: %v", err)
		return nil
	}

	return um
}

func (u *UserManager) InitWithPeers(peers []wgtypes.Peer, err error) {
	if err != nil {
		log.Errorf("failed to init user-manager from peers: %v", err)
		return
	}
	for _, peer := range peers {
		u.GetOrCreateUserForPeer(peer)
	}
}

func (u *UserManager) InitWithDevice(dev *wgtypes.Device, err error) {
	if err != nil {
		log.Errorf("failed to init user-manager from device: %v", err)
		return
	}
	u.GetOrCreateDevice(*dev)
}

func (u *UserManager) GetAllUsers() []User {
	users := make([]User, 0)
	u.db.Find(&users)

	for i := range users {
		users[i].AllowedIPs = strings.Split(users[i].AllowedIPsStr, ", ")
		users[i].IPs = strings.Split(users[i].IPsStr, ", ")
		tmpCfg, _ := u.GetPeerConfigFile(users[i])
		users[i].Config = string(tmpCfg)
	}

	return users
}

func (u *UserManager) GetDevice() Device {
	devices := make([]Device, 0, 1)
	u.db.Find(&devices)

	for i := range devices {
		devices[i].AllowedIPs = strings.Split(devices[i].AllowedIPsStr, ", ")
		devices[i].IPs = strings.Split(devices[i].IPsStr, ", ")
		devices[i].DNS = strings.Split(devices[i].DNSStr, ", ")
	}

	return devices[0]
}

func (u *UserManager) GetOrCreateUserForPeer(peer wgtypes.Peer) User {
	user := User{}
	u.db.Where("public_key = ?", peer.PublicKey.String()).FirstOrInit(&user)

	if user.PublicKey == "" { // user not found, create
		user.UID = fmt.Sprintf("u%x", md5.Sum([]byte(peer.PublicKey.String())))
		user.PublicKey = peer.PublicKey.String()
		user.PrivateKey = "" // UNKNOWN
		if peer.PresharedKey != (wgtypes.Key{}) {
			user.PresharedKey = peer.PresharedKey.String()
		}
		user.Email = "autodetected@example.com"
		user.Identifier = "Autodetected (" + user.PublicKey[0:8] + ")"
		user.UpdatedAt = time.Now()
		user.CreatedAt = time.Now()
		user.AllowedIPs = make([]string, 0) // UNKNOWN
		user.IPs = make([]string, len(peer.AllowedIPs))
		for i, ip := range peer.AllowedIPs {
			user.IPs[i] = ip.String()
		}
		user.AllowedIPsStr = strings.Join(user.AllowedIPs, ", ")
		user.IPsStr = strings.Join(user.IPs, ", ")

		res := u.db.Create(&user)
		if res.Error != nil {
			log.Errorf("failed to create autodetected peer: %v", res.Error)
		}
	}

	user.IPs = strings.Split(user.IPsStr, ", ")
	user.AllowedIPs = strings.Split(user.AllowedIPsStr, ", ")
	tmpCfg, _ := u.GetPeerConfigFile(user)
	user.Config = string(tmpCfg)

	return user
}

func (u *UserManager) GetUser(publicKey string) User {
	user := User{}
	u.db.Where("public_key = ?", publicKey).FirstOrInit(&user)

	user.IPs = strings.Split(user.IPsStr, ", ")
	user.AllowedIPs = strings.Split(user.AllowedIPsStr, ", ")
	tmpCfg, _ := u.GetPeerConfigFile(user)
	user.Config = string(tmpCfg)

	return user
}

func (u *UserManager) CreateUser(user User) error {
	user.UID = fmt.Sprintf("u%x", md5.Sum([]byte(user.PublicKey)))
	user.UpdatedAt = time.Now()
	user.CreatedAt = time.Now()
	user.AllowedIPsStr = strings.Join(user.AllowedIPs, ", ")
	user.IPsStr = strings.Join(user.IPs, ", ")

	res := u.db.Create(&user)
	if res.Error != nil {
		log.Errorf("failed to create user: %v", res.Error)
		return res.Error
	}

	return nil
}

func (u *UserManager) UpdateUser(user User) error {
	user.UpdatedAt = time.Now()
	user.AllowedIPsStr = strings.Join(user.AllowedIPs, ", ")
	user.IPsStr = strings.Join(user.IPs, ", ")

	res := u.db.Save(&user)
	if res.Error != nil {
		log.Errorf("failed to update user: %v", res.Error)
		return res.Error
	}

	return nil
}

func (u *UserManager) GetAllReservedIps() ([]string, error) {
	reservedIps := make([]string, 0)
	users := u.GetAllUsers()
	for _, user := range users {
		for _, cidr := range user.IPs {
			ip, _, err := net.ParseCIDR(cidr)
			if err != nil {
				log.WithFields(log.Fields{
					"err":  err,
					"cidr": cidr,
				}).Error("failed to ip from cidr")
			} else {
				reservedIps = append(reservedIps, ip.String())
			}
		}
	}

	device := u.GetDevice()
	for _, cidr := range device.IPs {
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			log.WithFields(log.Fields{
				"err":  err,
				"cidr": cidr,
			}).Error("failed to ip from cidr")
		} else {
			reservedIps = append(reservedIps, ip.String())
		}
	}

	return reservedIps, nil
}

// GetAvailableIp search for an available ip in cidr against a list of reserved ips
func (u *UserManager) GetAvailableIp(cidr string, reserved []string) (string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}

	// this two addresses are not usable
	broadcastAddr := common.BroadcastAddr(ipnet).String()
	networkAddr := ipnet.IP.String()

	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); common.IncreaseIP(ip) {
		ok := true
		address := ip.String()
		for _, r := range reserved {
			if address == r {
				ok = false
				break
			}
		}
		if ok && address != networkAddr && address != broadcastAddr {
			return address, nil
		}
	}

	return "", errors.New("no more available address from cidr")
}

func (u *UserManager) GetOrCreateDevice(dev wgtypes.Device) Device {
	device := Device{}
	u.db.Where("device_name = ?", dev.Name).FirstOrInit(&device)

	if device.PublicKey == "" { // device not found, create
		device.PublicKey = dev.PublicKey.String()
		device.PrivateKey = dev.PrivateKey.String()
		device.DeviceName = dev.Name
		device.ListenPort = dev.ListenPort
		device.Mtu = 0
		device.PersistentKeepalive = 16 // Default

		res := u.db.Create(&device)
		if res.Error != nil {
			log.Errorf("failed to create autodetected device: %v", res.Error)
		}
	}

	device.IPs = strings.Split(device.IPsStr, ", ")
	device.AllowedIPs = strings.Split(device.AllowedIPsStr, ", ")
	device.DNS = strings.Split(device.DNSStr, ", ")

	return device
}

func (u *UserManager) GetPeerConfigFile(user User) ([]byte, error) {
	tpl, err := template.New("client").Funcs(template.FuncMap{"StringsJoin": strings.Join}).Parse(wireguard.ClientCfgTpl)
	if err != nil {
		return nil, err
	}

	var tplBuff bytes.Buffer

	err = tpl.Execute(&tplBuff, struct {
		Client User
		Server Device
	}{
		Client: user,
		Server: u.GetDevice(),
	})
	if err != nil {
		return nil, err
	}

	return tplBuff.Bytes(), nil
}
