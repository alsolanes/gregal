// Package sandbox confina les ordres de shell de l'agent (eines bash i
// bash_background) a nivell de sistema operatiu, a sobre del classificador
// d'ordres i de les regles dures que ja hi havia.
//
// És opt-in (`sandbox.mode: off` per defecte) perquè canvia el que poden fer
// les ordres: amb el sandbox actiu, escriure fora del workspace (i dels camins
// que l'usuari hi afegeixi) falla. Els backends es trien sols segons la
// màquina:
//
//   - Linux: bubblewrap (bwrap) si hi és; si no, Landlock (syscalls del
//     kernel, sense dependències) a través d'un procés auxiliar de gregal.
//   - macOS: sandbox-exec amb un perfil generat.
//   - Qualsevol sistema (i l'única opció a Windows): Docker, només si es
//     demana explícitament amb `backend: docker`.
//
// Sense backend, `strict` es nega a executar res i `workspace` avisa una
// vegada i executa sense confinament: mai es fa veure que hi ha una protecció
// que no hi és.
package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"gregal/internal/shell"
)

// Modes.
const (
	ModeOff       = "off"
	ModeWorkspace = "workspace"
	ModeStrict    = "strict"
)

// Backends.
const (
	BackendAuto        = "auto"
	BackendNone        = "none"
	BackendBwrap       = "bwrap"
	BackendLandlock    = "landlock"
	BackendSandboxExec = "sandbox-exec"
	BackendDocker      = "docker"
)

// DefaultImage és la imatge del backend docker quan el config no en diu cap:
// petita i amb sh. Per compilar o passar tests cal posar-hi una imatge amb
// les eines del projecte (golang, node...).
const DefaultImage = "alpine:3.20"

// Config és la secció `sandbox:` del config.yaml.
type Config struct {
	// Mode: off (per defecte) | workspace | strict.
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`
	// Network: on (per defecte) | off.
	Network string `yaml:"network,omitempty" json:"network,omitempty"`
	// AllowWrite són camins extra on les ordres poden escriure.
	AllowWrite []string `yaml:"allow_write,omitempty" json:"allow_write,omitempty"`
	// Backend: auto (per defecte) | bwrap | landlock | sandbox-exec | docker.
	Backend string `yaml:"backend,omitempty" json:"backend,omitempty"`
	// Image és la imatge del backend docker.
	Image string `yaml:"image,omitempty" json:"image,omitempty"`
}

// ModeEff és el mode normalitzat.
func (c Config) ModeEff() string {
	switch strings.ToLower(strings.TrimSpace(c.Mode)) {
	case ModeWorkspace:
		return ModeWorkspace
	case ModeStrict:
		return ModeStrict
	}
	return ModeOff
}

// NetworkOff diu si s'ha demanat tallar la xarxa.
func (c Config) NetworkOff() bool {
	switch strings.ToLower(strings.TrimSpace(c.Network)) {
	case "off", "false", "no", "none":
		return true
	}
	return false
}

// BackendEff és el backend demanat normalitzat.
func (c Config) BackendEff() string {
	b := strings.ToLower(strings.TrimSpace(c.Backend))
	if b == "" {
		return BackendAuto
	}
	return b
}

// ImageEff és la imatge docker efectiva.
func (c Config) ImageEff() string {
	if s := strings.TrimSpace(c.Image); s != "" {
		return s
	}
	return DefaultImage
}

// Validate comprova els valors del config.
func (c Config) Validate() error {
	switch strings.ToLower(strings.TrimSpace(c.Mode)) {
	case "", ModeOff, ModeWorkspace, ModeStrict:
	default:
		return fmt.Errorf("sandbox.mode=%q invalid (off|workspace|strict)", c.Mode)
	}
	switch strings.ToLower(strings.TrimSpace(c.Network)) {
	case "", "on", "off", "true", "false", "yes", "no", "none":
	default:
		return fmt.Errorf("sandbox.network=%q invalid (on|off)", c.Network)
	}
	switch c.BackendEff() {
	case BackendAuto, BackendBwrap, BackendLandlock, BackendSandboxExec, BackendDocker:
	default:
		return fmt.Errorf("sandbox.backend=%q invalid (auto|bwrap|landlock|sandbox-exec|docker)", c.Backend)
	}
	for i, p := range c.AllowWrite {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("sandbox.allow_write[%d] is empty", i)
		}
	}
	return nil
}

// Status és el que el sandbox fa de debò en aquesta màquina. És el que es
// mostra a `gregal --doctor`, a /api/state (camp sandbox) i al menú de
// permisos.
type Status struct {
	Mode      string `json:"mode"`
	Requested string `json:"requested_backend"`
	// Backend és el que s'usa ("none" si cap).
	Backend string `json:"backend"`
	// Active: les ordres corren confinades.
	Active bool `json:"active"`
	// Refuses: mode strict sense backend; les ordres de shell es deneguen.
	Refuses bool `json:"refuses"`
	// Network: on|off tal com s'ha demanat; NetworkIsolated diu si el
	// backend l'aïlla de debò.
	Network         string   `json:"network"`
	NetworkIsolated bool     `json:"network_isolated"`
	Writable        []string `json:"allow_write,omitempty"`
	Warning         string   `json:"warning,omitempty"`
	// Guarantee explica en una frase què garanteix el backend actiu.
	Guarantee string `json:"guarantee"`
}

// probe són les comprovacions de la màquina; als tests se substitueixen.
type probe struct {
	goos        string
	look        func(string) bool
	landlockABI func() int
	dockerOK    func() bool
	helperOK    func() bool
}

func realProbe() probe {
	return probe{
		goos: runtime.GOOS,
		look: func(n string) bool {
			_, err := exec.LookPath(n)
			return err == nil
		},
		landlockABI: landlockABI,
		dockerOK:    dockerDisponible,
		helperOK: func() bool {
			exe, err := os.Executable()
			return err == nil && exe != ""
		},
	}
}

// candidats retorna els backends a provar, en ordre.
func candidats(cfg Config, goos string) []string {
	if b := cfg.BackendEff(); b != BackendAuto {
		return []string{b}
	}
	switch goos {
	case "linux":
		return []string{BackendBwrap, BackendLandlock}
	case "darwin":
		return []string{BackendSandboxExec}
	}
	// Windows i la resta: Docker canvia massa l'entorn (altres eines, altre
	// sistema) per triar-lo sol; cal demanar-lo.
	return nil
}

func garantia(backend string, netIso bool, mode string) string {
	net := "network unchanged"
	if netIso {
		net = "no network"
	}
	switch backend {
	case BackendBwrap:
		return "bubblewrap: whole filesystem read-only except the workspace, temp and allow_write paths; separate PID namespace; " + net
	case BackendLandlock:
		if netIso {
			net = "outbound/inbound TCP blocked (UDP not covered)"
		}
		return "Landlock (kernel LSM): writes allowed only under the workspace, temp and allow_write paths; reads unrestricted; " + net
	case BackendSandboxExec:
		return "macOS sandbox-exec: file writes allowed only under the workspace, temp and allow_write paths; " + net
	case BackendDocker:
		return "Docker container: only the workspace and allow_write paths are mounted from the host; " + net + "; commands run inside the image, not with host tools"
	}
	if mode == ModeOff {
		return "none: commands run with your user's full permissions (only the command classifier and hard-deny rules apply)"
	}
	return "none: no OS sandbox is active"
}

// resol tria el backend. És pura respecte a p: els tests hi passen màquines
// falses (Windows sense Docker, Linux sense bwrap...).
func resol(cfg Config, p probe) Status {
	mode := cfg.ModeEff()
	st := Status{Mode: mode, Requested: cfg.BackendEff(), Backend: BackendNone, Network: "on"}
	if cfg.NetworkOff() {
		st.Network = "off"
	}
	st.Writable = append([]string(nil), cfg.AllowWrite...)
	if mode == ModeOff {
		st.Guarantee = garantia(BackendNone, false, mode)
		return st
	}
	var motius []string
	for _, b := range candidats(cfg, p.goos) {
		ok, netIso, motiu := disponible(b, p)
		if !ok {
			motius = append(motius, motiu)
			continue
		}
		st.Backend, st.Active, st.NetworkIsolated = b, true, netIso
		if cfg.NetworkOff() && !netIso {
			msg := "sandbox: backend " + b + " cannot isolate the network on this machine (" + motiu + ")"
			if mode == ModeStrict {
				st.Active, st.Refuses = false, true
				st.Warning = msg + "; strict mode refuses to run shell commands. Use network: on, or another backend."
			} else {
				st.Warning = msg + "; network stays ON."
			}
		}
		st.Guarantee = garantia(b, netIso && cfg.NetworkOff(), mode)
		return st
	}
	detall := strings.Join(motius, "; ")
	if detall == "" {
		detall = "no OS sandbox backend is supported automatically on " + p.goos
	}
	pista := ""
	if p.goos == "windows" && cfg.BackendEff() != BackendDocker {
		pista = " Set sandbox.backend: docker (with Docker running) to sandbox commands on Windows."
	}
	if mode == ModeStrict {
		st.Refuses = true
		st.Warning = "sandbox strict: no sandbox backend available (" + detall + "); shell commands are refused." + pista
	} else {
		st.Warning = "sandbox: no sandbox backend available (" + detall + "); shell commands run UNSANDBOXED." + pista
	}
	st.Guarantee = garantia(BackendNone, false, mode)
	return st
}

// disponible diu si un backend funciona aquí, si aïlla la xarxa i, si no,
// per què.
func disponible(b string, p probe) (ok, netIso bool, motiu string) {
	switch b {
	case BackendBwrap:
		if p.goos != "linux" {
			return false, false, "bwrap only exists on Linux"
		}
		if !p.look("bwrap") {
			return false, false, "bwrap not found in PATH"
		}
		return true, true, ""
	case BackendLandlock:
		if p.goos != "linux" {
			return false, false, "Landlock only exists on Linux"
		}
		abi := p.landlockABI()
		if abi < 1 {
			return false, false, "Landlock not supported by this kernel (needs Linux 5.13+ with landlock enabled)"
		}
		if !p.helperOK() {
			return false, false, "cannot locate the gregal executable for the Landlock helper"
		}
		if abi < 4 {
			return true, false, fmt.Sprintf("Landlock ABI %d has no network rules (needs ABI 4, Linux 6.7+)", abi)
		}
		return true, true, ""
	case BackendSandboxExec:
		if p.goos != "darwin" {
			return false, false, "sandbox-exec only exists on macOS"
		}
		if !p.look("sandbox-exec") {
			return false, false, "sandbox-exec not found"
		}
		return true, true, ""
	case BackendDocker:
		if !p.look("docker") {
			return false, false, "docker not found in PATH"
		}
		if !p.dockerOK() {
			return false, false, "docker daemon not reachable (is Docker running?)"
		}
		return true, true, ""
	}
	return false, false, "unknown backend " + b
}

// --- estat global ---

var (
	mu       sync.Mutex
	cfgActiu Config
	estat    *Status
	avisat   bool
	provador = realProbe
)

// Configure instal·la el config del procés (config.Load el crida). L'estat
// es calcula mandrosament: provar Docker pot costar un segon i només cal
// quan el sandbox s'ha activat.
func Configure(c Config) {
	mu.Lock()
	defer mu.Unlock()
	cfgActiu = c
	estat = nil
	avisat = false
}

// Current retorna l'estat del sandbox per a la configuració activa.
func Current() Status {
	mu.Lock()
	defer mu.Unlock()
	return currentLocked()
}

func currentLocked() Status {
	if estat == nil {
		var st Status
		if cfgActiu.ModeEff() == ModeOff {
			// Sense sondejar res: off és off.
			st = resol(cfgActiu, probe{goos: runtime.GOOS})
		} else {
			st = resol(cfgActiu, provador())
		}
		estat = &st
	}
	return *estat
}

// Plan és com s'ha d'executar una ordre.
type Plan struct {
	Name string
	Args []string
	// Dir és el directori de treball per a exec.Cmd ("" = el del procés).
	// Amb docker és el directori del host (el client docker hi corre); el
	// del contenidor va a -w.
	Dir string
	// Cleanup allibera el que quedi viu si l'ordre es talla (el contenidor
	// docker no mor amb el client). Mai nil.
	Cleanup func()
	// Warning s'ha de mostrar (una sola vegada per configuració).
	Warning string
	Backend string
}

// Command prepara l'ordre de shell cmd per executar-se a dir amb el sandbox
// actiu. Torna error si el mode strict no la pot confinar.
func Command(dir, cmd string) (Plan, error) {
	mu.Lock()
	cfg := cfgActiu
	st := currentLocked()
	warn := ""
	if st.Warning != "" && !avisat {
		avisat = true
		warn = st.Warning
	}
	mu.Unlock()
	return planifica(cfg, st, dir, cmd, warn)
}

func noop() {}

func planifica(cfg Config, st Status, dir, cmd, warn string) (Plan, error) {
	if st.Refuses {
		return Plan{Cleanup: noop}, fmt.Errorf("%s", st.Warning)
	}
	if !st.Active {
		name, args := shell.Argv(shell.NormalitzaPathsWindows(cmd))
		return Plan{Name: name, Args: args, Dir: dir, Cleanup: noop, Warning: warn, Backend: BackendNone}, nil
	}
	ws := workspaceDe(dir)
	escriptura := camins(cfg, st.Mode, ws)
	netOff := cfg.NetworkOff() && st.NetworkIsolated
	switch st.Backend {
	case BackendBwrap:
		name, args := shell.Argv(cmd)
		a := BwrapArgs(BwrapOpts{Writable: escriptura, Dir: ws, NetworkOff: netOff, Strict: st.Mode == ModeStrict, TempDir: os.TempDir()}, name, args)
		return Plan{Name: "bwrap", Args: a, Dir: dir, Cleanup: noop, Warning: warn, Backend: st.Backend}, nil
	case BackendLandlock:
		exe, err := os.Executable()
		if err != nil {
			return Plan{Cleanup: noop}, fmt.Errorf("sandbox: landlock helper: %v", err)
		}
		name, args := shell.Argv(cmd)
		return Plan{Name: exe, Args: LandlockHelperArgs(escriptura, netOff, name, args), Dir: dir, Cleanup: noop, Warning: warn, Backend: st.Backend}, nil
	case BackendSandboxExec:
		name, args := shell.Argv(cmd)
		perfil := SeatbeltProfile(realpaths(escriptura), netOff)
		return Plan{Name: "sandbox-exec", Args: append([]string{"-p", perfil, name}, args...), Dir: dir, Cleanup: noop, Warning: warn, Backend: st.Backend}, nil
	case BackendDocker:
		nom := nomContenidor()
		opts := DockerOpts{
			Name: nom, Image: cfg.ImageEff(), Workspace: ws, Dir: ws, Extra: extres(cfg, st.Mode),
			NetworkOff: netOff, Strict: st.Mode == ModeStrict, User: usuariDocker(),
		}
		cleanup := func() { esborraContenidor(nom) }
		return Plan{Name: "docker", Args: DockerArgs(opts, cmd), Dir: dir, Cleanup: cleanup, Warning: warn, Backend: st.Backend}, nil
	}
	return Plan{Cleanup: noop}, fmt.Errorf("sandbox: unknown backend %q", st.Backend)
}

// workspaceDe és el directori de l'ordre en absolut (el del procés si és
// buit).
func workspaceDe(dir string) string {
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return dir
}

// extres són els camins d'escriptura que no són el workspace.
func extres(cfg Config, mode string) []string {
	var out []string
	for _, p := range cfg.AllowWrite {
		p = expandeix(p)
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		out = append(out, p)
	}
	if mode == ModeWorkspace {
		// La memòria cau de l'usuari (go-build, pip, npm...): sense ella un
		// `go test` o un `npm ci` dins del workspace fallaria en mode
		// workspace. El mode strict no la dona: cal posar-la a allow_write.
		if c, err := os.UserCacheDir(); err == nil && c != "" {
			out = append(out, c)
		}
	}
	return out
}

func camins(cfg Config, mode, ws string) []string {
	out := []string{ws}
	if mode == ModeWorkspace || runtime.GOOS == "darwin" {
		// Linux strict fa servir un /tmp privat (tmpfs); macOS no en té i
		// necessita el temp del sistema per a quasi qualsevol eina.
		out = append(out, os.TempDir())
		if runtime.GOOS != "windows" {
			out = append(out, "/tmp")
		}
	}
	out = append(out, extres(cfg, mode)...)
	return dedup(out)
}

func expandeix(p string) string {
	p = os.ExpandEnv(strings.TrimSpace(p))
	if strings.HasPrefix(p, "~/") || p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(h, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func dedup(in []string) []string {
	vist := map[string]bool{}
	var out []string
	for _, p := range in {
		if p == "" || vist[p] {
			continue
		}
		vist[p] = true
		out = append(out, p)
	}
	return out
}

// realpaths resol enllaços (a macOS /tmp és /private/tmp i el perfil compara
// camins reals).
func realpaths(in []string) []string {
	var out []string
	for _, p := range in {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			p = r
		}
		out = append(out, p)
	}
	return dedup(out)
}
