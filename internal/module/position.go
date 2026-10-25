package module

import (
	"fmt"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Position tracks the client's own position from clientbound movement packets
// and exposes it through the `.pos` chat command.
type Position struct {
	mu     sync.RWMutex
	rid    uint64
	hasRid bool
	pos    [3]float32
	hasPos bool
}

func NewPosition() *Position { return &Position{} }

func (p *Position) Name() string { return "position" }

func (p *Position) OnJoin(s *Session) {
	p.mu.Lock()
	p.hasRid, p.hasPos = false, false
	p.mu.Unlock()
	if c, ok := s.Registry.Get("commands").(*Commands); ok {
		c.RegisterCommand("pos", "print your coordinates", func(args []string) {
			p.mu.RLock()
			defer p.mu.RUnlock()
			if !p.hasPos {
				s.Chat("no position yet")
				return
			}
			s.Chat(fmt.Sprintf("%.1f, %.1f, %.1f", p.pos[0], p.pos[1], p.pos[2]))
		})
	}
}

func (p *Position) OnPacket(pk packet.Packet, dir Direction) (packet.Packet, bool) {
	if dir != ClientBound {
		return pk, true
	}
	switch pkt := pk.(type) {
	case *packet.StartGame:
		p.mu.Lock()
		p.rid, p.hasRid = pkt.EntityRuntimeID, true
		p.mu.Unlock()
	case *packet.MovePlayer:
		p.mu.Lock()
		if p.hasRid && pkt.EntityRuntimeID == p.rid {
			p.pos, p.hasPos = pkt.Position, true
		}
		p.mu.Unlock()
	}
	return pk, true
}
