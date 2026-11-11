package web

// /api/ordinador: l'ordinador personal (paquet B2,
// docs/assistant-contract.ca.md) vist des de l'amfitrió.
//
// Dos modes:
//   - Amfitrió amb ordinadors (GREGAL_ORDINADORS, el pilot): cada compte té
//     una VM que s'encén quan cal (internal/ordinador). La salut ve del
//     hipervisor i de l'agent de la VM, amb topalls curts: VM apagada →
//     stopped, sense penjar-se. POST {action: wake|suspend|stop}.
//   - Sense registre (Gregal d'escriptori, servidor d'una màquina): la
//     mateixa màquina és l'ordinador i les comprovacions són locals
//     (tools.ProbeOrdinador).
//
// Propietat: cadascú veu només el seu. Un id d'un altre compte → 403; un id
// que no existeix → 404.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gregal/internal/ordinador"
	"gregal/internal/tools"
)

var (
	ordinadorHostOnce sync.Once
	ordinadorHostMgr  *ordinador.Manager
	ordinadorHostErr  error
)

// ordinadorHost és el gestor d'ordinadors de l'amfitrió (nil sense
// GREGAL_ORDINADORS). La primera crida engega el desat a disc per inactivitat.
// B4 el fa servir per a EnsureRunning/Exec i waiting_computer.
func ordinadorHost() (*ordinador.Manager, error) {
	ordinadorHostOnce.Do(func() {
		path := strings.TrimSpace(os.Getenv("GREGAL_ORDINADORS"))
		if path == "" {
			return
		}
		ordinadorHostMgr, ordinadorHostErr = ordinador.LoadRegistry(path)
		if ordinadorHostErr != nil {
			fmt.Fprintf(os.Stderr, "ordinadors: %v\n", ordinadorHostErr)
			return
		}
		go ordinadorHostMgr.RunIdleLoop(context.Background())
	})
	return ordinadorHostMgr, ordinadorHostErr
}

func ordinadorCaller(r *http.Request) (*User, string) {
	u := userFrom(r)
	if u != nil && !u.Tothom {
		return u, u.Name
	}
	return u, ""
}

// handleOrdinadorHost és /api/ordinador en mode amfitrió.
func (s *Server) handleOrdinadorHost(m *ordinador.Manager, w http.ResponseWriter, r *http.Request) {
	_, owner := ordinadorCaller(r)
	var c *ordinador.Computer
	if id := strings.TrimSpace(r.URL.Query().Get("id")); id != "" {
		c = m.ByID(id)
		if c == nil {
			writeJSONStatus(w, http.StatusNotFound, map[string]any{"error": "unknown computer"})
			return
		}
		if c.Owner != owner {
			writeJSONStatus(w, http.StatusForbidden, map[string]any{"error": "this computer belongs to another account"})
			return
		}
	} else {
		var err error
		if c, err = m.For(owner); err != nil {
			writeJSONStatus(w, http.StatusNotFound, map[string]any{"error": "no computer for this account"})
			return
		}
	}
	code := http.StatusOK
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		var req struct {
			Action string `json:"action"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req)
		var err error
		// Encendre a mà és «user» (obrir el visor serà «viewer», B3). Una
		// consulta de salut no l'encén mai.
		switch req.Action {
		case "wake":
			err = m.Wake(r.Context(), c, "user")
		case "suspend":
			err = m.Park(r.Context(), c, "user")
		case "stop":
			err = m.Stop(r.Context(), c, "user")
		default:
			writeJSONStatus(w, http.StatusBadRequest, map[string]any{"error": "action must be wake, suspend or stop"})
			return
		}
		if err != nil {
			writeJSONStatus(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
			return
		}
		code = http.StatusAccepted
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rep := m.Status(r.Context(), c)
	writeJSONStatus(w, code, map[string]any{
		"kind": "ordinador", "id": rep.ID, "owner": rep.Owner, "health": rep.Health,
		"capabilities": rep.Capabilities, "controlOwner": rep.ControlOwner,
		"leaseExpiry": nil, "lastSeenAt": rep.LastSeenAt, "browserMode": rep.BrowserMode,
		"power": rep.Power, "usage": rep.Usage, "checks": rep.Checks,
	})
}

var errOrdinadorConfig = errors.New("the computer registry (GREGAL_ORDINADORS) could not be loaded")

var (
	ordinadorSeenMu sync.Mutex
	// ordinadorSeen: última vegada que l'ordinador de cada propietari ha
	// respost sa (lastSeenAt). En memòria: en reiniciar, fins a la propera
	// comprovació, no se sap.
	ordinadorSeen = map[string]time.Time{}
)

// ordinadorID és l'identificador estable de l'ordinador d'un propietari.
func ordinadorID(ownerKey string) string {
	if v := strings.TrimSpace(os.Getenv("GREGAL_ORDINADOR_ID")); v != "" {
		return v
	}
	if ownerKey == tools.OrdinadorLocal {
		return "pc-local"
	}
	return "pc-" + ownerKey[:12]
}

func (s *Server) handleOrdinador(w http.ResponseWriter, r *http.Request) {
	if m, err := ordinadorHost(); err != nil {
		// Mai es cau al mode local: diria «running» d'una màquina que no és
		// l'ordinador de ningú.
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{"error": errOrdinadorConfig.Error()})
		return
	} else if m != nil {
		s.handleOrdinadorHost(m, w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := userFrom(r)
	owner := ""
	if u != nil && !u.Tothom {
		owner = u.Name
	}
	if declared := tools.OrdinadorOwnerName(); declared != "" && owner != "" && owner != declared {
		writeJSONStatus(w, http.StatusForbidden, map[string]any{"error": "this computer belongs to another user"})
		return
	}
	key := tools.OwnerKeyFor(owner)
	id := ordinadorID(key)
	if want := strings.TrimSpace(r.URL.Query().Get("id")); want != "" && want != id {
		writeJSONStatus(w, http.StatusNotFound, map[string]any{"error": "unknown computer"})
		return
	}
	p := tools.ProbeOrdinador(r.Context(), key, s.ordinadorFilesDir(u))

	ordinadorSeenMu.Lock()
	if p.Health == "running" {
		ordinadorSeen[key] = time.Now().UTC()
	}
	var lastSeen any
	if t, ok := ordinadorSeen[key]; ok {
		lastSeen = t.Format(time.RFC3339)
	}
	ordinadorSeenMu.Unlock()

	writeJSON(w, map[string]any{
		"kind":         "ordinador",
		"id":           id,
		"owner":        owner,
		"health":       p.Health,
		"capabilities": p.Capabilities,
		"controlOwner": p.ControlOwner,
		// La lease persona/agent és de B3: fins llavors, sense caducitat.
		"leaseExpiry": nil,
		"lastSeenAt":  lastSeen,
		"browserMode": p.BrowserMode,
		"checks":      p.Checks,
	})
}

// ordinadorFilesDir és la carpeta de resultats de l'ordinador:
// GREGAL_ORDINADOR_FILES a la VM; si no, la carpeta de treball de l'usuari
// o, en mode local, la de la sessió.
func (s *Server) ordinadorFilesDir(u *User) string {
	if v := strings.TrimSpace(os.Getenv("GREGAL_ORDINADOR_FILES")); v != "" {
		return v
	}
	if u != nil && !u.Tothom {
		if u.Home != "" {
			return u.Home
		}
		return u.Arrel()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}
