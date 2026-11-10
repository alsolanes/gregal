package web

import (
	"context"
	"net/http"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// El que un producte de fora (product.go) pot fer servir del servidor d'un
// usuari. És poc a propòsit: el que necessiten els jocs (jocs/server) i res
// més. Tot llegeix sota el candau del servidor, com fa el codi d'aquí.

// Global lliga un handler d'aplicació a la sessió default de qui pregunta
// (com les rutes de plugin_*.go): la mateixa autenticació que la resta.
func (h *Hub) Global(fn func(*Server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return h.global(fn)
}

// APIRoutes construeix les rutes del Hub i en torna les /api/ (amb les dels
// productes registrats): el test del contracte de cada producte les
// compara amb docs/api-contract.md.
func (h *Hub) APIRoutes() []string {
	h.mux()
	return append([]string(nil), routes...)
}

// UserFrom és l'usuari de la petició (nil en mode local, sense comptes, i
// per als convidats d'un producte).
func UserFrom(r *http.Request) *User { return userFrom(r) }

// WithUser torna la petició amb aquest usuari, com si hagués passat per
// l'autenticació (per a les proves dels productes).
func WithUser(r *http.Request, u *User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, u))
}

// ChatModel és el model de la conversa ara mateix: config, client, proveïdor
// i rol (amb el model i el nivell de raonament triats a la UI). cfg o client
// poden ser nil si no hi ha res configurat.
func (s *Server) ChatModel() (*config.Config, *llm.Client, config.Provider, config.Role) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		return nil, s.client, config.Provider{}, config.Role{}
	}
	p, r := s.roleRef()
	return s.cfg, s.client, p, r
}

// WindModel crida el model dels vents (el mateix que fa servir el Racó).
func (s *Server) WindModel(ctx context.Context, system, user string, maxTokens int) (string, error) {
	return cornerModel(s, ctx, system, user, maxTokens)
}

// Tr tria el text en català o en anglès segons l'idioma del config.
func (s *Server) Tr(ca, en string) string { return s.tr(ca, en) }

// StateDir és el directori d'estat compartit del servidor (al costat del
// config): hi viu el que és de tots els usuaris.
func (s *Server) StateDir() string { return goalDirFor(s.cfgPath) }

// UserStateDir és el directori d'estat personal de l'usuari (el mateix
// que fan servir els vents: al costat del config, o el de l'usuari amb
// comptes).
func (s *Server) UserStateDir() string { return s.windStateDir() }
