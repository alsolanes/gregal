package tools

import "strings"

// Registre canònic de noms d'eines.
//
// Abans la llista vivia a tres llocs amb tres continguts diferents:
// `knownTools` a internal/agent/loop.go (24 eines), la validació de
// `permissions.tools` a internal/config (14, sense les d'office ni el
// navegador) i `nativeTools` del menú de permisos del TUI (13). El
// resultat era que posar `office_edit: deny` al config feia fallar la
// càrrega amb «eina desconeguda» tot i que l'agent l'executava, i el menú
// de permisos no deixava tocar ni office ni el navegador.
//
// Aquest paquet és el més avall de tots (només depèn d'internal/shell), o
// sigui que agent, config i tui hi poden arribar sense cicles.

// Noms són totes les eines natives que l'agent pot executar. L'ordre és
// el de la conversa: primer les de llegir, després les d'escriure, la
// shell, la web i les auxiliars.
func Noms() []string {
	return []string{
		"read", "grep", "glob", "read_image",
		"write", "edit", "patch",
		"bash", "bash_background", "bash_output", "bash_kill",
		"web_search", "web_fetch", "browser",
		"office_read", "office_open", "office_edit", "office_create",
		"gh_issue", "gh_pr",
		"delegate", "question", "todowrite", "todoread", "skill",
	}
}

// EsNativa diu si el nom és una eina nativa del registre.
func EsNativa(name string) bool {
	for _, n := range Noms() {
		if n == name {
			return true
		}
	}
	return false
}

// PrefixMCP és el que porten les eines dels servidors MCP.
const PrefixMCP = "mcp_"

// EsMCP diu si el nom és una eina d'un servidor MCP.
func EsMCP(name string) bool { return strings.HasPrefix(name, PrefixMCP) }

// Configurable diu si té sentit donar-li una política allow/ask/deny.
// `question`, `todowrite`, `todoread` i `skill` no en tenen: no toquen
// res de fora i denegar-les només trenca el checklist, les preguntes a
// l'usuari o el coneixement que l'agent té de si mateix.
func Configurable(name string) bool {
	switch name {
	case "question", "todowrite", "todoread", "skill":
		return false
	}
	return EsNativa(name)
}

// Configurables són les eines que el menú de permisos i el config poden
// governar, en l'ordre de Noms.
func Configurables() []string {
	out := make([]string, 0, len(Noms()))
	for _, n := range Noms() {
		if Configurable(n) {
			out = append(out, n)
		}
	}
	return out
}
