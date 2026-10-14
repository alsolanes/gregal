package web

import (
	"context"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// runScout fa córrer l'investigador d'inici de torn (agent.RunScout) i en
// deixa rastre a la sala: un carril de l'investigador mentre treballa i un
// relleu cap al coordinador amb el resum. Torna la conversa amb el resum
// injectat a la tasca, o la mateixa si no n'hi ha.
func (s *Server) runScout(ctx context.Context, generation uint64, task, workspace string, convo []llm.Message, maxSteps int, uso *llm.UsageMeter, provider string, emit func(string, any)) []llm.Message {
	emit("status", map[string]string{"message": s.tr("Un subagent investigador revisa la tasca abans que el coordinador comenci…",
		"A researcher subagent is investigating the task before the coordinator starts…")})
	id := agent.NewDelegateID()
	s.updateWorkshop(generation, "tools", 0, maxSteps, "scout", "")
	lane, ho := s.beginLane(generation, 0, laneInfo{Role: roleResearcher, Kind: "scout", Tools: []string{"delegate"}}, id)
	if ho != nil {
		emit("handoff", handoffPayload(*ho))
	}
	if lane.Seq > 0 {
		emit("lane", lanePayload(lane))
	}
	res := agent.RunScout(ctx, agent.ScoutOptions{ID: id, Cfg: s.cfg, Task: task, Workspace: workspace, Session: s.id})
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
		return convo
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
		return convo
	}
	if back := s.handoff(generation, roleCoordinator, "brief", 0); back != nil {
		emit("handoff", handoffPayload(*back))
	}
	return agent.InjectScoutBrief(convo, res.Brief)
}
