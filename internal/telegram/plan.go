package telegram

import (
	"context"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// planKB són els botons de la targeta de pla (callback curt: el pla viu al
// servidor, no a les dades del botó).
func (b *Bot) planKB() Keyboard {
	return Keyboard{{{Text: "▶ " + b.tr("Executa", "Run"), Data: "plan:run"}, {Text: b.tr("Descarta", "Discard"), Data: "plan:drop"}}}
}

// planCommand explora en read-only i presenta el pla amb botons.
// /plan <tasca> — mateix contracte que el TUI i la webapp.
func (b *Bot) planCommand(ctx context.Context, chatID int64, st *chatState, arg string) {
	if strings.TrimSpace(arg) == "" {
		b.send(ctx, chatID, b.tr("ús: /plan &lt;tasca&gt; (explora sense actuar i proposa un pla)", "usage: /plan &lt;task&gt; (inspect without making changes and propose a plan)"))
		return
	}
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

	b.send(ctx, chatID, "🔍 "+b.tr("Explorant (sense actuar)…", "Inspecting (read-only)…"))
	hist := []llm.Message{
		{Role: "system", Content: agent.PlanSystem},
		{Role: "user", Content: arg},
	}
	if info := agent.ContextProjecte(b.dirOf(st)); info != "" {
		hist = append(hist, llm.Message{Role: "system", Content: info})
	}
	var plan string
	for step := 1; step <= b.cfg.Agent.MaxSteps; step++ {
		p, r := b.roleRefFor(chatID, st.role)
		cctx, cancelStep := context.WithTimeout(tctx, 180*time.Second)
		content, calls, _, err := b.client.ChatWithToolsFO(cctx, b.cfg.PrimTarget(p, r), b.cfg.FallbackTarget(r), hist,
			r.Temperature, r.MaxTokens, agent.Specs(),
			func(model string) {
				b.send(ctx, chatID, b.tr("↪ Primari caigut · respon el fallback ", "↪ Primary failed · using fallback ")+esc(model))
			})
		cancelStep()
		if err != nil {
			b.send(ctx, chatID, "⚠️ "+b.tr("Error del model: ", "Model error: ")+truncate(err.Error(), 300))
			return
		}
		hist = append(hist, llm.Message{Role: "assistant", Content: content, ToolCalls: calls})
		if strings.TrimSpace(content) != "" {
			plan = content
		}
		if len(calls) == 0 {
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
				out = "EINA NO DISPONIBLE (exploració read-only): " + reason
			}
			hist = append(hist, agent.ToolMsg(c, out, outImgs...))
		}
	}
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
