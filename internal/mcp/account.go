package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Connexions per compte (B5): el mateix OAuth 2.1/PKCE del gestor, però
// els tokens són d'una persona concreta i no es desen al magatzem compartit
// (mcp-oauth.json) sinó on digui qui crida (el magatzem xifrat per owner de
// internal/connexions). El gestor global continua sent dels servidors de
// l'administrador.

// AccountAuth és una autorització començada per a un compte. Conté el
// verificador PKCE i el client: qui la desa l'ha de tractar com a secret.
type AccountAuth struct {
	State              string `json:"state"`
	Verifier           string `json:"verifier"`
	RedirectURI        string `json:"redirect_uri"`
	ClientID           string `json:"client_id"`
	ClientSecret       string `json:"client_secret,omitempty"`
	AuthMethod         string `json:"auth_method,omitempty"`
	TokenEndpoint      string `json:"token_endpoint"`
	RevocationEndpoint string `json:"revocation_endpoint,omitempty"`
	Resource           string `json:"resource,omitempty"`
}

// AccountToken són els tokens d'un compte (secret).
type AccountToken struct {
	AccessToken        string    `json:"access_token"`
	RefreshToken       string    `json:"refresh_token,omitempty"`
	TokenType          string    `json:"token_type,omitempty"`
	Expiry             time.Time `json:"expiry,omitempty"`
	Scope              string    `json:"scope,omitempty"`
	ClientID           string    `json:"client_id,omitempty"`
	ClientSecret       string    `json:"client_secret,omitempty"`
	AuthMethod         string    `json:"auth_method,omitempty"`
	TokenEndpoint      string    `json:"token_endpoint,omitempty"`
	RevocationEndpoint string    `json:"revocation_endpoint,omitempty"`
	Resource           string    `json:"resource,omitempty"`
	IDToken            string    `json:"id_token,omitempty"`
}

func (t AccountToken) record() tokenRecord {
	return tokenRecord{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, TokenType: t.TokenType, Expiry: t.Expiry,
		Scope: t.Scope, ClientID: t.ClientID, ClientSecret: t.ClientSecret, AuthMethod: t.AuthMethod,
		TokenEndpoint: t.TokenEndpoint, Resource: t.Resource, IDToken: t.IDToken}
}

func accountToken(r tokenRecord, revocation string) AccountToken {
	return AccountToken{AccessToken: r.AccessToken, RefreshToken: r.RefreshToken, TokenType: r.TokenType, Expiry: r.Expiry,
		Scope: r.Scope, ClientID: r.ClientID, ClientSecret: r.ClientSecret, AuthMethod: r.AuthMethod,
		TokenEndpoint: r.TokenEndpoint, RevocationEndpoint: revocation, Resource: r.Resource, IDToken: r.IDToken}
}

// Expired diu si cal refrescar-lo abans de fer-lo servir.
func (t AccountToken) Expired() bool { return t.record().expired() }

// RemoteServer torna la configuració d'un servidor remot del gestor (el
// catàleg de connectors que l'administrador ha configurat).
func (m *Manager) RemoteServer(name string) (Server, bool) {
	if m == nil {
		return Server{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	srv, ok := m.servers[name]
	return srv, ok && srv.Remote()
}

// RemoteServers són els noms dels servidors remots configurats.
func (m *Manager) RemoteServers() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for n, s := range m.servers {
		if s.Remote() {
			out = append(out, n)
		}
	}
	return out
}

// HTTP és el client HTTP del gestor (el dels tests si n'hi ha).
func (m *Manager) HTTP() HTTPDoer {
	if m == nil || m.hc == nil {
		return defaultHTTP()
	}
	return m.hc
}

// ClientStore és el magatzem on es guarden els clients registrats
// dinàmicament (no són dades de cap compte: un client públic per servidor i
// redirect_uri).
func (m *Manager) ClientStore() *TokenStore {
	if m == nil {
		return NewTokenStore("")
	}
	return m.store
}

// BeginAccountAuth prepara l'URL de consentiment per connectar un compte a
// un servidor remot. clients guarda el registre dinàmic del client.
func BeginAccountAuth(ctx context.Context, hc HTTPDoer, clients *TokenStore, name string, srv Server, redirectURI string) (string, AccountAuth, error) {
	if hc == nil {
		hc = defaultHTTP()
	}
	if !srv.Remote() {
		return "", AccountAuth{}, errors.New("only remote connectors can be connected per account")
	}
	var ae *AuthRequiredError
	// Si el servidor ja diu on són les metadades (WWW-Authenticate), millor.
	if c, err := DialCtx(ctx, name, srv, DialOpts{HTTP: hc, Tokens: emptyTokens{}}); err == nil {
		c.Close()
	} else {
		errors.As(err, &ae)
	}
	u, p, err := beginAuth(ctx, hc, clients, name, srv, redirectURI, ae)
	if err != nil {
		return "", AccountAuth{}, err
	}
	return u, AccountAuth{State: p.State, Verifier: p.Verifier, RedirectURI: p.RedirectURI, ClientID: p.ClientID,
		ClientSecret: p.ClientSecret, AuthMethod: p.AuthMethod, TokenEndpoint: p.TokenEndpoint,
		RevocationEndpoint: p.RevocationEndpoint, Resource: p.Resource}, nil
}

// ExchangeAccountCode canvia el codi del callback per tokens del compte.
func ExchangeAccountCode(ctx context.Context, hc HTTPDoer, a AccountAuth, code string) (AccountToken, error) {
	if hc == nil {
		hc = defaultHTTP()
	}
	if strings.TrimSpace(code) == "" {
		return AccountToken{}, errors.New("the authorization server returned no code")
	}
	p := &pendingAuth{State: a.State, Verifier: a.Verifier, RedirectURI: a.RedirectURI, ClientID: a.ClientID,
		ClientSecret: a.ClientSecret, AuthMethod: a.AuthMethod, TokenEndpoint: a.TokenEndpoint, Resource: a.Resource}
	r, err := exchangeCode(ctx, hc, p, code)
	if err != nil {
		return AccountToken{}, err
	}
	return accountToken(r, a.RevocationEndpoint), nil
}

// RefreshAccountToken renova els tokens d'un compte. IsGrantRevoked(err)
// vol dir que cal tornar a iniciar sessió.
func RefreshAccountToken(ctx context.Context, hc HTTPDoer, t AccountToken) (AccountToken, error) {
	if hc == nil {
		hc = defaultHTTP()
	}
	r, err := refreshRecord(ctx, hc, t.record())
	if err != nil {
		return t, err
	}
	return accountToken(r, t.RevocationEndpoint), nil
}

// RevokeAccountToken revoca els tokens al proveïdor (RFC 7009). Sense
// revocation_endpoint torna ErrNoRevocation: qui crida els oblida igualment
// i ho diu.
func RevokeAccountToken(ctx context.Context, hc HTTPDoer, t AccountToken) error {
	if t.RevocationEndpoint == "" {
		return ErrNoRevocation
	}
	if hc == nil {
		hc = defaultHTTP()
	}
	var firstErr error
	for _, tok := range []struct{ v, hint string }{{t.RefreshToken, "refresh_token"}, {t.AccessToken, "access_token"}} {
		if tok.v == "" {
			continue
		}
		form := url.Values{}
		form.Set("token", tok.v)
		form.Set("token_type_hint", tok.hint)
		form.Set("client_id", t.ClientID)
		if t.ClientSecret != "" && t.AuthMethod != "client_secret_basic" {
			form.Set("client_secret", t.ClientSecret)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.RevocationEndpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if t.ClientSecret != "" && t.AuthMethod == "client_secret_basic" {
			req.SetBasicAuth(url.QueryEscape(t.ClientID), url.QueryEscape(t.ClientSecret))
		}
		resp, err := hc.Do(req)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		resp.Body.Close()
		// RFC 7009 §2.2: 200 també per a un token ja invàlid.
		if resp.StatusCode != http.StatusOK && firstErr == nil {
			firstErr = errors.New("revocation endpoint: HTTP " + resp.Status)
		}
	}
	return firstErr
}

// ErrNoRevocation: el proveïdor no anuncia cap endpoint de revocació.
var ErrNoRevocation = errors.New("the provider offers no token revocation endpoint")

// AccountLabel treu un nom visible del compte de l'id_token (email,
// preferred_username o name). No en verifica la signatura: només és
// l'etiqueta que veu la persona, mai una prova d'identitat.
func AccountLabel(t AccountToken) string {
	parts := strings.Split(t.IDToken, ".")
	if len(parts) < 2 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ""
	}
	var c map[string]any
	if json.Unmarshal(raw, &c) != nil {
		return ""
	}
	for _, k := range []string{"email", "preferred_username", "name"} {
		if v, ok := c[k].(string); ok && strings.TrimSpace(v) != "" {
			v = strings.TrimSpace(v)
			if r := []rune(v); len(r) > 80 {
				v = string(r[:80])
			}
			return v
		}
	}
	return ""
}

// AccountTokens és el TokenSource d'un compte sobre un magatzem qualsevol.
// Refresca abans de caducar i després d'un 401; OnRevoked s'avisa quan el
// refresc diu que la sessió ja no val.
type AccountTokens struct {
	Load      func() (AccountToken, error)
	Save      func(AccountToken) error
	OnRevoked func(error)
	HTTP      HTTPDoer
	mu        sync.Mutex
}

func (a *AccountTokens) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, err := a.Load()
	if err != nil {
		return "", nil // sense token: el servidor respondrà 401 i es dirà
	}
	if t.Expired() && t.RefreshToken != "" {
		nt, err := RefreshAccountToken(ctx, a.HTTP, t)
		if err == nil {
			_ = a.Save(nt)
			return nt.AccessToken, nil
		}
		if IsGrantRevoked(err) && a.OnRevoked != nil {
			a.OnRevoked(err)
		}
	}
	return t.AccessToken, nil
}

func (a *AccountTokens) Refresh(ctx context.Context) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	t, err := a.Load()
	if err != nil || t.RefreshToken == "" {
		return false
	}
	nt, err := RefreshAccountToken(ctx, a.HTTP, t)
	if err != nil {
		if IsGrantRevoked(err) && a.OnRevoked != nil {
			a.OnRevoked(err)
		}
		return false
	}
	return a.Save(nt) == nil
}

// emptyTokens és un TokenSource sense token (sondeja el WWW-Authenticate).
type emptyTokens struct{}

func (emptyTokens) Token(context.Context) (string, error) { return "", nil }
func (emptyTokens) Refresh(context.Context) bool          { return false }

// DialAccount connecta un servidor remot amb els tokens d'un compte.
func DialAccount(ctx context.Context, name string, srv Server, ts TokenSource, hc HTTPDoer) (*Client, error) {
	if hc == nil {
		hc = defaultHTTP()
	}
	return DialCtx(ctx, name, srv, DialOpts{HTTP: hc, Tokens: ts})
}

// ToolOwnerShared diu si una eina del gestor global es pot deixar fer servir
// a un compte que no és l'administrador: només si el seu servidor no porta
// cap credencial de l'administrador (ni OAuth, ni capçaleres, ni entorn).
// Un servidor remot amb token o un stdio amb claus a l'entorn són de
// l'administrador: un altre compte no els pot fer servir.
func (m *Manager) ToolOwnerShared(toolName string) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for server, tools := range m.toolsBy {
		for _, t := range tools {
			if t.Spec.Name != toolName {
				continue
			}
			srv, ok := m.servers[server]
			if !ok {
				return false
			}
			if len(srv.Headers) > 0 || len(srv.Env) > 0 || srv.OAuth != nil {
				return false
			}
			if srv.Remote() {
				// Un remot obert (sense credencials) és de tothom; amb el
				// token OAuth de l'administrador, no.
				_, authorized := m.store.get(tokenKey(server, srv))
				return !authorized
			}
			return true
		}
	}
	return false
}
