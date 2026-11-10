package server

import (
	"context"
	"net/http"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/web"
)

// Host és el que els jocs fan servir del servidor de Gregal d'un usuari.
// *web.Server el compleix (internal/web/product_host.go); els tests en
// poden posar un de propi.
type Host interface {
	// ChatModel és el model de la conversa: l'Arena hi juga.
	ChatModel() (*config.Config, *llm.Client, config.Provider, config.Role)
	// WindModel crida el model dels vents: la lliga hi narra.
	WindModel(ctx context.Context, system, user string, maxTokens int) (string, error)
	Tr(ca, en string) string
	// StateDir és l'estat compartit (l'Arena és de tot el servidor).
	StateDir() string
	// WindStateDir és l'estat dels vents de l'usuari (la seva lliga).
	WindStateDir() string
	Winds() *companion.WindStore
	RecordWindNews(w companion.Wind, source, text string)
}

var _ Host = (*web.Server)(nil)

// Server és el servidor dels jocs per a un usuari: embolica el seu servidor
// de Gregal (la sessió default de qui fa la petició) i només hi demana el
// que diu Host. L'estat viu dels jocs (partits en joc, competicions, cua
// de veus) és del paquet.
type Server struct {
	host Host
}

// New embolica el servidor de Gregal d'un usuari.
func New(h Host) *Server { return &Server{host: h} }

// global lliga un handler dels jocs a una ruta d'aplicació del Hub: la
// mateixa autenticació i la mateixa sessió que les rutes de Gregal.
func global(h *web.Hub, fn func(*Server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return h.Global(func(ws *web.Server, w http.ResponseWriter, r *http.Request) { fn(New(ws), w, r) })
}

// tr tria el text en català o en anglès segons el config.
func (s *Server) tr(ca, en string) string { return s.host.Tr(ca, en) }

// windStateDir és on viu l'estat dels vents de l'usuari.
func (s *Server) windStateDir() string { return s.host.WindStateDir() }
