package domain

import (
	"testing"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAmneziaWGOptionsValidation(t *testing.T) {
	require.NoError(t, (AmneziaWGOptions{"Jc": "4", "Jmin": "40", "Jmax": "70", "H1": "100-200", "S4": "16", "I1": "<b 0x1234>", "ContentPaddingAddition": "0-16", "RandomTrailers": "on"}).Validate())
	for _, options := range []AmneziaWGOptions{
		{"H1": "4294967296"}, {"H1": "200-100"}, {"Jc": "-1"}, {"Jc": "65536"},
		{"Jmin": "71", "Jmax": "70"}, {"RekeyTimeout": "x"}, {"PreUp": "evil"},
		{"I1": "abc\n[Peer]"}, {"I1": "abc#comment"}, {"RandomTrailers": "bad"}, {"HeaderProtectionKey": "bad"},
	} {
		require.ErrorIs(t, options.Validate(), ErrInvalidData)
	}
	iface := &Interface{Backend: config.LocalBackendName, AmneziaWG: AmneziaWGOptions{"Jc": "4"}}
	require.ErrorIs(t, iface.Validate(), ErrInvalidData)
	iface.Backend = config.AmneziaWGBackendName
	require.NoError(t, iface.Validate())
}

func TestAmneziaWGInterfaceImportMergeAndRouting(t *testing.T) {
	options := AmneziaWGOptions{"H1": "100-200", "Jc": "4"}
	physical := &PhysicalInterface{ImportSource: ControllerTypeAmneziaWG}
	physical.SetExtras(AmneziaWGInterfaceExtras{Options: options})
	iface := ConvertPhysicalInterface(physical)
	require.Equal(t, options, iface.AmneziaWG)
	iface.AmneziaWG["Jc"] = "5"
	require.Equal(t, "4", options["Jc"], "imported settings must not mutate physical extras")
	iface.Backend = config.AmneziaWGBackendName
	iface.AmneziaWG = AmneziaWGOptions{"Jc": "6"}
	MergeToPhysicalInterface(physical, iface)
	require.Equal(t, AmneziaWGInterfaceExtras{Options: AmneziaWGOptions{"H1": "100-200", "Jc": "6"}}, physical.GetExtras())
	require.Equal(t, "4", options["Jc"], "merging must not mutate the previous extras snapshot")
	iface.RoutingTable = "12345"
	require.Equal(t, 12345, iface.GetRoutingTable())
	iface.RoutingTable = "0x1234"
	require.Equal(t, 0x1234, iface.GetRoutingTable())
}

func TestAmneziaWGInterfaceMergeInitializesExtras(t *testing.T) {
	for _, existing := range []any{nil, AmneziaWGInterfaceExtras{}} {
		physical := &PhysicalInterface{ImportSource: ControllerTypeAmneziaWG}
		if existing != nil {
			physical.SetExtras(existing)
		}
		iface := &Interface{Backend: config.AmneziaWGBackendName, AmneziaWG: AmneziaWGOptions{"Jc": "4"}}
		MergeToPhysicalInterface(physical, iface)
		require.Equal(t, AmneziaWGInterfaceExtras{Options: iface.AmneziaWG}, physical.GetExtras())
		iface.AmneziaWG["Jc"] = "6"
		require.Equal(t, "4", physical.GetExtras().(AmneziaWGInterfaceExtras).Options["Jc"])
	}
}
