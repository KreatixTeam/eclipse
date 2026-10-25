package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sandertv/gophertunnel/minecraft/auth"
	"golang.org/x/oauth2"
)

// tokenSource loads a cached Microsoft Live token from disk or runs the
// device-code flow once (the code is printed to stderr for the operator).
// The resulting source refreshes itself and writes tokens back to cache.
func tokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	path := tokenPath()

	var token *oauth2.Token
	if data, err := os.ReadFile(path); err == nil {
		token = &oauth2.Token{}
		if err := json.Unmarshal(data, token); err != nil {
			return nil, fmt.Errorf("parse cached token %s: %w", path, err)
		}
	}

	src := auth.RefreshTokenSourceWriter(token, os.Stderr)
	// Persist refreshed tokens so later runs stay signed in.
	return oauth2.ReuseTokenSource(token, &persistTokenSource{src: src, path: path}), nil
}

func tokenPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "eclipse", "live-token.json")
}

// persistTokenSource wraps a TokenSource and writes every new token to disk.
type persistTokenSource struct {
	src  oauth2.TokenSource
	path string
}

func (p *persistTokenSource) Token() (*oauth2.Token, error) {
	t, err := p.src.Token()
	if err != nil {
		return nil, err
	}
	if t == nil || !t.Valid() {
		return nil, errors.New("eclipse: invalid Xbox Live token")
	}
	if err := os.MkdirAll(filepath.Dir(p.path), 0o700); err == nil {
		if data, err := json.Marshal(t); err == nil {
			_ = os.WriteFile(p.path, data, 0o600)
		}
	}
	return t, nil
}
