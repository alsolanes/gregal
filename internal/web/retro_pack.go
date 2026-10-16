package web

// Paquet d'evidència del #retro: un resum compacte i per agent del que va
// passar DE DEBÒ en un torn (eines, fitxers, ordres, proves, delegats,
// verificacions). És l'única font de veritat de les converses del canal:
// el model només pot afirmar el que hi ha aquí, i cada afirmació hi apunta
// amb un id (e1, e2…) que la UI pinta com a «rebut».

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/events"
	"gregal/internal/tools"
)

const (
	retroMaxEvidence = 60
	retroDetailMax   = 1500
)

// retroEvidence és un ítem verificable del torn.
type retroEvidence struct {
	ID     string `json:"id"`
	Agent  string `json:"agent"`
	Kind   string `json:"kind"` // file | command | test | tools | tool | delegate | summary | verify | checkpoint | error | blocked | outcome | deliverable | bet
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	OK     *bool  `json:"ok,omitempty"`
}

// retroAgent és un participant del canal.
type retroAgent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"` // coordinator | researcher | builder | reviewer
	Role   string `json:"role,omitempty"`
	Model  string `json:"model,omitempty"`
}

type retroPack struct {
	Agents   []retroAgent    `json:"agents"`
	Evidence []retroEvidence `json:"evidence"`
}

func retroClip(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// retroTail conserva el final: en una sortida de proves el veredicte hi és.
func retroTail(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n:])
}

func boolPtr(b bool) *bool { return &b }

// retroArgs llegeix els arguments d'una crida d'eina: al registre arriben com
// a string JSON (el que va enviar el model) o com a objecte.
func retroArgs(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		_ = json.Unmarshal([]byte(s), &out)
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func retroArg(args map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := args[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

var retroTestCmd = regexp.MustCompile(`(?i)\b(go\s+test|go\s+vet|npm\s+(run\s+)?test|pnpm\s+test|yarn\s+test|node\s+(--[\w-]+\s+)*--test|pytest|python\s+-m\s+(pytest|unittest)|cargo\s+test|make\s+(test|check)|jest|vitest|mvn\s+test|gradle\s+test|dotnet\s+test|tsc\b|eslint|go\s+build)`)

func retroIsTest(cmd string) bool { return retroTestCmd.MatchString(cmd) }

// retroToolOK decideix si el resultat d'una eina va bé: el motor marca les
// fallades amb el prefix ERROR: (agent/loop.go).
func retroToolOK(out string) bool {
	o := strings.TrimSpace(out)
	return !strings.HasPrefix(o, "ERROR") && !strings.HasPrefix(o, "⛔")
}

// retroDelegateName dona nom humà a un fill segons la seva tasca: és el que
// fa servir la sala (researcher/builder/reviewer) i el que la gent menciona.
func retroDelegateName(task string) string {
	t := strings.ToLower(task)
	for _, w := range []string{"review", "revis", "verify", "verifica", "audit", "check", "comprova"} {
		if strings.Contains(t, w) {
			return "Reviewer"
		}
	}
	for _, w := range []string{"research", "investig", "find", "search", "explore", "explora", "busca", "cerca", "read", "llegeix", "analy", "anali", "inspect", "locate", "list"} {
		if strings.Contains(t, w) {
			return "Researcher"
		}
	}
	return "Builder"
}

func retroSlug(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
}

type retroPackBuilder struct {
	agents   []retroAgent
	evidence []retroEvidence
	byAgent  map[string]bool
}

func (b *retroPackBuilder) agent(id, name, avatar, role, model string) {
	if b.byAgent[id] {
		return
	}
	b.byAgent[id] = true
	b.agents = append(b.agents, retroAgent{ID: id, Name: name, Avatar: avatar, Role: role, Model: model})
}

func (b *retroPackBuilder) add(e retroEvidence) {
	if len(b.evidence) >= retroMaxEvidence {
		return
	}
	e.ID = fmt.Sprintf("e%d", len(b.evidence)+1)
	e.Title = retroClip(e.Title, 140)
	e.Detail = retroClip(e.Detail, retroDetailMax)
	b.evidence = append(b.evidence, e)
}

// retroRelPath normalitza un camí d'eina perquè es pugui creuar amb el diff
// de git (relatiu al workspace, amb barres endavant).
func retroRelPath(cwd, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) && cwd != "" {
		if rel, err := filepath.Rel(cwd, p); err == nil && !strings.HasPrefix(rel, "..") {
			p = rel
		}
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// buildRetroPack construeix el paquet a partir dels events durables del torn
// i del diff actual del workspace (només els fitxers que el torn va tocar).
func buildRetroPack(evs []events.Event, runID int, diff []tools.DiffFile, cwd string) retroPack {
	b := &retroPackBuilder{byAgent: map[string]bool{}}
	b.agent("coordinator", "Coordinator", "coordinator", "main agent", "")

	type call struct {
		name string
		args map[string]any
	}
	calls := map[string]call{}
	type fileEdit struct {
		path      string
		edits     int
		ok, fails int
		last      string
	}
	files := map[string]*fileEdit{}
	var fileOrder []string
	type readAgg struct {
		count int
		items []string
	}
	reads := map[string]*readAgg{}
	var readOrder []string
	type child struct {
		id, task, model, status, output, name string
		tools                                 []string
		toolOut                               []string
	}
	children := map[string]*child{}
	var childOrder []string
	var reviewer []retroEvidence
	var tail []retroEvidence

	for _, e := range evs {
		if e.RunID != runID {
			continue
		}
		var p map[string]json.RawMessage
		if len(e.Payload) > 0 {
			_ = json.Unmarshal(e.Payload, &p)
		}
		str := func(k string) string {
			var s string
			if raw, ok := p[k]; ok {
				_ = json.Unmarshal(raw, &s)
			}
			return s
		}
		switch e.Kind {
		case "tool_call":
			calls[str("id")] = call{name: str("name"), args: retroArgs(p["args"])}
		case "tool_result":
			c, ok := calls[str("id")]
			if !ok {
				c = call{name: str("name"), args: map[string]any{}}
			}
			name := c.name
			if name == "" {
				name = str("name")
			}
			out := str("output")
			if out == "" {
				out = strings.TrimPrefix(e.Text, "↳ "+name+": ")
			}
			good := retroToolOK(out)
			switch name {
			case "write", "edit", "patch":
				path := retroRelPath(cwd, retroArg(c.args, "path", "file", "file_path"))
				if path == "" {
					path = "(unknown file)"
				}
				f := files[path]
				if f == nil {
					f = &fileEdit{path: path}
					files[path] = f
					fileOrder = append(fileOrder, path)
				}
				f.edits++
				if good {
					f.ok++
				} else {
					f.fails++
				}
				f.last = name + ": " + retroClip(out, 300)
			case "bash", "bash_background":
				cmd := retroArg(c.args, "command", "cmd")
				kind := "command"
				if retroIsTest(cmd) {
					kind = "test"
				}
				mark := "✓"
				if !good {
					mark = "✗"
				}
				b.add(retroEvidence{Agent: "coordinator", Kind: kind, Title: retroClip(cmd, 90) + " " + mark,
					Detail: "$ " + cmd + "\n" + retroTail(out, 1200), OK: boolPtr(good)})
			case "delegate":
				// Els delegats es narren pels seus propis events.
			case "read", "grep", "glob", "web_search", "web_fetch", "read_image", "todoread", "todowrite", "skill", "office_read":
				r := reads[name]
				if r == nil {
					r = &readAgg{}
					reads[name] = r
					readOrder = append(readOrder, name)
				}
				r.count++
				if arg := retroArg(c.args, "path", "pattern", "query", "url", "file"); arg != "" && len(r.items) < 12 {
					r.items = append(r.items, retroClip(arg, 120))
				}
			default:
				b.add(retroEvidence{Agent: "coordinator", Kind: "tool", Title: name, Detail: retroClip(out, 800), OK: boolPtr(good)})
			}
		case "delegate":
			id := str("id")
			if id == "" {
				continue
			}
			ch := children[id]
			if ch == nil {
				ch = &child{id: id}
				children[id] = ch
				childOrder = append(childOrder, id)
			}
			if t := str("task"); t != "" {
				ch.task = t
			}
			if m := str("model"); m != "" {
				ch.model = m
			}
			if st := str("status"); st != "" {
				ch.status = st
			}
			tool, out := str("tool"), str("output")
			if st := str("status"); st == agent.DelegateCompleted || st == "failed" || st == "cancelled" {
				if out != "" {
					ch.output = out
				}
			} else if tool != "" {
				if len(ch.tools) == 0 || ch.tools[len(ch.tools)-1] != tool || out != "" {
					if out != "" && len(ch.toolOut) < 6 {
						ch.toolOut = append(ch.toolOut, tool+" → "+retroClip(out, 200))
					}
					if len(ch.tools) == 0 || ch.tools[len(ch.tools)-1] != tool {
						ch.tools = append(ch.tools, tool)
					}
				}
			}
		case "autonomous_checkpoint":
			var checks []agent.AutonomousCheck
			_ = json.Unmarshal(p["checks"], &checks)
			for _, c := range checks {
				mark := "✓"
				if c.Code != 0 {
					mark = fmt.Sprintf("✗ exit %d", c.Code)
				}
				reviewer = append(reviewer, retroEvidence{Agent: "reviewer", Kind: "test", Title: retroClip(c.Command, 90) + " " + mark,
					Detail: "$ " + c.Command + fmt.Sprintf(" (exit %d)\n", c.Code) + retroTail(c.Output, 1000), OK: boolPtr(c.Code == 0)})
			}
			if rv := str("review"); rv != "" {
				reviewer = append(reviewer, retroEvidence{Agent: "reviewer", Kind: "checkpoint", Title: "checkpoint review: " + retroClip(rv, 80),
					Detail: rv, OK: boolPtr(strings.HasPrefix(rv, "APROVAT"))})
			}
		case "activity":
			if strings.HasPrefix(e.Text, "✅") {
				v := strings.TrimSpace(strings.TrimPrefix(e.Text, "✅"))
				low := strings.ToLower(v)
				ok := !strings.Contains(low, "cal revisar") && !strings.HasPrefix(low, "error") && !strings.Contains(low, "needs")
				reviewer = append(reviewer, retroEvidence{Agent: "reviewer", Kind: "verify", Title: "verify: " + retroClip(v, 80), Detail: v, OK: boolPtr(ok)})
			}
		case "bet":
			// Aposta de confiança resolta (caps.go): predicció vs resultat.
			ok := str("passed") == "true"
			mark := "✗"
			if ok {
				mark = "✓"
			}
			reviewer = append(reviewer, retroEvidence{Agent: "coordinator", Kind: "bet",
				Title:  "bet " + str("p") + "% " + mark + " " + retroClip(str("command"), 70),
				Detail: "predicted " + str("p") + "% that `" + str("command") + "` passes; it " + map[bool]string{true: "passed", false: "failed"}[ok], OK: boolPtr(ok)})
		case "blocked":
			tail = append(tail, retroEvidence{Agent: "coordinator", Kind: "blocked", Title: "blocked: " + retroClip(str("reason"), 80), Detail: e.Text, OK: boolPtr(false)})
		case "error":
			tail = append(tail, retroEvidence{Agent: "coordinator", Kind: "error", Title: retroClip(e.Text, 90), Detail: e.Text, OK: boolPtr(false)})
		case "done":
			if strings.TrimSpace(e.Text) != "" && e.Text != "torn completat" {
				tail = append(tail, retroEvidence{Agent: "coordinator", Kind: "summary", Title: "final answer", Detail: e.Text})
			}
		case "run_failed", "run_cancelled", "run_completed":
			tail = append(tail, retroEvidence{Agent: "coordinator", Kind: "outcome", Title: e.Kind, Detail: e.Text, OK: boolPtr(e.Kind == "run_completed")})
		}
	}

	// Fitxers: un ítem per fitxer amb els hunks reals de git si encara hi són.
	byPath := map[string]tools.DiffFile{}
	for _, d := range diff {
		byPath[filepath.ToSlash(d.Path)] = d
	}
	for _, path := range fileOrder {
		f := files[path]
		ev := retroEvidence{Agent: "coordinator", Kind: "file", OK: boolPtr(f.ok > 0)}
		if d, ok := byPath[path]; ok {
			ev.Title = fmt.Sprintf("%s (+%d −%d)", path, d.Added, d.Removed)
			var sb strings.Builder
			for i, h := range d.Hunks {
				if i == 3 {
					fmt.Fprintf(&sb, "… %d more hunks\n", len(d.Hunks)-3)
					break
				}
				sb.WriteString(h.Patch)
				if !strings.HasSuffix(h.Patch, "\n") {
					sb.WriteString("\n")
				}
			}
			ev.Detail = sb.String()
			if d.Untracked && ev.Detail == "" {
				ev.Detail = "new untracked file"
			}
		} else {
			ev.Title = fmt.Sprintf("%s (%d edit(s), %d failed; no diff vs HEAD)", path, f.edits, f.fails)
			ev.Detail = f.last
		}
		b.add(ev)
	}
	sort.Strings(readOrder)
	for _, name := range readOrder {
		r := reads[name]
		b.add(retroEvidence{Agent: "coordinator", Kind: "tools", Title: fmt.Sprintf("%s ×%d", name, r.count), Detail: strings.Join(r.items, "\n")})
	}
	names := map[string]int{}
	for _, id := range childOrder {
		ch := children[id]
		base := retroDelegateName(ch.task)
		names[base]++
		name := base
		if names[base] > 1 {
			name = fmt.Sprintf("%s %d", base, names[base])
		}
		aid := retroSlug(name)
		if aid == "reviewer" {
			// El revisor de verificació també és «reviewer»: el delegat no
			// li pot robar l'identitat.
			aid, name = "reviewer-delegate", "Reviewer (delegate)"
		}
		ch.name = aid
		b.agent(aid, name, strings.ToLower(base), "delegated: "+retroClip(ch.task, 160), ch.model)
		b.add(retroEvidence{Agent: aid, Kind: "delegate", Title: "task: " + retroClip(ch.task, 100) + " [" + ch.status + "]",
			Detail: ch.task, OK: boolPtr(ch.status == agent.DelegateCompleted)})
		if len(ch.tools) > 0 {
			b.add(retroEvidence{Agent: aid, Kind: "tools", Title: "tools: " + strings.Join(ch.tools, ", "), Detail: strings.Join(ch.toolOut, "\n")})
		}
		if ch.output != "" {
			b.add(retroEvidence{Agent: aid, Kind: "summary", Title: "result handed back", Detail: ch.output})
		}
	}
	if len(reviewer) > 0 {
		b.agent("reviewer", "Reviewer", "reviewer", "verification gate", "")
		for _, e := range reviewer {
			b.add(e)
		}
	}
	for _, e := range tail {
		b.add(e)
	}
	return retroPack{Agents: b.agents, Evidence: b.evidence}
}

// buildRetroTeamPack fa el mateix per a un torn d'equip: els rols són els
// agents i cada lliurable és la seva evidència.
func buildRetroTeamPack(a *workshopActivity) retroPack {
	b := &retroPackBuilder{byAgent: map[string]bool{}}
	if a == nil {
		return retroPack{}
	}
	for _, ag := range a.Agents {
		id := retroSlug(ag.ID)
		avatar := id
		switch avatar {
		case "coordinator", "researcher", "builder", "reviewer":
		default:
			avatar = "builder"
		}
		name := strings.ToUpper(id[:1]) + id[1:]
		b.agent(id, name, avatar, "team role", ag.Model)
		ev := retroEvidence{Agent: id, Kind: "deliverable", Title: name + " deliverable [" + ag.Status + "]", Detail: ag.Output, OK: boolPtr(ag.Status == agent.DelegateCompleted)}
		if strings.TrimSpace(ag.Output) == "" {
			ev.Title = name + " [" + ag.Status + "] (no deliverable recorded)"
		}
		b.add(ev)
	}
	if !b.byAgent["coordinator"] {
		b.agent("coordinator", "Coordinator", "coordinator", "team coordinator", "")
		b.agents = append([]retroAgent{b.agents[len(b.agents)-1]}, b.agents[:len(b.agents)-1]...)
	}
	b.add(retroEvidence{Agent: "coordinator", Kind: "outcome", Title: "team run " + a.State, Detail: a.Message, OK: boolPtr(a.State == "completed")})
	return retroPack{Agents: b.agents, Evidence: b.evidence}
}

// renderRetroEvidence és el paquet tal com el veu el model.
func renderRetroEvidence(p retroPack) string { return renderRetroEvidenceN(p, 500) }

// renderRetroEvidenceN deixa passar fins a max caràcters de cada detall (els
// revisors necessiten veure més diff que el resum).
func renderRetroEvidenceN(p retroPack, max int) string {
	var sb strings.Builder
	sb.WriteString("AGENTS:\n")
	for _, a := range p.Agents {
		fmt.Fprintf(&sb, "- %s (%s): %s\n", a.ID, a.Name, a.Role)
	}
	sb.WriteString("\nEVIDENCE (the ONLY facts you may state):\n")
	if len(p.Evidence) == 0 {
		sb.WriteString("(no evidence recorded for this run)\n")
	}
	for _, e := range p.Evidence {
		status := ""
		if e.OK != nil {
			status = " [ok]"
			if !*e.OK {
				status = " [FAILED]"
			}
		}
		fmt.Fprintf(&sb, "%s by %s · %s%s: %s\n", e.ID, e.Agent, e.Kind, status, e.Title)
		if d := strings.TrimSpace(e.Detail); d != "" {
			sb.WriteString("    " + strings.ReplaceAll(retroClip(d, max), "\n", "\n    ") + "\n")
		}
	}
	return sb.String()
}

// parseRetroMentions torna els agents mencionats amb @ (per id o per nom).
func parseRetroMentions(text string, agents []retroAgent) []string {
	var out []string
	seen := map[string]bool{}
	for _, m := range retroMentionRe.FindAllStringSubmatch(text, -1) {
		tok := strings.ToLower(strings.TrimRight(m[1], ".,;:!?-"))
		for _, a := range agents {
			if seen[a.ID] {
				continue
			}
			if tok == a.ID || tok == retroSlug(a.Name) || ((tok == "main" || tok == "coord") && a.ID == "coordinator") {
				seen[a.ID] = true
				out = append(out, a.ID)
				break
			}
		}
	}
	return out
}

var retroMentionRe = regexp.MustCompile(`(?:^|[^\w@])@([A-Za-z][\w-]*)`)
