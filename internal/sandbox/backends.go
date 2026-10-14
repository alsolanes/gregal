package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- bubblewrap ---

// BwrapOpts són les entrades de BwrapArgs.
type BwrapOpts struct {
	Writable   []string
	Dir        string
	NetworkOff bool
	Strict     bool
	TempDir    string
}

// BwrapArgs construeix els arguments de bwrap: tot el sistema de fitxers en
// només lectura, /dev i /proc nous, i a sobre els camins d'escriptura. Els
// camins que no existeixen s'ignoren (--bind-try) en comptes de fer fallar
// cada ordre.
func BwrapArgs(o BwrapOpts, name string, args []string) []string {
	a := []string{
		"--die-with-parent", "--new-session",
		"--ro-bind", "/", "/",
		"--dev", "/dev",
		"--proc", "/proc",
		"--unshare-pid",
	}
	if o.Strict {
		// /tmp privat i buit: res del que hi deixin altres processos.
		a = append(a, "--tmpfs", "/tmp")
		if t := o.TempDir; t != "" && t != "/tmp" {
			a = append(a, "--tmpfs", t)
		}
		a = append(a, "--unshare-ipc", "--unshare-uts")
	}
	for _, p := range o.Writable {
		if o.Strict && (p == "/tmp" || p == o.TempDir) {
			continue
		}
		a = append(a, "--bind-try", p, p)
	}
	if o.NetworkOff {
		a = append(a, "--unshare-net")
	}
	if o.Dir != "" {
		a = append(a, "--chdir", o.Dir)
	}
	a = append(a, "--", name)
	return append(a, args...)
}

// --- Landlock (procés auxiliar) ---

// HelperArg és el primer argument amb què gregal es re-executa per aplicar
// Landlock abans de fer exec de l'ordre (Go no pot executar codi entre el
// fork i l'exec).
const HelperArg = "__sandbox-exec"

// LandlockHelperArgs són els arguments del procés auxiliar.
func LandlockHelperArgs(writable []string, networkOff bool, name string, args []string) []string {
	a := []string{HelperArg}
	for _, p := range writable {
		a = append(a, "--rw", p)
	}
	if networkOff {
		a = append(a, "--net", "off")
	}
	a = append(a, "--", name)
	return append(a, args...)
}

// HelperSpec és el que el procés auxiliar ha de fer.
type HelperSpec struct {
	Writable   []string
	NetworkOff bool
	Argv       []string
}

// ParseHelperArgs és l'invers de LandlockHelperArgs (sense HelperArg).
func ParseHelperArgs(args []string) (HelperSpec, bool) {
	var s HelperSpec
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--rw":
			if i+1 >= len(args) {
				return s, false
			}
			s.Writable = append(s.Writable, args[i+1])
			i++
		case "--net":
			if i+1 >= len(args) {
				return s, false
			}
			s.NetworkOff = args[i+1] == "off"
			i++
		case "--":
			s.Argv = args[i+1:]
			return s, len(s.Argv) > 0
		default:
			return s, false
		}
	}
	return s, false
}

// --- macOS sandbox-exec ---

func sbplQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// SeatbeltProfile genera el perfil SBPL: tot permès menys escriure fora dels
// camins donats (i, amb xarxa tallada, qualsevol xarxa que no sigui un socket
// unix local).
func SeatbeltProfile(writable []string, networkOff bool) string {
	var b strings.Builder
	b.WriteString("(version 1)\n(allow default)\n(deny file-write*)\n(allow file-write*\n")
	for _, p := range writable {
		b.WriteString("  (subpath " + sbplQuote(p) + ")\n")
	}
	b.WriteString("  (literal \"/dev/null\") (literal \"/dev/zero\") (literal \"/dev/tty\") (literal \"/dev/dtracehelper\")\n")
	b.WriteString("  (regex #\"^/dev/fd/\") (regex #\"^/dev/ttys[0-9]+$\"))\n")
	if networkOff {
		b.WriteString("(deny network*)\n(allow network* (remote unix-socket))\n")
	}
	return b.String()
}

// --- Docker ---

// DockerOpts són les entrades de DockerArgs.
type DockerOpts struct {
	Name       string
	Image      string
	Workspace  string
	Dir        string
	Extra      []string
	NetworkOff bool
	Strict     bool
	// User és uid:gid (Unix) perquè els fitxers no quedin de root.
	User string
}

var reUnitat = regexp.MustCompile(`^([A-Za-z]):[\\/]?(.*)$`)

// ContainerPath tradueix un camí del host al del contenidor amb la mateixa
// forma: C:\dev\x → /c/dev/x; els camins Unix no canvien.
func ContainerPath(p string) string {
	if m := reUnitat.FindStringSubmatch(p); m != nil {
		rest := strings.ReplaceAll(m[2], `\`, "/")
		return "/" + strings.ToLower(m[1]) + "/" + strings.TrimPrefix(rest, "/")
	}
	return p
}

var reWinCmd = regexp.MustCompile(`\b([A-Za-z]):[\\/][^\s"'` + "`" + `]*`)

// containerCmd tradueix les rutes Windows de l'ordre (el model les escriu
// sempre així).
func containerCmd(cmd string) string {
	return reWinCmd.ReplaceAllStringFunc(cmd, ContainerPath)
}

// DockerArgs construeix `docker run` per a una ordre.
func DockerArgs(o DockerOpts, cmd string) []string {
	net := "bridge"
	if o.NetworkOff {
		net = "none"
	}
	a := []string{"run", "--rm", "-i", "--init", "--network", net, "--security-opt", "no-new-privileges"}
	if o.Name != "" {
		a = append(a, "--name", o.Name)
	}
	if o.Strict {
		a = append(a, "--read-only", "--tmpfs", "/tmp", "--cap-drop", "ALL")
	}
	if o.User != "" {
		a = append(a, "--user", o.User)
	}
	muntats := map[string]bool{}
	munta := func(p string) {
		if p == "" || muntats[p] {
			return
		}
		muntats[p] = true
		a = append(a, "--mount", "type=bind,src="+p+",dst="+ContainerPath(p))
	}
	munta(o.Workspace)
	for _, p := range o.Extra {
		if _, err := os.Stat(p); err == nil {
			munta(p)
		}
	}
	dir := o.Dir
	if dir == "" {
		dir = o.Workspace
	}
	a = append(a, "-w", ContainerPath(dir), o.Image, "sh", "-c", containerCmd(cmd))
	return a
}

func usuariDocker() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
}

func nomContenidor() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "gregal-sbx-" + hex.EncodeToString(b[:])
}

// esborraContenidor mata el contenidor d'una ordre tallada: matar el client
// docker no l'atura.
func esborraContenidor(nom string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", nom).Run()
}

var (
	dockerOnce sync.Once
	dockerOK   bool
)

// dockerDisponible pregunta al dimoni un cop per procés.
func dockerDisponible() bool {
	dockerOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}").Output()
		dockerOK = err == nil && strings.TrimSpace(string(out)) != ""
	})
	return dockerOK
}
