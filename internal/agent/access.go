package agent

import "strings"

// El que la persona tria, igual a totes les interfícies (web, escriptori,
// TUI, Android): tres modes i, només a Actua, un nivell de permís.
//
//	Pregunta (ask)  → chat: respon i llegeix, no canvia res.
//	Planifica (plan) → plan: investiga i proposa un pla, no canvia res.
//	Actua (act)     → code o autonomous, segons el permís:
//	    Demana abans de canviar (ask)       → code amb AskEdits
//	    Aprova les edicions del projecte (edits, per defecte) → code
//	    Autonomia total (full)              → autonomous
//
// Els ids interns (chat, inspect, code, goal, autonomous, debug) es
// mantenen perquè els fan servir el config, l'API, les sessions desades i
// els subagents (delegate_files.go). inspect es mostra com a Pregunta; goal
// com a Planifica; debug com a Actua. goal i debug continuen existint com a
// modes avançats (/goal, /debug), fora del selector principal.
//
// Demanat el 2026-10-28: menys modes i menys opcions de permís, com ChatGPT
// i Claude.

// Modes que veu la persona.
const (
	UIModeAsk  = "ask"
	UIModePlan = "plan"
	UIModeAct  = "act"
)

// Permisos d'Actua.
const (
	AccessAsk   = "ask"
	AccessEdits = "edits"
	AccessFull  = "full"
)

// UIModes és l'ordre del selector i del cicle de Tab.
var UIModes = []string{UIModeAsk, UIModePlan, UIModeAct}

// Accesses és l'ordre del menú de permisos.
var Accesses = []string{AccessAsk, AccessEdits, AccessFull}

// UIMode tradueix un mode intern al mode que es mostra.
func UIMode(mode string) string {
	switch mode {
	case ModeChat, ModeInspect:
		return UIModeAsk
	case ModePlan, ModeGoal:
		return UIModePlan
	}
	return UIModeAct
}

// AccessOf és el permís que es mostra per a un mode intern i la política.
func AccessOf(mode string, askEdits bool) string {
	if mode == ModeAutonomous {
		return AccessFull
	}
	if askEdits {
		return AccessAsk
	}
	return AccessEdits
}

// ValidAccess diu si a és un permís conegut.
func ValidAccess(a string) bool {
	return a == AccessAsk || a == AccessEdits || a == AccessFull
}

// AccessFromWord accepta també els noms antics dels menús de permisos
// (manual, accept, auto, review) perquè les preferències desades d'abans
// continuïn valent. Torna "" si no el coneix.
func AccessFromWord(w string) string {
	switch strings.ToLower(strings.TrimSpace(w)) {
	case AccessAsk, "demana", "ask-before-changes":
		return AccessAsk
	case AccessEdits, "", "manual", "accept", "review", "auto-review", "edicions", "default":
		return AccessEdits
	case AccessFull, "auto", "autonomous", "autonom", "autònom", "total", "yolo":
		return AccessFull
	}
	return ""
}

// ModeFor és el mode intern d'un mode mostrat i un permís. Sortir de
// Pregunta o Planifica cap a Actua recupera el permís triat.
func ModeFor(uiMode, access string) string {
	switch uiMode {
	case UIModeAsk:
		return ModeChat
	case UIModePlan:
		return ModePlan
	}
	if access == AccessFull {
		return ModeAutonomous
	}
	return ModeCode
}

// ReadOnlyMode diu si el mode no pot canviar res.
func ReadOnlyMode(mode string) bool {
	return mode == ModeChat || mode == ModeInspect || mode == ModeGoal || mode == ModePlan
}

// IsActWord diu si w és el nom d'Actua que veu la persona (no l'id intern
// code): tornar a Actua recupera l'últim mode d'Actua i el seu permís.
func IsActWord(w string) bool {
	switch strings.ToLower(strings.TrimSpace(w)) {
	case UIModeAct, "actua":
		return true
	}
	return false
}
