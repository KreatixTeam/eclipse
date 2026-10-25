// Package hub runs hosted Eclipse: players sign in with their own Microsoft
// account through the device-code flow, add servers, and get a personal proxy
// endpoint (host:port) to join from any Bedrock client.
package hub

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft/auth"
	"golang.org/x/oauth2"

	"github.com/KreatixTeam/eclipse/internal/module"
	"github.com/KreatixTeam/eclipse/internal/proxy"
)

// Account is a signed-in player with their upstream token and servers.
type Account struct {
	ID       string        `json:"id"`
	Gamertag string        `json:"gamertag,omitempty"`
	Token    *oauth2.Token `json:"token"`
	Servers  []*Server     `json:"servers"`
}

// Server is one upstream a player proxies through.
type Server struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Port    int    `json:"port"`
	Running bool   `json:"running"`
}

// Flow is an in-flight Microsoft device-code login.
type Flow struct {
	ID   string
	Code string
	URI  string
	Done bool
	Err  string
	Acct *Account
}

// Hub owns accounts, device-code flows and spawned proxy listeners.
type Hub struct {
	dir      string
	portFrom int
	portTo   int

	mu       sync.Mutex
	accounts map[string]*Account
	flows    map[string]*Flow
	proxies  map[string]*proxy.Proxy // serverID -> running proxy
	log      *slog.Logger
}

func New(dir string, portFrom, portTo int) (*Hub, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	h := &Hub{
		dir:      dir,
		portFrom: portFrom,
		portTo:   portTo,
		accounts: map[string]*Account{},
		flows:    map[string]*Flow{},
		proxies:  map[string]*proxy.Proxy{},
		log:      slog.Default(),
	}
	if err := h.load(); err != nil {
		return nil, err
	}
	return h, nil
}

func randID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- device-code login ---

// StartLogin begins a Microsoft device-code flow and returns the code and URL
// the player must open. Poll LoginStatus until Done.
func (h *Hub) StartLogin(ctx context.Context) (*Flow, error) {
	d, err := auth.AndroidConfig.DeviceAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("device auth: %w", err)
	}
	f := &Flow{ID: randID(), Code: d.UserCode, URI: d.VerificationURI}
	h.mu.Lock()
	h.flows[f.ID] = f
	h.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		tok, err := auth.AndroidConfig.DeviceAccessToken(ctx, d)
		h.mu.Lock()
		defer h.mu.Unlock()
		f.Done = true
		if err != nil {
			f.Err = err.Error()
			return
		}
		acct := &Account{ID: randID(), Token: tok}
		h.accounts[acct.ID] = acct
		f.Acct = acct
		_ = h.saveLocked(acct)
	}()
	return f, nil
}

// LoginStatus reports a flow's progress.
func (h *Hub) LoginStatus(id string) (*Flow, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	f, ok := h.flows[id]
	return f, ok
}

// Get returns an account or nil.
func (h *Hub) Get(id string) *Account {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.accounts[id]
}

// --- servers ---

// AddServer registers an upstream for an account, allocates a public port and
// starts a proxy on it.
func (h *Hub) AddServer(acct *Account, name, addr string) (*Server, error) {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr += ":19132"
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	port, err := h.allocPortLocked()
	if err != nil {
		return nil, err
	}
	s := &Server{ID: randID(), Name: name, Addr: addr, Port: port}
	acct.Servers = append(acct.Servers, s)
	if err := h.startLocked(acct, s); err != nil {
		acct.Servers = acct.Servers[:len(acct.Servers)-1]
		return nil, err
	}
	return s, h.saveLocked(acct)
}

// RemoveServer stops and deletes a server entry.
func (h *Hub) RemoveServer(acct *Account, id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, s := range acct.Servers {
		if s.ID != id {
			continue
		}
		h.stopLocked(s)
		acct.Servers = append(acct.Servers[:i], acct.Servers[i+1:]...)
		_ = h.saveLocked(acct)
		return true
	}
	return false
}

// ToggleServer starts/stops a server's proxy.
func (h *Hub) ToggleServer(acct *Account, id string, on bool) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range acct.Servers {
		if s.ID != id {
			continue
		}
		if on && !s.Running {
			if err := h.startLocked(acct, s); err != nil {
				h.log.Error("start proxy", "server", s.ID, "err", err)
				return false
			}
		} else if !on && s.Running {
			h.stopLocked(s)
		}
		_ = h.saveLocked(acct)
		return true
	}
	return false
}

func (h *Hub) allocPortLocked() (int, error) {
	used := map[int]bool{}
	for _, a := range h.accounts {
		for _, s := range a.Servers {
			used[s.Port] = true
		}
	}
	for p := h.portFrom; p <= h.portTo; p++ {
		if !used[p] {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free ports in %d-%d", h.portFrom, h.portTo)
}

// defaultModules is the shared module set every hosted proxy gets.
func defaultModules() *module.Registry {
	mods := module.NewRegistry()
	mods.Register(module.NewCommands())
	mods.Register(module.NewPosition())
	mods.Register(module.NewVelocity())
	mods.Register(module.NewFly())
	mods.Toggle("velocity", false)
	mods.Toggle("fly", false)
	return mods
}

func (h *Hub) startLocked(acct *Account, s *Server) error {
	p, err := proxy.New(proxy.Config{
		ListenAddr:  fmt.Sprintf(":%d", s.Port),
		ServerAddr:  s.Addr,
		Name:        "Eclipse · " + s.Name,
		Modules:     defaultModules(),
		TokenSource: auth.RefreshTokenSource(acct.Token),
	})
	if err != nil {
		return err
	}
	go func() {
		if err := p.Run(); err != nil {
			h.log.Error("proxy exited", "server", s.ID, "err", err)
		}
	}()
	h.proxies[s.ID] = p
	s.Running = true
	return nil
}

func (h *Hub) stopLocked(s *Server) {
	if p, ok := h.proxies[s.ID]; ok {
		p.Close()
		delete(h.proxies, s.ID)
	}
	s.Running = false
}

// --- persistence ---

func (h *Hub) path(id string) string {
	return filepath.Join(h.dir, "accounts", id+".json")
}

func (h *Hub) saveLocked(a *Account) error {
	if err := os.MkdirAll(filepath.Join(h.dir, "accounts"), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return os.WriteFile(h.path(a.ID), data, 0o600)
}

func (h *Hub) load() error {
	entries, err := os.ReadDir(filepath.Join(h.dir, "accounts"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(h.dir, "accounts", e.Name()))
		if err != nil {
			continue
		}
		a := &Account{}
		if json.Unmarshal(data, a) != nil || a.Token == nil {
			continue
		}
		h.accounts[a.ID] = a
		// Restart proxies for servers that were running.
		for _, s := range a.Servers {
			if s.Running {
				if err := h.startLocked(a, s); err != nil {
					h.log.Error("restart proxy", "server", s.ID, "err", err)
					s.Running = false
				}
			}
		}
	}
	return nil
}
