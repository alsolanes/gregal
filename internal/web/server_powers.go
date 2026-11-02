package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/internal/session"
)

// Poders de servidor: l'administrador deixa que els SEUS vents facin de
// sysadmin (com un agent amb terminal al servidor, a l'estil Hermes): shell
// sense la llista de «perilloses», baixades d'internet, editar webs servides
// des de la màquina i obrir túnels de Cloudflare. Desactivat per defecte; mai
// per als comptes que no són admin. Les notes del servidor són la memòria
// compartida de com està muntada la màquina: els vents les llegeixen a cada
// torn i les van posant al dia.

type serverPowersCfg struct {
	Enabled bool `json:"enabled"`
}

func serverDataDir() string    { return filepath.Dir(session.DefaultDir()) }
func serverPowersFile() string { return filepath.Join(serverDataDir(), "server-powers.json") }
func serverNotesFile() string  { return filepath.Join(serverDataDir(), "server-notes.md") }

const serverNotesMaxBrief = 4000

func serverPowersOn() bool {
	raw, err := os.ReadFile(serverPowersFile())
	if err != nil {
		return false
	}
	var c serverPowersCfg
	return json.Unmarshal(raw, &c) == nil && c.Enabled
}

func setServerPowers(on bool) error {
	raw, _ := json.MarshalIndent(serverPowersCfg{Enabled: on}, "", "  ")
	if err := os.MkdirAll(serverDataDir(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(serverPowersFile(), append(raw, '\n'), 0o600)
}

// hasServerPowers: els poders estan activats i aquesta sessió és de
// l'administrador (en local, sense comptes, la persona ho és).
func (s *Server) hasServerPowers() bool {
	if !serverPowersOn() {
		return false
	}
	u := s.user
	if u == nil || u.Tothom {
		return true
	}
	// La sessió guarda l'usuari de quan es va obrir: si després l'han fet
	// (o desfet) administrador, mana el que hi ha ara.
	if admin, ok := s.users.IsAdmin(u.Name); ok {
		return admin
	}
	return u.Admin
}

func serviceHome() string {
	h, _ := os.UserHomeDir()
	return h
}

// serverEnv és el que el vent ha de saber de la màquina (cau 5 minuts).
type serverEnvInfo struct {
	OS, User, Home string
	Tools          map[string]bool
}

var (
	serverEnvMu   sync.Mutex
	serverEnvAt   time.Time
	serverEnvLast serverEnvInfo
)

var serverEnvTools = []string{"cloudflared", "caddy", "nginx", "node", "npm", "python3", "git", "docker", "systemctl", "rsync"}

func serverEnv() serverEnvInfo {
	serverEnvMu.Lock()
	defer serverEnvMu.Unlock()
	if time.Since(serverEnvAt) < 5*time.Minute && serverEnvLast.Tools != nil {
		return serverEnvLast
	}
	info := serverEnvInfo{OS: runtime.GOOS + "/" + runtime.GOARCH, Home: serviceHome(), Tools: map[string]bool{}}
	if u, err := user.Current(); err == nil {
		info.User = u.Username
	}
	for _, t := range serverEnvTools {
		_, err := exec.LookPath(t)
		info.Tools[t] = err == nil
	}
	if p, err := cloudflaredBin(); err == nil && p != "" {
		info.Tools["cloudflared"] = true
	}
	serverEnvLast, serverEnvAt = info, time.Now()
	return info
}

func readServerNotes() string {
	raw, err := os.ReadFile(serverNotesFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// windServerBrief és el que llegeix el vent de l'administrador amb poders.
func (s *Server) windServerBrief() string {
	if !s.hasServerPowers() {
		return ""
	}
	env := serverEnv()
	var have, missing []string
	for _, t := range serverEnvTools {
		if env.Tools[t] {
			have = append(have, t)
		} else {
			missing = append(missing, t)
		}
	}
	public := strings.TrimSpace(os.Getenv("GREGAL_PUBLIC_URL"))
	if public == "" {
		public = "none configured (links use the address the user connects with)"
	}
	var tunnels []string
	for _, t := range tunnelsMgr.list(s.usuari()) {
		tunnels = append(tunnels, fmt.Sprintf("%v → %v (%v)", t["url"], t["target"], t["status"]))
	}
	tunnelLine := "none"
	if len(tunnels) > 0 {
		tunnelLine = strings.Join(tunnels, "; ")
	}
	notes := readServerNotes()
	if r := []rune(notes); len(r) > serverNotesMaxBrief {
		notes = string(r[:serverNotesMaxBrief]) + "\n…(truncated: read the file for the rest)"
	}
	if notes == "" {
		notes = "(empty: nothing recorded yet)"
	}
	var b strings.Builder
	b.WriteString("\n\nSERVER POWERS: the administrator gave you a terminal on this server, like a sysadmin assistant. You do real server work yourself: build and change websites, including the ones this machine already serves, deploy and publish.")
	fmt.Fprintf(&b, "\n- Machine: %s, user %s, home %s. Installed: %s.", env.OS, env.User, env.Home, strings.Join(have, ", "))
	if len(missing) > 0 {
		fmt.Fprintf(&b, " Not installed: %s.", strings.Join(missing, ", "))
	}
	fmt.Fprintf(&b, "\n- Public address of this Gregal: %s. Tunnels open: %s.", public, tunnelLine)
	b.WriteString("\n- You may run any shell command (git, npm, pip, builds, curl/wget to the internet, ssh) and edit files anywhere the system lets you. Never: sudo (nobody can type its password), stopping or restarting Gregal, rm -rf /, piping a download into a shell, or touching ~/.ssh and Gregal's own config. If the system refuses a write (read-only file system, permission denied), say which folder and that the administrator can allow it with: sudo gregalctl site-allow <folder>.")
	b.WriteString("\n- To put something online: a static site → publish; something that needs a server → app (stays running, restarts itself); on the internet with the user's domain or a quick link → tunnel (Cloudflare). Report the final URL.")
	b.WriteString("\n- Before changing a live site, look at how it is served and copy it first when the change is big (cp -a site site.bak-$(date +%Y%m%d-%H%M)); afterwards check it responds (curl -I the URL) and say what you changed.")
	fmt.Fprintf(&b, "\n- SERVER NOTES (%s): what the winds learned about this server (sites, their folders and domains, how each is deployed, tunnels). Read them first; when you learn something durable, add or correct one short line there with edit/write. Never store passwords or keys in them.\n%s", serverNotesFile(), notes)
	return b.String()
}

// handleServerPowers: GET l'estat (entorn, notes, túnels); POST {enabled?, notes?}.
func (s *Server) handleServerPowers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		env := serverEnv()
		tools := make([]string, 0, len(env.Tools))
		for t, ok := range env.Tools {
			if ok {
				tools = append(tools, t)
			}
		}
		sort.Strings(tools)
		writeJSON(w, map[string]any{
			"enabled": serverPowersOn(), "notes": readServerNotes(), "notes_path": serverNotesFile(),
			"os": env.OS, "user": env.User, "home": env.Home, "tools": tools,
			"tunnels": tunnelsMgr.list(s.usuari()),
		})
	case http.MethodPost:
		var req struct {
			Enabled *bool   `json:"enabled"`
			Notes   *string `json:"notes"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		if req.Enabled != nil {
			if err := setServerPowers(*req.Enabled); err != nil {
				http.Error(w, "cannot save", 500)
				return
			}
		}
		if req.Notes != nil {
			if len(*req.Notes) > 64<<10 {
				http.Error(w, "notes too long (64 KB max)", 400)
				return
			}
			if err := os.WriteFile(serverNotesFile(), []byte(strings.TrimSpace(*req.Notes)+"\n"), 0o600); err != nil {
				http.Error(w, "cannot save notes", 500)
				return
			}
		}
		writeJSON(w, map[string]any{"enabled": serverPowersOn()})
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", 405)
	}
}
