package tools

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Ordres que un vent no ha de fer sense el sí de la persona, encara que el
// classificador les deixi passar (ClassifyServer): instal·lar programari,
// baixar paquets o binaris, esborrar fora del projecte i tocar serveis del
// sistema. Els torns dels vents no tenen cap finestra d'aprovació pròpia:
// aquest detector és el que decideix quan s'ha de preguntar (Risky), i el
// servidor bloqueja l'ordre fins que la persona respon.
//
// És una heurística sobre el text de l'ordre, no una caixa de sorra: una ordre
// disfressada (base64, un script que en crea un altre) se'n pot escapar. Per
// això la consigna del vent també li demana que pregunti. Es pretén no molestar
// en la feina normal del projecte (crear fitxers, npm install local, compilar,
// provar, esborrar el seu propi build).

// Risk descriu una ordre arriscada.
type Risk struct {
	Kind string // install | download | delete | system
	What string // l'ordre o la part que ho és, curta
}

var (
	reSegSplit     = regexp.MustCompile(`\r?\n|;|&&|\|\||\||&`)
	reHeredocStart = regexp.MustCompile(`<<-?\s*['"]?(\w+)['"]?`)
	reEnvAssign    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=\S*$`)
	reShellC       = regexp.MustCompile(`(?s)^(?:sh|bash|zsh|dash|cmd|cmd\.exe|powershell|pwsh)(?:\.exe)?\s+(?:-c|-lc|/c|-command)\s+(.+)$`)
	reDlExt        = regexp.MustCompile(`(?i)\.(zip|tar|tgz|gz|xz|bz2|7z|rar|jar|apk|aab|deb|rpm|msi|exe|sh|run|bin|appimage|iso|dmg|pkg|whl)(\?|$|["'])`)
	reDlOutFlag    = regexp.MustCompile(`(?:^|\s)(-o|-O|--output|--remote-name|--remote-name-all|-P|--directory-prefix)(?:\s|=|$)`)
)

// Risky mira l'ordre i diu si cal demanar permís; (Risk{}, false) si és feina
// normal. projectDir és la carpeta del projecte (per saber si un esborrat és
// dins); home, la casa de l'usuari del servei.
func Risky(cmd, projectDir, home string) (Risk, bool) {
	cmd = stripHeredocs(cmd)
	// workDir segueix els «cd» de l'ordre: «cd C:/altre && rm -f x.txt»
	// esborra C:/altre/x.txt, no x.txt del projecte. Abans els camins relatius
	// sempre es resolien contra el projecte i aquest esborrat passava per «dins».
	workDir := ""
	for _, seg := range reSegSplit.Split(cmd, -1) {
		seg = strings.TrimSpace(seg)
		if d, ok := cdTarget(seg); ok {
			workDir = resolveWorkDir(d, workDir, projectDir, home)
			continue
		}
		if r, ok := riskySegment(seg, projectDir, workDir, home, 0); ok {
			return r, true
		}
	}
	return Risk{}, false
}

// cdTarget torna la carpeta d'un «cd x» (o pushd, Set-Location, chdir).
func cdTarget(seg string) (string, bool) {
	w := fields(seg)
	if len(w) != 2 {
		return "", false
	}
	switch strings.ToLower(w[0]) {
	case "cd", "pushd", "chdir", "set-location", "sl":
		if w[1] == "-" || strings.HasPrefix(w[1], "-") {
			return "", false
		}
		return w[1], true
	}
	return "", false
}

// resolveWorkDir és la carpeta de feina després d'un cd: absoluta, ~ o
// relativa a l'anterior (o al projecte si encara no se n'ha canviat).
func resolveWorkDir(d, workDir, projectDir, home string) string {
	d = strings.Trim(strings.TrimSpace(d), `"'`)
	switch {
	case d == "~" || strings.HasPrefix(d, "~/"):
		if home == "" {
			return ""
		}
		return filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(d, "~"), "/"))
	case strings.Contains(d, "$") || strings.Contains(d, "`"):
		// No sabem on va: que tot el que vingui després compti com a fora.
		return string(filepath.Separator) + "??"
	case filepath.IsAbs(d) || strings.HasPrefix(filepath.ToSlash(d), "/"):
		return filepath.Clean(d)
	}
	base := workDir
	if base == "" {
		base = projectDir
	}
	if base == "" {
		return ""
	}
	return filepath.Join(base, d)
}

// enWorkDir resol un camí relatiu contra la carpeta del «cd» anterior.
func enWorkDir(a, workDir string) string {
	if workDir == "" || a == "" || filepath.IsAbs(a) || strings.HasPrefix(filepath.ToSlash(a), "/") ||
		strings.HasPrefix(a, "~") || strings.Contains(a, "$") || strings.Contains(a, "`") {
		return a
	}
	return filepath.Join(workDir, a)
}

// stripHeredocs treu el cos dels «<<EOF … EOF»: és text que s'escriu a un fitxer,
// no ordres (un fitxer que conté «rm -rf /» no és un esborrat).
func stripHeredocs(cmd string) string {
	lines := strings.Split(cmd, "\n")
	var out []string
	for i := 0; i < len(lines); i++ {
		out = append(out, lines[i])
		if m := reHeredocStart.FindStringSubmatch(lines[i]); m != nil {
			end := m[1]
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) != end {
				j++
			}
			i = j // salta el cos i el terminador
		}
	}
	return strings.Join(out, "\n")
}

func fields(seg string) []string {
	// Treu cometes i claus senzilles: el que importa són les paraules.
	f := strings.Fields(seg)
	out := make([]string, 0, len(f))
	for _, w := range f {
		w = strings.Trim(w, `"'`+"`")
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

func riskySegment(seg, projectDir, workDir, home string, depth int) (Risk, bool) {
	if seg == "" {
		return Risk{}, false
	}
	// sh -c "…": s'hi mira a dins.
	if m := reShellC.FindStringSubmatch(seg); m != nil && depth < 3 {
		inner := strings.Trim(strings.TrimSpace(m[1]), `"'`)
		for _, s := range reSegSplit.Split(inner, -1) {
			if r, ok := riskySegment(strings.TrimSpace(s), projectDir, workDir, home, depth+1); ok {
				return r, true
			}
		}
		return Risk{}, false
	}
	w := fields(seg)
	// Prefixos que no canvien el que es fa: variables, sudo, time, nohup, env…
	for len(w) > 0 && (reEnvAssign.MatchString(w[0]) || w[0] == "sudo" || w[0] == "time" || w[0] == "nohup" || w[0] == "env" || w[0] == "command" || w[0] == "exec") {
		w = w[1:]
	}
	if len(w) == 0 {
		return Risk{}, false
	}
	prog := strings.ToLower(filepath.Base(strings.ReplaceAll(w[0], `\`, "/")))
	prog = strings.TrimSuffix(prog, ".exe")
	args := w[1:]
	has := func(vals ...string) bool {
		for _, a := range args {
			for _, v := range vals {
				if a == v {
					return true
				}
			}
		}
		return false
	}
	first := ""
	if len(args) > 0 {
		first = strings.ToLower(args[0])
	}
	short := func() string {
		s := strings.Join(w, " ")
		if r := []rune(s); len(r) > 100 {
			s = string(r[:100]) + "…"
		}
		return s
	}

	switch prog {
	case "sdkmanager", "avdmanager":
		if has("--list", "--version", "--help") && !has("--licenses") {
			return Risk{}, false
		}
		return Risk{"install", short()}, true
	case "apt", "apt-get", "aptitude", "dnf", "yum", "pacman", "apk", "zypper", "brew", "snap", "flatpak", "choco", "winget", "scoop", "port", "nix-env":
		switch first {
		case "install", "add", "-s", "reinstall", "upgrade", "dist-upgrade", "full-upgrade", "remove", "purge", "autoremove", "uninstall", "-syu", "-su":
			return Risk{"install", short()}, true
		}
	case "npm", "yarn", "pnpm", "bun":
		if (first == "install" || first == "i" || first == "add" || first == "global") && (has("-g", "--global") || first == "global") {
			return Risk{"install", short()}, true
		}
	case "pip", "pip3", "pipx", "uv":
		if first == "install" && has("--user", "--break-system-packages", "--system") || prog == "pipx" && (first == "install" || first == "uninstall") {
			return Risk{"install", short()}, true
		}
	case "cargo", "go", "gem", "dotnet":
		if first == "install" || (prog == "dotnet" && first == "tool" && has("install")) {
			return Risk{"install", short()}, true
		}
	case "curl", "wget", "invoke-webrequest", "iwr", "bitsadmin":
		if localhostURL(args) {
			return Risk{}, false
		}
		if prog == "wget" && !has("-O") && !has("-qO-") && !strings.Contains(seg, "-O-") {
			// wget guarda sempre un fitxer: només s'hi pregunta si sembla un paquet o un binari.
			if reDlExt.MatchString(seg) {
				return Risk{"download", short()}, true
			}
			return Risk{}, false
		}
		if (reDlOutFlag.MatchString(" "+seg) || strings.Contains(seg, "-o ") || strings.Contains(seg, "-O ")) && reDlExt.MatchString(seg) {
			return Risk{"download", short()}, true
		}
	case "rm", "rmdir", "del", "erase", "rd", "remove-item", "shred", "unlink":
		// Qualsevol objectiu fora del projecte (o el projecte sencer) demana permís;
		// netejar el propi build (rm -rf dist node_modules) és feina normal.
		for _, a := range args {
			if strings.HasPrefix(a, "-") || (strings.HasPrefix(a, "/") && len(a) <= 3) || a == "" {
				continue
			}
			if target := enWorkDir(a, workDir); !insideDir(target, projectDir, home) {
				if target != a {
					// Després d'un cd, el camí sencer: la targeta ha de poder dir on és.
					return Risk{"delete", prog + ` "` + filepath.ToSlash(target) + `"`}, true
				}
				return Risk{"delete", short()}, true
			}
		}
	case "find":
		if has("-delete") || has("-exec") && strings.Contains(seg, " rm ") {
			for _, a := range args {
				if !strings.HasPrefix(a, "-") && !insideDirRoot(enWorkDir(a, workDir), projectDir, home, true) {
					return Risk{"delete", short()}, true
				}
				break
			}
		}
	case "systemctl", "service", "launchctl", "sc":
		switch first {
		case "start", "stop", "restart", "enable", "disable", "reload", "mask", "unmask", "daemon-reload", "kill":
			return Risk{"system", short()}, true
		}
	case "crontab":
		if !has("-l") {
			return Risk{"system", short()}, true
		}
	case "docker", "podman":
		switch first {
		case "run", "rm", "rmi", "stop", "kill", "pull", "build", "compose", "system", "volume", "network", "prune":
			return Risk{"system", short()}, true
		}
	case "iptables", "ufw", "firewall-cmd", "nft", "useradd", "usermod", "userdel", "passwd", "mount", "umount", "visudo", "modprobe", "sysctl":
		return Risk{"system", short()}, true
	case "chmod", "chown", "chgrp":
		if has("-R", "-r", "--recursive") {
			for _, a := range args {
				if !strings.HasPrefix(a, "-") && strings.ContainsAny(a, "/.~") && !insideDir(a, projectDir, home) {
					return Risk{"system", short()}, true
				}
			}
		}
	}
	return Risk{}, false
}

func localhostURL(args []string) bool {
	for _, a := range args {
		l := strings.ToLower(a)
		if strings.Contains(l, "://127.0.0.1") || strings.Contains(l, "://localhost") || strings.Contains(l, "://[::1]") ||
			strings.HasPrefix(l, "127.0.0.1") || strings.HasPrefix(l, "localhost") {
			return true
		}
	}
	return false
}

// insideDir diu si el camí p cau dins la carpeta del projecte. Camins amb
// variables de shell, ~ o relatius amb .. que en surten, no.
func insideDir(p, projectDir, home string) bool { return insideDirRoot(p, projectDir, home, false) }

// insideDirRoot: amb allowRoot, la carpeta del projecte mateixa també compta com a dins
// (un punt de cerca de find, no un esborrat).
func insideDirRoot(p, projectDir, home string, allowRoot bool) bool {
	if projectDir == "" {
		return false
	}
	p = strings.TrimSpace(p)
	if strings.ContainsAny(p, "*?") && !strings.Contains(p, "/") && !strings.Contains(p, `\`) {
		// Un glob sense ruta (rm -rf *) actua al directori de feina: és el projecte.
		return true
	}
	switch {
	case p == "~" || strings.HasPrefix(p, "~/"):
		if home == "" {
			return false
		}
		p = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(p, "~"), "/"))
	case strings.HasPrefix(p, "$HOME") || strings.HasPrefix(p, "${HOME}"):
		if home == "" {
			return false
		}
		p = filepath.Join(home, strings.TrimLeft(strings.TrimPrefix(strings.TrimPrefix(p, "${HOME}"), "$HOME"), "/"))
	case strings.Contains(p, "$") || strings.Contains(p, "`"):
		return false
	}
	if !filepath.IsAbs(p) && !strings.HasPrefix(filepath.ToSlash(p), "/") {
		p = filepath.Join(projectDir, p)
	}
	root := filepath.Clean(projectDir)
	p = filepath.Clean(p)
	if p == root {
		// Esborrar la carpeta del projecte sencera no és «netejar un build».
		return allowRoot
	}
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel != ".." && !strings.HasPrefix(rel, "../")
}
