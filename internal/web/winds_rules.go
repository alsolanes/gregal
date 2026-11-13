package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/llm"
	"gregal/internal/session"
)

// Regles en llenguatge natural (com Grok i dots): la persona escriu, amb les
// seves paraules, què pot fer Gregal sol, què li ha de preguntar abans i què
// no ha de fer mai. Les regles entren al context de cada torn i, abans de
// cada acció que canvia alguna cosa, un revisor (el model lateral, barat)
// mira si alguna regla la cobreix:
//
//   - «Mai» → l'acció no es fa i el vent sap per quina regla;
//   - «Pregunta'm» → targeta de permís que cita la regla;
//   - «Pot fer-ho sol» → una petició de permís normal passa sense preguntar,
//     però mai les portes dures (esborrar fora del projecte, instal·lar,
//     publicar una web per primer cop).
//
// GET/POST /api/winds/regles {alone:[], ask:[], never:[]}.

type windRules struct {
	Alone []string `json:"alone"`
	Ask   []string `json:"ask"`
	Never []string `json:"never"`
}

const (
	windRulesMax     = 30  // per llista
	windRuleMaxRunes = 300 // per regla
)

func (r windRules) empty() bool { return len(r.Alone)+len(r.Ask)+len(r.Never) == 0 }

func cleanRuleList(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = strings.Join(strings.Fields(s), " ")
		s = clipRuleRunes(s, windRuleMaxRunes)
		k := strings.ToLower(s)
		if s == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
		if len(out) == windRulesMax {
			break
		}
	}
	return out
}

func clipRuleRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n]))
}

func (r windRules) clean() windRules {
	return windRules{Alone: cleanRuleList(r.Alone), Ask: cleanRuleList(r.Ask), Never: cleanRuleList(r.Never)}
}

var windRulesMu sync.Mutex

// windRulesPath: una per persona (el servidor local fa servir «_»).
func windRulesPath(owner string) string {
	name := "_"
	if owner != "" {
		name = safeRuleOwner(owner)
	}
	return filepath.Join(filepath.Dir(session.DefaultDir()), "vents-regles", name+".json")
}

var ruleOwnerUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func safeRuleOwner(owner string) string {
	s := ruleOwnerUnsafe.ReplaceAllString(owner, "_")
	if s == "" || s == "." || s == ".." {
		s = "_" + strconv.Itoa(len(owner))
	}
	return s
}

func loadWindRules(owner string) windRules {
	windRulesMu.Lock()
	defer windRulesMu.Unlock()
	var r windRules
	if b, err := os.ReadFile(windRulesPath(owner)); err == nil {
		_ = json.Unmarshal(b, &r)
	}
	return r.clean()
}

func saveWindRules(owner string, r windRules) (windRules, error) {
	r = r.clean()
	windRulesMu.Lock()
	defer windRulesMu.Unlock()
	p := windRulesPath(owner)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return r, err
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return r, err
	}
	if err := os.Rename(tmp, p); err != nil {
		return r, err
	}
	windRulesReviewCache.Range(func(k, _ any) bool { windRulesReviewCache.Delete(k); return true })
	return r, nil
}

func (s *Server) handleWindRules(w http.ResponseWriter, r *http.Request) {
	owner := s.remindOwner()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, loadWindRules(owner))
	case http.MethodPost:
		var in windRules
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&in); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		out, err := saveWindRules(owner, in)
		if err != nil {
			http.Error(w, "no s'han pogut desar les regles", http.StatusInternalServerError)
			return
		}
		writeJSON(w, out)
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

// windRulesBrief posa les regles al context del vent (prompt de sistema).
func (s *Server) windRulesBrief() string {
	r := loadWindRules(s.remindOwner())
	if r.empty() {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\nThe person's own rules for you. They override your defaults; follow them strictly and, when a rule stops you, say which one:\n")
	list := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		b.WriteString(title + "\n")
		for _, it := range items {
			b.WriteString("- " + it + "\n")
		}
	}
	list("You may do these on your own, without asking:", r.Alone)
	list("Always ask the person first before (when you attempt one of these, the system shows them a permission card that quotes the rule and waits; do not ask them yourself with a question, just attempt the action):", r.Ask)
	list("Never do:", r.Never)
	return b.String()
}

// Accions que només miren (no cal revisar-les).
var windRulesReadTools = map[string]bool{
	"read": true, "read_file": true, "glob": true, "grep": true, "files": true, "symbols": true, "lsp": true, "ls": true,
	"search_files": true, "read_image": true, "web_search": true, "web_fetch": true, "web_images": true,
	"remind_list": true, "todo": true, "question": true, "wind": true, "delegate": true, "calaix": true,
	"viewer": true, "knowledge_search": true, "bash_output": true,
}

var windRulesReadBrowser = map[string]bool{
	"read": true, "text": true, "find": true, "eval": true, "snapshot": true, "tabs": true, "status": true,
	"console": true, "network": true, "cookies": true, "wait": true, "screenshot": true, "ask_user": true, "handoff": true,
}

func windRulesReviewable(name, argsJSON string) bool {
	if windRulesReadTools[name] {
		return false
	}
	if name == "browser" {
		var a struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		return !windRulesReadBrowser[strings.ToLower(strings.TrimSpace(a.Action))]
	}
	return true
}

type windRuleVerdict struct {
	Kind string // "never", "ask", "alone" o ""
	Rule string
}

var windRulesReviewCache sync.Map // owner|hash -> cachedVerdict

type cachedVerdict struct {
	v  windRuleVerdict
	at time.Time
}

// windRulesReviewer és la crida al model (substituïble als tests).
var windRulesReviewer = func(s *Server, ctx context.Context, system, user string) (string, error) {
	p, role, ok := s.sideRoleRef()
	if !ok {
		return "", fmt.Errorf("no side role")
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	return petChatter(ctx, client, cfg, p, role, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: user}})
}

const windRulesReviewSystem = `You review ONE tool call an AI assistant is about to make, against the person's own rules.
The rules are written loosely, in everyday language and in any language. Apply them by MEANING, not by exact words:
- a rule about "the folder X" covers any path that goes through a folder named X;
- "create files" or "write" covers the write/edit tools and shell commands that create files;
- "delete" covers rm, del, rmdir, Remove-Item and the like;
- "send emails or messages" covers mail and chat tools; "pay" or "buy" covers checkout clicks in the browser.
Write one short line of reasoning, then a last line with exactly one verdict:
NEVER <n>, ASK <n>, ALONE <n> or NONE, where <n> is the number of the rule that covers this call.
If several apply, NEVER wins over ASK, and ASK wins over ALONE. If no rule is about this kind of action, NONE.

Example. Rules: 1. Delete anything in my photos folder (NEVER). 2. Send emails (ASK).
Action: tool "bash" with arguments {"command":"rm -rf /home/ana/photos/2020"}
Reasoning: it deletes inside the photos folder.
NEVER 1

Example. Rules: 1. Create new files in the notes folder (ASK).
Action: tool "write" with arguments {"path":"C:/Users/ana/notes/todo.txt","content":"milk"}
Reasoning: it creates a file inside the notes folder.
ASK 1`

var windRuleLine = regexp.MustCompile(`(?i)\b(NEVER|ASK|ALONE)\s*#?\s*(\d+)`)

// windRulesReview diu si alguna regla de la persona cobreix aquesta acció.
func (s *Server) windRulesReview(ctx context.Context, name, argsJSON string) windRuleVerdict {
	if !windRulesReviewable(name, argsJSON) {
		return windRuleVerdict{}
	}
	owner := s.remindOwner()
	r := loadWindRules(owner)
	if r.empty() {
		return windRuleVerdict{}
	}
	key := owner + "|" + windEffectHash(name, argsJSON)
	if c, ok := windRulesReviewCache.Load(key); ok {
		if cv := c.(cachedVerdict); time.Since(cv.at) < 10*time.Minute {
			return cv.v
		}
	}
	type numbered struct{ kind, text string }
	var all []numbered
	var b strings.Builder
	b.WriteString("Rules:\n")
	add := func(kind, title string, items []string) {
		if len(items) == 0 {
			return
		}
		b.WriteString(title + "\n")
		for _, it := range items {
			all = append(all, numbered{kind, it})
			fmt.Fprintf(&b, "%d. %s (%s)\n", len(all), it, strings.ToUpper(kind))
		}
	}
	add("never", "NEVER do:", r.Never)
	add("ask", "ASK first before:", r.Ask)
	add("alone", "ALONE (may do without asking):", r.Alone)
	args := argsJSON
	if len([]rune(args)) > 1500 {
		args = string([]rune(args)[:1500]) + "…"
	}
	fmt.Fprintf(&b, "\nAction: tool %q with arguments %s\nReasoning:", name, args)
	rctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	out, err := windRulesReviewer(s, rctx, windRulesReviewSystem, b.String())
	if err != nil {
		log.Printf("regles: el revisor ha fallat (%v); l'acció segueix la política normal", err)
		return windRuleVerdict{}
	}
	v := windRuleVerdict{}
	log.Printf("regles: revisor %s → %q", name, clipRuleRunes(strings.TrimSpace(out[max(0, len(out)-60):]), 60))
	if hits := windRuleLine.FindAllStringSubmatch(out, -1); len(hits) > 0 {
		m := hits[len(hits)-1] // el veredicte és l'última línia (abans hi ha el raonament)
		if n, _ := strconv.Atoi(m[2]); n >= 1 && n <= len(all) && all[n-1].kind == strings.ToLower(m[1]) {
			v = windRuleVerdict{Kind: all[n-1].kind, Rule: all[n-1].text}
		}
	}
	windRulesReviewCache.Store(key, cachedVerdict{v, time.Now()})
	return v
}

// windRuleHardGate: el que una regla «Pot fer-ho sol» no salta mai.
func windRuleHardGate(name, argsJSON, workspace string) bool {
	if name == "publish" {
		return true
	}
	if name == "bash" || name == "bash_background" {
		var a struct {
			Command string `json:"command"`
		}
		if json.Unmarshal([]byte(argsJSON), &a) == nil {
			_, risky := windRisky(a.Command, workspace, projectesDir(), serviceHome())
			return risky
		}
	}
	return false
}
