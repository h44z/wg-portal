package wireguard

import "context"

// Опціональна можливість для бекендів, які підтримують повне очищення списку peer'ів.
type SupportsClearPeers interface {
    ClearPeers(ctx context.Context, iface string) error
}