package agent

import (
	"fmt"
	"regexp"
	"strings"

	"gregal/internal/config"
	"gregal/internal/repomap"
)

// scoutMinFitxers: per sota d'aquí el projecte és petit i el scout no
// s'engega. El mapa del projecte ja dona al coordinador la llista sencera
// de fitxers, i el primer pas del coordinador els llegeix en paral·lel en
// una sola volta de model (~2-3 s). El scout, en canvi, és un subagent
// sencer de fins a 8 passos que corre abans del primer pas: al banc web
// (bench t6, 5 fitxers) hi afegia 17-43 s per torn sense estalviar cap crida
// d'eina al coordinador (docs/research/speed.md).
const scoutMinFitxers = 40

// ScoutWantedIn és ScoutWanted mirant també el workspace: en mode auto,
// un projecte petit no comença amb l'investigador. Amb agent.scout:
// always el scout corre igualment. En mode inspect decideix ExploreWanted
// (l'explorador de preguntes, explore.go).
func ScoutWantedIn(cfg *config.Config, mode, task, workspace string) bool {
	if mode == ModeInspect {
		return ExploreWanted(cfg, mode, task, workspace)
	}
	if !ScoutWanted(cfg, mode, task) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Agent.Scout), "always") {
		return true
	}
	// Una consulta («on es desa X?») o una tasca que ja diu el fitxer no
	// necessita un explorador: el primer grep del coordinador hi arriba
	// igual, i el scout costava 45-55 s (71-87% del torn) per endavant.
	if tascaJaDiuOnMirar(task) {
		return false
	}
	return !projectePetit(workspace)
}

// projectePetit diu si el workspace té menys de scoutMinFitxers fitxers
// (els que veu git o, fora de git, el recorregut del mapa). Sense
// workspace no se sap: no és petit.
func projectePetit(workspace string) bool {
	if strings.TrimSpace(workspace) == "" {
		return false
	}
	fitxers, _ := repomap.ListFiles(workspace)
	return len(fitxers) < scoutMinFitxers
}
var scoutFitxer = regexp.MustCompile(`[\w./-]+\.(go|kt|kts|py|js|ts|tsx|jsx|json|ya?ml|md|sh|css|html|toml|xml|sql|java|rs|c|cpp|h)\b`)

// Els verbs d'implementar hi són en català I en anglès (l'app i les tasques
// arriben en tots dos): una feina que ha de tocar codi vol el briefing encara
// que digui un fitxer, perquè el que ha de fer és entendre'l abans de canviar-lo.
var scoutImplementar = regexp.MustCompile(`(?i)\b(afegeix|afegeixi|afegeixes|arregla|arreglar|arregla-ho|corregeix|implementa|refactoritza|canvia|canviar|crea|esborra|elimina|migra|actualitza|puja|desplega|add|implement|fix|change|create|remove|delete|update|refactor|migrate|deploy|rename|move|write)\b`)

// tascaJaDiuOnMirar diu si la tasca ja assenyala on és la resposta: un camí,
// un nom de fitxer amb extensió o un identificador entre cometes. En aquest
// cas l'explorador previ és temps perdut (el primer `read`/`grep` del
// coordinador hi arriba igual) i el torn baixa de ~65 s a ~23 s, mesurat.
//
// Només això: saltar-lo també en les preguntes obertes es va provar i surt
// malament — sense briefing el coordinador rastreja el repo sencer i un torn
// que durava 63 s no acabava ni en quatre minuts. Per a les preguntes
// obertes el scout es queda, amb el pressupost de passos retallat (4 i no 8).
func tascaJaDiuOnMirar(task string) bool {
	t := strings.TrimSpace(task)
	if t == "" {
		return false
	}
	if scoutImplementar.MatchString(t) {
		return false
	}
	return strings.Contains(t, "`") || scoutFitxer.MatchString(t)
}
// ScoutReason diu en una línia per què el scout ha corregut o no. Sense això
// el journal no distingeix «no calia» de «s'ha saltat per error», i qualsevol
// mesura de quant temps es perd amb l'explorador queda a cegues.
func ScoutReason(cfg *config.Config, mode, task, workspace string) string {
	if mode == ModeInspect {
		if ExploreWanted(cfg, mode, task, workspace) {
			return "explorar (mode inspect)"
		}
		return "xat (mode inspect sense res a explorar)"
	}
	if !ScoutWanted(cfg, mode, task) {
		return "mode " + mode + " (el scout és per a feina de codi)"
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Agent.Scout), "always") {
		return "always (configurat a la força)"
	}
	if tascaJaDiuOnMirar(task) {
		return "la tasca ja diu on mirar (fitxer o codi)"
	}
	if strings.TrimSpace(workspace) == "" {
		return "auto (sense workspace, no es pot saber la mida)"
	}
	// Mateixa compte que projectePetit: si no es poden comptar, és petit.
	fitxers, _ := repomap.ListFiles(workspace)
	if len(fitxers) < scoutMinFitxers {
		return fmt.Sprintf("projecte petit (%d fitxers < %d)", len(fitxers), scoutMinFitxers)
	}
	return fmt.Sprintf("auto (%d fitxers)", len(fitxers))
}
