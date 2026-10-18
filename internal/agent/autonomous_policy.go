package agent

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"gregal/internal/knowledge"
	"gregal/internal/tools"
)

// Política del mode autònom: ha de treballar sol. Programar, llegir, cercar
// a la web, escriure dins del projecte i executar les ordres normals de
// desenvolupament (tests, builds, instal·lar dependències del projecte,
// crear o esborrar fitxers de dins) passen sense preguntar. Només es pregunta
// per allò perillós: esborrats massius o fora del projecte, git destructiu,
// instal·lacions globals, aturar processos, contenidors, registre, escriure
// fora de la carpeta, serveis externs (MCP), commits i PRs. El que el
// classificador ja denega (sudo, curl, rm -rf /…) continua denegat.
//
// Demanat el 2026-10-13: «el mode autònom no ha de fer preguntes… si
// l'únic que fa és programar o buscar webs o llegir fitxers de la carpeta
// no cal que pregunti».
//
// Des del 2026-10-16 el mode autònom ja no pregunta ni pel perillós
// (Policy.Decide converteix tot «ask» en «allow»): els «ask» d'aquí només
// queden com a motiu, que el registre d'activitat pot mostrar. Els «deny»
// continuen manant.

// perillosa: patrons que en mode autònom sempre demanen confirmació.
var perillosa = regexp.MustCompile(`(?i)` + strings.Join([]string{
	`\bgit\s+(push|reset\s+--hard|clean\b|rebase\b|filter-branch|branch\s+-D|checkout\s+--\s|restore\s+--source|stash\s+(drop|clear))`,
	`\b(npm|pnpm|yarn)\s+(i|install|add|remove|uninstall)\b[^|;&]*\s(-g|--global)\b`,
	`\bpip3?\s+install\b[^|;&]*\s--user\b`,
	`\b(choco|winget|scoop|brew|apt(-get)?|dnf|yum|pacman)\s+(install|remove|uninstall|upgrade)\b`,
	`\b(kill|killall|pkill|taskkill|Stop-Process)\b`,
	`\b(systemctl|service|launchctl|shutdown|reboot)\b|\bsc(\.exe)?\s+(stop|delete|config)\b|\b(Restart|Stop|Remove)-Service\b`,
	`\b(docker|podman|kubectl)\b`,
	`\b(reg\s+(add|delete)|Set-ItemProperty|New-ItemProperty|setx)\b`,
	`\bchmod\s+-R\b|\bchown\b`,
	`\brm\s+(-[a-zA-Z]*r[a-zA-Z]*|--recursive)\b[^|;&]*(\s|^)(\.|\*|\./\*|\.\.[/\\]?\S*)(\s|$)`,
}, "|"))

// escripturaFora: verbs que escriuen o esborren, amb un camí absolut, ~ o
// .. que surt del projecte.
var verbEscriptura = regexp.MustCompile(`(?i)(^|[\s;&|(])(rm|rmdir|mv|cp|tee|touch|mkdir|ln|del|erase|move|copy|xcopy|robocopy|Remove-Item|Move-Item|Copy-Item|Set-Content|Out-File)\b|>{1,2}`)
var camiCandidat = regexp.MustCompile(`(?:^|[\s"'=>])((?:[A-Za-z]:[\\/]|/|~|\.\.[\\/])[^\s"'|;&<>]*)`)

func (p *Policy) decideAutonomous(name, argsJSON string) (string, string, bool) {
	if p != nil && name != "bash" && name != "bash_background" {
		if d, why, ok := p.ruleDecision(name, argsJSON); ok {
			return d, why, true
		}
		if d, ok := p.Tools[name]; ok && (d == "allow" || d == "ask" || d == "deny") {
			return d, "permissions del config", true
		}
	}
	projecte := ""
	if p != nil {
		projecte = p.ProjectDir
	}
	switch name {
	case "question":
		return "deny", "autonomous mode: do not ask the user; make a reasonable choice, keep going and list your assumptions in the final answer", true
	case "write", "edit", "patch", "office_edit", "office_create":
		if InsideProject(projecte, toolPath(name, argsJSON)) {
			return "allow", "", true
		}
		return "ask", "autonomous: writing outside the project folder", true
	case "bash", "bash_background":
		var a struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		var allow, deny []string
		if p != nil {
			allow, deny = p.BashAllow, p.BashDeny
		}
		if d, reason := tools.ClassifyWith(a.Command, allow, deny); d == "deny" {
			return d, reason, true
		}
		// Un permís explícit del config (el més estricte de bash i
		// bash_background) mana, però mai relaxa un bloqueig dur.
		if d, why, ok := p.ruleDecision(name, argsJSON); ok {
			return d, why, true
		}
		if override := bashPermissionOverride(p); override != "" {
			return override, "permissions del config", true
		}
		if perillosa.MatchString(a.Command) {
			return "ask", "autonomous: potentially destructive command", true
		}
		if escriuFora(a.Command, projecte) {
			return "ask", "autonomous: command writes outside the project folder", true
		}
		return "allow", "", true
	case "browser":
		var a struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		if a.Action == "eval" {
			return "ask", "autonomous: running custom JavaScript in the browser", true
		}
		return "allow", "", true
	case "bash_kill":
		return "allow", "", true // només processos que ha engegat ell
	case "knowledge_note":
		// Les notes de projecte i de sessió són feina d'aquí; una nota
		// global la veurien tots els projectes: es pregunta.
		if knowledge.NoteScope(knowledge.Where{Project: projecte, Session: procSession}, argsJSON) == knowledge.ScopeGlobal {
			return "ask", "autonomous: note to a global knowledge collection", true
		}
		return "allow", "", true
	case "git_commit", "gh_pr_create":
		return "ask", "autonomous: commits and pull requests stay with you", true
	}
	if isMCP(name) && !IsReadOnlyMCP(name) {
		return "ask", "autonomous: external service (MCP)", true
	}
	return "", "", false
}

// escriuFora diu si una ordre amb un verb d'escriptura apunta a un camí
// fora del projecte (absolut, ~ o ..). Sense projecte conegut, qualsevol
// camí absolut d'escriptura compta com a fora.
var redireccioInofensiva = regexp.MustCompile(`\d*>&\d+|\d*>\s*/dev/null|\d*>\s*(?i:nul)`)
var camiGitBash = regexp.MustCompile(`^/([a-zA-Z])/`)

func escriuFora(cmd, projecte string) bool {
	cmd = redireccioInofensiva.ReplaceAllString(cmd, " ")
	if !verbEscriptura.MatchString(cmd) {
		return false
	}
	for _, m := range camiCandidat.FindAllStringSubmatch(cmd, -1) {
		c := strings.TrimSpace(m[1])
		if c == "" || c == "/" && strings.Contains(cmd, "//") {
			continue
		}
		// /dev/null i similars no són escriptures reals.
		if strings.HasPrefix(c, "/dev/") || strings.EqualFold(c, "nul") {
			continue
		}
		if projecte == "" {
			return true
		}
		if strings.HasPrefix(c, "..") {
			c = filepath.Join(projecte, c)
		}
		// Git Bash: /c/Users/… és C:/Users/…
		if m := camiGitBash.FindStringSubmatch(c); m != nil {
			c = strings.ToUpper(m[1]) + ":/" + c[3:]
		}
		if !InsideProject(projecte, c) {
			return true
		}
	}
	return false
}
