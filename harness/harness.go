// Package harness embeds Gregal's shared HTTP backend in a Go application.
package harness

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gregal/internal/config"
	"gregal/internal/web"
)

// Identity is an authenticated employee/service principal, never browser input.
// AccountID must be stable, contain only letters, digits or underscores, and
// have at most 24 characters. Roots and Home are server-controlled paths.
// An account's roots, home, and admin flag are immutable for a Runtime; a
// policy change requires a new Runtime so cached sessions cannot retain old grants.
type Identity struct {
	AccountID string
	Roots     []string
	Home      string
	Admin     bool
}

// AccountID creates a stable, pseudonymous namespace from a verified subject.
func AccountID(subject string) string {
	if strings.TrimSpace(subject) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(subject))
	return hex.EncodeToString(sum[:12])
}

type Options struct {
	ConfigPath string
	Token      string
	// Authenticate overrides native bearer/user authentication. Verify the
	// IdP session or JWT here; returning an error denies the request.
	Authenticate func(*http.Request) (Identity, error)
	// ServeUI includes the existing web interface. False exposes only /api/.
	ServeUI bool
	// AllowUnauthenticated explicitly opts into development-only open access.
	AllowUnauthenticated bool
}

type Runtime struct {
	server           *web.Server
	handler          http.Handler
	cancel           context.CancelFunc
	backgroundDone   <-chan struct{}
	closeOnce        sync.Once
	closed           atomic.Bool
	identityMu       sync.Mutex
	identityPolicies map[string][32]byte
}

var accountIDPattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,24}$`)
var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const maxOpenSessionBody = 1 << 20

type identityPolicy struct {
	Roots []string `json:"roots"`
	Home  string   `json:"home"`
	Admin bool     `json:"admin"`
}

// New reuses the same Hub, queue, agent engine, integrations and wire contract
// as gregal serve. Run one runtime/configuration per process: tools and provider
// caches currently include process-wide state.
func New(ctx context.Context, options Options) (*Runtime, error) {
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if strings.TrimSpace(options.ConfigPath) == "" {
		return nil, errors.New("ConfigPath is required")
	}
	if _, err := os.Stat(options.ConfigPath); err != nil {
		return nil, err
	}
	cfg, cfgPath, err := config.Load(options.ConfigPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(options.Token) == "" && len(cfg.Users) == 0 && options.Authenticate == nil && !options.AllowUnauthenticated {
		return nil, errors.New("configure a token, users, or an identity authenticator")
	}
	web.SetupMCP(cfg)
	server := web.New(cfg, cfgPath)
	server.SetToken(strings.TrimSpace(options.Token))
	runtime := &Runtime{server: server, identityPolicies: map[string][32]byte{}}
	var handler http.Handler
	if options.Authenticate != nil {
		handler = server.HandlerWithIdentity(func(r *http.Request) (*web.User, error) {
			identity, err := options.Authenticate(r)
			if err != nil || !accountIDPattern.MatchString(identity.AccountID) {
				return nil, errors.New("invalid identity")
			}
			if err := validateSessionRequest(r, identity.AccountID); err != nil {
				return nil, err
			}
			identity, err = normalizeIdentity(identity)
			if err != nil {
				return nil, err
			}
			user := &web.User{Name: identity.AccountID, Roots: identity.Roots, Home: identity.Home, Admin: identity.Admin}
			if identity.Home != "" && user.Allow(identity.Home) != nil {
				return nil, errors.New("identity home must be inside an allowed root")
			}
			if err := runtime.pinIdentityPolicy(identity); err != nil {
				return nil, err
			}
			return user, nil
		})
	} else {
		handler = server.Handler()
	}
	if !options.ServeUI {
		api := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			api.ServeHTTP(w, r)
		})
	}
	background, cancel := context.WithCancel(ctx)
	runtime.cancel = cancel
	runtime.handler = handler
	runtime.backgroundDone = server.StartBackground(background)
	context.AfterFunc(background, runtime.stop)
	return runtime, nil
}

func (r *Runtime) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if r.closed.Load() {
			http.Error(w, "runtime closed", http.StatusServiceUnavailable)
			return
		}
		r.handler.ServeHTTP(w, req)
	})
}

// Close asks active and queued work to stop, then waits up to five seconds for
// the background scheduler to exit. It does not drain arbitrary HTTP handlers;
// the embedding application owns its HTTP server shutdown order.
func (r *Runtime) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = r.CloseContext(ctx)
}

// CloseContext asks active and queued work to stop, then waits for the
// background scheduler to finish unwinding. It does not wait for unrelated
// request handlers. Shut down the embedding HTTP server separately.
func (r *Runtime) CloseContext(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("shutdown context is required")
	}
	r.stop()
	select {
	case <-r.backgroundDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Runtime) stop() {
	r.closeOnce.Do(func() {
		r.closed.Store(true)
		r.cancel()
		r.server.Stop()
	})
}

func normalizeIdentity(identity Identity) (Identity, error) {
	identity.Roots = append([]string(nil), identity.Roots...)
	for i, root := range identity.Roots {
		root = strings.TrimSpace(root)
		if root == "" {
			return Identity{}, errors.New("identity root cannot be empty")
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return Identity{}, err
		}
		identity.Roots[i] = filepath.Clean(abs)
	}
	identity.Home = strings.TrimSpace(identity.Home)
	if identity.Home != "" {
		abs, err := filepath.Abs(identity.Home)
		if err != nil {
			return Identity{}, err
		}
		identity.Home = filepath.Clean(abs)
	}
	return identity, nil
}

func (r *Runtime) pinIdentityPolicy(identity Identity) error {
	raw, err := json.Marshal(identityPolicy{Roots: identity.Roots, Home: identity.Home, Admin: identity.Admin})
	if err != nil {
		return err
	}
	fingerprint := sha256.Sum256(raw)
	r.identityMu.Lock()
	defer r.identityMu.Unlock()
	if pinned, ok := r.identityPolicies[identity.AccountID]; ok && pinned != fingerprint {
		return errors.New("identity policy changed; restart the runtime to apply it")
	}
	r.identityPolicies[identity.AccountID] = fingerprint
	return nil
}

func validateSessionRequest(r *http.Request, accountID string) error {
	for _, id := range []string{
		strings.TrimSpace(r.Header.Get("X-Gregal-Session")),
		strings.TrimSpace(r.URL.Query().Get("session")),
	} {
		if id != "" && !validSessionID(id, accountID) {
			return errors.New("invalid session ID")
		}
	}
	if r.URL.Path != "/api/sessions/open" {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxOpenSessionBody+1))
	if err != nil {
		return err
	}
	if len(body) > maxOpenSessionBody {
		return errors.New("session request body too large")
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var request struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return nil // Preserve the existing handler's malformed-body behavior.
	}
	if id := strings.TrimSpace(request.ID); id != "" && !validSessionID(id, accountID) {
		return errors.New("invalid session ID")
	}
	return nil
}

func validSessionID(id, accountID string) bool {
	if !sessionIDPattern.MatchString(id) {
		return false
	}
	local := strings.TrimPrefix(id, accountID+"-")
	return len(local) > 0 && len(local) <= 64-len(accountID)-1 && sessionIDPattern.MatchString(local)
}
