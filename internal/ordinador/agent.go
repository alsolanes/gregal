// Package ordinador és l'ordinador personal del pla v2 (§5, paquet B2) en
// dues peces:
//
//   - L'agent de l'ordinador (aquest fitxer) corre dins la VM de cada
//     persona: diu la seua salut i executa les eines que li envia
//     l'amfitrió (navegador, terminal, fitxers) amb el runner de sempre.
//     No té comptes, converses ni webs: això és de l'amfitrió.
//   - El gestor de l'amfitrió (host.go) sap quin ordinador és de qui,
//     l'encén quan una feina el necessita, el desa a disc per inactivitat i en
//     dona la salut honesta encara que la VM estigui apagada.
//
// El canal és HTTP amb un testimoni secret per ordinador (Bearer) sobre una
// xarxa privada (WireGuard/Tailscale o la xarxa interna del proveïdor). Ni
// l'agent ni el CDP ni el VNC escolten mai en una adreça pública.
package ordinador

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/identity"
	"gregal/internal/tools"
)

// Health és la resposta de GET /v1/health de l'agent.
type Health struct {
	ID           string                 `json:"id"`
	Owner        string                 `json:"owner"`
	Health       string                 `json:"health"`
	Capabilities []string               `json:"capabilities"`
	ControlOwner string                 `json:"controlOwner"`
	BrowserMode  string                 `json:"browserMode,omitempty"`
	Checks       []tools.OrdinadorCheck `json:"checks"`
}

// ExecRequest és una crida d'eina de l'amfitrió a l'ordinador.
type ExecRequest struct {
	// Owner és el compte de l'amfitrió que fa la feina; ha de ser el
	// propietari de l'ordinador.
	Owner string `json:"owner"`
	// Session és la clau de sessió de l'amfitrió (identity.Key + "-" + id):
	// dona una pestanya per feina al navegador del propietari.
	Session string `json:"session"`
	Tool    string `json:"tool"`
	Args    string `json:"args"`
	// Dir és relatiu a la carpeta de fitxers de l'ordinador ("" = l'arrel).
	Dir string `json:"dir,omitempty"`
}

// ExecResponse és el resultat de l'eina.
type ExecResponse struct {
	Output string   `json:"output"`
	Images []string `json:"images,omitempty"`
	Error  string   `json:"error,omitempty"`
}

// AgentConfig configura l'agent de l'ordinador.
type AgentConfig struct {
	ID       string
	Owner    string
	FilesDir string
	Token    string
	// ViewerAddr és el noVNC de la VM (loopback, p. ex. 127.0.0.1:6080).
	// L'amfitrió hi arriba per /v1/viewer/ amb el mateix testimoni (B3).
	// Buit = sense visor.
	ViewerAddr string
}

// computerTools són les eines que l'ordinador executa. La resta (vents,
// publicar, correu, memòria…) són de l'amfitrió i aquí es deneguen.
var computerTools = map[string]bool{
	"browser": true, "bash": true, "bash_background": true, "bash_output": true, "bash_kill": true,
	"read": true, "write": true, "edit": true, "patch": true, "grep": true, "glob": true,
	"read_image": true, "office_read": true, "office_create": true, "office_edit": true, "office_render": true,
	"pdf_read": true, "pdf_create": true, "pdf_edit": true,
}

// execTool és el runner de sempre; variable perquè els tests no executin res.
var execTool = agent.ExecCtx

// MinTokenLen: el testimoni el genera setup.sh (32 bytes en hex).
const MinTokenLen = 32

const maxExecBody = 4 << 20

// NewAgent torna el handler HTTP de l'agent.
func NewAgent(cfg AgentConfig) (http.Handler, error) {
	if len(cfg.Token) < MinTokenLen {
		return nil, fmt.Errorf("ordinador: el testimoni ha de tenir com a mínim %d caràcters", MinTokenLen)
	}
	if cfg.Owner == "" {
		return nil, errors.New("ordinador: cal el propietari (--owner)")
	}
	if cfg.ID == "" {
		return nil, errors.New("ordinador: cal l'identificador (--id)")
	}
	if cfg.FilesDir == "" || !filepath.IsAbs(cfg.FilesDir) {
		return nil, errors.New("ordinador: la carpeta de fitxers ha de ser un camí absolut")
	}
	a := &agentServer{cfg: cfg, ownerKey: identity.Key(cfg.Owner)}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", a.auth(a.health))
	mux.HandleFunc("/v1/exec", a.auth(a.exec))
	if cfg.ViewerAddr != "" {
		vp, err := NewLocalViewerProxy(cfg.ViewerAddr, "/v1/viewer/")
		if err != nil {
			return nil, fmt.Errorf("ordinador: --viewer: %v", err)
		}
		mux.HandleFunc("/v1/viewer/", a.auth(vp.ServeHTTP))
	}
	return mux, nil
}

type agentServer struct {
	cfg      AgentConfig
	ownerKey string
}

func (a *agentServer) auth(next http.HandlerFunc) http.HandlerFunc {
	want := []byte("Bearer " + a.cfg.Token)
	return func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (a *agentServer) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	// A l'ordinador hi ha un sol propietari; amb el Chrome supervisat el
	// perfil que es comprova és GREGAL_ORDINADOR_PROFILE.
	p := tools.ProbeOrdinador(r.Context(), a.ownerKey, a.cfg.FilesDir)
	writeJSON(w, http.StatusOK, Health{
		ID: a.cfg.ID, Owner: a.cfg.Owner, Health: p.Health, Capabilities: p.Capabilities,
		ControlOwner: p.ControlOwner, BrowserMode: p.BrowserMode, Checks: p.Checks,
	})
}

func (a *agentServer) exec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req ExecRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, maxExecBody)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	// Defensa en profunditat: l'amfitrió ja filtra per propietari, però
	// l'ordinador tampoc accepta feina d'un altre compte.
	if req.Owner != a.cfg.Owner {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "this computer belongs to another account"})
		return
	}
	if k := tools.BrowserOwnerKey(req.Session); k != tools.OrdinadorLocal && k != a.ownerKey {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "session of another account"})
		return
	}
	if !computerTools[req.Tool] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tool not available on the computer: " + req.Tool})
		return
	}
	dir, err := a.workDir(req.Dir)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	session := req.Session
	if session == "" {
		session = "default"
	}
	out, imgs, err := execTool(r.Context(), session, dir, req.Tool, req.Args)
	resp := ExecResponse{Output: out, Images: imgs}
	if err != nil {
		resp.Error = err.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// workDir resol el directori de treball dins la carpeta de fitxers; mai
// en surt.
func (a *agentServer) workDir(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return a.cfg.FilesDir, nil
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) {
		return "", errors.New("dir must be relative to the computer's files folder")
	}
	full := filepath.Join(a.cfg.FilesDir, filepath.Clean(rel))
	back, err := filepath.Rel(a.cfg.FilesDir, full)
	if err != nil || back == ".." || strings.HasPrefix(back, ".."+string(filepath.Separator)) {
		return "", errors.New("dir escapes the computer's files folder")
	}
	if err := os.MkdirAll(full, 0o700); err != nil {
		return "", err
	}
	return full, nil
}

// CheckPrivateAddr accepta només adreces de loopback o de xarxa privada
// (RFC 1918, CGNAT de Tailscale 100.64/10, ULA fc00::/7). Mai 0.0.0.0 ni una
// IP pública: l'agent no s'exposa a Internet.
func CheckPrivateAddr(hostport string) error {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return fmt.Errorf("adreça %q: cal host:port", hostport)
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("adreça %q: cal una IP (no un nom) per poder comprovar que és privada", hostport)
	}
	if privateIP(ip) {
		return nil
	}
	return fmt.Errorf("adreça %q: no és privada ni loopback; l'ordinador no s'exposa a Internet", hostport)
}

var cgnat = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func privateIP(ip net.IP) bool {
	if ip.IsUnspecified() {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || cgnat.Contains(ip)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// ServeAgent escolta a addr (privada) fins que ctx s'acabi.
func ServeAgent(ctx context.Context, addr string, h http.Handler) error {
	if err := CheckPrivateAddr(addr); err != nil {
		return err
	}
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sh, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sh)
	}()
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
