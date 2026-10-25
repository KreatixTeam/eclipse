package proxy

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sandertv/gophertunnel/minecraft"

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
}

// Proxy accepts Bedrock clients and bridges each connection to an upstream
// server, letting modules inspect and modify packets in flight.
type Proxy struct {
	cfg      Config
	listener *minecraft.Listener
	log      *slog.Logger
}

func New(cfg Config) (*Proxy, error) {
	if cfg.Modules == nil {
		cfg.Modules = module.NewRegistry()
	}
	if cfg.Name == "" {
		cfg.Name = "Eclipse"
	}
	l, err := minecraft.ListenConfig{
		StatusProvider:         minecraft.NewStatusProvider(cfg.Name, "Eclipse Proxy"),
		AuthenticationDisabled: cfg.Offline,
		AllowUnknownPackets:    true,
	}.Listen("raknet", cfg.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", cfg.ListenAddr, err)
	}
	return &Proxy{cfg: cfg, listener: l, log: slog.Default()}, nil
}

// Run accepts connections until the listener is closed.
func (p *Proxy) Run() error {
	p.log.Info("eclipse proxy listening", "addr", p.cfg.ListenAddr, "upstream", p.cfg.ServerAddr)
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		go p.handle(conn.(*minecraft.Conn))
	}
}

// Close stops accepting new connections.
func (p *Proxy) Close() { p.listener.Close() }

func (p *Proxy) handle(client *minecraft.Conn) {
	remote := client.RemoteAddr().String()

	server, err := p.dialUpstream(context.Background())
	if err != nil {
		p.log.Error("dial upstream", "client", remote, "err", err)
		client.Close()
		return
	}
	p.log.Info("client connected", "client", remote, "upstream", p.cfg.ServerAddr)

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
func (p *Proxy) dialUpstream(ctx context.Context) (*minecraft.Conn, error) {
	d := minecraft.Dialer{}
	if !p.cfg.Offline {
		src, err := tokenSource(ctx)
		if err != nil {
			return nil, err
		}
		d.TokenSource = src
	}
	conn, err := d.DialContext(ctx, "raknet", p.cfg.ServerAddr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", p.cfg.ServerAddr, err)
	}
	return conn, nil
}
