package portal

import "github.com/h44z/wg-portal/internal/wireguard"

var man wireguard.Manager

func init() {
	man, _ = wireguard.NewPersistentManager(nil, nil, nil)
}
