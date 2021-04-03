package wireguard

// WireGuard documentation: https://manpages.debian.org/unstable/wireguard-tools/wg.8.en.html

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"net"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/h44z/wg-portal/internal/common"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

//
// CUSTOM VALIDATORS ----------------------------------------------------------------------------
//
var cidrList validator.Func = func(fl validator.FieldLevel) bool {
	cidrListStr := fl.Field().String()

	cidrList := common.ParseStringList(cidrListStr)
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

	ipList := common.ParseStringList(ipListStr)
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
		_ = v.RegisterValidation("cidrlist", cidrList)
		_ = v.RegisterValidation("iplist", ipList)
	}
}

//
//  PEER ----------------------------------------------------------------------------------------
//

type Peer struct {
	Peer   *wgtypes.Peer `gorm:"-"`                     // WireGuard peer
	Device *Device       `gorm:"foreignKey:DeviceName"` // linked WireGuard device
	Config string        `gorm:"-"`

	UID                  string `form:"uid" binding:"alphanum"` // uid for html identification
	DeviceName           string `gorm:"index"`
	Identifier           string `form:"identifier" binding:"required,max=64"` // Identifier AND Email make a WireGuard peer unique
	Email                string `gorm:"index" form:"mail" binding:"omitempty,email"`
	IgnoreGlobalSettings bool   `form:"ignoreglobalsettings"`

	IsOnline          bool   `gorm:"-"`
	IsNew             bool   `gorm:"-"`
	LastHandshake     string `gorm:"-"`
	LastHandshakeTime string `gorm:"-"`

	// Core WireGuard Settings
	PublicKey           string `gorm:"primaryKey" form:"pubkey" binding:"required,base64"`
	PresharedKey        string `form:"presharedkey" binding:"omitempty,base64"`
	AllowedIPsStr       string `form:"allowedip" binding:"cidrlist"` // a comma separated list of IPs that are used in the client config file
	Endpoint            string `form:"endpoint" binding:"omitempty,hostname_port"`
	PersistentKeepalive int    `form:"keepalive" binding:"gte=0"`

	// Misc. WireGuard Settings
	PrivateKey string `form:"privkey" binding:"omitempty,base64"`
	IPsStr     string `form:"ip" binding:"cidrlist"` // a comma separated list of IPs of the client
	DNSStr     string `form:"dns" binding:"iplist"`  // comma separated list of the DNS servers for the client

	DeactivatedAt *time.Time
	CreatedBy     string
	UpdatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (p *Peer) SetIPAddresses(addresses ...string) {
	p.IPsStr = common.ListToString(addresses)
}

func (p Peer) GetIPAddresses() []string {
	return common.ParseStringList(p.IPsStr)
}

func (p *Peer) SetDNSServers(addresses ...string) {
	p.DNSStr = common.ListToString(addresses)
}

func (p Peer) GetDNSServers() []string {
	return common.ParseStringList(p.DNSStr)
}

func (p *Peer) SetAllowedIPs(addresses ...string) {
	p.AllowedIPsStr = common.ListToString(addresses)
}

func (p Peer) GetAllowedIPs() []string {
	return common.ParseStringList(p.AllowedIPsStr)
}

func (p Peer) GetConfig() wgtypes.PeerConfig {
	publicKey, _ := wgtypes.ParseKey(p.PublicKey)
	var presharedKey *wgtypes.Key
	if p.PresharedKey != "" {
		presharedKeyTmp, _ := wgtypes.ParseKey(p.PresharedKey)
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
		AllowedIPs:                  make([]net.IPNet, len(p.GetIPAddresses())),
	}
	for i, ip := range p.GetIPAddresses() {
		_, ipNet, err := net.ParseCIDR(ip)
		if err == nil {
			cfg.AllowedIPs[i] = *ipNet
		}
	}

	return cfg
}

func (p Peer) GetConfigFile(device Device) ([]byte, error) {
	tpl, err := template.New("client").Funcs(template.FuncMap{"StringsJoin": strings.Join}).Parse(ClientCfgTpl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse client template")
	}

	var tplBuff bytes.Buffer

	err = tpl.Execute(&tplBuff, struct {
		Client Peer
		Server Device
	}{
		Client: p,
		Server: device,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute client template")
	}

	return tplBuff.Bytes(), nil
}

func (p Peer) GetQRCode() ([]byte, error) {
	png, err := qrcode.Encode(p.Config, qrcode.Medium, 250)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"err": err,
		}).Error("failed to create qrcode")
		return nil, errors.Wrap(err, "failed to encode qrcode")
	}
	return png, nil
}

func (p Peer) IsValid() bool {
	if p.PublicKey == "" {
		return false
	}

	return true
}

func (p Peer) ToMap() map[string]string {
	out := make(map[string]string)

	v := reflect.ValueOf(p)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		// gets us a StructField
		fi := typ.Field(i)
		if tagv := fi.Tag.Get("form"); tagv != "" {
			// set key of map to value in struct field
			out[tagv] = v.Field(i).String()
		}
	}
	return out
}

func (p Peer) GetConfigFileName() string {
	reg := regexp.MustCompile("[^a-zA-Z0-9_-]+")
	return reg.ReplaceAllString(strings.ReplaceAll(p.Identifier, " ", "-"), "") + ".conf"
}

//
//  DEVICE --------------------------------------------------------------------------------------
//

type DeviceType string

const (
	DeviceTypeServer DeviceType = "server"
	DeviceTypeClient DeviceType = "client"
	DeviceTypeCustom DeviceType = "custom"
)

type Device struct {
	Interface *wgtypes.Device `gorm:"-"`

	Type        DeviceType `form:"devicetype" binding:"required,oneof=client server custom"`
	DeviceName  string     `form:"device" gorm:"primaryKey" binding:"required,alphanum"`
	DisplayName string     `form:"displayname" binding:"omitempty,max=200"`

	// Core WireGuard Settings (Interface section)
	PrivateKey   string `form:"privkey" binding:"required,base64"`
	ListenPort   int    `form:"port" binding:"gt=0,lt=65535"`
	FirewallMark int32  `form:"firewallmark" binding:"gte=0"`
	// Misc. WireGuard Settings
	PublicKey    string `form:"pubkey" binding:"required,base64"`
	Mtu          int    `form:"mtu" binding:"gte=0,lte=1500"`   // the interface MTU, wg-quick addition
	IPsStr       string `form:"ip" binding:"required,cidrlist"` // comma separated list of the IPs of the client, wg-quick addition
	DNSStr       string `form:"dns" binding:"iplist"`           // comma separated list of the DNS servers of the client, wg-quick addition
	RoutingTable string `form:"routingtable"`                   // the routing table, wg-quick addition
	PreUp        string `form:"preup"`                          // pre up script, wg-quick addition
	PostUp       string `form:"postup"`                         // post up script, wg-quick addition
	PreDown      string `form:"predown"`                        // pre down script, wg-quick addition
	PostDown     string `form:"postdown"`                       // post down script, wg-quick addition
	SaveConfig   bool   `form:"saveconfig"`                     // if set to `true', the configuration is saved from the current state of the interface upon shutdown, wg-quick addition

	// Settings that are applied to all peer by default
	DefaultEndpoint            string `form:"endpoint" binding:"omitempty,hostname_port"`
	DefaultAllowedIPsStr       string `form:"allowedip" binding:"cidrlist"` // comma separated list  of IPs that are used in the client config file
	DefaultPersistentKeepalive int    `form:"keepalive" binding:"gte=0"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (d Device) IsValid() bool {
	switch d.Type {
	case DeviceTypeServer:
		if d.PublicKey == "" {
			return false
		}
		if len(d.GetIPAddresses()) == 0 {
			return false
		}
		if d.DefaultEndpoint == "" {
			return false
		}
	case DeviceTypeClient:
		if d.PublicKey == "" {
			return false
		}
		if len(d.GetIPAddresses()) == 0 {
			return false
		}
	case DeviceTypeCustom:
	}

	return true
}

func (d *Device) SetIPAddresses(addresses ...string) {
	d.IPsStr = common.ListToString(addresses)
}

func (d Device) GetIPAddresses() []string {
	return common.ParseStringList(d.IPsStr)
}

func (d *Device) SetDNSServers(addresses ...string) {
	d.DNSStr = common.ListToString(addresses)
}

func (d Device) GetDNSServers() []string {
	return common.ParseStringList(d.DNSStr)
}

func (d *Device) SetDefaultAllowedIPs(addresses ...string) {
	d.DefaultAllowedIPsStr = common.ListToString(addresses)
}

func (d Device) GetDefaultAllowedIPs() []string {
	return common.ParseStringList(d.DefaultAllowedIPsStr)
}

func (d Device) GetConfig() wgtypes.Config {
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

func (d Device) GetConfigFile(peers []Peer) ([]byte, error) {
	tpl, err := template.New("server").Funcs(template.FuncMap{"StringsJoin": strings.Join}).Parse(DeviceCfgTpl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse server template")
	}

	var tplBuff bytes.Buffer

	err = tpl.Execute(&tplBuff, struct {
		Clients []Peer
		Server  Device
	}{
		Clients: peers,
		Server:  d,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute server template")
	}

	return tplBuff.Bytes(), nil
}

//
//  PEER-MANAGER --------------------------------------------------------------------------------
//

type PeerManager struct {
	db *gorm.DB
	wg *Manager
}

func NewPeerManager(db *gorm.DB, wg *Manager) (*PeerManager, error) {
	pm := &PeerManager{db: db, wg: wg}

	if err := pm.db.AutoMigrate(&Peer{}, &Device{}); err != nil {
		return nil, errors.WithMessage(err, "failed to migrate peer database")
	}

	// check if peers without device name exist (from version <= 1.0.3), if so assign them to the default device.
	peers := make([]Peer, 0)
	pm.db.Find(&peers)
	for i := range peers {
		if peers[i].DeviceName == "" {
			peers[i].DeviceName = wg.Cfg.GetDefaultDeviceName()
			pm.db.Save(&peers[i])
		}
	}

	return pm, nil
}

// InitFromPhysicalInterface read all WireGuard peers from the WireGuard interface configuration. If a peer does not
// exist in the local database, it gets created.
func (m *PeerManager) InitFromPhysicalInterface() error {
	for _, deviceName := range m.wg.Cfg.DeviceNames {
		peers, err := m.wg.GetPeerList(deviceName)
		if err != nil {
			return errors.Wrapf(err, "failed to get peer list for device %s", deviceName)
		}
		device, err := m.wg.GetDeviceInfo(deviceName)
		if err != nil {
			return errors.Wrapf(err, "failed to get device info for device %s", deviceName)
		}
		var ipAddresses []string
		var mtu int
		if m.wg.Cfg.ManageIPAddresses {
			if ipAddresses, err = m.wg.GetIPAddress(deviceName); err != nil {
				return errors.Wrapf(err, "failed to get ip address for device %s", deviceName)
			}
			if mtu, err = m.wg.GetMTU(deviceName); err != nil {
				return errors.Wrapf(err, "failed to get MTU for device %s", deviceName)
			}
		}

		// Check if entries already exist in database, if not create them
		for _, peer := range peers {
			if err := m.validateOrCreatePeer(deviceName, peer); err != nil {
				return errors.WithMessagef(err, "failed to validate peer %s for device %s", peer.PublicKey, deviceName)
			}
		}
		if err := m.validateOrCreateDevice(*device, ipAddresses, mtu); err != nil {
			return errors.WithMessagef(err, "failed to validate device %s", device.Name)
		}
	}

	return nil
}

// validateOrCreatePeer checks if the given WireGuard peer already exists in the database, if not, the peer entry will be created
func (m *PeerManager) validateOrCreatePeer(device string, wgPeer wgtypes.Peer) error {
	peer := Peer{}
	m.db.Where("public_key = ?", wgPeer.PublicKey.String()).FirstOrInit(&peer)

	if peer.PublicKey == "" { // peer not found, create
		peer.UID = fmt.Sprintf("u%x", md5.Sum([]byte(wgPeer.PublicKey.String())))
		peer.PublicKey = wgPeer.PublicKey.String()
		peer.PrivateKey = "" // UNKNOWN
		if wgPeer.PresharedKey != (wgtypes.Key{}) {
			peer.PresharedKey = wgPeer.PresharedKey.String()
		}
		peer.Email = "autodetected@example.com"
		peer.Identifier = "Autodetected (" + peer.PublicKey[0:8] + ")"
		peer.UpdatedAt = time.Now()
		peer.CreatedAt = time.Now()
		IPs := make([]string, len(wgPeer.AllowedIPs))
		for i, ip := range wgPeer.AllowedIPs {
			IPs[i] = ip.String()
		}
		peer.AllowedIPsStr = "" // UNKNOWN
		peer.SetIPAddresses(IPs...)
		peer.DeviceName = device

		res := m.db.Create(&peer)
		if res.Error != nil {
			return errors.Wrapf(res.Error, "failed to create autodetected peer %s", peer.PublicKey)
		}
	}

	return nil
}

// validateOrCreateDevice checks if the given WireGuard device already exists in the database, if not, the peer entry will be created
func (m *PeerManager) validateOrCreateDevice(dev wgtypes.Device, ipAddresses []string, mtu int) error {
	device := Device{}
	m.db.Where("device_name = ?", dev.Name).FirstOrInit(&device)

	if device.PublicKey == "" { // device not found, create
		device.Type = DeviceTypeCustom // imported device, we do not (easily) know if it is a client or server
		device.PublicKey = dev.PublicKey.String()
		device.PrivateKey = dev.PrivateKey.String()
		device.DeviceName = dev.Name
		device.ListenPort = dev.ListenPort
		device.Mtu = 0
		device.DefaultPersistentKeepalive = 16 // Default
		device.IPsStr = strings.Join(ipAddresses, ", ")
		if mtu == DefaultMTU {
			mtu = 0
		}
		device.Mtu = mtu

		res := m.db.Create(&device)
		if res.Error != nil {
			return errors.Wrapf(res.Error, "failed to create autodetected device")
		}
	}

	return nil
}

// populatePeerData enriches the peer struct with WireGuard live data like last handshake, ...
func (m *PeerManager) populatePeerData(peer *Peer) {
	// Set config file
	tmpCfg, _ := peer.GetConfigFile(m.GetDevice(peer.DeviceName))
	peer.Config = string(tmpCfg)

	// set data from WireGuard interface
	peer.Peer, _ = m.wg.GetPeer(peer.DeviceName, peer.PublicKey)
	peer.LastHandshake = "never"
	peer.LastHandshakeTime = "Never connected, or user is disabled."
	if peer.Peer != nil {
		since := time.Since(peer.Peer.LastHandshakeTime)
		sinceSeconds := int(since.Round(time.Second).Seconds())
		sinceMinutes := sinceSeconds / 60
		sinceSeconds -= sinceMinutes * 60

		if sinceMinutes > 2*10080 { // 2 weeks
			peer.LastHandshake = "a while ago"
		} else if sinceMinutes > 10080 { // 1 week
			peer.LastHandshake = "a week ago"
		} else {
			peer.LastHandshake = fmt.Sprintf("%02dm %02ds", sinceMinutes, sinceSeconds)
		}
		peer.LastHandshakeTime = peer.Peer.LastHandshakeTime.Format(time.UnixDate)
	}
	peer.IsOnline = false
}

// populateDeviceData enriches the device struct with WireGuard live data like interface information
func (m *PeerManager) populateDeviceData(device *Device) {
	// set data from WireGuard interface
	device.Interface, _ = m.wg.GetDeviceInfo(device.DeviceName)
}

func (m *PeerManager) GetAllPeers(device string) []Peer {
	peers := make([]Peer, 0)
	m.db.Where("device_name = ?", device).Find(&peers)

	for i := range peers {
		m.populatePeerData(&peers[i])
	}

	return peers
}

func (m *PeerManager) GetActivePeers(device string) []Peer {
	peers := make([]Peer, 0)
	m.db.Where("device_name = ? AND deactivated_at IS NULL", device).Find(&peers)

	for i := range peers {
		m.populatePeerData(&peers[i])
	}

	return peers
}

func (m *PeerManager) GetFilteredAndSortedPeers(device, sortKey, sortDirection, search string) []Peer {
	peers := make([]Peer, 0)
	m.db.Where("device_name = ?", device).Find(&peers)

	filteredPeers := make([]Peer, 0, len(peers))
	for i := range peers {
		m.populatePeerData(&peers[i])

		if search == "" ||
			strings.Contains(peers[i].Email, search) ||
			strings.Contains(peers[i].Identifier, search) ||
			strings.Contains(peers[i].PublicKey, search) {
			filteredPeers = append(filteredPeers, peers[i])
		}
	}

	sort.Slice(filteredPeers, func(i, j int) bool {
		var sortValueLeft string
		var sortValueRight string

		switch sortKey {
		case "id":
			sortValueLeft = filteredPeers[i].Identifier
			sortValueRight = filteredPeers[j].Identifier
		case "pubKey":
			sortValueLeft = filteredPeers[i].PublicKey
			sortValueRight = filteredPeers[j].PublicKey
		case "mail":
			sortValueLeft = filteredPeers[i].Email
			sortValueRight = filteredPeers[j].Email
		case "ip":
			sortValueLeft = filteredPeers[i].IPsStr
			sortValueRight = filteredPeers[j].IPsStr
		case "handshake":
			if filteredPeers[i].Peer == nil {
				return false
			} else if filteredPeers[j].Peer == nil {
				return true
			}
			sortValueLeft = filteredPeers[i].Peer.LastHandshakeTime.Format(time.RFC3339)
			sortValueRight = filteredPeers[j].Peer.LastHandshakeTime.Format(time.RFC3339)
		}

		if sortDirection == "asc" {
			return sortValueLeft < sortValueRight
		} else {
			return sortValueLeft > sortValueRight
		}
	})

	return filteredPeers
}

func (m *PeerManager) GetSortedPeersForEmail(sortKey, sortDirection, email string) []Peer {
	peers := make([]Peer, 0)
	m.db.Where("email = ?", email).Find(&peers)

	for i := range peers {
		m.populatePeerData(&peers[i])
	}

	sort.Slice(peers, func(i, j int) bool {
		var sortValueLeft string
		var sortValueRight string

		switch sortKey {
		case "id":
			sortValueLeft = peers[i].Identifier
			sortValueRight = peers[j].Identifier
		case "pubKey":
			sortValueLeft = peers[i].PublicKey
			sortValueRight = peers[j].PublicKey
		case "mail":
			sortValueLeft = peers[i].Email
			sortValueRight = peers[j].Email
		case "ip":
			sortValueLeft = peers[i].IPsStr
			sortValueRight = peers[j].IPsStr
		case "handshake":
			if peers[i].Peer == nil {
				return true
			} else if peers[j].Peer == nil {
				return false
			}
			sortValueLeft = peers[i].Peer.LastHandshakeTime.Format(time.RFC3339)
			sortValueRight = peers[j].Peer.LastHandshakeTime.Format(time.RFC3339)
		}

		if sortDirection == "asc" {
			return sortValueLeft < sortValueRight
		} else {
			return sortValueLeft > sortValueRight
		}
	})

	return peers
}

func (m *PeerManager) GetDevice(device string) Device {
	dev := Device{}

	m.db.Where("device_name = ?", device).First(&dev)
	m.populateDeviceData(&dev)

	return dev
}

func (m *PeerManager) GetPeerByKey(publicKey string) Peer {
	peer := Peer{}
	m.db.Where("public_key = ?", publicKey).FirstOrInit(&peer)
	m.populatePeerData(&peer)
	return peer
}

func (m *PeerManager) GetPeersByMail(mail string) []Peer {
	var peers []Peer
	m.db.Where("email = ?", mail).Find(&peers)
	for i := range peers {
		m.populatePeerData(&peers[i])
	}

	return peers
}

// ---- Database helpers -----

func (m *PeerManager) CreatePeer(peer Peer) error {
	peer.UID = fmt.Sprintf("u%x", md5.Sum([]byte(peer.PublicKey)))
	peer.UpdatedAt = time.Now()
	peer.CreatedAt = time.Now()

	res := m.db.Create(&peer)
	if res.Error != nil {
		logrus.Errorf("failed to create peer: %v", res.Error)
		return errors.Wrap(res.Error, "failed to create peer")
	}

	return nil
}

func (m *PeerManager) UpdatePeer(peer Peer) error {
	peer.UpdatedAt = time.Now()

	res := m.db.Save(&peer)
	if res.Error != nil {
		logrus.Errorf("failed to update peer: %v", res.Error)
		return errors.Wrap(res.Error, "failed to update peer")
	}

	return nil
}

func (m *PeerManager) DeletePeer(peer Peer) error {
	res := m.db.Delete(&peer)
	if res.Error != nil {
		logrus.Errorf("failed to delete peer: %v", res.Error)
		return errors.Wrap(res.Error, "failed to delete peer")
	}

	return nil
}

func (m *PeerManager) UpdateDevice(device Device) error {
	device.UpdatedAt = time.Now()

	res := m.db.Save(&device)
	if res.Error != nil {
		logrus.Errorf("failed to update device: %v", res.Error)
		return errors.Wrap(res.Error, "failed to update device")
	}

	return nil
}

// ---- IP helpers ----

func (m *PeerManager) GetAllReservedIps(device string) ([]string, error) {
	reservedIps := make([]string, 0)
	peers := m.GetAllPeers(device)
	for _, user := range peers {
		for _, cidr := range user.GetIPAddresses() {
			if cidr == "" {
				continue
			}
			ip, _, err := net.ParseCIDR(cidr)
			if err != nil {
				return nil, errors.Wrap(err, "failed to parse cidr")
			}
			reservedIps = append(reservedIps, ip.String())
		}
	}

	dev := m.GetDevice(device)
	for _, cidr := range dev.GetIPAddresses() {
		if cidr == "" {
			continue
		}
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse cidr")
		}

		reservedIps = append(reservedIps, ip.String())
	}

	return reservedIps, nil
}

func (m *PeerManager) IsIPReserved(device string, cidr string) bool {
	reserved, err := m.GetAllReservedIps(device)
	if err != nil {
		return true // in case something failed, assume the ip is reserved
	}
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return true
	}

	// this two addresses are not usable
	broadcastAddr := common.BroadcastAddr(ipnet).String()
	networkAddr := ipnet.IP.String()
	address := ip.String()

	if address == broadcastAddr || address == networkAddr {
		return true
	}

	for _, r := range reserved {
		if address == r {
			return true
		}
	}

	return false
}

// GetAvailableIp search for an available ip in cidr against a list of reserved ips
func (m *PeerManager) GetAvailableIp(device string, cidr string) (string, error) {
	reserved, err := m.GetAllReservedIps(device)
	if err != nil {
		return "", errors.WithMessagef(err, "failed to get all reserved IP addresses for %s", device)
	}
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse cidr")
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
			netMask := "/32"
			if common.IsIPv6(address) {
				netMask = "/128"
			}
			return address + netMask, nil
		}
	}

	return "", errors.New("no more available address from cidr")
}
