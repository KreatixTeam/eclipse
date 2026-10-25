package hub

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler exposes the hub's JSON API. The embedded SPA (hub/dist) is served
// by the caller's mux alongside it.
func (h *Hub) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", h.login)
	mux.HandleFunc("GET /api/login/", h.loginStatus)
	mux.HandleFunc("GET /api/servers", h.listServers)
	mux.HandleFunc("POST /api/servers", h.addServer)
	mux.HandleFunc("POST /api/servers/", h.serverOps)
	mux.HandleFunc("DELETE /api/servers/", h.removeServer)
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Hub) login(w http.ResponseWriter, r *http.Request) {
	f, err := h.StartLogin(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"id": f.ID, "code": f.Code, "uri": f.URI})
}

func (h *Hub) loginStatus(w http.ResponseWriter, r *http.Request) {
	f, ok := h.LoginStatus(strings.TrimPrefix(r.URL.Path, "/api/login/"))
	if !ok {
		http.Error(w, "unknown flow", http.StatusNotFound)
		return
	}
	out := map[string]any{"done": f.Done}
	if f.Err != "" {
		out["error"] = f.Err
	}
	if f.Acct != nil {
		out["account"] = f.Acct.ID
		out["gamertag"] = f.Acct.Gamertag
	}
	writeJSON(w, out)
}

// account resolves ?account= or the account field of a JSON body.
func (h *Hub) account(r *http.Request, bodyAcct string) *Account {
	id := r.URL.Query().Get("account")
	if id == "" {
		id = bodyAcct
	}
	return h.Get(id)
}

func (h *Hub) listServers(w http.ResponseWriter, r *http.Request) {
	a := h.account(r, "")
	if a == nil {
		http.Error(w, "unknown account", http.StatusUnauthorized)
		return
	}
	writeJSON(w, a.Servers)
}

func (h *Hub) addServer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Account string `json:"account"`
		Name    string `json:"name"`
		Addr    string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a := h.account(r, body.Account)
	if a == nil {
		http.Error(w, "unknown account", http.StatusUnauthorized)
		return
	}
	if body.Name == "" || body.Addr == "" {
		http.Error(w, "name and addr required", http.StatusBadRequest)
		return
	}
	s, err := h.AddServer(a, body.Name, body.Addr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s)
}

func (h *Hub) serverOps(w http.ResponseWriter, r *http.Request) {
	// POST /api/servers/{id}/toggle?on=true&account=…
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/servers/"), "/")
	if len(parts) == 2 && parts[1] == "toggle" {
		a := h.account(r, "")
		if a == nil {
			http.Error(w, "unknown account", http.StatusUnauthorized)
			return
		}
		on := r.URL.Query().Get("on") == "true"
		if !h.ToggleServer(a, parts[0], on) {
			http.Error(w, "unknown server or start failed", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	http.Error(w, "not found", http.StatusNotFound)
}

func (h *Hub) removeServer(w http.ResponseWriter, r *http.Request) {
	a := h.account(r, "")
	if a == nil {
		http.Error(w, "unknown account", http.StatusUnauthorized)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/servers/")
	if !h.RemoveServer(a, id) {
		http.Error(w, "unknown server", http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
