package web

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/mcp"
)

// Pàgina de connectors: connectar (OAuth), desconnectar, afegir i treure
// servidors MCP. Afegir i treure escriuen el config.yaml amb Config.Save
// (com /api/mode); les rutes que canvien res són només de l'admin, perquè
// un servidor stdio és una comanda que s'executa a la màquina.

// mcpManager és el gestor viu (en crea un de buit si no n'hi havia: el
// primer servidor afegit des de la UI).
func (s *Server) mcpManager() *mcp.Manager {
	if s.mcp != nil {
		return s.mcp
	}
	if m := currentMCP(); m != nil {
		return m
	}
	mcpMu.Lock()
	defer mcpMu.Unlock()
	if activeMCP == nil {
		activeMCP = mcp.Setup(nil)
	}
	return activeMCP
}

// mcpPromptRows són els prompts per a la paleta d'ordres.
func mcpPromptRows(m *mcp.Manager) []map[string]any {
	out := []map[string]any{}
	for _, p := range m.Prompts() {
		args := []string{}
		for _, a := range p.Arguments {
			n := a.Name
			if a.Required {
				n += "*"
			}
			args = append(args, n)
		}
		out = append(out, map[string]any{"command": "/" + p.Command(), "server": p.Server, "name": p.Name, "description": p.Description, "arguments": args})
	}
	return out
}

// oauthRedirect és l'URL de callback tal com el navegador veu aquest
// servidor (el desktop i la web obren la UI pel mateix host).
func oauthRedirect(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host + mcp.OAuthCallbackPath
}

func readName(r *http.Request) (string, error) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16)).Decode(&req); err != nil {
		return "", err
	}
	return strings.TrimSpace(req.Name), nil
}

// handleMCPConnect connecta (o reconnecta) un servidor. Si cal OAuth torna
// auth_url: la UI l'obre al navegador i el callback acaba la connexió.
func (s *Server) handleMCPConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	name, err := readName(r)
	if err != nil || name == "" {
		http.Error(w, "name required", 400)
		return
	}
	m := s.mcpManager()
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	authURL, err := m.Connect(ctx, name, oauthRedirect(r))
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "servers": m.Status()})
		return
	}
	if authURL != "" {
		writeJSON(w, map[string]any{"status": mcp.StatusNeedsAuth, "auth_url": authURL})
		return
	}
	writeJSON(w, map[string]any{"status": mcp.StatusConnected, "servers": m.Status()})
}

// handleMCPDisconnect oblida el token d'un servidor remot i el desconnecta.
func (s *Server) handleMCPDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	name, err := readName(r)
	if err != nil || name == "" {
		http.Error(w, "name required", 400)
		return
	}
	m := s.mcpManager()
	_ = m.Disconnect(name)
	writeJSON(w, map[string]any{"servers": m.Status()})
}

// mcpServerReq és el servidor que la UI vol afegir.
type mcpServerReq struct {
	Name      string            `json:"name"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	URL       string            `json:"url"`
	Transport string            `json:"transport"`
	Headers   map[string]string `json:"headers"`
	ClientID  string            `json:"client_id"`
	Scopes    []string          `json:"scopes"`
	// ReadOnly marca les eines del servidor com a lectura (sense permís).
	ReadOnly bool `json:"readonly"`
	// Preset "demo" omple la resta: la base de dades de demostració amb
	// aquest mateix executable (`gregal mcp demo`), readonly.
	Preset string `json:"preset"`
}

// demoExecutable és os.Executable (els tests el substitueixen: el binari
// de test no sap fer de `gregal mcp demo`).
var demoExecutable = os.Executable

// demoServer és el servidor MCP de la base de dades de demostració: el
// binari que corre ara mateix amb `mcp demo`.
func demoServer() (config.MCPServerCfg, error) {
	exe, err := demoExecutable()
	if err != nil {
		return config.MCPServerCfg{}, err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	_ = exe
	// «gregal» es resol al binari actual en cada arrencada (mcp.ResolveCommand):
	// desar el camí absolut trencava el connector quan l'app portable canviava
	// de carpeta temporal.
	return config.MCPServerCfg{Command: "gregal", Args: []string{"mcp", "demo"}, ReadOnly: true}, nil
}

func init() {
	mcp.SelfExecutable = func() (string, error) { return demoExecutable() }
}

// handleMCPServers afegeix (POST) o treu (DELETE ?name=) un servidor del
// config i del gestor viu.
func (s *Server) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	m := s.mcpManager()
	switch r.Method {
	case http.MethodPost:
		var req mcpServerReq
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			http.Error(w, "unreadable request", 400)
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Preset == "demo" && req.Name == "" {
			req.Name = "demo"
		}
		if req.Name == "" || strings.ContainsAny(req.Name, " /\\:") {
			http.Error(w, "invalid name (no spaces, slashes or colons)", 400)
			return
		}
		sc := config.MCPServerCfg{
			Command: strings.TrimSpace(req.Command), Args: req.Args, Env: req.Env,
			URL: strings.TrimSpace(req.URL), Transport: strings.TrimSpace(req.Transport), Headers: req.Headers,
			ReadOnly: req.ReadOnly,
		}
		if req.ClientID != "" || len(req.Scopes) > 0 {
			sc.OAuth = &config.MCPOAuthCfg{ClientID: strings.TrimSpace(req.ClientID), Scopes: req.Scopes}
		}
		switch req.Preset {
		case "":
		case "demo":
			demo, err := demoServer()
			if err != nil {
				http.Error(w, "cannot locate the gregal executable: "+err.Error(), 500)
				return
			}
			sc = demo
		default:
			http.Error(w, "unknown preset (demo)", 400)
			return
		}
		if err := config.ValidaMCP(req.Name, sc); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		s.mu.Lock()
		prev, had := s.cfg.MCP[req.Name]
		if s.cfg.MCP == nil {
			s.cfg.MCP = map[string]config.MCPServerCfg{}
		}
		s.cfg.MCP[req.Name] = sc
		err := s.cfg.Save(s.cfgPath)
		if err != nil {
			if had {
				s.cfg.MCP[req.Name] = prev
			} else {
				delete(s.cfg.MCP, req.Name)
			}
		}
		s.mu.Unlock()
		if err != nil {
			http.Error(w, "not saved: "+err.Error(), 500)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		connErr := m.Add(ctx, req.Name, mcp.ServerFromConfig(sc))
		out := map[string]any{"saved": true, "servers": m.Status()}
		if connErr != nil && !mcp.IsAuthRequired(connErr) {
			out["error"] = connErr.Error()
		}
		writeJSON(w, out)
	case http.MethodDelete:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			http.Error(w, "name required", 400)
			return
		}
		s.mu.Lock()
		prev, had := s.cfg.MCP[name]
		delete(s.cfg.MCP, name)
		err := s.cfg.Save(s.cfgPath)
		if err != nil && had {
			s.cfg.MCP[name] = prev
		}
		s.mu.Unlock()
		if err != nil {
			http.Error(w, "not saved: "+err.Error(), 500)
			return
		}
		m.Remove(name)
		writeJSON(w, map[string]any{"saved": true, "servers": m.Status()})
	default:
		http.Error(w, "POST or DELETE", http.StatusMethodNotAllowed)
	}
}

// handleMCPPrompt expandeix un prompt MCP per posar-lo al compositor.
func (s *Server) handleMCPPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Text string            `json:"text"`
		Args map[string]string `json:"args"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "unreadable request", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var (
		out string
		ok  bool
		err error
	)
	if req.Args != nil {
		// Arguments ja separats (la paleta els demana un per un).
		out, ok, err = s.mcpManager().ExpandPrompt(ctx, req.Text, req.Args)
	} else {
		out, ok, err = s.mcpManager().ExpandPromptCommand(ctx, req.Text)
	}

	if !ok {
		http.Error(w, "not an MCP prompt", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]any{"text": out})
}

// handleMCPOAuthCallback rep la redirecció del servidor d'autorització. No
// porta el token de Gregal (ve d'un lloc extern): l'autentica el state
// d'un sol ús que només coneix qui ha començat l'autorització.
func (s *Server) handleMCPOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET", http.StatusMethodNotAllowed)
		return
	}
	_, _ = s.mcpManager().HandleCallback(w, r)
}

func writeJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
