package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

// TestBridge runs client -> proxy -> fake server and checks that the client
// spawns and receives a packet the server sends after spawn.
func TestBridge(t *testing.T) {
	srv, err := minecraft.ListenConfig{AuthenticationDisabled: true}.Listen("raknet", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go func() {
		c, err := srv.Accept()
		if err != nil {
			return
		}
		conn := c.(*minecraft.Conn)
		if err := conn.StartGame(minecraft.GameData{WorldName: "upstream"}); err != nil {
			t.Error("server start game:", err)
			return
		}
		_ = conn.WritePacket(&packet.Text{TextType: packet.TextTypeRaw, Message: "hello-from-upstream"})
		time.Sleep(3 * time.Second)
		conn.Close()
	}()

	p, err := New(Config{ListenAddr: "127.0.0.1:0", ServerAddr: srv.Addr().String(), Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	go p.Run()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := minecraft.Dialer{}.DialContext(ctx, "raknet", p.listener.Addr().String())
	if err != nil {
		t.Fatal("dial proxy:", err)
	}
	defer client.Close()
	if err := client.DoSpawnContext(ctx); err != nil {
		t.Fatal("spawn:", err)
	}
	if got := client.GameData().WorldName; got != "upstream" {
		t.Fatalf("world name %q, want upstream", got)
	}
	for {
		pk, err := client.ReadPacket()
		if err != nil {
			t.Fatal("read:", err)
		}
		if txt, ok := pk.(*packet.Text); ok && txt.Message == "hello-from-upstream" {
			return
		}
	}
}
