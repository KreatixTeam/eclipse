package module

import (
	"sync"

	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Fly forces the client's flight abilities on by rewriting UpdateAbilities
// and legacy AdventureSettings packets headed to the client. Works on
// servers that trust the client's ability state; anticheat servers will
// rubber-band it.
type Fly struct {
	mu      sync.Mutex
	session *Session
	last    *protocol.AbilityData
}

func NewFly() *Fly { return &Fly{} }

func (f *Fly) Name() string { return "fly" }

func (f *Fly) OnJoin(s *Session) {
	f.mu.Lock()
	f.session = s
	f.last = nil
	f.mu.Unlock()
}

// OnToggle grants or revokes flight client-side the moment `.fly on|off` runs.
func (f *Fly) OnToggle(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.session == nil {
		return
	}
	data := f.last
	if data == nil {
		data = &protocol.AbilityData{
			Layers: []protocol.AbilityLayer{{
				Type:             protocol.AbilityLayerTypeBase,
				FlySpeed:         protocol.AbilityBaseFlySpeed,
				VerticalFlySpeed: protocol.AbilityBaseVerticalFlySpeed,
				WalkSpeed:        protocol.AbilityBaseWalkSpeed,
			}},
		}
	}
	patched := *data
	patched.Layers = make([]protocol.AbilityLayer, len(data.Layers))
	copy(patched.Layers, data.Layers)
	for i := range patched.Layers {
		if patched.Layers[i].Type != protocol.AbilityLayerTypeBase {
			continue
		}
		bits := uint32(protocol.AbilityMayFly) | uint32(protocol.AbilityFlySpeed) | uint32(protocol.AbilityVerticalFlySpeed)
		patched.Layers[i].Abilities |= bits
		if on {
			patched.Layers[i].Abilities |= uint32(protocol.AbilityFlying)
			patched.Layers[i].Values |= bits | uint32(protocol.AbilityFlying)
		} else {
			patched.Layers[i].Abilities &^= uint32(protocol.AbilityFlying)
			patched.Layers[i].Values &^= uint32(protocol.AbilityFlying)
			patched.Layers[i].Values |= bits
		}
	}
	_ = f.session.SendClient(&packet.UpdateAbilities{AbilityData: patched})
}

func (f *Fly) OnPacket(pk packet.Packet, dir Direction) (packet.Packet, bool) {
	if dir != ClientBound {
		return pk, true
	}
	switch pkt := pk.(type) {
	case *packet.UpdateAbilities:
		f.mu.Lock()
		f.last = &pkt.AbilityData
		on := f.session != nil && f.session.Registry.Enabled("fly")
		for i := range pkt.AbilityData.Layers {
			if pkt.AbilityData.Layers[i].Type != protocol.AbilityLayerTypeBase {
				continue
			}
			bits := uint32(protocol.AbilityMayFly) | uint32(protocol.AbilityFlySpeed) | uint32(protocol.AbilityVerticalFlySpeed)
			pkt.AbilityData.Layers[i].Abilities |= bits
			pkt.AbilityData.Layers[i].Values |= bits
			if on {
				pkt.AbilityData.Layers[i].Abilities |= uint32(protocol.AbilityFlying)
				pkt.AbilityData.Layers[i].Values |= uint32(protocol.AbilityFlying)
			}
		}
		f.mu.Unlock()
	case *packet.AdventureSettings:
		f.mu.Lock()
		on := f.session != nil && f.session.Registry.Enabled("fly")
		pkt.Flags |= packet.AdventureFlagAllowFlight
		if on {
			pkt.Flags |= packet.AdventureFlagFlying
		}
		f.mu.Unlock()
	}
	return pk, true
}
