package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// OAuth 2.1 per a servidors MCP remots (spec MCP 2025-06-18,
// «Authorization»): metadades del recurs protegit (RFC 9728) → metadades
// del servidor d'autorització (RFC 8414 / OIDC) → registre dinàmic del
// client si cal (RFC 7591) → codi d'autorització amb PKCE S256 → tokens,
// amb el paràmetre resource (RFC 8707) i refresc.
//
// Els tokens es desen a <dades de Gregal>/mcp-oauth.json amb permisos 0600.

// OAuthCallbackPath és la ruta on el navegador torna amb el codi. La web
// la serveix al seu propi port; el TUI n'obre una al loopback.
const OAuthCallbackPath = "/api/mcp/oauth/callback"

// DataDir és on Gregal desa les dades (GREGAL_DATA_DIR o
// ~/.local/share/gregal, com les converses).
func DataDir() string {
	if d := strings.TrimSpace(os.Getenv("GREGAL_DATA_DIR")); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "gregal")
	}
	return filepath.Join(home, ".local", "share", "gregal")
}

// tokenRecord és el que es desa d'un servidor autoritzat.
type tokenRecord struct {
	AccessToken   string    `json:"access_token"`
	RefreshToken  string    `json:"refresh_token,omitempty"`
	TokenType     string    `json:"token_type,omitempty"`
	Expiry        time.Time `json:"expiry,omitempty"`
	Scope         string    `json:"scope,omitempty"`
	ClientID      string    `json:"client_id,omitempty"`
	ClientSecret  string    `json:"client_secret,omitempty"`
	AuthMethod    string    `json:"auth_method,omitempty"`
	TokenEndpoint string    `json:"token_endpoint,omitempty"`
	Resource      string    `json:"resource,omitempty"`
}

func (r tokenRecord) expired() bool {
	return !r.Expiry.IsZero() && time.Now().Add(30*time.Second).After(r.Expiry)
}

// clientRecord és un client registrat dinàmicament (per redirect_uri).
type clientRecord struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	AuthMethod   string `json:"auth_method,omitempty"`
}

type storeFile struct {
	Tokens  map[string]tokenRecord  `json:"tokens"`
	Clients map[string]clientRecord `json:"clients"`
}

// TokenStore desa tokens i clients registrats en un fitxer 0600.
type TokenStore struct {
	path string
	mu   sync.Mutex
}

// NewTokenStore obre (sense crear-lo encara) el magatzem de path; buit =
// DataDir()/mcp-oauth.json.
func NewTokenStore(path string) *TokenStore {
	if path == "" {
		path = filepath.Join(DataDir(), "mcp-oauth.json")
	}
	return &TokenStore{path: path}
}

func (s *TokenStore) load() storeFile {
	f := storeFile{Tokens: map[string]tokenRecord{}, Clients: map[string]clientRecord{}}
	raw, err := os.ReadFile(s.path)
	if err == nil {
		_ = json.Unmarshal(raw, &f)
	}
	if f.Tokens == nil {
		f.Tokens = map[string]tokenRecord{}
	}
	if f.Clients == nil {
		f.Clients = map[string]clientRecord{}
	}
	return f
}

func (s *TokenStore) save(f storeFile) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(tmp, 0o600)
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(s.path, 0o600)
}

func (s *TokenStore) get(key string) (tokenRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.load().Tokens[key]
	return r, ok && r.AccessToken != ""
}

func (s *TokenStore) put(key string, r tokenRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.load()
	f.Tokens[key] = r
	return s.save(f)
}

// Forget esborra el token d'un servidor (desconnectar o treure'l).
func (s *TokenStore) Forget(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.load()
	if _, ok := f.Tokens[key]; !ok {
		return nil
	}
	delete(f.Tokens, key)
	return s.save(f)
}

func (s *TokenStore) client(key string) (clientRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.load().Clients[key]
	return c, ok && c.ClientID != ""
}

func (s *TokenStore) putClient(key string, c clientRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.load()
	f.Clients[key] = c
	return s.save(f)
}

// tokenKey identifica els tokens d'un servidor: nom i URL (canviar l'URL
// d'un servidor no reaprofita tokens d'un altre lloc).
func tokenKey(name string, srv Server) string { return name + "|" + strings.TrimSpace(srv.URL) }

// authMeta són les metadades del servidor d'autorització.
type authMeta struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	RegistrationEndpoint  string   `json:"registration_endpoint"`
	ScopesSupported       []string `json:"scopes_supported"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
}

type resourceMeta struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported"`
}

func getJSON(ctx context.Context, hc HTTPDoer, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", u, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}

// wellKnown construeix <origen>/.well-known/<nom><camí>.
func wellKnown(u *url.URL, name string, withPath bool) string {
	p := ""
	if withPath {
		p = strings.TrimSuffix(u.EscapedPath(), "/")
	}
	return u.Scheme + "://" + u.Host + "/.well-known/" + name + p
}

// canonicalResource és l'URI canònic del servidor MCP (RFC 8707): sense
// fragment ni barra final.
func canonicalResource(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	s := u.String()
	if u.Path == "/" && u.RawQuery == "" {
		s = strings.TrimSuffix(s, "/")
	}
	return s
}

// discover troba el servidor d'autorització d'un servidor MCP.
func discover(ctx context.Context, hc HTTPDoer, serverURL, resourceMetadata string) (authMeta, resourceMeta, error) {
	su, err := url.Parse(serverURL)
	if err != nil {
		return authMeta{}, resourceMeta{}, err
	}
	var prm resourceMeta
	cands := []string{}
	if resourceMetadata != "" {
		cands = append(cands, resourceMetadata)
	}
	if strings.Trim(su.Path, "/") != "" {
		cands = append(cands, wellKnown(su, "oauth-protected-resource", true))
	}
	cands = append(cands, wellKnown(su, "oauth-protected-resource", false))
	for _, c := range cands {
		var m resourceMeta
		if getJSON(ctx, hc, c, &m) == nil && len(m.AuthorizationServers) > 0 {
			prm = m
			break
		}
	}
	issuer := su.Scheme + "://" + su.Host
	if len(prm.AuthorizationServers) > 0 {
		issuer = prm.AuthorizationServers[0]
	}
	iu, err := url.Parse(issuer)
	if err != nil {
		return authMeta{}, prm, err
	}
	var asCands []string
	if strings.Trim(iu.Path, "/") != "" {
		asCands = append(asCands,
			wellKnown(iu, "oauth-authorization-server", true),
			wellKnown(iu, "openid-configuration", true),
			strings.TrimSuffix(issuer, "/")+"/.well-known/openid-configuration")
	} else {
		asCands = append(asCands,
			wellKnown(iu, "oauth-authorization-server", false),
			wellKnown(iu, "openid-configuration", false))
	}
	for _, c := range asCands {
		var m authMeta
		if getJSON(ctx, hc, c, &m) == nil && m.AuthorizationEndpoint != "" && m.TokenEndpoint != "" {
			return m, prm, nil
		}
	}
	// Sense metadades (spec 2025-03-26): els camins per defecte a l'origen.
	base := iu.Scheme + "://" + iu.Host
	return authMeta{
		Issuer: issuer, AuthorizationEndpoint: base + "/authorize",
		TokenEndpoint: base + "/token", RegistrationEndpoint: base + "/register",
	}, prm, nil
}

// pendingAuth és una autorització començada que espera el callback.
type pendingAuth struct {
	Server        string
	Key           string
	State         string
	Verifier      string
	RedirectURI   string
	ClientID      string
	ClientSecret  string
	AuthMethod    string
	TokenEndpoint string
	Resource      string
	Created       time.Time
}

func randomURLSafe(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// beginAuth prepara l'URL d'autorització d'un servidor.
func beginAuth(ctx context.Context, hc HTTPDoer, store *TokenStore, name string, srv Server, redirectURI string, ae *AuthRequiredError) (string, *pendingAuth, error) {
	if redirectURI == "" {
		return "", nil, errors.New("no OAuth redirect URI available")
	}
	var rmURL, scopeHint string
	if ae != nil {
		rmURL, scopeHint = ae.ResourceMetadata, ae.Scope
	}
	meta, prm, err := discover(ctx, hc, srv.URL, rmURL)
	if err != nil {
		return "", nil, err
	}
	resource := canonicalResource(srv.URL)
	if prm.Resource != "" {
		resource = prm.Resource
	}
	p := &pendingAuth{
		Server: name, Key: tokenKey(name, srv), State: randomURLSafe(24), Verifier: randomURLSafe(48),
		RedirectURI: redirectURI, TokenEndpoint: meta.TokenEndpoint, Resource: resource, Created: time.Now(),
	}
	scopes := ""
	if srv.OAuth != nil && len(srv.OAuth.Scopes) > 0 {
		scopes = strings.Join(srv.OAuth.Scopes, " ")
	} else if scopeHint != "" {
		scopes = scopeHint
	} else if len(prm.ScopesSupported) > 0 {
		scopes = strings.Join(prm.ScopesSupported, " ")
	}
	switch {
	case srv.OAuth != nil && srv.OAuth.ClientID != "":
		p.ClientID, p.ClientSecret = srv.OAuth.ClientID, os.ExpandEnv(srv.OAuth.ClientSecret)
		if p.ClientSecret != "" {
			p.AuthMethod = "client_secret_post"
		}
	default:
		ckey := p.Key + "|" + meta.Issuer + "|" + redirectURI
		if c, ok := store.client(ckey); ok {
			p.ClientID, p.ClientSecret, p.AuthMethod = c.ClientID, c.ClientSecret, c.AuthMethod
			break
		}
		if meta.RegistrationEndpoint == "" {
			return "", nil, errors.New("the authorization server offers no dynamic client registration: set mcp.<name>.oauth.client_id")
		}
		c, err := registerClient(ctx, hc, meta.RegistrationEndpoint, redirectURI, scopes)
		if err != nil {
			return "", nil, err
		}
		_ = store.putClient(ckey, c)
		p.ClientID, p.ClientSecret, p.AuthMethod = c.ClientID, c.ClientSecret, c.AuthMethod
	}
	au, err := url.Parse(meta.AuthorizationEndpoint)
	if err != nil {
		return "", nil, err
	}
	q := au.Query()
	q.Set("response_type", "code")
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", p.State)
	q.Set("code_challenge", pkceChallenge(p.Verifier))
	q.Set("code_challenge_method", "S256")
	q.Set("resource", resource)
	if scopes != "" {
		q.Set("scope", scopes)
	}
	au.RawQuery = q.Encode()
	return au.String(), p, nil
}

// registerClient fa el registre dinàmic (RFC 7591) com a client públic.
func registerClient(ctx context.Context, hc HTTPDoer, endpoint, redirectURI, scopes string) (clientRecord, error) {
	body := map[string]any{
		"client_name":                "Gregal",
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	}
	if scopes != "" {
		body["scope"] = scopes
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(raw)))
	if err != nil {
		return clientRecord{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return clientRecord{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return clientRecord{}, fmt.Errorf("dynamic client registration: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		AuthMethod   string `json:"token_endpoint_auth_method"`
	}
	if err := json.Unmarshal(data, &out); err != nil || out.ClientID == "" {
		return clientRecord{}, fmt.Errorf("dynamic client registration: no client_id in the response")
	}
	method := out.AuthMethod
	if method == "" && out.ClientSecret != "" {
		method = "client_secret_basic"
	}
	return clientRecord{ClientID: out.ClientID, ClientSecret: out.ClientSecret, AuthMethod: method}, nil
}

// tokenRequest fa un POST al token endpoint.
func tokenRequest(ctx context.Context, hc HTTPDoer, endpoint, clientID, secret, method string, form url.Values) (tokenRecord, error) {
	form.Set("client_id", clientID)
	if secret != "" && method != "client_secret_basic" {
		form.Set("client_secret", secret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenRecord{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if secret != "" && method == "client_secret_basic" {
		req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(secret))
	}
	resp, err := hc.Do(req)
	if err != nil {
		return tokenRecord{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}
	_ = json.Unmarshal(data, &out)
	if resp.StatusCode != http.StatusOK || out.AccessToken == "" {
		msg := firstNonEmpty(out.ErrorDesc, out.Error, strings.TrimSpace(string(data)))
		return tokenRecord{}, fmt.Errorf("token endpoint: HTTP %d: %s", resp.StatusCode, msg)
	}
	r := tokenRecord{AccessToken: out.AccessToken, RefreshToken: out.RefreshToken, TokenType: out.TokenType, Scope: out.Scope}
	if out.ExpiresIn > 0 {
		r.Expiry = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second)
	}
	return r, nil
}

// exchangeCode canvia el codi del callback per tokens.
func exchangeCode(ctx context.Context, hc HTTPDoer, p *pendingAuth, code string) (tokenRecord, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", p.RedirectURI)
	form.Set("code_verifier", p.Verifier)
	if p.Resource != "" {
		form.Set("resource", p.Resource)
	}
	r, err := tokenRequest(ctx, hc, p.TokenEndpoint, p.ClientID, p.ClientSecret, p.AuthMethod, form)
	if err != nil {
		return r, err
	}
	r.ClientID, r.ClientSecret, r.AuthMethod = p.ClientID, p.ClientSecret, p.AuthMethod
	r.TokenEndpoint, r.Resource = p.TokenEndpoint, p.Resource
	return r, nil
}

// refreshRecord renova un token amb el refresh_token.
func refreshRecord(ctx context.Context, hc HTTPDoer, old tokenRecord) (tokenRecord, error) {
	if old.RefreshToken == "" || old.TokenEndpoint == "" {
		return old, errors.New("no refresh token")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", old.RefreshToken)
	if old.Resource != "" {
		form.Set("resource", old.Resource)
	}
	r, err := tokenRequest(ctx, hc, old.TokenEndpoint, old.ClientID, old.ClientSecret, old.AuthMethod, form)
	if err != nil {
		return old, err
	}
	if r.RefreshToken == "" {
		r.RefreshToken = old.RefreshToken // rotació opcional
	}
	r.ClientID, r.ClientSecret, r.AuthMethod = old.ClientID, old.ClientSecret, old.AuthMethod
	r.TokenEndpoint, r.Resource = old.TokenEndpoint, old.Resource
	return r, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// storeTokens és el TokenSource d'un servidor sobre el magatzem.
type storeTokens struct {
	store *TokenStore
	key   string
	hc    HTTPDoer
	mu    sync.Mutex
}

func (s *storeTokens) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.store.get(s.key)
	if !ok {
		return "", nil
	}
	if r.expired() && r.RefreshToken != "" {
		if nr, err := refreshRecord(ctx, s.hc, r); err == nil {
			_ = s.store.put(s.key, nr)
			return nr.AccessToken, nil
		}
	}
	return r.AccessToken, nil
}

func (s *storeTokens) Refresh(ctx context.Context) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.store.get(s.key)
	if !ok || r.RefreshToken == "" {
		return false
	}
	nr, err := refreshRecord(ctx, s.hc, r)
	if err != nil {
		return false
	}
	return s.store.put(s.key, nr) == nil
}
