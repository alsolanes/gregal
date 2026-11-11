package tools

// Control humà del navegador de l'ordinador (paquet B3).
//
// Quan la persona pren el control des del visor, l'agent no hi actua: cap
// acció del navegador (ni captures, ni lectures, ni tancar la pestanya) no
// s'executa fins que el retorna. No és un gate de permisos: és una pausa
// real, a sobre de browser_gate.go, que no canvia.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	browserControlMu sync.RWMutex
	// browserControlHeld diu si la persona té el control de l'ordinador del
	// propietari (clau OwnerKeyFor / BrowserOwnerKey). Nil = ningú no el pot
	// prendre (sense visor).
	browserControlHeld func(ownerKey string) bool
)

// BrowserControlWait és quant espera una acció de l'agent que la persona
// retorni el control abans de rendir-se. Variable per als tests.
var BrowserControlWait = 5 * time.Minute

// SetBrowserControlGuard connecta la lease de control (internal/web).
func SetBrowserControlGuard(held func(ownerKey string) bool) {
	browserControlMu.Lock()
	browserControlHeld = held
	browserControlMu.Unlock()
}

// BrowserControlHeld diu si la persona té ara el control del navegador
// d'aquest abast (sessió).
func BrowserControlHeld(scope string) bool {
	browserControlMu.RLock()
	held := browserControlHeld
	browserControlMu.RUnlock()
	return held != nil && held(BrowserOwnerKey(scope))
}

// ErrBrowserUserControl: la persona té el control i no l'ha retornat a temps.
var ErrBrowserUserControl = fmt.Errorf("browser: NOT RUN: the person has taken control of the computer and has not given it back yet. Do not retry straight away and do not take screenshots; tell them you will carry on when they give control back")

// waitBrowserControl espera que la persona retorni el control (o que el
// torn s'aturi). Sense lease, torna de seguida.
func waitBrowserControl(ctx context.Context, scope string) error {
	if !BrowserControlHeld(scope) {
		return nil
	}
	deadline := time.Now().Add(BrowserControlWait)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for BrowserControlHeld(scope) {
		if !time.Now().Before(deadline) {
			return ErrBrowserUserControl
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("browser: stopped while the person had control: %v", ctx.Err())
		case <-tick.C:
		}
	}
	return nil
}

// LoginCheck és el que se sap de la pàgina després d'un inici de sessió fet
// per la persona. Mai cap valor de camp: només on som i si encara hi ha un
// formulari de credencials o de codi.
type LoginCheck struct {
	// Page és origen + camí (sense query ni fragment: hi pot haver testimonis).
	Page  string `json:"page"`
	Title string `json:"title"`
	// PasswordField / CodeField: encara hi ha un camp de contrasenya o de
	// codi d'un sol ús visible (login a mig fer o 2FA pendent).
	PasswordField bool `json:"passwordField"`
	CodeField     bool `json:"codeField"`
}

// jsLoginCheck no llegeix cap .value: només la presència dels camps.
const jsLoginCheck = `(() => {
  const vis = e => !!(e && (e.offsetWidth || e.offsetHeight || e.getClientRects().length));
  const any = sel => Array.from(document.querySelectorAll(sel)).some(vis);
  return JSON.stringify({
    page: location.origin + location.pathname,
    title: (document.title || '').slice(0, 160),
    passwordField: any('input[type=password]'),
    codeField: any('input[autocomplete=one-time-code], input[name*=otp i], input[id*=otp i], input[name*=totp i]'),
  });
})()`

// SafePageURL treu query i fragment d'una adreça (poden portar testimonis).
func SafePageURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}

// BrowserPageURL és la pàgina on és ara el navegador de la sessió (sense
// query ni fragment), o buit si no n'hi ha cap d'obert.
func BrowserPageURL(scope string) string {
	b := sessionBrowser(scope)
	if b == nil {
		return ""
	}
	b.mu.Lock()
	last := b.lastURL
	b.mu.Unlock()
	if u, err := b.evalString("location.href"); err == nil {
		last = u
	}
	return SafePageURL(last)
}

// BrowserLoginCheck comprova, després d'un «ja està» de la persona, on és
// el navegador de la sessió. Si la pestanya s'ha perdut (reinici), torna a
// obrir fallback (la pàgina on es va demanar l'inici de sessió): les galetes
// són al perfil persistent, així que es veu si la sessió hi és.
func BrowserLoginCheck(ctx context.Context, scope, fallback string) (LoginCheck, error) {
	if err := waitBrowserControl(ctx, scope); err != nil {
		return LoginCheck{}, err
	}
	b, err := GetBrowserWant(scope, wantFromArgs(scope, browserArgs{}))
	if err != nil {
		return LoginCheck{}, err
	}
	actx, cancel := context.WithTimeout(ctx, BrowserActionTimeout)
	defer cancel()
	b.mu.Lock()
	b.actx = actx
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.actx = nil
		b.mu.Unlock()
	}()
	cur, _ := b.evalString("location.href")
	if SafePageURL(cur) == "" && SafePageURL(fallback) != "" {
		if err := b.Navigate(SafePageURL(fallback)); err != nil {
			return LoginCheck{}, err
		}
	}
	_ = b.esperaCarrega()
	raw, err := b.evalString(jsLoginCheck)
	if err != nil {
		return LoginCheck{}, err
	}
	var lc LoginCheck
	if err := json.Unmarshal([]byte(raw), &lc); err != nil {
		return LoginCheck{}, fmt.Errorf("browser: login check: %v", err)
	}
	lc.Page = SafePageURL(lc.Page)
	b.mu.Lock()
	b.ax = nil
	b.mu.Unlock()
	return lc, nil
}
