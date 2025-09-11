package fanout

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	cfgpkg "github.com/fedor-git/wg-portal-2/internal/config"
)

// Мінімальний інтерфейс під ваш eventbus (eventbus.go)
type EventBus interface {
	Subscribe(topic string, fn interface{}) error
}

// Службові заголовки
const (
	hdrOrigin = "X-WGP-Origin"
	hdrNoEcho = "X-WGP-NoEcho"
	hdrCorrID = "X-WGP-Correlation-ID"
)

// внутрішні налаштування (копія потрібних полів із cfg.Core.Fanout)
type settings struct {
	Enabled     bool
	Peers       []string
	AuthHeader  string
	AuthValue   string
	Timeout     time.Duration
	Debounce    time.Duration
	SelfURL     string
	Origin      string
	KickOnStart bool
	Topics      []string
}

// Публічний запуск fanout (бере все з cfg.Core.Fanout)
func Start(ctx context.Context, bus EventBus, fc cfgpkg.FanoutConfig) {
	s := settings{
		Enabled:     fc.Enabled,
		Peers:       append([]string(nil), fc.Peers...),
		AuthHeader:  fc.AuthHeader,
		AuthValue:   fc.AuthValue,
		Timeout:     fc.Timeout,
		Debounce:    fc.Debounce,
		SelfURL:     fc.SelfURL,
		Origin:      fc.Origin,
		KickOnStart: fc.KickOnStart,
		Topics:      append([]string(nil), fc.Topics...),
	}

	if !s.Enabled {
		slog.Info("[FANOUT] disabled, skip init")
		return
	}
	if s.Timeout <= 0 {
		s.Timeout = 5 * time.Second
	}
	if s.Debounce <= 0 {
		s.Debounce = 250 * time.Millisecond
	}

	// best-effort origin
	if s.Origin == "" {
		if host := getHost(s.SelfURL); host != "" {
			if hn, _ := net.LookupAddr(host); len(hn) > 0 {
				s.Origin = strings.TrimSuffix(hn[0], ".")
			}
			if s.Origin == "" {
				s.Origin = host
			}
		}
		if s.Origin == "" {
			s.Origin = "unknown-node"
		}
	}

	// дефолтні теми
	if len(s.Topics) == 0 {
		s.Topics = []string{
			"peer:created", "peer:updated", "peer:deleted",
			"interface:created", "interface:updated", "interface:deleted",
		}
	}

	f := &fanout{
		cfg: s,
		client: &http.Client{
			Timeout: s.Timeout,
		},
		debounce: newDebouncer(s.Debounce),
	}

	// Підписки на конкретні топіки (НЕ wildcard)
	if bus != nil {
		for _, topic := range s.Topics {
			t := topic
			if err := bus.Subscribe(t, func() {
				slog.Debug("[FANOUT] bump", "reason", "bus:"+t)
				f.bump("bus:" + t)
			}); err != nil {
				slog.Warn("[FANOUT] subscribe failed", "topic", t, "err", err)
			} else {
				slog.Debug("[FANOUT] subscribed", "topic", t)
			}
		}
	} else {
		slog.Debug("[FANOUT] no bus provided, event-driven bumps disabled")
	}

	// Головний цикл
	go f.loop(ctx)

	// Перший «пинок» — уже після підписок, щоб одразу роздати sync, якщо треба
	if s.KickOnStart {
		f.bump("startup")
	}

	slog.Info("[FANOUT] initialized",
		"peers", len(s.Peers),
		"topics", s.Topics,
		"self", s.SelfURL,
		"origin", s.Origin,
		"debounce", s.Debounce.String(),
		"timeout", s.Timeout.String(),
	)
}

// ------------------ реалізація ------------------

type fanout struct {
	cfg      settings
	client   *http.Client
	debounce *debouncer
}

func (f *fanout) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-f.debounce.C:
			f.fire(ctx)
		}
	}
}

func (f *fanout) bump(reason string) {
	slog.Debug("[FANOUT] bump", "reason", reason)
	f.debounce.Bump()
}

func (f *fanout) fire(ctx context.Context) {
	var wg sync.WaitGroup

	for _, base := range f.cfg.Peers {
		base = strings.TrimSpace(base)
		if base == "" {
			continue
		}
		if isSelf(base, f.cfg.SelfURL) {
			continue
		}
		endpoint := strings.TrimRight(base, "/") + "/api/v1/peer/sync"

		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
			if err != nil {
				slog.Warn("[FANOUT] build request failed", "url", u, "err", err)
				return
			}

			// Auth (за потреби)
			if f.cfg.AuthHeader != "" && f.cfg.AuthValue != "" {
				req.Header.Set(f.cfg.AuthHeader, f.cfg.AuthValue)
			}

			// Службові заголовки
			req.Header.Set(hdrOrigin, f.cfg.Origin)
			req.Header.Set(hdrNoEcho, "1")
			req.Header.Set(hdrCorrID, uuid.NewString())

			resp, err := f.client.Do(req)
			if err != nil {
				slog.Warn("[FANOUT] sync failed", "url", u, "err", err)
				return
			}
			_ = resp.Body.Close()

			if resp.StatusCode >= 300 {
				slog.Warn("[FANOUT] sync non-2xx", "url", u, "status", resp.Status)
				return
			}
			slog.Debug("[FANOUT] sync ok", "url", u)
		}(endpoint)
	}

	wg.Wait()
}

// ------------------ утиліти ------------------

type debouncer struct {
	d   time.Duration
	tmu sync.Mutex
	t   *time.Timer
	C   chan struct{}
}

func newDebouncer(d time.Duration) *debouncer {
	return &debouncer{
		d: d,
		C: make(chan struct{}, 1),
	}
}

func (d *debouncer) Bump() {
	d.tmu.Lock()
	defer d.tmu.Unlock()

	if d.t == nil {
		d.t = time.AfterFunc(d.d, func() {
			select {
			case d.C <- struct{}{}:
			default:
			}
			d.reset()
		})
		return
	}

	// перезапуск
	if !d.t.Stop() {
		// якщо таймер уже майже «вистрілив», знімаємо імпульс з каналу
		select {
		case <-d.C:
		default:
		}
	}
	d.t.Reset(d.d)
}

func (d *debouncer) reset() {
	d.tmu.Lock()
	defer d.tmu.Unlock()
	d.t = nil
}

func isSelf(targetBase, selfBase string) bool {
	if selfBase == "" || targetBase == "" {
		return false
	}

	tu, terr := url.Parse(strings.TrimSpace(targetBase))
	su, serr := url.Parse(strings.TrimSpace(selfBase))
	if terr != nil || serr != nil {
		return strings.TrimRight(targetBase, "/") == strings.TrimRight(selfBase, "/")
	}

	ts := strings.ToLower(tu.Scheme)
	ss := strings.ToLower(su.Scheme)
	if ts == "" {
		ts = "http"
	}
	if ss == "" {
		ss = "http"
	}

	th := normalizeHostPort(tu.Host, ts)
	sh := normalizeHostPort(su.Host, ss)

	return ts == ss && th == sh
}

func normalizeHostPort(host, scheme string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return ""
	}
	if !strings.Contains(h, ":") {
		switch scheme {
		case "https":
			h += ":443"
		default:
			h += ":80"
		}
	}
	return h
}

func getHost(base string) string {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return ""
	}
	h := u.Host
	if i := strings.IndexByte(h, ':'); i >= 0 {
		h = h[:i]
	}
	return h
}
