package module

import (
	"sync"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Direction a packet is travelling through the proxy.
type Direction int

const (
	// ClientBound is a packet flowing from the upstream server to the client.
	ClientBound Direction = iota
	// ServerBound is a packet flowing from the client to the upstream server.
	ServerBound
)

// Module is a single proxy feature. Modules observe and optionally mutate or
// drop packets in either direction.
type Module interface {
	// Name identifies the module in the registry and config.
	Name() string

	// OnPacket is called for every packet in both directions while the module
	// is enabled. Returning a non-nil packet replaces the packet being
	// forwarded; returning nil, false drops it.
	OnPacket(pk packet.Packet, dir Direction) (packet.Packet, bool)
}

// Registry holds the active modules and applies them to every packet.
type Registry struct {
	mu   sync.RWMutex
	mods map[string]Module
	on   map[string]bool
}

func NewRegistry() *Registry {
	return &Registry{mods: map[string]Module{}, on: map[string]bool{}}
}

func (r *Registry) Register(m Module) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mods[m.Name()] = m
	r.on[m.Name()] = true
}

// Toggle enables or disables a module by name.
func (r *Registry) Toggle(name string, enabled bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.mods[name]; !ok {
		return false
	}
	r.on[name] = enabled
	return true
}

// Names lists registered module names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.mods))
	for n := range r.mods {
		names = append(names, n)
	}
	return names
}

// Apply runs a packet through every enabled module. It returns the packet to
// forward, or false if a module dropped it.
func (r *Registry) Apply(pk packet.Packet, dir Direction) (packet.Packet, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, m := range r.mods {
		if !r.on[name] {
			continue
		}
		out, ok := m.OnPacket(pk, dir)
		if !ok {
			return nil, false
		}
		pk = out
	}
	return pk, true
}
