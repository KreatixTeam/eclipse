// Package webui serves Eclipse's embedded web dashboard: live packet feed,
// module toggles and connected players over plain HTTP + SSE.
package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"

	"github.com/KreatixTeam/eclipse/internal/module"
)

// dist holds the built Vuetify dashboard (web/dist, produced by
// `npm run build` in web/).
//
//go:embed all:dist
var dist embed.FS

// Server is the dashboard HTTP server.
type Server struct {
	reg   *module.Registry
	proxy ProxyInfo

	mu      sync.Mutex
	subs    map[chan []byte]struct{}
	feed    []feedEntry // capped ring of recent packets
	players map[string]time.Time
}

type feedEntry struct {
	T   time.Time `json:"t"`
	Dir string    `json:"dir"`
	Pkt string    `json:"pkt"`
}

// ProxyInfo supplies read-only state about the running proxy.
type ProxyInfo interface {
	ListenAddr() string
	ServerAddr() string
	Clients() []string
}

func New(reg *module.Registry, info ProxyInfo) *Server {
	s := &Server{
		reg:     reg,
		proxy:   info,
		subs:    map[chan []byte]struct{}{},
		players: map[string]time.Time{},
	}
	reg.Watch = func(pk packet.Packet, dir module.Direction) {
		t := reflect.TypeOf(pk)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		d := "down"
		if dir == module.ServerBound {
			d = "up"
		}
		s.broadcast(feedEntry{T: time.Now(), Dir: d, Pkt: t.Name()})
	}
	return s
}

// PlayerJoin / PlayerLeave update the players list shown in the dashboard.
func (s *Server) PlayerJoin(addr string) {
	s.mu.Lock()
	s.players[addr] = time.Now()
	s.mu.Unlock()
}

func (s *Server) PlayerLeave(addr string) {
	s.mu.Lock()
	delete(s.players, addr)
	s.mu.Unlock()
}

func (s *Server) broadcast(e feedEntry) {
	s.mu.Lock()
	s.feed = append(s.feed, e)
	if len(s.feed) > 200 {
		s.feed = s.feed[len(s.feed)-200:]
	}
	data, _ := json.Marshal(e)
	for ch := range s.subs {
		select {
		case ch <- data:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static := http.FileServer(http.FS(dist))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Strip the leading "dist" so / maps to dist/index.html.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/dist" + r.URL.Path
		static.ServeHTTP(w, r2)
	})
	mux.HandleFunc("/api/state", s.state)
	mux.HandleFunc("/api/toggle", s.toggle)
	mux.HandleFunc("/api/feed", s.feedSSE)
	return mux
}

type stateResponse struct {
	Listen  string          `json:"listen"`
	Server  string          `json:"server"`
	Players []string        `json:"players"`
	Modules map[string]bool `json:"modules"`
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	mods := map[string]bool{}
	for _, n := range s.reg.Names() {
		mods[n] = s.reg.Enabled(n)
	}
	s.mu.Lock()
	players := make([]string, 0, len(s.players))
	for a := range s.players {
		players = append(players, a)
	}
	feed := append([]feedEntry(nil), s.feed...)
	s.mu.Unlock()
	writeJSON(w, map[string]any{
		"listen":  s.proxy.ListenAddr(),
		"server":  s.proxy.ServerAddr(),
		"players": players,
		"modules": mods,
		"feed":    feed,
	})
}

func (s *Server) toggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Name string `json:"name"`
		On   bool   `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !s.reg.Toggle(body.Name, body.On) {
		http.Error(w, "unknown module", http.StatusNotFound)
		return
	}
	if m := s.reg.Get(body.Name); m != nil {
		if t, ok := m.(module.ToggleAware); ok {
			t.OnToggle(body.On)
		}
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) feedSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/event-stream")
	w.Header().Set("cache-control", "no-cache")
	ch := make(chan []byte, 64)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}()
	flusher, _ := w.(http.Flusher)
	for {
		select {
		case data := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", data)
			if flusher != nil {
				flusher.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
