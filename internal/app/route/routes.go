package route

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/lowlevel"
)

type routeRuleInfo struct {
	ifaceId    domain.InterfaceIdentifier
	fwMark     uint32
	table      int
	family     int
	hasDefault bool
}

// Manager is try to mimic wg-quick behaviour (https://git.zx2c4.com/wireguard-tools/tree/src/wg-quick/linux.bash)
// for default routes.
type Manager struct {
	cfg *config.Config
	bus evbus.MessageBus

	wg lowlevel.WireGuardClient
	nl lowlevel.NetlinkClient
	db InterfaceAndPeerDatabaseRepo
}

func NewRouteManager(cfg *config.Config, bus evbus.MessageBus, db InterfaceAndPeerDatabaseRepo) (*Manager, error) {
	wg, err := wgctrl.New()
	if err != nil {
		panic("failed to init wgctrl: " + err.Error())
	}

	nl := &lowlevel.NetlinkManager{}

	m := &Manager{
		cfg: cfg,
		bus: bus,

		db: db,
		wg: wg,
		nl: nl,
	}

	m.connectToMessageBus()

	return m, nil
}

func (m Manager) connectToMessageBus() {
	_ = m.bus.Subscribe(app.TopicRouteUpdate, m.handleRouteUpdateEvent)
	_ = m.bus.Subscribe(app.TopicRouteRemove, m.handleRouteRemoveEvent)
}

func (m Manager) StartBackgroundJobs(ctx context.Context) {
}

func (m Manager) handleRouteUpdateEvent(srcDescription string) {
	logrus.Debugf("handling route update event: %s", srcDescription)

	err := m.syncRoutes(context.Background())
	if err != nil {
		logrus.Errorf("failed to synchronize routes for event %s: %v", srcDescription, err)
	}

	logrus.Debugf("routes synchronized, event: %s", srcDescription)
}

func (m Manager) handleRouteRemoveEvent(info domain.RoutingTableInfo) {
	logrus.Debugf("handling route remove event for: %s", info.String())

	if !info.ManagementEnabled() {
		return // route management disabled
	}

	if err := m.removeFwMarkRules(info.FwMark, info.GetRoutingTable(), netlink.FAMILY_V4); err != nil {
		logrus.Errorf("failed to remove v4 fwmark rules: %v", err)
	}
	if err := m.removeFwMarkRules(info.FwMark, info.GetRoutingTable(), netlink.FAMILY_V6); err != nil {
		logrus.Errorf("failed to remove v6 fwmark rules: %v", err)
	}

	logrus.Debugf("routes removed, table: %s", info.String())
}

func (m Manager) syncRoutes(ctx context.Context) error {
	interfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to find all interfaces: %w", err)
	}

	rules := map[int][]routeRuleInfo{
		netlink.FAMILY_V4: nil,
		netlink.FAMILY_V6: nil,
	}
	for _, iface := range interfaces {
		if iface.IsDisabled() {
			continue // disabled interface does not need route entries
		}
		if !iface.ManageRoutingTable() {
			continue
		}

		peers, err := m.db.GetInterfacePeers(ctx, iface.Identifier)
		if err != nil {
			return fmt.Errorf("failed to find peers for %s: %w", iface.Identifier, err)
		}
		allowedIPs := iface.GetAllowedIPs(peers)
		defRouteV4, defRouteV6 := m.containsDefaultRoute(allowedIPs)

		link, err := m.nl.LinkByName(string(iface.Identifier))
		if err != nil {
			return fmt.Errorf("failed to find physical link for %s: %w", iface.Identifier, err)
		}

		table, fwmark, err := m.getRoutingTableAndFwMark(&iface, allowedIPs, link)
		if err != nil {
			return fmt.Errorf("failed to get table and fwmark for %s: %w", iface.Identifier, err)
		}

		if err := m.setInterfaceRoutes(link, table, allowedIPs); err != nil {
			return fmt.Errorf("failed to set routes for %s: %w", iface.Identifier, err)
		}

		if err := m.removeDeprecatedRoutes(link, netlink.FAMILY_V4, allowedIPs); err != nil {
			return fmt.Errorf("failed to remove deprecated v4 routes for %s: %w", iface.Identifier, err)
		}
		if err := m.removeDeprecatedRoutes(link, netlink.FAMILY_V6, allowedIPs); err != nil {
			return fmt.Errorf("failed to remove deprecated v6 routes for %s: %w", iface.Identifier, err)
		}

		if table != 0 {
			rules[netlink.FAMILY_V4] = append(rules[netlink.FAMILY_V4], routeRuleInfo{
				ifaceId:    iface.Identifier,
				fwMark:     fwmark,
				table:      table,
				family:     netlink.FAMILY_V4,
				hasDefault: defRouteV4,
			})
		}
		if table != 0 {
			rules[netlink.FAMILY_V6] = append(rules[netlink.FAMILY_V6], routeRuleInfo{
				ifaceId:    iface.Identifier,
				fwMark:     fwmark,
				table:      table,
				family:     netlink.FAMILY_V6,
				hasDefault: defRouteV6,
			})
		}
	}

	return m.syncRouteRules(rules)
}

func (m Manager) syncRouteRules(allRules map[int][]routeRuleInfo) error {
	for family, rules := range allRules {
		// update fwmark rules
		if err := m.setFwMarkRules(rules, family); err != nil {
			return err
		}

		// update main rule
		if err := m.setMainRule(rules, family); err != nil {
			return err
		}

		// cleanup old main rules
		if err := m.cleanupMainRule(rules, family); err != nil {
			return err
		}
	}

	return nil
}

func (m Manager) setFwMarkRules(rules []routeRuleInfo, family int) error {
	for _, rule := range rules {
		existingRules, err := m.nl.RuleList(family)
		if err != nil {
			return fmt.Errorf("failed to get existing rules for family %d: %w", family, err)
		}

		ruleExists := false
		for _, existingRule := range existingRules {
			if rule.fwMark == existingRule.Mark && rule.table == existingRule.Table {
				ruleExists = true
				break
			}
		}

		if ruleExists {
			continue // rule already exists, no need to recreate it
		}

		// create missing rule
		if err := m.nl.RuleAdd(&netlink.Rule{
			Family:            family,
			Table:             rule.table,
			Mark:              rule.fwMark,
			Invert:            true,
			SuppressIfgroup:   -1,
			SuppressPrefixlen: -1,
			Priority:          m.getRulePriority(existingRules),
			Mask:              nil,
			Goto:              -1,
			Flow:              -1,
		}); err != nil {
			return fmt.Errorf("failed to setup rule for fwmark %d and table %d: %w", rule.fwMark, rule.table, err)
		}
	}
	return nil
}

func (m Manager) removeFwMarkRules(fwmark uint32, table int, family int) error {
	existingRules, err := m.nl.RuleList(family)
	if err != nil {
		return fmt.Errorf("failed to get existing rules for family %d: %w", family, err)
	}

	for _, existingRule := range existingRules {
		if fwmark == existingRule.Mark && table == existingRule.Table {
			existingRule.Family = family // set family, somehow the RuleList method does not populate the family field
			if err := m.nl.RuleDel(&existingRule); err != nil {
				return fmt.Errorf("failed to delete fwmark rule: %w", err)
			}
		}
	}
	return nil
}

func (m Manager) setMainRule(rules []routeRuleInfo, family int) error {
	shouldHaveMainRule := false
	for _, rule := range rules {
		if rule.hasDefault == true {
			shouldHaveMainRule = true
			break
		}
	}
	if !shouldHaveMainRule {
		return nil
	}

	existingRules, err := m.nl.RuleList(family)
	if err != nil {
		return fmt.Errorf("failed to get existing rules for family %d: %w", family, err)
	}

	ruleExists := false
	for _, existingRule := range existingRules {
		if existingRule.Table == unix.RT_TABLE_MAIN && existingRule.SuppressPrefixlen == 0 {
			ruleExists = true
			break
		}
	}

	if ruleExists {
		return nil // rule already exists, skip re-creation
	}

	if err := m.nl.RuleAdd(&netlink.Rule{
		Family:            family,
		Table:             unix.RT_TABLE_MAIN,
		SuppressIfgroup:   -1,
		SuppressPrefixlen: 0,
		Priority:          m.getMainRulePriority(existingRules),
		Mark:              0,
		Mask:              nil,
		Goto:              -1,
		Flow:              -1,
	}); err != nil {
		return fmt.Errorf("failed to setup rule for main table: %w", err)
	}

	return nil
}

func (m Manager) cleanupMainRule(rules []routeRuleInfo, family int) error {
	existingRules, err := m.nl.RuleList(family)
	if err != nil {
		return fmt.Errorf("failed to get existing rules for family %d: %w", family, err)
	}

	shouldHaveMainRule := false
	for _, rule := range rules {
		if rule.hasDefault == true {
			shouldHaveMainRule = true
			break
		}
	}

	mainRules := 0
	for _, existingRule := range existingRules {
		if existingRule.Table == unix.RT_TABLE_MAIN && existingRule.SuppressPrefixlen == 0 {
			mainRules++
		}
	}

	removalCount := 0
	if mainRules > 1 {
		removalCount = mainRules - 1 // we only want one single rule
	}
	if !shouldHaveMainRule {
		removalCount = mainRules
	}

	for _, existingRule := range existingRules {
		if existingRule.Table == unix.RT_TABLE_MAIN && existingRule.SuppressPrefixlen == 0 {
			if removalCount > 0 {
				existingRule.Family = family // set family, somehow the RuleList method does not populate the family field
				if err := m.nl.RuleDel(&existingRule); err != nil {
					return fmt.Errorf("failed to delete main rule: %w", err)
				}
				removalCount--
			}
		}
	}

	return nil
}

func (m Manager) getMainRulePriority(existingRules []netlink.Rule) int {
	prio := m.cfg.Advanced.RulePrioOffset
	for {
		isFresh := true
		for _, existingRule := range existingRules {
			if existingRule.Priority == prio {
				isFresh = false
				break
			}
		}
		if isFresh {
			break
		} else {
			prio++
		}
	}
	return prio
}

func (m Manager) getRulePriority(existingRules []netlink.Rule) int {
	prio := 32700 // linux main rule has a prio of 32766
	for {
		isFresh := true
		for _, existingRule := range existingRules {
			if existingRule.Priority == prio {
				isFresh = false
				break
			}
		}
		if isFresh {
			break
		} else {
			prio--
		}
	}
	return prio
}

func (m Manager) setInterfaceRoutes(link netlink.Link, table int, allowedIPs []domain.Cidr) error {
	for _, allowedIP := range allowedIPs {
		err := m.nl.RouteReplace(&netlink.Route{
			LinkIndex: link.Attrs().Index,
			Dst:       allowedIP.IpNet(),
			Table:     table,
			Scope:     unix.RT_SCOPE_LINK,
			Type:      unix.RTN_UNICAST,
		})
		if err != nil {
			return fmt.Errorf("failed to add/update route %s: %w", allowedIP.String(), err)
		}
	}

	return nil
}

func (m Manager) removeDeprecatedRoutes(link netlink.Link, family int, allowedIPs []domain.Cidr) error {
	rawRoutes, err := m.nl.RouteListFiltered(family, &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Table:     unix.RT_TABLE_UNSPEC, // all tables
		Scope:     unix.RT_SCOPE_LINK,
		Type:      unix.RTN_UNICAST,
	}, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_TYPE|netlink.RT_FILTER_OIF)
	if err != nil {
		return fmt.Errorf("failed to fetch raw routes: %w", err)
	}
	for _, rawRoute := range rawRoutes {
		if rawRoute.Dst == nil { // handle default route
			var netlinkAddr domain.Cidr
			if family == netlink.FAMILY_V4 {
				netlinkAddr, _ = domain.CidrFromString("0.0.0.0/0")
			} else {
				netlinkAddr, _ = domain.CidrFromString("::/0")
			}
			rawRoute.Dst = netlinkAddr.IpNet()
		}

		netlinkAddr := domain.CidrFromIpNet(*rawRoute.Dst)
		remove := true
		for _, allowedIP := range allowedIPs {
			if netlinkAddr == allowedIP {
				remove = false
				break
			}
		}

		if !remove {
			continue
		}

		err := m.nl.RouteDel(&rawRoute)
		if err != nil {
			return fmt.Errorf("failed to remove deprecated route %s: %w", netlinkAddr.String(), err)
		}
	}
	return nil
}

func (m Manager) getRoutingTableAndFwMark(
	iface *domain.Interface,
	allowedIPs []domain.Cidr,
	link netlink.Link,
) (table int, fwmark uint32, err error) {
	table = iface.GetRoutingTable()
	fwmark = iface.FirewallMark

	if fwmark == 0 {
		// generate a new (temporary) firewall mark based on the interface index
		fwmark = uint32(m.cfg.Advanced.RouteTableOffset + link.Attrs().Index)
		logrus.Debugf("%s: using fwmark %d to handle routes", iface.Identifier, table)

		// apply the temporary fwmark to the wireguard interface
		err = m.setFwMark(iface.Identifier, int(fwmark))
	}
	if table == 0 {
		table = int(fwmark) // generate a new routing table base on interface index
		logrus.Debugf("%s: using routing table %d to handle default routes", iface.Identifier, table)
	}
	return
}

func (m Manager) setFwMark(id domain.InterfaceIdentifier, fwmark int) error {
	err := m.wg.ConfigureDevice(string(id), wgtypes.Config{
		FirewallMark: &fwmark,
	})
	if err != nil {
		return fmt.Errorf("failed to update fwmark to: %d: %w", fwmark, err)
	}
	return nil
}

func (m Manager) containsDefaultRoute(allowedIPs []domain.Cidr) (ipV4, ipV6 bool) {
	for _, allowedIP := range allowedIPs {
		if ipV4 && ipV6 {
			break // speed up
		}

		if allowedIP.Prefix().Bits() == 0 {
			if allowedIP.IsV4() {
				ipV4 = true
			} else {
				ipV6 = true
			}
		}
	}

	return
}
