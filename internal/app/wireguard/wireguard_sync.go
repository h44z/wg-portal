package wireguard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/fedor-git/wg-portal-2/internal/app"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type peerLister interface {
	GetAllPeers(ctx context.Context) ([]domain.Peer, error)
}

func (m Manager) SyncAllPeersFromDB(ctx context.Context) (int, error) {
    if err := domain.ValidateAdminAccessRights(ctx); err != nil {
        return 0, err
    }
    if m.db == nil { return 0, fmt.Errorf("db repo is nil") }
    if m.wg == nil { return 0, fmt.Errorf("wg controller is nil") }

    ifaces, err := m.db.GetAllInterfaces(ctx)
    if err != nil {
        return 0, fmt.Errorf("list interfaces: %w", err)
    }

    applied := 0
    for _, in := range ifaces {
        // 1) за потреби відновили/привели інтерфейс у консистентний стан
        if err := m.RestoreInterfaceState(ctx, true, in.Identifier); err != nil {
            slog.ErrorContext(ctx, "restore interface state failed", "iface", in.Identifier, "err", err)
            continue
        }

        // 2) дістали бажаний список пірів з БД (фільтруємо disabled)
        peers, err := m.db.GetInterfacePeers(ctx, in.Identifier)
        if err != nil {
            slog.ErrorContext(ctx, "peer sync: failed to load peers", "iface", in.Identifier, "err", err)
            continue
        }
        if len(peers) == 0 {
            // або ReplacePeers=true з пустим списком, або спеціальний ClearPeers
            if err := m.wg.ClearPeers(ctx, string(in.Identifier)); err != nil {
                slog.ErrorContext(ctx, "clear peers failed", "iface", in.Identifier, "err", err)
            }
            continue
        }
        desired := make([]domain.Peer, 0, len(peers))
        for i := range peers {
            if !peers[i].IsDisabled() {
                desired = append(desired, peers[i])
            }
        }

        // 3) ЗАСТОСОВУЄМО ПОВНУ ЗАМІНУ (ключове!)
        if err := m.replacePeers(ctx, in.Identifier, desired); err != nil {
            // якщо інтерфейсу не існує/файл відсутній – пробуємо ще раз після restore
            if isNoSuchFile(err) {
                slog.WarnContext(ctx, "replacePeers failed (no iface/file), restoring and retrying",
                    "iface", in.Identifier, "err", err)
                if rErr := m.RestoreInterfaceState(ctx, true, in.Identifier); rErr != nil {
                    slog.ErrorContext(ctx, "retry restore interface failed", "iface", in.Identifier, "err", rErr)
                    continue
                }
                if r2 := m.replacePeers(ctx, in.Identifier, desired); r2 != nil {
                    slog.ErrorContext(ctx, "replacePeers retry failed", "iface", in.Identifier, "err", r2)
                    continue
                }
            } else {
                slog.ErrorContext(ctx, "replacePeers failed", "iface", in.Identifier, "err", err)
                continue
            }
        }

        applied += len(desired)
    }

    return applied, nil
}

// replacePeers робить повну заміну складу peer-ів на інтерфейсі.
// Усередині має викликати бекенд з ReplacePeers=true.
// Реалізацію підженете під ваш controller (wgctrl, локальний тощо).
func (m Manager) replacePeers(ctx context.Context, iface domain.InterfaceIdentifier, peers []domain.Peer) error {
    // ВАРІАНТ A: якщо контролер уміє "Replace" напряму:
    // return m.wg.ReplacePeers(ctx, string(iface), peers)

    // ВАРІАНТ B: якщо є низькорівневий доступ до wgctrl:
    //   - зібрати []wgtypes.PeerConfig з domain.Peer
    //   - викликати ConfigureDevice(..., wgtypes.Config{ReplacePeers: true, Peers: pcs})
    //
    // ВАРІАНТ C (fallback, якщо немає Replace API):
    //   - спочатку "очистити" пірів (ReplacePeers: true, Peers: nil)
    //   - потім додати кожного з desired через існуючий m.savePeers(ctx, &p)

    // Нижче – універсальний fallback «очистити і додати»:
    if err := m.clearPeers(ctx, iface); err != nil {
        return err
    }
    for i := range peers {
        if err := m.savePeers(ctx, &peers[i]); err != nil {
            return fmt.Errorf("add peer %s on %s: %w", peers[i].Identifier, iface, err)
        }
        // ВАЖЛИВО: під час sync не публікуємо події, аби не ловити шторм fanout
        // (перенесіть publish із savePeers в той шар, де є user-driven зміни).
    }
    return nil
}

func (m Manager) clearPeers(ctx context.Context, iface domain.InterfaceIdentifier) error {
	return m.wg.ClearPeers(ctx, string(iface))
}

// func (m Manager) applyPeers(ctx context.Context, peers []domain.Peer) error {
// 	var firstErr error
// 	for i := range peers {
// 		p := &peers[i]
// 		if p.IsDisabled() {
// 			continue
// 		}
// 		if err := m.savePeers(ctx, p); err != nil {
// 			if firstErr == nil {
// 				firstErr = fmt.Errorf("apply peer %s (iface %s): %w",
// 					p.Identifier, p.InterfaceIdentifier, err)
// 			}
// 			continue
// 		}
// 		m.bus.Publish(app.TopicPeerUpdated, *p)
// 	}
// 	return firstErr
// }

func (m Manager) applyPeers(ctx context.Context, peers []domain.Peer) error {
    var firstErr error
    for i := range peers {
        p := &peers[i]
        if p.IsDisabled() { continue }
        if err := m.savePeers(ctx, p); err != nil {
            if firstErr == nil {
                firstErr = fmt.Errorf("apply peer %s (iface %s): %w", p.Identifier, p.InterfaceIdentifier, err)
            }
            continue
        }
        // <-- тут головне
        if !app.NoFanout(ctx) {
            m.bus.Publish(app.TopicPeerUpdated)
        }
    }
    return firstErr
}

func isNoSuchFile(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "file does not exist")
}