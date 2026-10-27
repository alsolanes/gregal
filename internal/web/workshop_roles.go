package web

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// Rols de la sala d'agents: cada pas del coordinador s'atribueix a un
// carril (coordinador, investigador, constructor, provador, revisor) segons
// les eines que fa servir. És només visual: el motor no canvia. Quan el
// carril canvia hi ha un relleu (handoff) amb una etiqueta curta que la
// sala anima com un sobre que vola d'una taula a l'altra. Els fills reals
// (delegate, scout) continuen sent agents a part.

const (
	roleCoordinator = "coordinator"
	roleResearcher  = "researcher"
	roleBuilder     = "builder"
	roleTester      = "tester"
	roleReviewer    = "reviewer"
)

// maxWorkshopLanes topa la línia de temps de carrils d'un torn; els
// relleus en tenen la meitat. Els retallats es compten (LanesTrimmed).
const (
	maxWorkshopLanes    = 60
	maxWorkshopHandoffs = 30
	maxLaneFiles        = 8
)

// workshopLane és un pas atribuït a un rol: què ha fet (kind + target),
// amb quines eines, sobre quins fitxers i, en acabar, amb quin resultat.
type workshopLane struct {
	Seq     int      `json:"seq"`
	Step    int      `json:"step"`
	Role    string   `json:"role"`
	Kind    string   `json:"kind"`
	Target  string   `json:"target,omitempty"`
	N       int      `json:"n,omitempty"`
	Tools   []string `json:"tools,omitempty"`
	Files   []string `json:"files,omitempty"`
	Command string   `json:"command,omitempty"`
	// Result: ok | fail quan el pas ha acabat; buit mentre corre.
	Result     string    `json:"result,omitempty"`
	Done       bool      `json:"done,omitempty"`
	Agent      string    `json:"agent,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms,omitempty"`
}

// workshopHandoff és un relleu entre dos rols.
type workshopHandoff struct {
	Seq    int       `json:"seq"`
	Step   int       `json:"step"`
	From   string    `json:"from"`
	To     string    `json:"to"`
	Kind   string    `json:"kind"`
	N      int       `json:"n,omitempty"`
	Target string    `json:"target,omitempty"`
	Files  []string  `json:"files,omitempty"`
	At     time.Time `json:"at"`
}

// workshopUsage són els tokens que s'han consumit mentre un rol tenia el
// torn (aproximació honesta: el delta del comptador entre dues lectures).
type workshopUsage struct {
	Up    int `json:"up_tokens,omitempty"`
	Down  int `json:"down_tokens,omitempty"`
	Calls int `json:"calls,omitempty"`
}

// laneInfo és la classificació d'un pas, abans de tenir número.
type laneInfo struct {
	Role, Kind, Target, Command string
	N                           int
	Tools, Files                []string
}

var testCommand = regexp.MustCompile(`(?i)(^|[\s;&|(])(go (test|vet|build)|pytest|py\.test|unittest|python3? -m (pytest|unittest)|npm (run )?(test|build|lint)|pnpm (test|build|lint)|yarn (test|build|lint)|jest|vitest|cargo (test|build|check|clippy)|dotnet (test|build)|make( |$)|tsc|eslint|ruff|mypy|flake8|mvn|gradle|ctest|node --test|lint|vet)`)

// readishTool: eines desconegudes (MCP…) que pel nom només consulten.
var readishTool = regexp.MustCompile(`(read|get|list|search|fetch|query|find|show)`)

var readOnlyCommand = regexp.MustCompile(`(?i)^\s*(ls|dir|cat|type|head|tail|find|grep|rg|wc|tree|pwd|stat|file|which|where|git (status|log|diff|show|branch|blame|ls-files)|sed -n|less|more|echo)\b`)

// classifyCall diu a quin rol pertany una crida i què fa.
func classifyCall(name, args string) laneInfo {
	var a map[string]any
	_ = json.Unmarshal([]byte(args), &a)
	str := func(k string) string { v, _ := a[k].(string); return strings.TrimSpace(v) }
	info := laneInfo{Tools: []string{name}}
	file := func(p string) {
		if p != "" {
			info.Files = append(info.Files, p)
			if info.Target == "" {
				info.Target = p
			}
		}
	}
	switch name {
	case "todowrite", "todoread", "question", "skill", "preview":
		info.Role, info.Kind = roleCoordinator, "plan"
	case "delegate":
		info.Role, info.Kind = roleCoordinator, "delegate"
		if ts, ok := a["tasks"].([]any); ok {
			info.N = len(ts)
		}
	case "read", "read_image", "office_read", "pdf_read", "symbols":
		info.Role, info.Kind = roleResearcher, "read"
		file(str("path"))
	case "grep", "glob", "knowledge_search", "gh_issue", "gh_pr":
		info.Role, info.Kind = roleResearcher, "search"
		info.Target = firstNonEmpty(str("pattern"), str("query"), str("action"))
	case "web_search":
		info.Role, info.Kind = roleResearcher, "web"
		info.Target = str("query")
	case "web_fetch":
		info.Role, info.Kind = roleResearcher, "web"
		info.Target = hostOf(str("url"))
	case "browser":
		info.Role, info.Kind = roleResearcher, "web"
		info.Target = firstNonEmpty(hostOf(str("url")), str("action"))
	case "write", "edit", "patch", "office_edit", "office_create", "pdf_edit", "pdf_create":
		info.Role, info.Kind = roleBuilder, "edit"
		file(str("path"))
	case "git_commit", "gh_pr_create":
		info.Role, info.Kind = roleBuilder, "commit"
	case "bash", "bash_background":
		cmd := str("command")
		info.Command = workshopText(cmd, 160)
		info.Target = workshopText(cmd, 60)
		switch {
		case agent.EsComprovacio(cmd) || testCommand.MatchString(cmd):
			info.Role, info.Kind = roleTester, "test"
		case readOnlyCommand.MatchString(cmd):
			info.Role, info.Kind = roleResearcher, "search"
		default:
			info.Role, info.Kind = roleBuilder, "run"
		}
	case "bash_output", "bash_kill":
		// Segueix el procés de fons: el rol que ja tenia el torn.
		info.Kind = "run"
	default:
		low := strings.ToLower(name)
		if readishTool.MatchString(low) {
			info.Role, info.Kind = roleResearcher, "search"
		} else {
			info.Role, info.Kind = roleCoordinator, "tool"
		}
		info.Target = name
	}
	return info
}

// rolePriority: quan un pas barreja eines de rols diferents, mana el que
// més canvia el món (provar > construir > investigar > coordinar).
var rolePriority = map[string]int{roleTester: 4, roleBuilder: 3, roleReviewer: 3, roleResearcher: 2, roleCoordinator: 1}

// classifyCalls resumeix les crides d'un pas en un sol carril.
func classifyCalls(calls []llm.ToolCall) laneInfo {
	var out laneInfo
	seenFile := map[string]bool{}
	seenTool := map[string]bool{}
	count := 0
	for _, c := range calls {
		info := classifyCall(c.Function.Name, c.Function.Arguments)
		if !seenTool[c.Function.Name] {
			seenTool[c.Function.Name] = true
			out.Tools = append(out.Tools, c.Function.Name)
		}
		for _, f := range info.Files {
			if !seenFile[f] && len(out.Files) < maxLaneFiles {
				seenFile[f] = true
				out.Files = append(out.Files, f)
			}
		}
		if info.Role == "" {
			if out.Role == "" && out.Kind == "" {
				out.Kind = info.Kind
			}
			continue
		}
		switch {
		case rolePriority[info.Role] > rolePriority[out.Role]:
			out.Role, out.Kind, out.Target, out.Command, out.N = info.Role, info.Kind, info.Target, info.Command, info.N
			count = 1
		case info.Role == out.Role && info.Kind == out.Kind:
			count++
			if out.Command == "" {
				out.Command = info.Command
			}
		}
	}
	if out.N == 0 && count > 1 {
		out.N = count
	}
	if (out.Kind == "read" || out.Kind == "edit") && len(out.Files) > 1 {
		out.N = len(out.Files)
	}
	return out
}

// laneResult decideix ok/fail d'un pas acabat: proves vermelles, eines amb
// ERROR. Un pas d'investigació sense resultats no és un fracàs.
func laneResult(info laneInfo, outs []agent.Execucio) string {
	failed := false
	for _, o := range outs {
		out := strings.TrimSpace(o.Sortida)
		switch info.Role {
		case roleTester:
			if strings.HasPrefix(out, "ERROR") || agent.SemblaFracas(out) {
				failed = true
			}
		case roleBuilder:
			if strings.HasPrefix(out, "ERROR") || strings.HasPrefix(out, "TOOL DENIED") {
				failed = true
			}
		}
	}
	if failed {
		return "fail"
	}
	return "ok"
}

func hostOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return workshopText(raw, 60)
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

// handoffKind és l'etiqueta del relleu segons qui el rep.
func handoffKind(to string, info laneInfo) string {
	switch to {
	case roleResearcher:
		if info.Kind == "web" {
			return "web"
		}
		return "read"
	case roleBuilder:
		if info.Kind == "edit" {
			return "edit"
		}
		return info.Kind
	case roleTester:
		return "test"
	case roleReviewer:
		return "review"
	}
	return "report"
}

// beginLane obre el carril d'un pas, decideix el relleu si el rol canvia i
// fa del rol nou el que té el torn. Torna seq 0 si el torn ja no és aquest.
func (s *Server) beginLane(generation uint64, step int, info laneInfo, agentID string) (workshopLane, *workshopHandoff) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.workshopSeq || s.activity == nil {
		return workshopLane{}, nil
	}
	a := s.activity
	role := info.Role
	if role == "" {
		role = a.Role
	}
	if role == "" {
		role = roleCoordinator
	}
	now := time.Now().UTC()
	a.laneSeq++
	lane := workshopLane{Seq: a.laneSeq, Step: step, Role: role, Kind: info.Kind, Target: workshopText(info.Target, 120),
		N: info.N, Tools: info.Tools, Files: info.Files, Command: info.Command, Agent: agentID, StartedAt: now}
	var ho *workshopHandoff
	prev := a.Role
	if prev == "" {
		prev = roleCoordinator
	}
	if prev != role {
		ho = s.addHandoffLocked(step, prev, role, handoffKind(role, info), info.N, lane.Target, info.Files, now)
	}
	a.Role = role
	a.Lanes = append(a.Lanes, lane)
	if len(a.Lanes) > maxWorkshopLanes {
		a.LanesTrimmed += len(a.Lanes) - maxWorkshopLanes
		a.Lanes = append([]workshopLane{}, a.Lanes[len(a.Lanes)-maxWorkshopLanes:]...)
	}
	a.UpdatedAt = now
	return lane, ho
}

// addHandoffLocked desa un relleu (cal tenir s.mu i activity no nil).
func (s *Server) addHandoffLocked(step int, from, to, kind string, n int, target string, files []string, at time.Time) *workshopHandoff {
	a := s.activity
	a.laneSeq++
	if kind == "edit" && n == 0 && len(files) > 0 {
		n = len(files)
	}
	h := workshopHandoff{Seq: a.laneSeq, Step: step, From: from, To: to, Kind: kind, N: n, Target: target, Files: files, At: at}
	a.Handoffs = append(a.Handoffs, h)
	if len(a.Handoffs) > maxWorkshopHandoffs {
		a.Handoffs = append([]workshopHandoff{}, a.Handoffs[len(a.Handoffs)-maxWorkshopHandoffs:]...)
	}
	return &h
}

// handoff afegeix un relleu explícit (el scout lliura el resum, el torn
// acaba i tot torna al coordinador) i passa el torn a `to`.
func (s *Server) handoff(generation uint64, to, kind string, n int) *workshopHandoff {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.workshopSeq || s.activity == nil {
		return nil
	}
	a := s.activity
	from := a.Role
	if from == "" {
		from = roleCoordinator
	}
	if from == to {
		return nil
	}
	h := s.addHandoffLocked(a.Step, from, to, kind, n, "", nil, time.Now().UTC())
	a.Role = to
	return h
}

// endLane tanca un carril amb el resultat i la durada.
func (s *Server) endLane(generation uint64, seq int, result string) (workshopLane, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation != s.workshopSeq || s.activity == nil || seq == 0 {
		return workshopLane{}, false
	}
	a := s.activity
	for i := len(a.Lanes) - 1; i >= 0; i-- {
		if a.Lanes[i].Seq != seq {
			continue
		}
		l := &a.Lanes[i]
		l.Result, l.Done = result, true
		l.DurationMs = time.Since(l.StartedAt).Milliseconds()
		a.UpdatedAt = time.Now().UTC()
		return *l, true
	}
	return workshopLane{}, false
}

// noteRoleUsageLocked reparteix el delta de tokens des de l'última lectura
// al rol que tenia el torn. Cal tenir s.mu.
func (a *workshopActivity) noteRoleUsageLocked(up, down, calls int) {
	du, dd, dc := up-a.lastUp, down-a.lastDown, calls-a.lastCalls
	a.lastUp, a.lastDown, a.lastCalls = up, down, calls
	if du <= 0 && dd <= 0 && dc <= 0 {
		return
	}
	role := a.Role
	if role == "" {
		role = roleCoordinator
	}
	if a.RoleUsage == nil {
		a.RoleUsage = map[string]workshopUsage{}
	}
	u := a.RoleUsage[role]
	u.Up += max(du, 0)
	u.Down += max(dd, 0)
	u.Calls += max(dc, 0)
	a.RoleUsage[role] = u
}

// lanePayload és l'event durable «lane» (la repetició el llegeix).
func lanePayload(l workshopLane) map[string]any {
	return map[string]any{
		"seq": l.Seq, "step": l.Step, "role": l.Role, "kind": l.Kind, "target": l.Target, "n": l.N,
		"tools": l.Tools, "files": l.Files, "command": l.Command, "result": l.Result, "done": l.Done,
		"agent": l.Agent, "started_at": l.StartedAt, "duration_ms": l.DurationMs,
		"text": strings.TrimSpace(l.Role + ": " + l.Kind + " " + l.Target + " " + l.Result),
	}
}

// handoffPayload és l'event durable «handoff».
func handoffPayload(h workshopHandoff) map[string]any {
	return map[string]any{
		"seq": h.Seq, "step": h.Step, "from": h.From, "to": h.To, "kind": h.Kind, "n": h.N,
		"target": h.Target, "files": h.Files, "at": h.At,
		"text": h.From + " → " + h.To + ": " + h.Kind,
	}
}
