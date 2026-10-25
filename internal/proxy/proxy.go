package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/login"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"golang.org/x/oauth2"

	"github.com/KreatixTeam/eclipse/internal/module"
)

// Config configures a Proxy.
type Config struct {
	// ListenAddr is the address clients connect to, e.g. ":19132".
	ListenAddr string
	// ServerAddr is the upstream Bedrock server packets are forwarded to.
	ServerAddr string
	// Name is advertised as the server name on LAN pings.
	Name string
	// Offline disables Xbox Live authentication towards the client and the
	// upstream server. Only usable with servers that have auth disabled.
	Offline bool
	// Modules receives every packet for inspection/mutation.
	Modules *module.Registry
	// Hooks, if set, receive player join/leave and lifecycle callbacks
	// (e.g. the web dashboard).
	Hooks Hooks
	// TokenSource overrides Xbox Live auth for upstream dials (hosted mode:
	// each player's own Microsoft account). Nil falls back to the
	// operator's cached token.
	TokenSource oauth2.TokenSource
	// PackCacheDir stores the upstream's resource packs on disk so joins
	// don't re-download them. Empty keeps them in memory only.
	PackCacheDir string
}

// Hooks are optional callbacks fired on proxy events.
type Hooks interface {
	PlayerJoin(addr string)
	PlayerLeave(addr string)
}

// Proxy accepts Bedrock clients and bridges each connection to an upstream
// server, letting modules inspect and modify packets in flight.
type Proxy struct {
	cfg      Config
	listener *minecraft.Listener
	log      *slog.Logger
	mu       sync.Mutex
	clients  map[string]struct{}

	cache         minecraft.ResourcePackCache
	upstreamPacks []*resource.Pack
	done          chan struct{}
	closeOnce     sync.Once
}

func New(cfg Config) (*Proxy, error) {
	if cfg.Modules == nil {
		cfg.Modules = module.NewRegistry()
	}
	if cfg.Name == "" {
		cfg.Name = "Eclipse"
	}
	p := &Proxy{cfg: cfg, log: slog.Default(), clients: map[string]struct{}{}, done: make(chan struct{})}
	if cfg.PackCacheDir != "" {
		p.cache = minecraft.DirResourcePackCache{Dir: cfg.PackCacheDir}
	} else {
		p.cache = &memPackCache{m: map[minecraft.ResourcePackCacheKey]*resource.Pack{}}
	}
	l, err := minecraft.ListenConfig{
		StatusProvider:         minecraft.NewStatusProvider(cfg.Name, "Eclipse Proxy"),
		AuthenticationDisabled: cfg.Offline,
		AllowUnknownPackets:    true,
		FetchResourcePacks: func(login.IdentityData, login.ClientData, []*resource.Pack) []*resource.Pack {
			return p.packs()
		},
	}.Listen("raknet", cfg.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}
	p.listener = l
	return p, nil
}

// packs returns the upstream's resource packs, once prefetched.
func (p *Proxy) packs() []*resource.Pack {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.upstreamPacks
}

// prefetchPacks logs into the upstream once to download its resource packs,
// which can take minutes on big servers. Client joins never wait on this:
// they only reuse packs that are already cached and skip the rest.
func (p *Proxy) prefetchPacks() {
	// Servers like Hive drop slow pack downloads after a few seconds; every
	// attempt keeps what it got in the cache, so retrying makes progress.
	for attempt := 1; attempt <= 20; attempt++ {
		if p.prefetchOnce() {
			return
		}
		select {
		case <-p.done:
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (p *Proxy) prefetchOnce() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	start := time.Now()
	conn, err := p.dialUpstream(ctx, nil)
	if err != nil {
		p.log.Warn("prefetch packs", "upstream", p.cfg.ServerAddr, "err", err)
		return false
	}
	packs := conn.ResourcePacks()
	conn.Close()
	p.mu.Lock()
	p.upstreamPacks = packs
	p.mu.Unlock()
	p.log.Info("prefetched packs", "upstream", p.cfg.ServerAddr, "packs", len(packs), "took", time.Since(start).Round(time.Millisecond))
	return true
}

// havePack reports whether a pack is already prefetched, so a join can load
// it from cache instead of downloading it.
func (p *Proxy) havePack(id uuid.UUID, version string) bool {
	for _, pk := range p.packs() {
		if pk.UUID() == id && pk.Version() == version {
			return true
		}
	}
	return false
}

type memPackCache struct {
	mu sync.Mutex
	m  map[minecraft.ResourcePackCacheKey]*resource.Pack
}

func (c *memPackCache) Load(_ context.Context, k minecraft.ResourcePackCacheKey) (*resource.Pack, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[k], nil
}

func (c *memPackCache) Store(_ context.Context, k minecraft.ResourcePackCacheKey, pk *resource.Pack) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = pk
	return nil
}

// Run accepts connections until the listener is closed.
func (p *Proxy) Run() error {
	p.log.Info("eclipse proxy listening", "addr", p.cfg.ListenAddr, "upstream", p.cfg.ServerAddr)
	go p.prefetchPacks()
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		go p.handle(conn.(*minecraft.Conn))
	}
}

// Close stops accepting new connections.
func (p *Proxy) Close() {
	p.closeOnce.Do(func() { close(p.done) })
	p.listener.Close()
}

// SetHooks installs event callbacks after construction.
func (p *Proxy) SetHooks(h Hooks) { p.cfg.Hooks = h }

// ListenAddr/ServerAddr describe the configured route, for the dashboard.
func (p *Proxy) ListenAddr() string { return p.cfg.ListenAddr }
func (p *Proxy) ServerAddr() string { return p.cfg.ServerAddr }

// Clients lists the addresses of currently connected players.
func (p *Proxy) Clients() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0, len(p.clients))
	for a := range p.clients {
		out = append(out, a)
	}
	return out
}

func (p *Proxy) handle(client *minecraft.Conn) {
	remote := client.RemoteAddr().String()
	p.mu.Lock()
	p.clients[remote] = struct{}{}
	p.mu.Unlock()
	if p.cfg.Hooks != nil {
		p.cfg.Hooks.PlayerJoin(remote)
	}
	defer func() {
		p.mu.Lock()
		delete(p.clients, remote)
		p.mu.Unlock()
		if p.cfg.Hooks != nil {
			p.cfg.Hooks.PlayerLeave(remote)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	server, err := p.dialUpstream(ctx, client)
	if err != nil {
		p.log.Error("dial upstream", "client", remote, "err", err)
		_ = client.WritePacket(&packet.Disconnect{Message: "Eclipse couldn't reach " + p.cfg.ServerAddr + "\n" + err.Error()})
		client.Close()
		return
	}
	// Finish the handshake on both legs: the client gets the upstream's
	// StartGame data while we spawn into the upstream world.
	var spawnErr, startErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); startErr = client.StartGameContext(ctx, server.GameData()) }()
	go func() { defer wg.Done(); spawnErr = server.DoSpawnContext(ctx) }()
	wg.Wait()
	if startErr != nil || spawnErr != nil {
		p.log.Error("handshake", "client", remote, "start", startErr, "spawn", spawnErr)
		_ = client.WritePacket(&packet.Disconnect{Message: "Eclipse couldn't join " + p.cfg.ServerAddr})
		client.Close()
		server.Close()
		return
	}
	p.log.Info("client connected", "client", remote, "upstream", p.cfg.ServerAddr)

	session := &module.Session{Client: client, Server: server, Registry: p.cfg.Modules}
	p.cfg.Modules.NotifyJoin(session)

	// Bridge: forward every packet in both directions through the modules.
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		for {
			pk, err := client.ReadPacket()
			if err != nil {
				return
			}
			out, ok := p.cfg.Modules.Apply(pk, module.ServerBound)
			if !ok {
				continue
			}
			if err := server.WritePacket(out); err != nil {
				return
			}
		}
	}()
	for {
		pk, err := server.ReadPacket()
		if err != nil {
			break
		}
		out, ok := p.cfg.Modules.Apply(pk, module.ClientBound)
		if !ok {
			continue
		}
		if err := client.WritePacket(out); err != nil {
			break
		}
	}
	client.Close()
	server.Close()
	<-clientDone
	p.log.Info("client disconnected", "client", remote)
}

// dialUpstream connects to the configured server. In online mode the operator
// signs the proxy into Xbox Live once via a device-code flow; the token is
// cached on disk and refreshed automatically.
func (p *Proxy) dialUpstream(ctx context.Context, client *minecraft.Conn) (*minecraft.Conn, error) {
	d := minecraft.Dialer{ResourcePackCache: p.cache}
	if client != nil {
		d.ClientData = client.ClientData()
		if p.cfg.Offline {
			d.IdentityData = client.IdentityData()
		}
		d.DownloadResourcePack = func(id uuid.UUID, version string, _, _ int) bool {
			return p.havePack(id, version)
		}
	}
	if !p.cfg.Offline {
		src := p.cfg.TokenSource
		if src == nil {
			var err error
			src, err = tokenSource(ctx)
			if err != nil {
				return nil, err
			}
		}
		d.TokenSource = src
	}
	conn, err := d.DialContext(ctx, "raknet", p.cfg.ServerAddr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", p.cfg.ServerAddr, err)
	}
	return conn, nil
}
