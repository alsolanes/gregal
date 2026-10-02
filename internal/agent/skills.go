package agent

import (
	"fmt"
	"path/filepath"
	"strings"

	"gregal/internal/config"
	"gregal/internal/skills"
)

// Les skills són el coneixement que l'agent obre quan li fa falta (vegeu
// internal/skills). Aquí només hi ha d'on es llegeixen i com entren al
// prompt: el paquet skills no ha de saber res del config.

// DirsSkills són els directoris on es busquen, de menys a més específic:
// primer les de la persona, després les del projecte (que manen).
func DirsSkills(cwd string) []string {
	var dirs []string
	if base := filepath.Dir(config.DefaultPath()); base != "" && base != "." {
		dirs = append(dirs, filepath.Join(base, "skills"))
	}
	if cwd = strings.TrimSpace(cwd); cwd != "" {
		dirs = append(dirs, filepath.Join(cwd, ".gregal", "skills"))
	}
	return dirs
}

// IndexSkills és el bloc d'una línia per skill que va al prompt.
func IndexSkills(cwd string) string {
	return skills.Index(skills.Cataleg(DirsSkills(cwd)...))
}

// LlegeixSkill és el que executa l'eina `skill`. Si el nom no hi és, es
// diuen els que hi ha: un nom mal escrit no ha de costar un pas perdut.
func LlegeixSkill(cwd, nom string) (string, []string, error) {
	dirs := DirsSkills(cwd)
	s, ok := skills.Per(nom, dirs...)
	if !ok {
		disponibles := skills.Noms(dirs...)
		if len(disponibles) == 0 {
			return "", nil, fmt.Errorf("no hi ha cap skill")
		}
		return "", nil, fmt.Errorf("no hi ha cap skill que es digui %q; les que hi ha: %s", nom, strings.Join(disponibles, ", "))
	}
	return "SKILL " + s.Nom + "\n\n" + s.Cos, nil, nil
}

// ContextProjecte és el que se sap del lloc on es treballa: la memòria
// (AGENTS.md i les notes) i l'índex de skills. Va en una sola funció
// perquè els sis llocs que componen un prompt de sistema no se n'hagin de
// recordar per separat; abans cadascun cridava LoadProjectMemory i prou.
func ContextProjecte(cwd string) string {
	parts := make([]string, 0, 2)
	if mem := LoadProjectMemory(cwd); mem != "" {
		parts = append(parts, mem)
	}
	if idx := IndexSkills(cwd); idx != "" {
		parts = append(parts, idx)
	}
	return strings.Join(parts, "\n\n")
}
