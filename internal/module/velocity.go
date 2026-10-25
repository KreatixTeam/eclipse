package module

import (
	"sync"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Velocity (a.k.a. anti-knockback) zeroes SetActorMotion packets aimed at the
// client's own entity, so knockback and server-pushed movement never arrive.
type Velocity struct {
	mu     sync.RWMutex
	rid    uint64
	hasRid bool
}

func NewVelocity() *Velocity { return &Velocity{} }

func (v *Velocity) Name() string { return "velocity" }

func (v *Velocity) OnJoin(s *Session) {
	v.mu.Lock()
	v.hasRid = false
	v.mu.Unlock()
}

func (v *Velocity) OnPacket(pk packet.Packet, dir Direction) (packet.Packet, bool) {
	if dir != ClientBound {
		return pk, true
	}
	switch pkt := pk.(type) {
	case *packet.StartGame:
		v.mu.Lock()
		v.rid, v.hasRid = pkt.EntityRuntimeID, true
		v.mu.Unlock()
	case *packet.SetActorMotion:
		v.mu.RLock()
		own := v.hasRid && pkt.EntityRuntimeID == v.rid
		v.mu.RUnlock()
		if own {
			pkt.Velocity = [3]float32{}
		}
	}
	return pk, true
}
