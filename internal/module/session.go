package module

import (
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Session is a single client bridged through the proxy. Modules use it to
// send packets to the client or upstream server outside the normal flow.
type Session struct {
	// Client is the connection to the player's game.
	Client *minecraft.Conn
	// Server is the connection to the upstream server.
	Server *minecraft.Conn
	// Registry the session's modules live in.
	Registry *Registry
}

// SendClient writes a packet straight to the client, bypassing the pipeline.
func (s *Session) SendClient(pk packet.Packet) error {
	return s.Client.WritePacket(pk)
}

// SendServer writes a packet straight to the upstream server.
func (s *Session) SendServer(pk packet.Packet) error {
	return s.Server.WritePacket(pk)
}

// Chat sends a system chat message that only this client sees.
func (s *Session) Chat(msg string) error {
	return s.SendClient(&packet.Text{
		TextType:   packet.TextTypeSystem,
		SourceName: "Eclipse",
		Message:    msg,
	})
}

// JoinAware is implemented by modules that need a hook when a client joins.
type JoinAware interface {
	OnJoin(s *Session)
}

// ToggleAware is implemented by modules that react to being switched on/off.
type ToggleAware interface {
	OnToggle(on bool)
}
