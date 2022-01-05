package oauth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
)

type State interface {
	IsValid(remoteAddr string) bool
	LoginURL() string
}

type state struct {
	expireAt   time.Time
	remoteAddr string
	loginURL   string
}

func (s state) IsValid(remoteAddr string) bool {
	if time.Now().After(s.expireAt) {
		return false
	}

	oParts := strings.Split(s.remoteAddr, ":")
	nParts := strings.Split(remoteAddr, ":")

	return oParts[0] == nParts[0]
}

func (s state) LoginURL() string {
	return s.loginURL
}

const (
	stateTTL = time.Minute * 5
)

type StateManager struct {
	states map[string]state
	mu     sync.RWMutex
}

var (
	once     sync.Once
	instance StateManager
)

func GetStateManager(ctx context.Context) *StateManager {
	once.Do(func() {
		instance.states = make(map[string]state)
		go instance.stateCleaner(ctx)
	})

	return &instance
}

func (sm *StateManager) NewState(remoteAddr, loginURL string) (string, error) {
	id, err := uuid.DefaultGenerator.NewV4()
	if err != nil {
		return "", fmt.Errorf("cannot generate oauth code: %s", err)
	}

	sm.mu.Lock()
	sm.states[id.String()] = state{
		expireAt:   time.Now().Add(stateTTL),
		remoteAddr: remoteAddr,
		loginURL:   loginURL,
	}
	sm.mu.Unlock()

	return id.String(), nil
}

func (sm *StateManager) GetState(state string) (State, error) {
	sm.mu.RLock()
	s, ok := sm.states[state]
	sm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("specified state not found: %s", state)
	}

	return s, nil
}

func (sm *StateManager) DeleteState(s string) {
	sm.mu.Lock()
	delete(sm.states, s)
	sm.mu.Unlock()
}

func (sm *StateManager) stateCleaner(ctx context.Context) {
	t := time.NewTimer(stateTTL)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sm.mu.Lock()
			for i := range sm.states {
				if time.Now().After(sm.states[i].expireAt) {
					delete(sm.states, i)
				}
			}
			sm.mu.Unlock()
		}
	}
}
