package agent

import (
	"context"
	"strings"

	"gregal/internal/llm"
)

// La part dels hooks del cicle de vida que viu dins del motor (Torn):
// session_start i user_prompt en arrencar el torn, pre_tool en decidir
// cada crida i stop abans de donar el torn per acabat. Corren síncrons
// amb el context del torn (OpcionsTorn.Ctx): cancel·lar el torn els mata.

func (t *Torn) hookCtx() context.Context {
	if t.o.Ctx != nil {
		return t.o.Ctx
	}
	return context.Background()
}

func (t *Torn) hookInput() HookInput {
	return HookInput{Session: t.o.Sessio, Workspace: t.o.Workspace, Mode: t.o.Mode}
}

// HookBloqueig és el motiu si un hook user_prompt ha aturat el torn
// abans de començar ("" si no). El TUI el pinta: OrdreError no porta
// events.
func (t *Torn) HookBloqueig() string { return t.hookBloqueig }

// hooksInici corre session_start (només en una conversa nova) i
// user_prompt. Torna fals si el torn ha quedat bloquejat.
func (t *Torn) hooksInici() bool {
	if !HiHaHooks(HookSessionStart) && !HiHaHooks(HookUserPrompt) {
		return true
	}
	ctx := t.hookCtx()
	var extra []string
	if HiHaHooks(HookSessionStart) && !t.teAssistant() {
		r := RunHooks(ctx, HookSessionStart, t.hookInput())
		if r.Context != "" {
			extra = append(extra, r.Context)
		}
	}
	if HiHaHooks(HookUserPrompt) {
		in := t.hookInput()
		in.Prompt = t.o.Tasca
		if in.Prompt == "" {
			in.Prompt = t.ultimUsuari()
		}
		r := RunHooks(ctx, HookUserPrompt, in)
		if r.Bloqueja() {
			t.hookBloqueig = "blocked by user_prompt hook: " + firstOf(r.Reason, "no reason given")
			t.acabat, t.fallit, t.motiu = true, true, t.hookBloqueig
			return false
		}
		if r.Context != "" {
			extra = append(extra, r.Context)
		}
	}
	if len(extra) > 0 {
		t.afegeixAlPrompt("\n\n[hook context]\n" + strings.Join(extra, "\n"))
	}
	return true
}

// teAssistant diu si l'historial ja porta respostes (conversa reprenent).
func (t *Torn) teAssistant() bool {
	for _, m := range t.Hist {
		if m.Role == "assistant" {
			return true
		}
	}
	return false
}

func (t *Torn) ultimUsuari() string {
	for i := len(t.Hist) - 1; i >= 0; i-- {
		if t.Hist[i].Role == "user" {
			return t.Hist[i].Content
		}
	}
	return ""
}

// afegeixAlPrompt enganxa text a l'últim missatge d'usuari (el de la
// tasca): un missatge nou al darrere faria dos torns d'usuari seguits.
func (t *Torn) afegeixAlPrompt(text string) {
	for i := len(t.Hist) - 1; i >= 0; i-- {
		if t.Hist[i].Role == "user" {
			t.Hist[i].Content += text
			return
		}
	}
	t.Hist = append(t.Hist, llm.Message{Role: "user", Content: strings.TrimSpace(text)})
}

// hookPreTool aplica pre_tool a una crida que la política deixa passar.
// deny la bloqueja (com una denegació de la política), ask la fa passar
// per l'aprovació i allow se salta l'ask de la política. Les denegacions
// de la política no les pot desfer cap hook.
func (t *Torn) hookPreTool(c llm.ToolCall, pl *CallPlan) []Event {
	if !HiHaHooks(HookPreTool) {
		return nil
	}
	in := t.hookInput()
	in.Tool, in.Args = c.Function.Name, argsRaw(c.Function.Arguments)
	r := RunHooks(t.hookCtx(), HookPreTool, in)
	var evs []Event
	for _, a := range r.Avisos {
		evs = append(evs, avis(a))
	}
	if r.Cancellat {
		return evs
	}
	switch r.Decision {
	case "deny":
		motiu := firstOf(r.Reason, "no reason given")
		pl.Fixed = "BLOCKED by pre_tool hook: " + motiu
		pl.Reason = "hook: " + motiu
		pl.Ask = false
	case "ask":
		pl.Ask = true
		pl.ForceAsk = true
	case "allow":
		pl.Ask = false
	}
	if r.Context != "" && pl.Fixed == "" {
		evs = append(evs, nota("hook: "+r.Context))
	}
	return evs
}

// hookStop corre stop abans de tancar el torn amb una resposta. Torna cert
// si un hook demana continuar: s'afegeix el motiu com a missatge d'usuari
// i el torn segueix (com a molt stop_max_continues vegades per torn).
func (t *Torn) hookStop(final string) ([]Event, bool) {
	if !HiHaHooks(HookStop) || t.sintesiDemanada {
		return nil, false
	}
	max := hooksActius().StopMaxContinues
	if max <= 0 {
		max = stopMaxDefecte
	}
	if t.stopContinues >= max {
		return nil, false
	}
	in := t.hookInput()
	in.Prompt, in.Answer, in.StopCount = t.o.Tasca, final, t.stopContinues
	r := RunHooks(t.hookCtx(), HookStop, in)
	var evs []Event
	for _, a := range r.Avisos {
		evs = append(evs, avis(a))
	}
	if r.Cancellat || !r.Continue {
		return evs, false
	}
	t.stopContinues++
	motiu := firstOf(r.Reason, r.Context, "the stop hook asked you to keep working")
	t.Hist = append(t.Hist, llm.Message{Role: "user", Content: "A stop hook asked you to keep working before finishing: " + motiu})
	return append(evs, nota("stop hook: continue — "+motiu)), true
}
