package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"gregal/internal/config"
	"gregal/internal/knowledge"
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

// DirsSkillsWith és DirsSkills amb directoris extra al davant: primer
// l'extra (usuari), després la persona, i el projecte mana al final.
func DirsSkillsWith(extra []string, cwd string) []string {
	var dirs []string
	dirs = append(dirs, extra...)
	dirs = append(dirs, DirsSkills(cwd)...)
	return dirs
}

// WithSkillsDirs guarda directoris de skills extra al context, perquè
// l'eina `skill` i l'índex del prompt els vegin sense tocar signatures.
type skillsDirsKey struct{}

func WithSkillsDirs(ctx context.Context, dirs ...string) context.Context {
	if len(dirs) == 0 {
		return ctx
	}
	return context.WithValue(ctx, skillsDirsKey{}, dirs)
}

// SkillsDirsOf retorna els directoris extra del context (nil si no n'hi ha).
func SkillsDirsOf(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(skillsDirsKey{}).([]string); ok {
		return v
	}
	return nil
}

// LlegeixSkill és el que executa l'eina `skill`. Si el nom no hi és, es
// diuen els que hi ha: un nom mal escrit no ha de costar un pas perdut.
func LlegeixSkill(cwd, nom string) (string, []string, error) {
	return LlegeixSkillDirs(cwd, nom)
}

// LlegeixSkillDirs és LlegeixSkill amb directoris extra al davant (p. ex.
// les skills d'un usuari d'un servidor multiusuari).
func LlegeixSkillDirs(cwd, nom string, extra ...string) (string, []string, error) {
	dirs := DirsSkillsWith(extra, cwd)
	s, ok := skills.Per(nom, dirs...)
	if ok && !skills.Habilitada(s.Nom) {
		return "", nil, fmt.Errorf("la skill %q està desactivada als ajustos", s.Nom)
	}
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
func ContextProjecte(cwd string) string { return ContextProjecteSessio(cwd, "") }

// ContextProjecteSessio és ContextProjecte sabent també la sessió: així
// el prompt inclou les col·leccions de coneixement d'aquesta conversa.
func ContextProjecteSessio(cwd, session string) string {
	return ContextProjecteExtra(cwd, session)
}

// ContextProjecteExtra és ContextProjecteSessio amb directoris de skills
// extra (per usuari d'un servidor): entren a l'índex del prompt.
func ContextProjecteExtra(cwd, session string, extraDirs ...string) string {
	parts := make([]string, 0, 4)
	if mem := LoadProjectMemory(cwd); mem != "" {
		parts = append(parts, mem)
	}
	if idx := skills.Index(skills.Cataleg(DirsSkillsWith(extraDirs, cwd)...)); idx != "" {
		parts = append(parts, idx)
	}
	if idx := IndexSubagents(cwd); idx != "" {
		parts = append(parts, idx)
	}
	if idx := knowledge.PromptIndex(knowledge.Where{Project: strings.TrimSpace(cwd), Session: session}); idx != "" {
		parts = append(parts, idx)
	}
	return strings.Join(parts, "\n\n")
}
