package module

import (
	"fmt"
	"io"
	"reflect"
	"sync"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// PacketLogger logs the name of every packet that flows through the proxy.
type PacketLogger struct {
	mu sync.Mutex
	w  io.Writer
}

func NewPacketLogger(w io.Writer) *PacketLogger {
	return &PacketLogger{w: w}
}

func (l *PacketLogger) Name() string { return "packet-logger" }

func (l *PacketLogger) OnPacket(pk packet.Packet, dir Direction) (packet.Packet, bool) {
	arrow := "S->C"
	if dir == ServerBound {
		arrow = "C->S"
	}
	t := reflect.TypeOf(pk)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	l.mu.Lock()
	fmt.Fprintf(l.w, "%s %s\n", arrow, t.Name())
	l.mu.Unlock()
	return pk, true
}
