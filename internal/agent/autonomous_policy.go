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
	case "write", "edit", "patch", "office_edit", "office_create", "pdf_edit", "pdf_create":
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
		if a.Action == "eval" || strings.Contains(argsJSON, `"eval"`) {
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

// WindAutoAllow diu si un «ask» d'un torn interactiu de vent/colla es pot
// aprovar sol. Aquests torns no tenen cap UI d'aprovació: un «ask» no el
// pot respondre ningú i l'eina queda sense executar («REQUIRES APPROVAL»),
// de manera que demanar codi al vent sempre acaba en «falta permís».
// La instrucció explícita del xat JA és l'aprovació per a la feina
// d'aquí: escriure i executar dins del projecte, sense res destructiu.
// Continua manant tot «deny» (classificador: sudo, curl a internet…;
// regles explícites del config), i queden fora: esborrats/perilloses,
// escriure fora del projecte, commits i PRs, serveis externs (MCP) i notes
// globals. Per a això cal el xat principal.
func (p *Policy) WindAutoAllow(name, argsJSON string) bool {
	projecte := ""
	var allow, deny []string
	if p != nil {
		projecte = p.ProjectDir
		allow, deny = p.BashAllow, p.BashDeny
	}
	// Un permís explícit del config (el més restrictiu) mana: l'autoaprovat
	// del xat no el relaxa mai.
	// Amb poders de servidor l'administrador ja ha donat el permís: un «ask»
	// del config (la plantilla porta write/edit: ask, pensat per al xat
	// principal) no l'atura; un «deny» explícit, sí.
	blocks := func(d string) bool { return d != "allow" && !(p.ServerPowers && d == "ask") }
	if p != nil {
		if d, _, ok := p.ruleDecision(name, argsJSON); ok && blocks(d) {
			return false
		}
		if name == "bash" || name == "bash_background" {
			if d, _, ok := p.ruleDecision("bash", argsJSON); ok && blocks(d) {
				return false
			}
			if d, _, ok := p.ruleDecision("bash_background", argsJSON); ok && blocks(d) {
				return false
			}
			if o := bashPermissionOverride(p); o != "" && blocks(o) {
				return false
			}
		} else if d, ok := p.Tools[name]; ok && blocks(d) {
			return false
		}
	}
	switch name {
	case "write", "edit", "patch", "office_edit", "office_create", "pdf_edit", "pdf_create":
		if p != nil && p.ServerPowers {
			if path := toolPath(name, argsJSON); path != "" && !tools.EscripturaCritica(path, p.ServerHome) {
				return true
			}
		}
		return projecte != "" && InsideProject(projecte, toolPath(name, argsJSON))
	case "lsp":
		// Renombrar símbols toca diversos fitxers: només dins del projecte.
		// Consultar (definition, references, hover, diagnostics) no canvia res.
		if !lspEsRename(argsJSON) {
			return true
		}
		return projecte != "" && InsideProject(projecte, toolPath(name, argsJSON))
	case "git_commit":
		// Un commit local dins del projecte no destrueix res i el xat del vent no
		// té cap finestra on confirmar-lo: deixat en «ask», la feina s'acaba
		// dient «feta» sense commit i amb una nota d'aprovació pendent que
		// sembla una errada. Els PRs i els pushos al remot continuen sent del
		// xat principal.
		return projecte != ""
	case "knowledge_note":
		// Una nota global la veurien tots els projectes: això es queda per
		// al xat principal, on la persona ho pot confirmar.
		return knowledge.NoteScope(knowledge.Where{Project: projecte, Session: procSession}, argsJSON) != knowledge.ScopeGlobal
	case "bash", "bash_background":
		var a struct {
			Command string `json:"command"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		if p != nil && p.ServerPowers {
			d, _ := tools.ClassifyServer(a.Command, p.ServerHome, deny)
			return d == "allow"
		}
		return ordreSegura(a.Command, projecte, allow, deny)
	case "bash_kill", "bash_output":
		// Només els processos que el mateix torn ha engegat.
		return true
	case "app":
		// Engegar una app és executar la seva ordre a la carpeta del
		// projecte: les mateixes regles que bash, i la carpeta a dins.
		if appActionReadOnly(argsJSON) {
			return true
		}
		var a struct {
			Action  string `json:"action"`
			Dir     string `json:"dir"`
			Command string `json:"command"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		if a.Action == "stop" || a.Action == "restart" {
			// Atura o torna a engegar una app que ja hi és, amb la mateixa
			// ordre i carpeta que es van acceptar en engegar-la.
			return true
		}
		if p != nil && p.ServerPowers {
			d, _ := tools.ClassifyServer(a.Command, p.ServerHome, deny)
			return a.Command == "" || d == "allow"
		}
		// Sense dir, l'app va a la carpeta del torn (el projecte).
		if projecte == "" || (a.Dir != "" && !InsideProject(projecte, a.Dir)) {
			return false
		}
		return a.Command == "" || ordreSegura(a.Command, projecte, allow, deny)
	case "publish":
		// La persona ha demanat la web: publicar-la (un enllaç no
		// endevinable, revocable) és la feina. Només carpetes del projecte.
		var a struct {
			Dir string `json:"dir"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		if p != nil && p.ServerPowers {
			return true
		}
		return projecte != "" && InsideProject(projecte, a.Dir)
	case "tunnel":
		// Obrir un servei a internet és cosa de l'administrador: només amb
		// poders de servidor. Consultar-los sempre.
		return tunnelActionReadOnly(argsJSON) || (p != nil && p.ServerPowers)
	}
	// Amb poders de servidor l'administrador ja ha dit que sí: un torn de vent o
	// de colla no es pot quedar a mitges per una eina que no surt a la llista de
	// dalt (MCP del propi equip, PRs, preguntes, enviar fitxers…). El que està
	// barrat ho està més amunt: regles «deny» del config, classificador de shell
	// (sudo, rm -rf, curl a internet) i escriptura fora del projecte. Sense
	// poders, aquesta cua continua tancada.
	return p != nil && p.ServerPowers
}

// tunnelActionReadOnly: consultar túnels (list, status, logs) no canvia res.
func tunnelActionReadOnly(argsJSON string) bool {
	var a struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	switch strings.TrimSpace(a.Action) {
	case "list", "status", "logs":
		return true
	}
	return false
}

// ordreSegura: una ordre que un vent pot executar sol (sense res denegat,
// perillós ni escriptures fora del projecte).
func ordreSegura(cmd, projecte string, allow, deny []string) bool {
	// Un heredoc que no va a una shell és dades (el fitxer que s'escriu, un
	// script de python): «kill» o «/api» dins d'un joc no són ordres.
	if d, _ := tools.ClassifyWith(cmd, allow, deny); d == "deny" && treuDades(cmd) == cmd {
		return false
	}
	cmd = treuDades(cmd)
	if d, _ := tools.ClassifyWith(cmd, allow, deny); d == "deny" {
		return false
	}
	if perillosa.MatchString(cmd) {
		return false
	}
	return !escriuFora(cmd, projecte)
}

// appActionReadOnly: consultar una app (estat, registres, llista) no
// canvia res.
func appActionReadOnly(argsJSON string) bool {
	var a struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	switch strings.TrimSpace(a.Action) {
	case "status", "logs", "list":
		return true
	}
	return false
}

// escriuFora diu si una ordre amb un verb d'escriptura apunta a un camí
// fora del projecte (absolut, ~ o ..). Sense projecte conegut, qualsevol
// camí absolut d'escriptura compta com a fora.
var redireccioInofensiva = regexp.MustCompile(`\d*>&\d+|\d*>\s*/dev/null|\d*>\s*(?i:nul)`)
var camiGitBash = regexp.MustCompile(`^/([a-zA-Z])/`)

func escriuFora(cmd, projecte string) bool {
	cmd = treuDades(cmd)
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

// heredocInici reconeix l'obertura d'un heredoc: <<EOF, <<-EOF, <<'EOF', <<"EOF".
var heredocInici = regexp.MustCompile(`<<-?\s*['"]?([A-Za-z_][A-Za-z0-9_]*)['"]?`)

// heredocAShell: un heredoc que alimenta una shell (bash <<EOF, | sh) és codi
// de shell i es mira sencer.
var heredocAShell = regexp.MustCompile(`(?i)(^|[;&|(]\s*|\|\s*)(sudo\s+)?(sh|bash|zsh|dash|ksh|fish|pwsh|powershell|cmd|eval|xargs|source|\.)(\.exe)?(\s|$|<)`)

// treuHeredocs treu el cos dels heredocs d'una ordre. El cos és el contingut
// que s'escriu (un script, una pàgina amb camins com '/api' o '//'), no camins
// on s'escriu: abans un `cat > joc.js <<'EOF'` amb codi JavaScript dins semblava
// una escriptura fora del projecte i el vent no podia crear el fitxer.
// La línia de l'ordre (amb la redirecció i el destí) es conserva sencera.
func treuHeredocs(cmd string) string {
	lines := strings.Split(cmd, "\n")
	var out []string
	var tancament []string
	for _, l := range lines {
		if len(tancament) > 0 {
			if strings.TrimSpace(l) == tancament[0] {
				tancament = tancament[1:]
			}
			continue
		}
		out = append(out, l)
		if heredocAShell.MatchString(l) {
			continue // el cos s'executa: s'ha de mirar sencer
		}
		for _, m := range heredocInici.FindAllStringSubmatch(l, -1) {
			tancament = append(tancament, m[1])
		}
	}
	return strings.Join(out, "\n")
}

// scriptEnLinia: el codi d'un `node -e "…"` o `python -c '…'` és un programa
// (amb comparacions `>` i camins com '/api'), no redireccions ni camins on
// la shell escriu.
var scriptEnLinia = regexp.MustCompile(`(?i)\b(node|nodejs|deno|bun|python3?|py|ruby|perl|php)(\.exe)?\s+(-e|-p|-c|-r|--eval|--print|eval)\s+("(?:[^"\\]|\\.)*"|'[^']*')`)

// treuDades treu el que és contingut i no ordre de shell: els cossos dels
// heredocs i els scripts en línia dels intèrprets.
func treuDades(cmd string) string {
	return scriptEnLinia.ReplaceAllString(treuHeredocs(cmd), "$1$2 $3 ''")
}

// NotesActionReadOnly diu si una crida a l'eina notes només llegeix.
func NotesActionReadOnly(argsJSON string) bool {
	var a struct {
		Action string `json:"action"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "", "list", "search", "read", "agenda":
		return true
	}
	return false
}
