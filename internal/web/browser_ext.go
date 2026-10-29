package web

// Connexió de l'extensió de Chrome per deixar que l'agent faci servir el
// Chrome de l'usuari (tools/browser_ext.go). És un websocket que obre
// l'extensió: només s'accepta des d'un origen chrome-extension:// (una
// pàgina web no pot fer-se passar per l'extensió) i la clau va al primer
// missatge, perquè un websocket del navegador no pot posar capçaleres.

import (
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/net/websocket"

	"gregal/internal/tools"
)

const browserExtensionPath = "/api/browser/extension"

func (h *Hub) handleBrowserExtension(w http.ResponseWriter, r *http.Request) {
	s := h.def
	srv := websocket.Server{
		Handshake: func(c *websocket.Config, r *http.Request) error {
			if !strings.HasPrefix(r.Header.Get("Origin"), "chrome-extension://") {
				return fmt.Errorf("only the Gregal Chrome extension can connect here")
			}
			return nil
		},
		Handler: func(ws *websocket.Conn) {
			tools.ServeBrowserExtension(ws, s.browserExtensionKeyOK)
		},
	}
	srv.ServeHTTP(w, r)
}

// browserExtensionKeyOK valida la clau del hello amb les mateixes regles que
// requireUser: sense token ni usuaris (l'app d'escriptori), tot val.
func (s *Server) browserExtensionKeyOK(key string) bool {
	if strings.TrimSpace(s.Token) == "" && (s.users == nil || !s.users.Actiu()) {
		return true
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	if s.users != nil {
		if u, ok := s.users.Resolve(key); ok && u != nil && u.Admin {
			return true
		}
	}
	return s.Token != "" && !subtleCompare(s.Token, key)
}

// handleBrowserExtensionStatus: si el Chrome de l'usuari està connectat.
func (s *Server) handleBrowserExtensionStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, tools.BrowserExtensionStatus())
}

// browserReason posa en la llengua de la sessió el motiu del permís del
// navegador (tools.BrowserGate el dona en anglès, per al model).
func (s *Server) browserReason(r string) string {
	if s.tr("ca", "en") != "ca" {
		return r
	}
	cut := func(prefix, until string) (string, bool) {
		if !strings.HasPrefix(r, prefix) {
			return "", false
		}
		rest := strings.TrimPrefix(r, prefix)
		if i := strings.Index(rest, until); i >= 0 {
			rest = rest[:i]
		}
		return rest, true
	}
	if h, ok := cut("browser: use your Chrome on ", " ("); ok {
		return "Navegador: fer servir el teu Chrome a " + h + " (un cop permès, Gregal hi podrà llegir i actuar durant aquesta sessió)"
	}
	if h, ok := cut("browser: act on ", " ("); ok {
		return "Navegador: actuar a " + h + " (un cop permès, Gregal hi podrà clicar i escriure durant aquesta sessió)"
	}
	if n, ok := cut("browser: irreversible action ", " — "); ok {
		return "Navegador: acció irreversible " + n + ". Confirma-la."
	}
	if strings.HasPrefix(r, "browser: accept a page dialog") {
		return "Navegador: acceptar un diàleg de la pàgina (normalment «n'estàs segur?»). Confirma-ho."
	}
	return r
}
