package module

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// Dump writes every packet that flows through the proxy to a file as JSON
// lines — packet type, direction, timestamp and full fields.
type Dump struct {
	mu sync.Mutex
	w  *os.File
}

// NewDump opens path for appending packet logs.
func NewDump(path string) (*Dump, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Dump{w: f}, nil
}

func (d *Dump) Name() string { return "dump" }

func (d *Dump) OnPacket(pk packet.Packet, dir Direction) (packet.Packet, bool) {
	t := reflect.TypeOf(pk)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	dr := "down"
	if dir == ServerBound {
		dr = "up"
	}
	fields, _ := json.Marshal(pk)
	d.mu.Lock()
	fmt.Fprintf(d.w, `{"t":%q,"dir":%q,"pkt":%q,"fields":%s}`+"\n",
		time.Now().Format(time.RFC3339Nano), dr, t.Name(), fields)
	d.mu.Unlock()
	return pk, true
}

// Close flushes and closes the log file.
func (d *Dump) Close() error { return d.w.Close() }
