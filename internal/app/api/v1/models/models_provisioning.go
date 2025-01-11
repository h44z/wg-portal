package models

import "github.com/h44z/wg-portal/internal/domain"

// UserInformation represents the information about a user and its linked peers.
type UserInformation struct {
	UserIdentifier string                `json:"UserIdentifier" example:"uid-1234567"`
	PeerCount      int                   `json:"PeerCount" example:"2"`
	Peers          []UserInformationPeer `json:"Peers"`
}

// UserInformationPeer represents the information about a peer.
type UserInformationPeer struct {
	// Identifier is the unique identifier of the peer. It equals the public key of the peer.
	Identifier string `json:"Identifier" example:"peer-1234567"`
	// DisplayName is a user-defined description of the peer.
	DisplayName string `json:"DisplayName" example:"My iPhone"`
	// IPAddresses is a list of IP addresses in CIDR format assigned to the peer.
	IpAddresses []string `json:"IpAddresses" example:"10.11.12.2/24"`
	// IsDisabled is a flag that specifies if the peer is enabled or not. Disabled peers are not able to connect.
	IsDisabled bool `json:"IsDisabled,omitempty" example:"true"`

	// InterfaceIdentifier is the unique identifier of the WireGuard Portal device the peer is connected to.
	InterfaceIdentifier string `json:"InterfaceIdentifier" example:"wg0"`
}

func NewUserInformation(user *domain.User, peers []domain.Peer) *UserInformation {
	if user == nil {
		return &UserInformation{}
	}

	ui := &UserInformation{
		UserIdentifier: string(user.Identifier),
		PeerCount:      len(peers),
	}

	for _, peer := range peers {
		ui.Peers = append(ui.Peers, NewUserInformationPeer(peer))
	}

	if len(ui.Peers) == 0 {
		ui.Peers = []UserInformationPeer{} // Ensure that the JSON output is an empty array instead of null.
	}

	return ui
}

func NewUserInformationPeer(peer domain.Peer) UserInformationPeer {
	up := UserInformationPeer{
		Identifier:          string(peer.Identifier),
		DisplayName:         peer.DisplayName,
		IpAddresses:         domain.CidrsToStringSlice(peer.Interface.Addresses),
		IsDisabled:          peer.IsDisabled(),
		InterfaceIdentifier: string(peer.InterfaceIdentifier),
	}

	return up
}
