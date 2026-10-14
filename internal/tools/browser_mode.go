package tools

// On corre el navegador de l'agent (config `browser.mode`):
//
//   - auto (per defecte): `app` si l'app d'escriptori ha engegat aquest
//     backend i hi ha registrat el pont del seu navegador; si no, headless.
//   - app: el panell de navegador de l'app d'escriptori (l'usuari ho veu i
//     pot prendre el control). Sense pont, headless amb una nota.
//   - headless: Chrome/Edge de la màquina sense finestra.
//   - window: Chrome/Edge amb finestra visible (per iniciar sessió a mà al
//     perfil del gregal).
//
// GREGAL_BROWSER_HEADLESS=1 mana sobre el config i força headless.

import (
	"os"
	"strings"
	"sync"
)

const (
	BrowserModeAuto     = "auto"
	BrowserModeApp      = "app"
	BrowserModeHeadless = "headless"
	BrowserModeWindow   = "window"
)

// BrowserModes són els valors acceptats a browser.mode.
var BrowserModes = []string{BrowserModeAuto, BrowserModeApp, BrowserModeHeadless, BrowserModeWindow}

var (
	browserModeMu  sync.Mutex
	browserModeCfg = BrowserModeAuto
)

// ValidBrowserMode diu si m és un browser.mode vàlid (buit = auto).
func ValidBrowserMode(m string) bool {
	m = strings.ToLower(strings.TrimSpace(m))
	if m == "" {
		return true
	}
	for _, v := range BrowserModes {
		if m == v {
			return true
		}
	}
	return false
}

// SetBrowserMode instal·la el browser.mode del config (config.Load el crida).
// Un navegador ja obert amb un altre mode es reobre a la propera acció.
func SetBrowserMode(m string) {
	m = strings.ToLower(strings.TrimSpace(m))
	if !ValidBrowserMode(m) || m == "" {
		m = BrowserModeAuto
	}
	browserModeMu.Lock()
	browserModeCfg = m
	browserModeMu.Unlock()
}

// chooseBrowserMode és la decisió pura: config, override d'entorn i si hi
// ha pont de l'app. Torna app, headless o window (mai auto).
func chooseBrowserMode(cfg string, forceHeadless, bridge bool) string {
	if forceHeadless {
		return BrowserModeHeadless
	}
	switch strings.ToLower(strings.TrimSpace(cfg)) {
	case BrowserModeApp:
		return BrowserModeApp
	case BrowserModeHeadless:
		return BrowserModeHeadless
	case BrowserModeWindow:
		return BrowserModeWindow
	}
	if bridge {
		return BrowserModeApp
	}
	return BrowserModeHeadless
}

// EffectiveBrowserMode és el mode que toca ara mateix. app pot acabar en
// headless si el pont falla en connectar (openBrowser ho anota).
func EffectiveBrowserMode() string {
	browserModeMu.Lock()
	cfg := browserModeCfg
	browserModeMu.Unlock()
	return chooseBrowserMode(cfg, os.Getenv("GREGAL_BROWSER_HEADLESS") == "1", browserAppBridge() != "")
}
