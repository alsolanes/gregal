package telegram

import (
	"context"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// planKB són els botons de la targeta de pla (callback curt: el pla viu al
// servidor, no a les dades del botó).
func (b *Bot) planKB() Keyboard {
	return Keyboard{{{Text: "▶ " + b.tr("Executa", "Run"), Data: "plan:run"}, {Text: b.tr("Descarta", "Discard"), Data: "plan:drop"}}}
}

// planKBParcial afegeix «Continua explorant» a un pla parcial.
func (b *Bot) planKBParcial() Keyboard {
	return Keyboard{
		{{Text: "↻ " + b.tr("Continua explorant", "Continue exploring"), Data: "plan:more"}},
		{{Text: "▶ " + b.tr("Executa igualment", "Run anyway"), Data: "plan:run"}, {Text: b.tr("Descarta", "Discard"), Data: "plan:drop"}},
	}
}

// planCommand explora en read-only i presenta el pla amb botons.
// /plan <tasca> — mateix contracte que el TUI i la webapp.
func (b *Bot) planCommand(ctx context.Context, chatID int64, st *chatState, arg string) {
	if strings.TrimSpace(arg) == "" {
		b.send(ctx, chatID, b.tr("ús: /plan &lt;tasca&gt; (explora sense actuar i proposa un pla)", "usage: /plan &lt;task&gt; (inspect without making changes and propose a plan)"))
		return
	}
	// La directiva d'idioma tanca l'últim missatge de sistema.
	llengua := config.LanguageDirective(b.cfg.Lang())
	hist := []llm.Message{
		{Role: "system", Content: agent.PlanSystem},
		{Role: "user", Content: arg},
	}
	if info := agent.ContextProjecte(b.dirOf(st)); info != "" {
		hist = append(hist, llm.Message{Role: "system", Content: info + "\n\n" + llengua})
	} else {
		hist[0].Content += "\n\n" + llengua
	}
	b.planExplora(ctx, chatID, st, hist, "🔍 "+b.tr("Explorant (sense actuar)…", "Inspecting (read-only)…"))
}

// planContinue reprèn l'última exploració parcial amb pressupost nou i la
// conversa intacta (botó «Continua explorant»).
func (b *Bot) planContinue(ctx context.Context, chatID int64, st *chatState) {
	b.mu.Lock()
	hist := b.planHists[chatID]
	b.mu.Unlock()
	if len(hist) == 0 {
		b.send(ctx, chatID, b.tr("⚠️ No hi ha cap exploració per continuar (fes /plan &lt;tasca&gt;)", "⚠️ Nothing to continue (use /plan &lt;task&gt;)"))
		return
	}
	hist = agent.AfegeixUsuari(append([]llm.Message(nil), hist...), agent.PromptContinuaExploracio)
	b.planExplora(ctx, chatID, st, hist, "🔍 "+b.tr("Continuo l’exploració on s’havia quedat…", "Continuing the exploration where it stopped…"))
}

// planExplora fa el bucle d'exploració read-only a partir de hist. Mai no
// acaba sense res per pressupost: avisa el model abans del final i, en
// arribar-hi (o si el model cau a mig camí), demana la millor resposta
// sense eines i la presenta com a PLA PARCIAL amb «Continua explorant».
func (b *Bot) planExplora(ctx context.Context, chatID int64, st *chatState, hist []llm.Message, intro string) {
	st.mu.Lock()
	if st.busy {
		st.mu.Unlock()
		b.sendKB(ctx, chatID, "⏳ "+b.tr("Encara treballo en aquest xat · /stop per aturar-ho", "I’m still working in this chat · use /stop to cancel"), b.stopKB())
		return
	}
	st.busy = true
	st.mu.Unlock()

	tctx, cancel := context.WithCancel(ctx)
	st.mu.Lock()
	st.cancel = cancel
	st.mu.Unlock()
	defer func() {
		cancel()
		st.mu.Lock()
		st.busy = false
		st.cancel = nil
		st.mu.Unlock()
	}()

	// "escrivint…" mentre dura l'exploració.
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				_ = b.api.SendChatAction(tctx, chatID, "typing")
			}
		}
	}()
	defer close(done)

	b.send(ctx, chatID, intro)
	limit := agent.PassosPla(b.cfg.Agent.MaxSteps)
	var plan string
	acabat := false
	motiu := agent.MotiuPassos
	for step := 1; step <= limit; step++ {
		p, r := b.roleRefFor(chatID, st.role)
		hist = agent.AmbAvisPassos(hist, step, limit)
		cctx, cancelStep := context.WithTimeout(tctx, 180*time.Second)
		content, calls, _, err := b.client.ChatWithToolsFO(cctx, b.cfg.PrimTarget(p, r), b.cfg.FallbackTarget(r), hist,
			r.Temperature, r.MaxTokens, agent.Specs(),
			func(model string) {
				b.send(ctx, chatID, b.tr("↪ Primari caigut · respon el fallback ", "↪ Primary failed · using fallback ")+esc(model))
			})
		cancelStep()
		if err != nil {
			b.send(ctx, chatID, "⚠️ "+b.tr("Error del model: ", "Model error: ")+truncate(err.Error(), 300))
			// Sense res explorat (o aturat amb /stop): no hi ha feina a salvar.
			if step == 1 || tctx.Err() != nil {
				return
			}
			motiu = err.Error()
			break
		}
		hist = append(hist, llm.Message{Role: "assistant", Content: content, ToolCalls: calls})
		if strings.TrimSpace(content) != "" {
			plan = content
		}
		if len(calls) == 0 {
			acabat = true
			break
		}
		for _, c := range calls {
			dec, reason := b.polOf(st).Decide(agent.ModeInspect, c.Function.Name, c.Function.Arguments)
			var out string
			var outImgs []string
			switch dec {
			case "allow":
				o, oi, err := agent.Exec(c.Function.Name, c.Function.Arguments)
				outImgs = oi
				if err != nil {
					o = "ERROR: " + err.Error()
				}
				out = o
			default:
				out = "TOOL NOT AVAILABLE (read-only exploration): " + reason
			}
			hist = append(hist, agent.ToolMsg(c, out, outImgs...))
		}
	}
	if !acabat {
		// Pressupost exhaurit (o model caigut): crida final sense eines.
		if motiu == agent.MotiuPassos {
			b.send(ctx, chatID, "⏳ "+b.tr("S’han acabat els passos d’exploració: demano el millor pla amb el que s’ha vist…", "Exploration steps used up: asking for the best plan from what was seen…"))
		}
		p, r := b.roleRefFor(chatID, st.role)
		fh := append(append([]llm.Message(nil), hist...), llm.Message{Role: "user", Content: agent.PromptFinalExploracio(b.cfg.Lang())})
		fctx, cancelF := context.WithTimeout(tctx, agent.TimeoutDecisio)
		out, _, ferr := b.client.ChatFO(fctx, b.cfg.PrimTarget(p, r), b.cfg.FallbackTarget(r), fh, r.Temperature, r.MaxTokens, nil)
		cancelF()
		plan = agent.NetejaFinal(out)
		if ferr != nil || plan == "" {
			m := motiu
			if ferr != nil {
				m += "; " + ferr.Error()
			}
			plan = agent.ResumDeFeina(hist, m, b.cfg.Lang())
		}
		hist = append(hist, llm.Message{Role: "assistant", Content: plan})
	}
	b.mu.Lock()
	if b.planHists == nil {
		b.planHists = map[int64][]llm.Message{}
	}
	b.planHists[chatID] = hist
	b.mu.Unlock()
	if strings.TrimSpace(plan) == "" {
		b.send(ctx, chatID, "⚠️ "+b.tr("Exploració buida: torna-ho a provar", "No plan was produced; try again"))
		return
	}
	b.mu.Lock()
	if b.pendingPlans == nil {
		b.pendingPlans = map[int64]string{}
	}
	b.pendingPlans[chatID] = plan
	b.mu.Unlock()
	if !acabat {
		nota := b.tr("s’han acabat els passos abans d’acabar d’explorar", "the steps ran out before the exploration finished")
		if motiu != agent.MotiuPassos {
			nota = b.tr("l’exploració s’ha aturat abans d’hora", "the exploration stopped early")
		}
		b.sendKB(ctx, chatID, "◇ <b>"+b.tr("PLA PARCIAL", "PARTIAL PLAN")+"</b> ("+nota+")\n"+esc(plan), b.planKBParcial())
		return
	}
	b.sendKB(ctx, chatID, "◇ <b>"+b.tr("PLA", "PLAN")+"</b> ("+b.tr("exploració read-only · res modificat", "read-only inspection · no changes made")+")\n"+esc(plan), b.planKB())
}

// planRun executa el pla pendent en mode codi.
func (b *Bot) planRun(ctx context.Context, chatID int64, st *chatState) {
	b.mu.Lock()
	plan, ok := b.pendingPlans[chatID]
	if ok {
		delete(b.pendingPlans, chatID)
	}
	b.mu.Unlock()
	if !ok || strings.TrimSpace(plan) == "" {
		b.send(ctx, chatID, b.tr("⚠️ No hi ha cap pla pendent (fes /plan &lt;tasca&gt;)", "⚠️ No pending plan (use /plan &lt;task&gt;)"))
		return
	}
	st.mu.Lock()
	st.mode = agent.ModeCode
	st.mu.Unlock()
	b.send(ctx, chatID, "◆ "+b.tr("Executant el pla en mode codi", "Running the plan in code mode"))
	b.turn(ctx, chatID, st, "Executa aquest pla pas a pas:\n"+plan)
}

// planDrop descarta el pla pendent.
func (b *Bot) planDrop(ctx context.Context, chatID int64) {
	b.mu.Lock()
	_, ok := b.pendingPlans[chatID]
	if ok {
		delete(b.pendingPlans, chatID)
	}
	b.mu.Unlock()
	if !ok {
		b.send(ctx, chatID, b.tr("⚠️ No hi ha cap pla pendent", "⚠️ No pending plan"))
		return
	}
	b.send(ctx, chatID, b.tr("Pla descartat", "Plan discarded"))
}
