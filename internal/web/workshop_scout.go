package web

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// runScout fa córrer l'investigador d'inici de torn (agent.RunScout) i en
// deixa rastre a la sala: un carril de l'investigador mentre treballa i un
// relleu cap al coordinador amb el resum. Torna la conversa amb el resum
// injectat a la tasca, o la mateixa si no n'hi ha, i si n'hi ha posat.
func (s *Server) runScout(ctx context.Context, generation uint64, mode, task, workspace string, convo []llm.Message, maxSteps int, uso *llm.UsageMeter, provider string, emit func(string, any)) ([]llm.Message, bool) {
	brief, ok := s.runScoutBrief(ctx, generation, mode, task, workspace, maxSteps, uso, provider, emit)
	if !ok {
		return convo, false
	}
	return agent.InjectBriefFor(convo, brief, mode), true
}

// runScoutBrief fa la feina de l'investigador i torna el resum, sense tocar
// la conversa. Així el mateix codi serveix per al camí bloquejant (les
// preguntes de mode inspect, on el resum ÉS la resposta) i per al camí
// paral·lel, que el lliura al torn en curs pel canal de steering.
func (s *Server) runScoutBrief(ctx context.Context, generation uint64, mode, task, workspace string, maxSteps int, uso *llm.UsageMeter, provider string, emit func(string, any)) (string, bool) {
	if mode == agent.ModeInspect {
		emit("status", map[string]string{"message": s.tr("Un subagent explorador busca la resposta al codi abans que el coordinador respongui…",
			"An explorer subagent is searching the code for the answer before the coordinator replies…")})
	} else if n := len(agent.ScoutFocuses(task)); n > 1 && agent.ParallelSubagents(s.cfg) > 1 {
		emit("status", map[string]string{"message": s.tr(fmt.Sprintf("%d subagents investigadors revisen la tasca en paral·lel, un per àrea…", n),
			fmt.Sprintf("%d researcher subagents are investigating the task in parallel, one per area…", n))})
	} else {
		emit("status", map[string]string{"message": s.tr("Un subagent investigador revisa la tasca abans que el coordinador comenci…",
			"A researcher subagent is investigating the task before the coordinator starts…")})
	}
	id := agent.NewDelegateID()
	s.updateWorkshop(generation, "tools", 0, maxSteps, "scout", "")
	lane, ho := s.beginLane(generation, 0, laneInfo{Role: roleResearcher, Kind: "scout", Tools: []string{"delegate"}}, id)
	if ho != nil {
		emit("handoff", handoffPayload(*ho))
	}
	if lane.Seq > 0 {
		emit("lane", lanePayload(lane))
	}
	res := agent.RunScouts(ctx, agent.ScoutOptions{ID: id, Cfg: s.cfg, Task: task, Workspace: workspace, Session: s.id, Mode: mode})
	// Els tokens del scout van a l'investigador abans que el torn canviï de mans.
	s.noteUsage(generation, uso, provider)
	result := "ok"
	if res.Brief == "" {
		result = "fail"
	}
	if done, ok := s.endLane(generation, lane.Seq, result); ok {
		emit("lane", lanePayload(done))
	}
	if ctx.Err() != nil {
		return "", false
	}
	if res.Brief == "" {
		if back := s.handoff(generation, roleCoordinator, "report", 0); back != nil {
			emit("handoff", handoffPayload(*back))
		}
		msg := ""
		if res.Err != nil {
			msg = redactError(res.Err)
		}
		emit("status", map[string]string{"message": s.tr("L'investigador no ha pogut acabar; el coordinador continua sense resum. ", "The researcher could not finish; the coordinator continues without a brief. ") + msg})
		return "", false
	}
	if back := s.handoff(generation, roleCoordinator, "brief", 0); back != nil {
		emit("handoff", handoffPayload(*back))
	}
	return res.Brief, true
}

// scoutAlDarrere executa l'investigador sense fer esperar el coordinador i
// lliura el brief al tornar, pel canal de steering del torn en curs.
//
// Per què: mesurat en aquest repo, el scout s'enduia 45-57 s per torn
// (71-87% del total) ABANS del primer pas del coordinador. El brief no cal
// per al primer pas: cal quan el coordinador ja ha començat a moure's. Si el
// torn s'acaba abans que l'investigador, el brief es descarta (queueSteer
// torna false) i no queda res penjat.
func (s *Server) scoutAlDarrere(ctx context.Context, mu *sync.Mutex, generation uint64, mode, task, workspace string, maxSteps int, uso *llm.UsageMeter, provider string, emit func(string, any)) {
	t0 := time.Now()
	// Els events del scout surten per la mateixa connexió SSE que els del
	// coordinador: es serialitzen amb el mateix pany que els del delegate.
	emitSegur := func(ev string, v any) {
		mu.Lock()
		defer mu.Unlock()
		emit(ev, v)
	}
	brief, ok := s.runScoutBrief(ctx, generation, mode, task, workspace, maxSteps, uso, provider, emitSegur)
	if !ok || brief == "" {
		return
	}
	if !s.queueSteer(agent.ScoutNote(brief), false) {
		log.Printf("scout: brief de %d caràcters descartat (el torn ja havia acabat, %.1fs)", len(brief), time.Since(t0).Seconds())
		return
	}
	log.Printf("scout: brief lliurat al torn en curs al cap de %.1fs (%d caràcters)", time.Since(t0).Seconds(), len(brief))
}
