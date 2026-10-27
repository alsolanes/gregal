package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gregal/internal/llm"
)

// Implementar en paral·lel: a delegate, una tasca pot portar `files`, els
// fitxers que aquell subagent és l'únic que pot crear o editar. Així el pare
// reparteix una feina gran per fitxers o àrees i els fills la fan alhora
// sense trepitjar-se; el pare integra i prova al final. Només en modes que
// escriuen (code, autònom, debug), amb la mateixa regla que el pare (escriure
// dins del projecte passa sol, si el config no diu el contrari).

// MaxOwnedFiles topa els fitxers d'un sol subagent.
const MaxOwnedFiles = 12

// MaxOwnedPrompt: la tasca d'un fill que implementa porta la interfície
// compartida (noms, signatures, formats), i 2000 caràcters no hi caben.
const MaxOwnedPrompt = 6000

// ownedChildSteps: un fill que implementa necessita més passos que un que llegeix.
const ownedChildSteps = 24

type turnModeKey struct{}

// WithTurnMode diu a les eines quin és el mode del torn (delegate el fa
// servir per saber si els fills poden escriure).
func WithTurnMode(ctx context.Context, mode string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, turnModeKey{}, mode)
}

func turnModeFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	m, _ := ctx.Value(turnModeKey{}).(string)
	return m
}

// modeWrites diu si en aquest mode l'agent pot escriure fitxers.
func modeWrites(mode string) bool {
	return mode == ModeCode || mode == ModeAutonomous || mode == ModeDebug
}

type ownedFilesKey struct{}

func withOwnedFiles(ctx context.Context, files []string) context.Context {
	if len(files) == 0 {
		return ctx
	}
	return context.WithValue(ctx, ownedFilesKey{}, files)
}

func ownedFilesFrom(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	f, _ := ctx.Value(ownedFilesKey{}).([]string)
	return f
}

// resolveOwnedFiles valida els `files` de cada tasca: camins dins del
// projecte, com a molt MaxOwnedFiles per tasca i cap fitxer a dues tasques.
// Torna els camins absoluts per tasca.
func resolveOwnedFiles(mode, workspace string, perTask [][]string) ([][]string, error) {
	has := false
	for _, f := range perTask {
		if len(f) > 0 {
			has = true
		}
	}
	if !has {
		return make([][]string, len(perTask)), nil
	}
	if !modeWrites(mode) {
		return nil, fmt.Errorf("files: subagents can only write in a mode that writes (code, autonomous, debug); in this mode delegate read-only tasks")
	}
	owner := map[string]int{}
	out := make([][]string, len(perTask))
	for i, files := range perTask {
		if len(files) > MaxOwnedFiles {
			return nil, fmt.Errorf("task %d: at most %d files per subagent (asked %d)", i+1, MaxOwnedFiles, len(files))
		}
		for _, f := range files {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			abs := f
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(workspace, abs)
			}
			abs = filepath.Clean(abs)
			if !InsideProject(workspace, abs) {
				return nil, fmt.Errorf("task %d: %s is outside the project", i+1, f)
			}
			key := strings.ToLower(abs)
			if j, ok := owner[key]; ok && j != i {
				return nil, fmt.Errorf("tasks %d and %d both own %s: give every file to one subagent only", j+1, i+1, f)
			}
			if _, ok := owner[key]; !ok {
				owner[key] = i
				out[i] = append(out[i], abs)
			}
		}
	}
	return out, nil
}

// ownsPath diu si path és un dels fitxers del fill.
func ownsPath(owned []string, workspace, path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspace, path)
	}
	path = strings.ToLower(filepath.Clean(path))
	for _, o := range owned {
		if strings.ToLower(o) == path {
			return true
		}
	}
	return false
}

// ownedBrief és el que el fill sap de la seva part.
func ownedBrief(owned []string, workspace string) string {
	rel := make([]string, 0, len(owned))
	for _, o := range owned {
		if r, err := filepath.Rel(workspace, o); err == nil {
			rel = append(rel, filepath.ToSlash(r))
		} else {
			rel = append(rel, o)
		}
	}
	sort.Strings(rel)
	return "You implement one part of a larger change, in parallel with other subagents. You own exactly these files and may create or edit only them: " +
		strings.Join(rel, ", ") + ". Other files belong to other subagents or to the parent: do not modify them; if they need a change, say exactly which in your summary. " +
		"You can only run read-only commands: the parent integrates and runs the full checks. Read what you need, write complete working code, and finish with a short summary: files changed, what you did, and anything the parent must do."
}

// planOwnedCall decideix una crida d'un fill amb fitxers: escriure els seus
// passa amb la política de code; escriure'n d'altres o executar res que no
// sigui lectura, no.
func planOwnedCall(pol *Policy, owned []string, workspace string, c llm.ToolCall, plan CallPlan) CallPlan {
	switch c.Function.Name {
	case "write", "edit", "patch":
		if !ownsPath(owned, workspace, toolPath(c.Function.Name, c.Function.Arguments)) {
			why := "not your file: you own only the files you were given; describe the change in your summary instead"
			return CallPlan{Fixed: "TOOL DENIED: " + why, Reason: why}
		}
		return plan
	}
	if plan.Fixed != "" {
		return plan
	}
	if d, _ := pol.Decide(ModeInspect, c.Function.Name, c.Function.Arguments); d != "allow" {
		why := "subagents that implement cannot run this; the parent integrates and runs commands"
		return CallPlan{Fixed: "TOOL DENIED: " + why, Reason: why}
	}
	return plan
}

// Una tasca que demana crear o editar fitxers sense `files` deixa el fill en
// només lectura: escriu el codi a la resposta i el pare l'ha de copiar (es
// perd tot el paral·lelisme). Es detecta per un verb d'escriure i un camí
// de fitxer, i es torna l'error perquè el model hi posi els fitxers.
var (
	// L'ordre ha d'obrir una frase: «Create …», no «llegeix i mira on afegir».
	writeVerbRe = regexp.MustCompile(`(?im)(?:^|[.:!?]\s+)\s*(?:create|write|implement|add|edit|modify|rewrite|crea|escriu|implementa|afegeix|modifica|reescriu)\b`)
	filePathRe  = regexp.MustCompile(`(?i)[\w./\-]+\.(go|js|mjs|cjs|ts|tsx|jsx|py|rb|rs|java|kt|kts|c|cc|cpp|h|hpp|cs|php|swift|html|css|scss|md|json|ya?ml|toml|sql|sh|ps1)\b`)
)

func asksToWriteFiles(prompt string) bool {
	return writeVerbRe.MatchString(prompt) && filePathRe.MatchString(prompt)
}

// missingFilesError diu quines tasques volen escriure i no tenen `files`.
func missingFilesError(mode string, prompts []string, files [][]string) error {
	if !modeWrites(mode) {
		return nil
	}
	var bad []string
	for i, p := range prompts {
		if len(files[i]) == 0 && asksToWriteFiles(p) {
			bad = append(bad, fmt.Sprint(i+1))
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf("task %s asks to create or edit files but has no `files`: without it the subagent is read-only and you would have to copy its code. Add `files` with the exact files each subagent writes (no file in two tasks), or rephrase the task as read-only", strings.Join(bad, ", "))
}
