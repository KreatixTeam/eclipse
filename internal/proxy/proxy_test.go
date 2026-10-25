package proxy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
	"github.com/sandertv/gophertunnel/minecraft/resource"
)

// TestBridge runs client -> proxy -> fake server and checks that the client
// spawns and receives a packet the server sends after spawn.
func TestBridge(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"format_version":2,"header":{"name":"t","description":"t","uuid":"7c1ad3a1-4f0e-4c1a-9a43-2b1f6a0d9e11","version":[1,0,0],"min_engine_version":[1,20,0]},"modules":[{"type":"resources","uuid":"0b8e5f0e-6c55-4b8a-8f0e-5a0c3e9d1f22","version":[1,0,0]}]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	pack, err := resource.ReadPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := minecraft.ListenConfig{AuthenticationDisabled: true, ResourcePacks: []*resource.Pack{pack}}.Listen("raknet", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go func() {
		for {
			c, err := srv.Accept()
			if err != nil {
				return
			}
			go func(conn *minecraft.Conn) {
				defer conn.Close()
				if err := conn.StartGame(minecraft.GameData{WorldName: "upstream"}); err != nil {
					return
				}
				_ = conn.WritePacket(&packet.Text{TextType: packet.TextTypeRaw, Message: "hello-from-upstream"})
				time.Sleep(3 * time.Second)
			}(c.(*minecraft.Conn))
		}
	}()

	p, err := New(Config{ListenAddr: "127.0.0.1:0", ServerAddr: srv.Addr().String(), Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	go p.Run()
	for i := 0; len(p.packs()) == 0; i++ {
		if i > 100 {
			t.Fatal("packs never prefetched")
		}
		time.Sleep(100 * time.Millisecond)
	}

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
	if n := len(client.ResourcePacks()); n != 1 {
		t.Fatalf("client got %d packs, want 1", n)
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
