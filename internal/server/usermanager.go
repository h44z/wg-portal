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

	"github.com/gin-gonic/gin/binding"

	"github.com/go-playground/validator/v10"

	"github.com/h44z/wg-portal/internal/wireguard"

	"github.com/h44z/wg-portal/internal/common"

	"github.com/h44z/wg-portal/internal/ldap"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//
// CUSTOM VALIDATORS ----------------------------------------------------------------------------
//
var cidrList validator.Func = func(fl validator.FieldLevel) bool {
	cidrListStr := fl.Field().String()

	cidrList := common.ParseIPList(cidrListStr)
	for i := range cidrList {
		_, _, err := net.ParseCIDR(cidrList[i])
		if err != nil {
			return false
		}
	}
	return true
}

var ipList validator.Func = func(fl validator.FieldLevel) bool {
	ipListStr := fl.Field().String()

	ipList := common.ParseIPList(ipListStr)
	for i := range ipList {
		ip := net.ParseIP(ipList[i])
		if ip == nil {
			return false
		}
	}
	return true
}

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation("cidrlist", cidrList)
		v.RegisterValidation("iplist", ipList)
	}
}

//
//  USER ----------------------------------------------------------------------------------------
//

type User struct {
	Peer     *wgtypes.Peer              `gorm:"-"`
	LdapUser *ldap.UserCacheHolderEntry `gorm:"-"` // optional, it is still possible to have users without ldap
	Config   string                     `gorm:"-"`

	UID        string `form:"uid" binding:"alphanum"` // uid for html identification
	IsOnline   bool   `gorm:"-"`
	Identifier string `form:"identifier" binding:"required,lt=64"` // Identifier AND Email make a WireGuard peer unique
	Email      string `gorm:"index" form:"mail" binding:"required,email"`

	IgnorePersistentKeepalive bool     `form:"ignorekeepalive"`
	PresharedKey              string   `form:"presharedkey" binding:"omitempty,base64"`
	AllowedIPsStr             string   `form:"allowedip" binding:"cidrlist"`
	IPsStr                    string   `form:"ip" binding:"cidrlist"`
	AllowedIPs                []string `gorm:"-"` // IPs that are used in the client config file
	IPs                       []string `gorm:"-"` // The IPs of the client
	PrivateKey                string   `form:"privkey" binding:"omitempty,base64"`
	PublicKey                 string   `gorm:"primaryKey" form:"pubkey" binding:"required,base64"`

	DeactivatedAt *time.Time
	CreatedBy     string
	UpdatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (u User) GetClientConfigFile(device Device) ([]byte, error) {
	tpl, err := template.New("client").Funcs(template.FuncMap{"StringsJoin": strings.Join}).Parse(wireguard.ClientCfgTpl)
	if err != nil {
		return nil, err
	}

	var tplBuff bytes.Buffer

	err = tpl.Execute(&tplBuff, struct {
		Client User
		Server Device
	}{
		Client: u,
		Server: device,
	})
	if err != nil {
		return nil, err
	}

	return tplBuff.Bytes(), nil
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

func (u User) IsValid() bool {
	if u.PublicKey == "" {
		return false
	}

	return true
}

//
//  DEVICE --------------------------------------------------------------------------------------
//

type Device struct {
	Interface *wgtypes.Device `gorm:"-"`

	DeviceName          string   `form:"device" gorm:"primaryKey" binding:"required,alphanum"`
	PrivateKey          string   `form:"privkey" binding:"base64"`
	PublicKey           string   `form:"pubkey" binding:"required,base64"`
	PersistentKeepalive int      `form:"keepalive" binding:"gte=0"`
	ListenPort          int      `form:"port" binding:"required,gt=0"`
	Mtu                 int      `form:"mtu" binding:"gte=0,lte=1500"`
	Endpoint            string   `form:"endpoint" binding:"required,hostname_port"`
	AllowedIPsStr       string   `form:"allowedip" binding:"cidrlist"`
	IPsStr              string   `form:"ip" binding:"required,cidrlist"`
	AllowedIPs          []string `gorm:"-"` // IPs that are used in the client config file
	IPs                 []string `gorm:"-"` // The IPs of the client
	DNSStr              string   `form:"dns" binding:"iplist"`
	DNS                 []string `gorm:"-"` // The DNS servers of the client
	PreUp               string   `form:"preup"`
	PostUp              string   `form:"postup"`
	PreDown             string   `form:"predown"`
	PostDown            string   `form:"postdown"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (d Device) IsValid() bool {
	if d.PublicKey == "" {
		return false
	}
	if len(d.IPs) == 0 {
		return false
	}
	if d.Endpoint == "" {
		return false
	}

	return true
}

func (d Device) GetDeviceConfig() wgtypes.Config {
	var privateKey *wgtypes.Key
	if d.PrivateKey != "" {
		pKey, _ := wgtypes.ParseKey(d.PrivateKey)
		privateKey = &pKey
	}

	cfg := wgtypes.Config{
		PrivateKey: privateKey,
		ListenPort: &d.ListenPort,
	}

	return cfg
}

//
//  USER-MANAGER --------------------------------------------------------------------------------
//

type UserManager struct {
	db        *gorm.DB
	wg        *wireguard.Manager
	ldapUsers *ldap.SynchronizedUserCacheHolder
}

func NewUserManager(wg *wireguard.Manager, ldapUsers *ldap.SynchronizedUserCacheHolder) *UserManager {
	um := &UserManager{wg: wg, ldapUsers: ldapUsers}
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

func (u *UserManager) InitFromCurrentInterface() error {
	peers, err := u.wg.GetPeerList()
	if err != nil {
		log.Errorf("failed to init user-manager from peers: %v", err)
		return err
	}
	device, err := u.wg.GetDeviceInfo()
	if err != nil {
		log.Errorf("failed to init user-manager from device: %v", err)
		return err
	}

	// Check if entries already exist in database, if not create them
	for _, peer := range peers {
		if err := u.validateOrCreateUserForPeer(peer); err != nil {
			return err
		}
	}
	if err := u.validateOrCreateDevice(*device); err != nil {
		return err
	}

	return nil
}

func (u *UserManager) validateOrCreateUserForPeer(peer wgtypes.Peer) error {
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
			return res.Error
		}
	}

	return nil
}

func (u *UserManager) validateOrCreateDevice(dev wgtypes.Device) error {
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
			return res.Error
		}
	}

	return nil
}

func (u *UserManager) populateUserData(user *User) {
	user.AllowedIPs = strings.Split(user.AllowedIPsStr, ", ")
	user.IPs = strings.Split(user.IPsStr, ", ")
	// Set config file
	tmpCfg, _ := user.GetClientConfigFile(u.GetDevice())
	user.Config = string(tmpCfg)

	// set data from WireGuard interface
	user.Peer, _ = u.wg.GetPeer(user.PublicKey)
	user.IsOnline = false // todo: calculate online status

	// set ldap data
	user.LdapUser = u.ldapUsers.GetUserData(u.ldapUsers.GetUserDNByMail(user.Email))
}

func (u *UserManager) populateDeviceData(device *Device) {
	device.AllowedIPs = strings.Split(device.AllowedIPsStr, ", ")
	device.IPs = strings.Split(device.IPsStr, ", ")
	device.DNS = strings.Split(device.DNSStr, ", ")

	// set data from WireGuard interface
	device.Interface, _ = u.wg.GetDeviceInfo()
}

func (u *UserManager) GetAllUsers() []User {
	users := make([]User, 0)
	u.db.Find(&users)

	for i := range users {
		u.populateUserData(&users[i])
	}

	return users
}

func (u *UserManager) GetDevice() Device {
	devices := make([]Device, 0, 1)
	u.db.Find(&devices)

	for i := range devices {
		u.populateDeviceData(&devices[i])
	}

	return devices[0] // use first device for now... more to come?
}

func (u *UserManager) GetUserByKey(publicKey string) User {
	user := User{}
	u.db.Where("public_key = ?", publicKey).FirstOrInit(&user)
	u.populateUserData(&user)
	return user
}

func (u *UserManager) GetUserByMail(mail string) User {
	user := User{}
	u.db.Where("email = ?", mail).FirstOrInit(&user)
	u.populateUserData(&user)

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

func (u *UserManager) UpdateDevice(device Device) error {
	device.UpdatedAt = time.Now()
	device.AllowedIPsStr = strings.Join(device.AllowedIPs, ", ")
	device.IPsStr = strings.Join(device.IPs, ", ")
	device.DNSStr = strings.Join(device.DNS, ", ")

	res := u.db.Save(&device)
	if res.Error != nil {
		log.Errorf("failed to update device: %v", res.Error)
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
