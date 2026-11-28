// Package google connecta un compte de Google d'una persona (OAuth 2.0 amb
// PKCE, accés offline) i en llegeix el calendari, el correu i el Drive, i hi
// prepara esdeveniments i esborranys. No guarda res: qui el fa servir li
// passa com carregar i desar el token (el Vault xifrat de connexions).
//
// El client OAuth el registra qui instal·la Gregal (Google Cloud Console,
// tipus «Web application», en mode de prova per a ús personal): Google no té
// registre dinàmic de clients. Vegeu docs/google.md.
package google

import (
	"bytes"
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
	"strings"
	"sync"
	"time"
)

// Les adreces de Google (variables perquè els tests hi posin un servidor fals).
var (
	authURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	tokenURL    = "https://oauth2.googleapis.com/token"
	revokeURL   = "https://oauth2.googleapis.com/revoke"
	userinfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	apiBase     = "https://www.googleapis.com"
	gmailBase   = "https://gmail.googleapis.com"
)

// Scopes per defecte: identitat, calendari (llegir i crear), correu (llegir
// i fer esborranys, mai enviar) i Drive en lectura. Gmail i Drive són àmbits
// «restringits»: una app en mode de prova (ús personal, <100 usuaris) els pot
// demanar sense l'avaluació CASA.
var DefaultScopes = []string{
	"openid", "email", "profile",
	"https://www.googleapis.com/auth/calendar.events",
	"https://www.googleapis.com/auth/gmail.readonly",
	"https://www.googleapis.com/auth/gmail.compose",
	"https://www.googleapis.com/auth/drive.readonly",
}

// Doer fa peticions HTTP (un *http.Client, o el del servidor amb els seus límits).
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// ErrNotConfigured: el servidor no té client OAuth de Google.
var ErrNotConfigured = errors.New("google: no OAuth client configured (google.client_id and google.client_secret in config.yaml)")

// ErrRevoked: Google ja no accepta el token (revocat, caducat o sense refresh).
var ErrRevoked = errors.New("google: the account needs to be connected again")

// Config és el client OAuth. ClientSecret pot ser ${VAR}: s'expandeix en fer-lo servir.
type Config struct {
	ClientID     string   `yaml:"client_id,omitempty"`
	ClientSecret string   `yaml:"client_secret,omitempty"`
	Scopes       []string `yaml:"scopes,omitempty"`
}

func (c Config) id() string     { return strings.TrimSpace(os.ExpandEnv(c.ClientID)) }
func (c Config) secret() string { return strings.TrimSpace(os.ExpandEnv(c.ClientSecret)) }

// Configured diu si hi ha un client OAuth complet.
func (c Config) Configured() bool { return c.id() != "" && c.secret() != "" }

func (c Config) scopes() []string {
	if len(c.Scopes) > 0 {
		return c.Scopes
	}
	return DefaultScopes
}

// Pending és el que cal recordar entre el consentiment i el callback.
type Pending struct {
	State    string `json:"state"`
	Verifier string `json:"verifier"`
	Redirect string `json:"redirect"`
}

// Token és el que es desa xifrat per compte.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	Scope        string    `json:"scope,omitempty"`
	Email        string    `json:"email,omitempty"`
}

// Has diu si el token porta l'àmbit (o un de més ample).
func (t Token) Has(scope string) bool {
	for _, s := range strings.Fields(t.Scope) {
		if s == scope {
			return true
		}
	}
	return false
}

func random() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Begin prepara el consentiment: l'adreça on enviar la persona i el pendent.
func Begin(cfg Config, redirect string) (string, Pending, error) {
	if !cfg.Configured() {
		return "", Pending{}, ErrNotConfigured
	}
	state, err := random()
	if err != nil {
		return "", Pending{}, err
	}
	verifier, err := random()
	if err != nil {
		return "", Pending{}, err
	}
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"client_id":              {cfg.id()},
		"redirect_uri":           {redirect},
		"response_type":          {"code"},
		"scope":                  {strings.Join(cfg.scopes(), " ")},
		"state":                  {state},
		"code_challenge":         {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method":  {"S256"},
		"access_type":            {"offline"},
		"prompt":                 {"consent"},
		"include_granted_scopes": {"true"},
	}
	return authURL + "?" + q.Encode(), Pending{State: state, Verifier: verifier, Redirect: redirect}, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

func postToken(ctx context.Context, hc Doer, form url.Values) (tokenResponse, error) {
	var tr tokenResponse
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tr, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := hc.Do(req)
	if err != nil {
		return tr, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	_ = json.Unmarshal(body, &tr)
	if res.StatusCode != http.StatusOK || tr.AccessToken == "" {
		if tr.Error == "invalid_grant" {
			return tr, ErrRevoked
		}
		msg := strings.TrimSpace(tr.Error + " " + tr.Description)
		if msg == "" {
			msg = res.Status
		}
		return tr, fmt.Errorf("google token: %s", msg)
	}
	return tr, nil
}

// Exchange canvia el codi del callback per tokens i hi afegeix el correu del compte.
func Exchange(ctx context.Context, hc Doer, cfg Config, p Pending, code string) (Token, error) {
	if !cfg.Configured() {
		return Token{}, ErrNotConfigured
	}
	tr, err := postToken(ctx, hc, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {p.Redirect},
		"client_id": {cfg.id()}, "client_secret": {cfg.secret()}, "code_verifier": {p.Verifier},
	})
	if err != nil {
		return Token{}, err
	}
	t := Token{AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken, Scope: tr.Scope,
		Expiry: time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)}
	if t.RefreshToken == "" {
		return t, errors.New("google did not give offline access; remove Gregal from your Google account's third-party access and connect again")
	}
	var who struct {
		Email string `json:"email"`
	}
	if err := getJSON(ctx, hc, t.AccessToken, userinfoURL, &who); err == nil {
		t.Email = who.Email
	}
	return t, nil
}

// Refresh renova el token d'accés amb el de refresc.
func Refresh(ctx context.Context, hc Doer, cfg Config, t Token) (Token, error) {
	if t.RefreshToken == "" {
		return t, ErrRevoked
	}
	tr, err := postToken(ctx, hc, url.Values{
		"grant_type": {"refresh_token"}, "refresh_token": {t.RefreshToken},
		"client_id": {cfg.id()}, "client_secret": {cfg.secret()},
	})
	if err != nil {
		return t, err
	}
	t.AccessToken = tr.AccessToken
	t.Expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	if tr.Scope != "" {
		t.Scope = tr.Scope
	}
	if tr.RefreshToken != "" {
		t.RefreshToken = tr.RefreshToken
	}
	return t, nil
}

// Revoke retira el permís a Google (el de refresc arrossega l'accés).
func Revoke(ctx context.Context, hc Doer, t Token) error {
	tok := t.RefreshToken
	if tok == "" {
		tok = t.AccessToken
	}
	if tok == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL, strings.NewReader(url.Values{"token": {tok}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusBadRequest {
		return fmt.Errorf("google revoke: %s", res.Status)
	}
	return nil
}

func getJSON(ctx context.Context, hc Doer, access, u string, out any) error {
	return doJSON(ctx, hc, access, http.MethodGet, u, nil, out)
}

type apiError struct {
	Status int
	Msg    string
}

func (e *apiError) Error() string { return fmt.Sprintf("google api %d: %s", e.Status, e.Msg) }

func doJSON(ctx context.Context, hc Doer, access, method, u string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if res.StatusCode/100 != 2 {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		msg := e.Error.Message
		if msg == "" {
			msg = res.Status
		}
		return &apiError{res.StatusCode, msg}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// Client fa crides en nom d'un compte i renova el token quan cal.
type Client struct {
	HTTP Doer
	Cfg  Config
	Load func() (Token, error)
	Save func(Token) error
	// OnRevoked s'avisa quan Google ja no accepta el compte.
	OnRevoked func(error)

	mu  sync.Mutex
	tok *Token
}

func (c *Client) token(ctx context.Context, force bool) (Token, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tok == nil {
		t, err := c.Load()
		if err != nil {
			return Token{}, ErrRevoked
		}
		c.tok = &t
	}
	if force || time.Until(c.tok.Expiry) < time.Minute {
		t, err := Refresh(ctx, c.HTTP, c.Cfg, *c.tok)
		if err != nil {
			if errors.Is(err, ErrRevoked) && c.OnRevoked != nil {
				c.OnRevoked(err)
			}
			return Token{}, err
		}
		c.tok = &t
		if c.Save != nil {
			_ = c.Save(t)
		}
	}
	return *c.tok, nil
}

// Token torna el token vigent (renovat si calia).
func (c *Client) Token(ctx context.Context) (Token, error) { return c.token(ctx, false) }

func (c *Client) call(ctx context.Context, method, u string, body, out any) error {
	t, err := c.token(ctx, false)
	if err != nil {
		return err
	}
	err = doJSON(ctx, c.HTTP, t.AccessToken, method, u, body, out)
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusUnauthorized {
		if t, err = c.token(ctx, true); err != nil {
			return err
		}
		err = doJSON(ctx, c.HTTP, t.AccessToken, method, u, body, out)
	}
	if errors.As(err, &ae) && ae.Status == http.StatusForbidden && strings.Contains(strings.ToLower(ae.Msg), "insufficient") {
		return fmt.Errorf("google: this account did not grant the permission for that (%s); connect it again and tick it", ae.Msg)
	}
	return err
}

// Whoami comprova que el compte respon i en torna el correu.
func (c *Client) Whoami(ctx context.Context) (string, error) {
	var who struct {
		Email string `json:"email"`
	}
	if err := c.call(ctx, http.MethodGet, userinfoURL, nil, &who); err != nil {
		return "", err
	}
	return who.Email, nil
}

// UseEndpoints apunta el paquet a un servidor fals (tests d'altres paquets):
// base serveix /auth, /token, /userinfo, i les API (calendar, gmail, drive).
func UseEndpoints(base string) (restore func()) {
	a, t, u, api, g := authURL, tokenURL, userinfoURL, apiBase, gmailBase
	authURL, tokenURL, userinfoURL, apiBase, gmailBase = base+"/auth", base+"/token", base+"/userinfo", base, base
	return func() { authURL, tokenURL, userinfoURL, apiBase, gmailBase = a, t, u, api, g }
}
