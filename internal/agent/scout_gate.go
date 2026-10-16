package agent

import (
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
// always el scout corre igualment.
func ScoutWantedIn(cfg *config.Config, mode, task, workspace string) bool {
	if !ScoutWanted(cfg, mode, task) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(cfg.Agent.Scout), "always") {
		return true
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
