// source taken from https://git.prolicht.digital/golib/healthcheck/-/blob/master/healthcheck.go

package healthcheck

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

type service struct {
	listenAddress string
	checkFunc     func() int
}

type Option func(svc *service)

// New creates a new healthcheck instance that can be started with either Start() or StartWithContext().
func New(opts ...Option) *service {
	svc := &service{
		listenAddress: ":11223",
		checkFunc: func() int {
			return http.StatusOK
		},
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// Start starts a background goroutine with the healthcheck webserver. This goroutine is only stopped
// if the whole program is shut down.
func (s *service) Start() {
	s.StartWithContext(context.Background())
}

// StartForeground starts a goroutine with the healthcheck webserver. This function will block until the context
// gets canceled or the healthcheck server crashes.
func (s *service) StartForeground(ctx context.Context) {
	router := http.NewServeMux()
	router.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(s.checkFunc())
	}))

	srv := &http.Server{
		Addr:         s.listenAddress,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	srvContext, cancelFn := context.WithCancel(ctx)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Printf("[HEALTHCHECK] web service on %s exited: %v\n", s.listenAddress, err)
			cancelFn()
		}
	}()

	// Wait for the main context to end, this call blocks
	<-srvContext.Done()

	// 1-second grace period
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	srv.SetKeepAlivesEnabled(false) // disable keep-alive kills idle connections
	_ = srv.Shutdown(shutdownCtx)

	fmt.Println("[HEALTHCHECK] web service stopped")
}

// StartWithContext starts a background goroutine with the healthcheck webserver. The goroutine will be
// stopped if the context gets canceled or the healthcheck server crashes.
func (s *service) StartWithContext(ctx context.Context) {
	go s.StartForeground(ctx)
}

// ListenOn allows to change the default listening address of ":11223".
func ListenOn(addr string) Option {
	return func(svc *service) {
		svc.listenAddress = addr
	}
}

// WithCustomCheck allows to use a custom check function. The integer return value of the check
// function is used as HTTP status code.
func WithCustomCheck(fnc func() int) Option {
	return func(svc *service) {
		if fnc != nil {
			svc.checkFunc = fnc
		}
	}
}

// ListenOnFromEnv sets the listening address to a value retrieved from the environment variable
// HC_LISTEN_ADDR.
// If the argument list is not empty, the  listening address value will be loaded from an
// environment variable with the name of the first list entry.
// If the environment variable was empty, the listening address will not be overridden.
func ListenOnFromEnv(envName ...string) Option {
	return func(svc *service) {
		varName := "HC_LISTEN_ADDR"
		if len(envName) > 0 {
			varName = envName[0]
		}

		listenAddr := os.Getenv(varName)
		if listenAddr != "" {
			svc.listenAddress = listenAddr
		}
	}
}
