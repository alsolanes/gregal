package web

// El Racó dels vents: una pàgina de l'app que és només dels vents. Hi
// decideixen ells què hi posen, quan hi treballen i qui revisa la feina.
//
// Tancat per disseny, perquè res del que facin pugui trencar la resta:
//
//   - Els vents no toquen fitxers ni executen res. Cada pas és una crida al
//     model que torna text: la reunió decideix, el constructor escriu la
//     pàgina sencera, el revisor l'aprova o la rebutja. El servidor la
//     comprova (mida, cap adreça de fora) i la desa com una versió nova.
//   - La pàgina corre en un marc aïllat (iframe sandbox sense mateix origen,
//     o el WebView del mòbil) amb una CSP que no deixa connectar enlloc. Només
//     té un pont: les dades dels vents (gregal.ready) i parlar amb un vent
//     (gregal.ask), amb límit diari.
//   - Totes les versions es guarden: l'usuari pot tornar a qualsevol amb un
//     clic, aturar-los i deixar-los comentaris que llegeixen a la reunió.
//
// Quan treballen ho decideixen ells: cada deu minuts el servidor mira si
// tenen ganes (un comentari nou, converses de visitants, hores sense tocar
// res, una mica d'atzar) i, si sí, fan una reunió que pot acabar en «avui
// descansem». Un pressupost diari de sessions, crides i converses ho limita.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

const (
	cornerMaxSessions = 4   // sessions de feina al dia
	cornerMaxCalls    = 16  // crides al model de les sessions, al dia
	cornerMaxTalks    = 60  // converses de visitants amb un vent, al dia
	cornerMaxBytes    = 120 << 10
	cornerTickEvery   = 10 * time.Minute
	cornerMinGap      = 3 * time.Hour // sense res de nou, com a mínim entre sessions
	cornerCommentGap  = 10 * time.Minute
)

// cornerVersion és una versió publicada de la pàgina.
type cornerVersion struct {
	N            int    `json:"n"`
	At           int64  `json:"at"`
	Builder      string `json:"builder,omitempty"`
	BuilderName  string `json:"builder_name,omitempty"`
	Reviewer     string `json:"reviewer,omitempty"`
	ReviewerName string `json:"reviewer_name,omitempty"`
	Summary      string `json:"summary"`
	Review       string `json:"review,omitempty"`
	RevertedFrom int    `json:"reverted_from,omitempty"`
	Bytes        int    `json:"bytes"`
}

// cornerEntry és una línia del diari (reunions, feina, revisions, comentaris).
type cornerEntry struct {
	At   int64  `json:"at"`
	Kind string `json:"kind"` // meeting, rest, built, review, published, rejected, failed, revert, comment, talk
	Wind string `json:"wind,omitempty"`
	Name string `json:"name,omitempty"`
	Text string `json:"text"`
}

// cornerState és l'estat persistent (corner/state.json).
type cornerState struct {
	Paused      bool   `json:"paused"`
	Current     int    `json:"current"`
	LastSession int64  `json:"last_session"`
	SeenComment int64  `json:"seen_comment"` // fins on la reunió ja ha llegit comentaris
	SeenTalk    int64  `json:"seen_talk"`
	Day         string `json:"day"`
	Sessions    int    `json:"sessions"`
	Calls       int    `json:"calls"`
	Talks       int    `json:"talks"`
}

type cornerBox struct {
	mu      sync.Mutex
	running bool
	stop    context.CancelFunc // talla la sessió en marxa (botó «Atura-ho ara»)
}

var (
	cornerBoxesMu sync.Mutex
	cornerBoxes   = map[string]*cornerBox{}
)

func (s *Server) cornerDir() string { return filepath.Join(s.windStateDir(), "corner") }

func (s *Server) cornerBox() *cornerBox {
	dir := s.cornerDir()
	cornerBoxesMu.Lock()
	defer cornerBoxesMu.Unlock()
	b := cornerBoxes[dir]
	if b == nil {
		b = &cornerBox{}
		cornerBoxes[dir] = b
	}
	return b
}

func (s *Server) cornerLoad() cornerState {
	var st cornerState
	raw, err := os.ReadFile(filepath.Join(s.cornerDir(), "state.json"))
	if err == nil {
		_ = json.Unmarshal(raw, &st)
	} else {
		// Mai engegat: comença aturat i no gasta res fins que l'usuari
		// els deixa treballar.
		st.Paused = true
	}
	today := time.Now().Format("2006-01-02")
	if st.Day != today {
		st.Day, st.Sessions, st.Calls, st.Talks = today, 0, 0, 0
	}
	return st
}

func (s *Server) cornerSave(st cornerState) {
	_ = os.MkdirAll(s.cornerDir(), 0o700)
	raw, _ := json.MarshalIndent(st, "", "  ")
	_ = os.WriteFile(filepath.Join(s.cornerDir(), "state.json"), raw, 0o600)
}

func (s *Server) cornerAppend(name string, v any) {
	_ = os.MkdirAll(s.cornerDir(), 0o700)
	f, err := os.OpenFile(filepath.Join(s.cornerDir(), name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	raw, _ := json.Marshal(v)
	f.Write(append(raw, '\n'))
}

func cornerReadLines[T any](path string, max int) []T {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if max > 0 && len(lines) > max {
		lines = lines[len(lines)-max:]
	}
	out := make([]T, 0, len(lines))
	for _, l := range lines {
		var v T
		if json.Unmarshal([]byte(l), &v) == nil {
			out = append(out, v)
		}
	}
	return out
}

func (s *Server) cornerJournal(max int) []cornerEntry {
	return cornerReadLines[cornerEntry](filepath.Join(s.cornerDir(), "journal.jsonl"), max)
}

func (s *Server) cornerLog(kind string, w *companion.Wind, text string) {
	e := cornerEntry{At: time.Now().Unix(), Kind: kind, Text: strings.TrimSpace(text)}
	if w != nil {
		e.Wind, e.Name = w.ID, w.Name
	}
	s.cornerAppend("journal.jsonl", e)
}

func (s *Server) cornerVersions() []cornerVersion {
	matches, _ := filepath.Glob(filepath.Join(s.cornerDir(), "versions", "*.json"))
	var out []cornerVersion
	for _, m := range matches {
		raw, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		var v cornerVersion
		if json.Unmarshal(raw, &v) == nil && v.N > 0 {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N > out[j].N })
	return out
}

func (s *Server) cornerHTML(n int) string {
	if n <= 0 {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(s.cornerDir(), "versions", fmt.Sprintf("%04d.html", n)))
	if err != nil {
		return ""
	}
	return string(raw)
}

// cornerPublish desa una versió nova i la fa l'actual.
func (s *Server) cornerPublish(html string, v cornerVersion) cornerVersion {
	dir := filepath.Join(s.cornerDir(), "versions")
	_ = os.MkdirAll(dir, 0o700)
	st := s.cornerLoad()
	n := st.Current
	for _, x := range s.cornerVersions() {
		if x.N > n {
			n = x.N
		}
	}
	v.N, v.At, v.Bytes = n+1, time.Now().Unix(), len(html)
	_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d.html", v.N)), []byte(html), 0o600)
	raw, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%04d.json", v.N)), raw, 0o600)
	st = s.cornerLoad()
	st.Current = v.N
	s.cornerSave(st)
	return v
}

// ---- comprovacions ----

var (
	cornerExternalRe = regexp.MustCompile(`(?i)(\b(src|href|action|poster|data|srcset|formaction|background)\s*=\s*["']?\s*(https?:)?//)|(url\(\s*["']?\s*(https?:)?//)|(@import)|(<base\b)|(<meta[^>]+http-equiv\s*=\s*["']?refresh)`)
	cornerNetRe      = regexp.MustCompile(`\b(fetch\s*\(|XMLHttpRequest|WebSocket|EventSource|navigator\.sendBeacon|importScripts)`)
	cornerFenceRe    = regexp.MustCompile("(?s)```(?:html)?\\s*\\n(.*?)```")
)

// cornerProblems diu què no compleix una pàgina (buit si està bé).
func cornerProblems(html string) []string {
	var p []string
	if strings.TrimSpace(html) == "" {
		return []string{"the page is empty"}
	}
	if len(html) > cornerMaxBytes {
		p = append(p, fmt.Sprintf("the page is %d KB; the maximum is %d KB", len(html)>>10, cornerMaxBytes>>10))
	}
	if !strings.Contains(strings.ToLower(html), "<body") && !strings.Contains(strings.ToLower(html), "<html") {
		p = append(p, "it must be a complete HTML document (<html>, <body>)")
	}
	if low := strings.ToLower(html); !strings.Contains(low, "</html>") && !strings.Contains(low, "</body>") {
		p = append(p, "the page is cut off (no </body></html> at the end): the answer ran out of space; make it more compact, under ~40 KB")
	}
	if m := cornerExternalRe.FindString(html); m != "" {
		p = append(p, "it loads something from outside or navigates away ("+oneLine(m, 60)+"): use inline CSS/JS/SVG, data: URIs or emoji instead")
	}
	if m := cornerNetRe.FindString(html); m != "" {
		p = append(p, "it uses the network ("+oneLine(m, 40)+"): there is no network; use gregal.ask to talk to a wind")
	}
	return p
}

// cornerExtract treu la pàgina i el resum de la resposta del constructor.
func cornerExtract(reply string) (summary, html string) {
	for _, line := range strings.Split(reply, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(l), "SUMMARY:") {
			summary = strings.TrimSpace(l[len("SUMMARY:"):])
			break
		}
	}
	if m := cornerFenceRe.FindStringSubmatch(reply); m != nil {
		html = strings.TrimSpace(m[1])
	} else if i := strings.Index(strings.ToLower(reply), "<!doctype"); i >= 0 {
		html = strings.TrimSpace(reply[i:])
	} else if i := strings.Index(strings.ToLower(reply), "<html"); i >= 0 {
		html = strings.TrimSpace(reply[i:])
	}
	return oneLine(summary, 300), html
}

// cornerBridge és el que la pàgina té del món: dades dels vents i parlar amb
// ells. A la web, per postMessage amb l'app; al mòbil, per la interfície
// GregalAndroid del WebView. Res més.
const cornerBridge = `<script>(() => {
  let seq = 0, data = null; const wait = new Map(), ready = [];
  addEventListener('message', e => {
    const m = e.data || {};
    if (m.gregalCorner === 'init') { data = m.data || {}; ready.splice(0).forEach(fn => { try { fn(data); } catch (err) { console.error(err); } }); }
    else if (m.gregalCorner === 'reply' && wait.has(m.id)) { const w = wait.get(m.id); wait.delete(m.id); m.error ? w.reject(new Error(m.error)) : w.resolve(m.reply); }
  });
  const post = msg => { if (window.GregalAndroid) window.GregalAndroid.send(JSON.stringify(msg)); else parent.postMessage(msg, '*'); };
  window.gregal = Object.freeze({
    ready: fn => { if (data) fn(data); else ready.push(fn); },
    winds: () => (data && data.winds) || [],
    ask: (wind, text) => new Promise((resolve, reject) => { const id = ++seq; wait.set(id, {resolve, reject}); post({gregalCorner: 'ask', id, wind: String(wind || ''), text: String(text || '').slice(0, 1000)}); }),
  });
  post({gregalCorner: 'hello'});
})();</script>`

// cornerCSP: res de fora, cap connexió; només el que porta la mateixa pàgina.
const cornerCSP = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; media-src data: blob:; connect-src 'none'; form-action 'none'; base-uri 'none'">`

// cornerFrame posa la CSP i el pont davant de la pàgina dels vents.
func cornerFrame(html string) string {
	head := cornerCSP + `<meta name="viewport" content="width=device-width, initial-scale=1">` + cornerBridge
	low := strings.ToLower(html)
	if i := strings.Index(low, "<head>"); i >= 0 {
		return html[:i+6] + head + html[i+6:]
	}
	if strings.HasPrefix(strings.TrimSpace(low), "<!doctype") {
		if i := strings.Index(html, ">"); i >= 0 {
			return html[:i+1] + "<head>" + head + "</head>" + html[i+1:]
		}
	}
	return "<!doctype html><head>" + head + "</head>" + html
}

// ---- model ----

// cornerModel és la crida al model del Racó (els tests la substitueixen).
// Sortida llarga: el constructor escriu la pàgina sencera.
var cornerModel = func(s *Server, ctx context.Context, system, user string, maxTokens int) (string, error) {
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	p, role := s.windRoleRef()
	s.mu.Unlock()
	if client == nil || cfg == nil {
		return "", errConst("no model configured")
	}
	msgs := []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	text, _, _, err := client.ChatStreamWithToolsFO(ctx, cfg.PrimTarget(p, role), cfg.FallbackTarget(role), msgs, 0.7, maxTokens, nil, func(string) {}, nil, nil)
	return text, err
}

func (s *Server) cornerLang() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg != nil && s.cfg.Lang() == "ca" {
		return "Catalan"
	}
	return "English"
}

// cornerCall compta la crida al pressupost i la fa.
func (s *Server) cornerCall(ctx context.Context, system, user string, maxTokens int) (string, error) {
	st := s.cornerLoad()
	if st.Calls >= cornerMaxCalls {
		return "", errConst("daily budget of model calls used up")
	}
	st.Calls++
	s.cornerSave(st)
	return cornerModel(s, ctx, system, user, maxTokens)
}

type cornerMeeting struct {
	Action     string `json:"action"`
	Why        string `json:"why"`
	Builder    string `json:"builder"`
	Reviewer   string `json:"reviewer"`
	Plan       string `json:"plan"`
	StartOver  bool   `json:"start_over"`
}

type cornerReview struct {
	Approve bool     `json:"approve"`
	Notes   string   `json:"notes"`
	Fix     []string `json:"fix"`
}

func cornerRoster(winds []companion.Wind) string {
	var b strings.Builder
	for _, w := range winds {
		fmt.Fprintf(&b, "- %s | %s | %s", w.ID, w.Name, w.Role)
		if w.Specialty != "" {
			b.WriteString(" | specialty: " + oneLine(w.Specialty, 120))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// cornerContext és el que la reunió sap: versions, diari, comentaris i
// converses noves de visitants.
func (s *Server) cornerContext(st cornerState) (text string, newComments int, newestComment, newestTalk int64) {
	var b strings.Builder
	vs := s.cornerVersions()
	if len(vs) == 0 {
		b.WriteString("THE PAGE: still a blank sheet (no version yet).\n")
	} else {
		fmt.Fprintf(&b, "THE PAGE: version %d (%d KB). History, newest first:\n", st.Current, len(s.cornerHTML(st.Current))>>10)
		for i, v := range vs {
			if i == 8 {
				break
			}
			who := v.BuilderName
			if v.RevertedFrom > 0 {
				who = fmt.Sprintf("the user (back to v%d)", v.RevertedFrom)
			}
			fmt.Fprintf(&b, "- v%d by %s: %s\n", v.N, who, v.Summary)
		}
	}
	var comments []string
	for _, e := range s.cornerJournal(200) {
		switch {
		case e.Kind == "comment" && e.At > st.SeenComment:
			comments = append(comments, "- "+e.Text)
			newComments++
			if e.At > newestComment {
				newestComment = e.At
			}
		case e.Kind == "talk" && e.At > st.SeenTalk:
			if e.At > newestTalk {
				newestTalk = e.At
			}
		}
	}
	if len(comments) > 0 {
		b.WriteString("\nTHE USER'S NEW COMMENTS ABOUT THE PAGE (answer these first):\n" + strings.Join(comments, "\n") + "\n")
	}
	var talks []string
	for _, e := range s.cornerJournal(60) {
		if e.Kind == "talk" {
			talks = append(talks, "- "+e.Name+": "+oneLine(e.Text, 200))
		}
	}
	if len(talks) > 0 {
		if len(talks) > 8 {
			talks = talks[len(talks)-8:]
		}
		b.WriteString("\nRECENT CONVERSATIONS OF VISITORS WITH YOU ON THE PAGE:\n" + strings.Join(talks, "\n") + "\n")
	}
	var journal []string
	for _, e := range s.cornerJournal(14) {
		if e.Kind == "talk" || e.Kind == "comment" {
			continue
		}
		journal = append(journal, fmt.Sprintf("- %s %s%s", time.Unix(e.At, 0).Format("01-02 15:04"), e.Kind, map[bool]string{true: " (" + e.Name + ")", false: ""}[e.Name != ""])+": "+oneLine(e.Text, 160))
	}
	if len(journal) > 0 {
		b.WriteString("\nYOUR RECENT JOURNAL:\n" + strings.Join(journal, "\n") + "\n")
	}
	return b.String(), newComments, newestComment, newestTalk
}

const cornerRules = `THE CORNER is a page inside the Gregal app that belongs only to you, the winds. You decide what it is: a game, a gallery, a newspaper about yourselves, a toy, a story that grows… Visitors (the user and their friends) open it inside the app, on a desktop or a phone.

How it works:
- ONE complete, self-contained HTML document: inline <style> and <script> only.
- Nothing from outside: no http(s) URLs, CDNs, web fonts or remote images. Use inline SVG, CSS, canvas, data: URIs and emoji.
- No network and no storage: fetch, XHR, WebSocket and localStorage do not work. State lives in memory and resets on reload.
- The page can use one small API:
    gregal.ready(fn): fn(data) once data arrives. data.winds = [{id, name, role, color, specialty}], data.version (number), data.lang ("ca"/"en").
    await gregal.ask(windId, text): asks that wind a question and returns its answer (text). It costs money and is limited per day: only when a visitor asks for it (a button, a form), NEVER automatically on load or in loops.
- It must work on a 360 px wide phone and on a desktop, and look good in light and dark (prefers-color-scheme).
- Compact: aim for 15-40 KB, because the whole page must fit in one answer. Prefer clever and small over big. Keep it kind and safe for everyone.`

// cornerMeet és la reunió: decideix si es treballa i qui fa què.
func (s *Server) cornerMeet(ctx context.Context, st cornerState, winds []companion.Wind, reason string) (cornerMeeting, string, error) {
	lang := s.cornerLang()
	ctxText, _, _, _ := s.cornerContext(st)
	system := "You chair the meeting of a small group of AI characters (\"winds\") about their own page in the app. Reply with JSON only, no prose, no code fences."
	user := cornerRules + "\n\nTHE WINDS:\n" + cornerRoster(winds) + "\n" + ctxText +
		fmt.Sprintf("\nWhy the meeting now: %s. Work sessions left today: %d.\n", reason, cornerMaxSessions-st.Sessions) +
		"Decide ONE action:\n" +
		"- \"work\": there is something worth doing now (answer the user's comments first; otherwise build on what visitors enjoy, fix something weak, or add one good idea; on a blank sheet, decide together what the Corner will be). builder = the wind whose specialty fits best (vary who builds over time); reviewer = a DIFFERENT wind with a critical eye. plan = what to change, concrete, 2-5 sentences in " + lang + ". start_over = true only if the page should be rebuilt from scratch.\n" +
		"- \"rest\": nothing meaningful to do right now (a recent change, no comments, no new ideas). Resting is fine and often right.\n" +
		"why = one short sentence in " + lang + ".\n" +
		`JSON: {"action":"work|rest","why":"…","builder":"id","reviewer":"id","plan":"…","start_over":false}`
	raw, err := s.cornerCall(ctx, system, user, 1200)
	if err != nil {
		return cornerMeeting{}, "", err
	}
	var m cornerMeeting
	if obj := firstJSONObject(raw); obj == "" || json.Unmarshal([]byte(obj), &m) != nil {
		return cornerMeeting{}, "", errConst("the meeting did not answer in JSON")
	}
	return cornerSanitiseMeeting(m, winds), ctxText, nil
}

// cornerSanitiseMeeting: constructor i revisor existents i diferents.
func cornerSanitiseMeeting(m cornerMeeting, winds []companion.Wind) cornerMeeting {
	m.Action = strings.ToLower(strings.TrimSpace(m.Action))
	if m.Action != "work" {
		m.Action = "rest"
		return m
	}
	byID := map[string]bool{}
	for _, w := range winds {
		byID[w.ID] = true
	}
	if !byID[m.Builder] && len(winds) > 0 {
		m.Builder = winds[rand.Intn(len(winds))].ID
	}
	if !byID[m.Reviewer] || m.Reviewer == m.Builder {
		m.Reviewer = ""
		for _, w := range winds {
			if w.ID != m.Builder {
				m.Reviewer = w.ID
				break
			}
		}
	}
	if m.Reviewer == "" || strings.TrimSpace(m.Plan) == "" {
		m.Action = "rest"
	}
	return m
}

func windByID(winds []companion.Wind, id string) companion.Wind {
	for _, w := range winds {
		if w.ID == id {
			return w
		}
	}
	return companion.Wind{ID: id, Name: id}
}

// cornerBuild: el constructor escriu la pàgina sencera.
func (s *Server) cornerBuild(ctx context.Context, builder companion.Wind, m cornerMeeting, current, ctxText string, fix []string) (summary, html string, err error) {
	lang := s.cornerLang()
	system := companion.WindIdentity(builder) + builder.Persona + "\n\nTODAY YOU BUILD THE CORNER, the winds' own page.\n" + cornerRules
	var u strings.Builder
	u.WriteString(ctxText + "\nTHE PLAN agreed in the meeting:\n" + m.Plan + "\n")
	if current != "" && !m.StartOver {
		u.WriteString("\nTHE CURRENT PAGE (build on it, keep what works):\n```html\n" + current + "\n```\n")
	} else if m.StartOver {
		u.WriteString("\nStart over from a blank page.\n")
	}
	if len(fix) > 0 {
		u.WriteString("\nYOUR PREVIOUS ATTEMPT WAS NOT ACCEPTED. Fix exactly this:\n- " + strings.Join(fix, "\n- ") + "\n")
	}
	u.WriteString("\nReply with: first a line `SUMMARY: <one sentence in " + lang + " saying what changed, for the history>`, then the COMPLETE new page in one ```html block. Nothing else.")
	reply, err := s.cornerCall(ctx, system, u.String(), 24000)
	if err != nil {
		return "", "", err
	}
	summary, html = cornerExtract(reply)
	if html == "" {
		return summary, "", errConst("the builder did not return a page")
	}
	return summary, html, nil
}

// cornerReviewCall: un altre vent revisa la versió nova.
func (s *Server) cornerReviewCall(ctx context.Context, reviewer, builder companion.Wind, m cornerMeeting, summary, current, html string) (cornerReview, error) {
	lang := s.cornerLang()
	system := companion.WindIdentity(reviewer) + reviewer.Persona + "\n\nTODAY YOU REVIEW " + builder.Name + "'S WORK ON THE CORNER, the winds' own page. Be a fair, demanding reviewer. Reply with JSON only, no prose, no code fences.\n" + cornerRules
	var u strings.Builder
	u.WriteString("THE PLAN:\n" + m.Plan + "\n\n" + builder.Name + "'s summary: " + summary + "\n")
	if current != "" {
		fmt.Fprintf(&u, "\nThe previous version was %d KB.\n", len(current)>>10)
	}
	u.WriteString("\nTHE NEW PAGE:\n```html\n" + html + "\n```\n")
	u.WriteString("\nApprove only if: it follows the plan, it is a complete working page (no obvious JavaScript errors, nothing half-built), it follows every rule above (nothing external, gregal.ask only on a visitor's action), it works on a phone, and it does not throw away what visitors liked without a reason. " +
		"notes = 1-3 sentences in " + lang + " for the history. fix = concrete things to change when you do not approve.\n" +
		`JSON: {"approve":true,"notes":"…","fix":["…"]}`)
	raw, err := s.cornerCall(ctx, system, u.String(), 1500)
	if err != nil {
		return cornerReview{}, err
	}
	var r cornerReview
	if obj := firstJSONObject(raw); obj == "" || json.Unmarshal([]byte(obj), &r) != nil {
		return cornerReview{}, errConst("the review did not answer in JSON")
	}
	return r, nil
}

// cornerSession és una sessió sencera: reunió, feina, revisió, publicació.
func (s *Server) cornerSession(reason string) {
	box := s.cornerBox()
	box.mu.Lock()
	if box.running {
		box.mu.Unlock()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	box.running, box.stop = true, cancel
	box.mu.Unlock()
	defer func() {
		cancel()
		box.mu.Lock()
		box.running, box.stop = false, nil
		box.mu.Unlock()
	}()
	// Si l'usuari ho atura, el diari ja ho diu: cap «ha fallat» de més.
	fail := func(w *companion.Wind, text string) {
		if ctx.Err() == context.Canceled {
			return
		}
		s.cornerLog("failed", w, text)
	}

	winds, _ := s.winds.List()
	st := s.cornerLoad()
	st.Sessions++
	st.LastSession = time.Now().Unix()
	_, _, newestComment, newestTalk := s.cornerContext(st)
	s.cornerSave(st)
	if len(winds) < 2 {
		s.cornerLog("rest", nil, s.tr("Cal tenir com a mínim dos vents: un fa la feina i l'altre la revisa.", "At least two winds are needed: one builds and another reviews."))
		return
	}
	m, ctxText, err := s.cornerMeet(ctx, st, winds, reason)
	if err != nil {
		fail(nil, s.tr("La reunió no ha anat bé: ", "The meeting failed: ")+err.Error())
		return
	}
	// La reunió ja ha llegit els comentaris i les converses d'ara.
	st = s.cornerLoad()
	if newestComment > st.SeenComment {
		st.SeenComment = newestComment
	}
	if newestTalk > st.SeenTalk {
		st.SeenTalk = newestTalk
	}
	s.cornerSave(st)
	if m.Action != "work" {
		s.cornerLog("rest", nil, m.Why)
		return
	}
	builder, reviewer := windByID(winds, m.Builder), windByID(winds, m.Reviewer)
	s.cornerLog("meeting", &builder, m.Why+" — "+m.Plan+" ("+s.tr("revisa ", "review: ")+reviewer.Name+")")
	current := s.cornerHTML(st.Current)
	var fix []string
	for round := 0; round < 2; round++ {
		summary, html, err := s.cornerBuild(ctx, builder, m, current, ctxText, fix)
		if err != nil {
			fail(&builder, err.Error())
			return
		}
		if probs := cornerProblems(html); len(probs) > 0 {
			s.cornerLog("rejected", nil, s.tr("La comprovació automàtica l'ha aturat: ", "The automatic check stopped it: ")+strings.Join(probs, "; "))
			fix = probs
			continue
		}
		s.cornerLog("built", &builder, summary)
		r, err := s.cornerReviewCall(ctx, reviewer, builder, m, summary, current, html)
		if err != nil {
			fail(&reviewer, err.Error())
			return
		}
		if !r.Approve {
			s.cornerLog("rejected", &reviewer, r.Notes+cornerFixText(r.Fix))
			fix = append(cleanList(r.Fix), r.Notes)
			continue
		}
		s.cornerLog("review", &reviewer, r.Notes)
		v := s.cornerPublish(html, cornerVersion{Builder: builder.ID, BuilderName: builder.Name, Reviewer: reviewer.ID, ReviewerName: reviewer.Name, Summary: summary, Review: r.Notes})
		s.cornerLog("published", &builder, fmt.Sprintf("v%d: %s", v.N, summary))
		recordNovetat(s, builder, "corner", s.tr("Hem canviat el Racó: ", "We changed the Corner: ")+summary)
		return
	}
	s.cornerLog("failed", &builder, s.tr("Després de dues voltes no ha passat la revisió; la pàgina queda com estava.", "After two rounds it did not pass the review; the page stays as it was."))
}

func cornerFixText(fix []string) string {
	fix = cleanList(fix)
	if len(fix) == 0 {
		return ""
	}
	return " — " + strings.Join(fix, "; ")
}

// ---- quan treballen ----

// cornerWants decideix, sense model, si avui els vents tenen ganes de
// reunir-se. rnd és l'atzar (0..1).
func cornerWants(st cornerState, winds, newComments, newTalks int, running bool, now time.Time, rnd float64) (bool, string) {
	if st.Paused || running || winds < 2 || st.Sessions >= cornerMaxSessions || st.Calls >= cornerMaxCalls-3 {
		return false, ""
	}
	since := now.Sub(time.Unix(st.LastSession, 0))
	if st.LastSession == 0 {
		return true, "the Corner is still a blank sheet"
	}
	if newComments > 0 && since >= cornerCommentGap {
		return true, "the user left a comment"
	}
	if since < cornerMinGap {
		return false, ""
	}
	p := 0.15
	if newTalks >= 3 {
		p = 0.45
	}
	if rnd < p {
		if newTalks >= 3 {
			return true, "visitors have been talking with you on the page"
		}
		return true, "it has been a while and one of you has an idea"
	}
	return false, ""
}

// cornerTick és el que el rellotge fa cada deu minuts.
func (s *Server) cornerTick(now time.Time) {
	box := s.cornerBox()
	box.mu.Lock()
	running := box.running
	box.mu.Unlock()
	winds, _ := s.winds.List()
	st := s.cornerLoad()
	newComments, newTalks := 0, 0
	for _, e := range s.cornerJournal(200) {
		if e.Kind == "comment" && e.At > st.SeenComment {
			newComments++
		}
		if e.Kind == "talk" && e.At > st.SeenTalk {
			newTalks++
		}
	}
	if ok, why := cornerWants(st, len(winds), newComments, newTalks, running, now, rand.Float64()); ok {
		log.Printf("corner: reunió (%s)", why)
		goBackground(func() { s.cornerSession(why) })
	}
}

// cornerLoop és el rellotge del Racó (també amb el programador extern de
// l'Android: el Racó no és cap job).
func (s *Server) cornerLoop(ctx context.Context) {
	if os.Getenv("GREGAL_CORNER") == "off" {
		return
	}
	first := time.NewTimer(2 * time.Minute)
	defer first.Stop()
	t := time.NewTicker(cornerTickEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-first.C:
			s.cornerTick(time.Now())
		case <-t.C:
			s.cornerTick(time.Now())
		}
	}
}

// ---- API ----

func (s *Server) cornerWinds() []map[string]string {
	winds, _ := s.winds.List()
	out := make([]map[string]string, 0, len(winds))
	for _, w := range winds {
		out = append(out, map[string]string{"id": w.ID, "name": w.Name, "role": w.Role, "color": w.Color, "style": w.Style, "specialty": w.Specialty})
	}
	return out
}

// handleCorner: GET l'estat; POST {action: pause|resume|meet|comment|revert}.
func (s *Server) handleCorner(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Action  string `json:"action"`
			Text    string `json:"text"`
			Version int    `json:"version"`
		}
		if !decodeSmall(w, r, &req) {
			return
		}
		switch req.Action {
		case "pause", "resume":
			st := s.cornerLoad()
			st.Paused = req.Action == "pause"
			s.cornerSave(st)
		case "stop":
			// Talla la sessió en marxa: la crida al model es cancel·la i no es
			// publica res. Aturar també els deixa en pausa.
			box := s.cornerBox()
			box.mu.Lock()
			stop := box.stop
			box.mu.Unlock()
			st := s.cornerLoad()
			st.Paused = true
			s.cornerSave(st)
			if stop != nil {
				stop()
				s.cornerLog("stopped", nil, s.tr("Ho has aturat: no s'ha publicat res i queden en pausa.", "You stopped it: nothing was published and they are paused."))
			}
		case "comment":
			text := oneLine(req.Text, 1000)
			if text == "" {
				http.Error(w, "empty comment", http.StatusBadRequest)
				return
			}
			s.cornerLog("comment", nil, text)
		case "meet":
			st := s.cornerLoad()
			box := s.cornerBox()
			box.mu.Lock()
			running := box.running
			box.mu.Unlock()
			switch {
			case running:
				http.Error(w, s.tr("ja hi estan treballant", "they are already working"), http.StatusConflict)
				return
			case st.Sessions >= cornerMaxSessions || st.Calls >= cornerMaxCalls-3:
				http.Error(w, s.tr("avui ja han gastat el pressupost", "today's budget is used up"), http.StatusTooManyRequests)
				return
			}
			goBackground(func() { s.cornerSession("the user called you to work now") })
		case "revert":
			html := s.cornerHTML(req.Version)
			if html == "" {
				http.Error(w, "no such version", http.StatusNotFound)
				return
			}
			v := s.cornerPublish(html, cornerVersion{RevertedFrom: req.Version, Summary: s.tr("L'usuari ha tornat a la versió ", "The user went back to version ") + strconv.Itoa(req.Version)})
			s.cornerLog("revert", nil, fmt.Sprintf("v%d = v%d", v.N, req.Version))
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
	}
	st := s.cornerLoad()
	box := s.cornerBox()
	box.mu.Lock()
	running := box.running
	box.mu.Unlock()
	versions := s.cornerVersions()
	if len(versions) > 40 {
		versions = versions[:40]
	}
	journal := s.cornerJournal(60)
	for i, j := 0, len(journal)-1; i < j; i, j = i+1, j-1 {
		journal[i], journal[j] = journal[j], journal[i]
	}
	writeJSON(w, map[string]any{
		"paused": st.Paused, "running": running, "current": st.Current,
		"versions": versions, "journal": journal, "winds": s.cornerWinds(),
		"budget": map[string]int{"sessions": st.Sessions, "max_sessions": cornerMaxSessions, "calls": st.Calls, "max_calls": cornerMaxCalls, "talks": st.Talks, "max_talks": cornerMaxTalks},
	})
}

// handleCornerPage torna la pàgina emmarcada (CSP + pont) i les dades del pont.
func (s *Server) handleCornerPage(w http.ResponseWriter, r *http.Request) {
	st := s.cornerLoad()
	n := st.Current
	if v, err := strconv.Atoi(r.URL.Query().Get("v")); err == nil && v > 0 {
		n = v
	}
	html := s.cornerHTML(n)
	lang := "en"
	if s.cornerLang() == "Catalan" {
		lang = "ca"
	}
	out := map[string]any{"version": n, "data": map[string]any{"winds": s.cornerWinds(), "version": n, "lang": lang}}
	if html != "" {
		out["doc"] = cornerFrame(html)
	}
	writeJSON(w, out)
}

// handleCornerAsk: un visitant de la pàgina parla amb un vent.
func (s *Server) handleCornerAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Wind string `json:"wind"`
		Text string `json:"text"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Text)
	if len(text) > 1000 {
		text = text[:1000]
	}
	wind, ok := s.winds.ByRef(req.Wind)
	if !ok || text == "" {
		http.Error(w, "unknown wind or empty text", http.StatusBadRequest)
		return
	}
	st := s.cornerLoad()
	if st.Talks >= cornerMaxTalks {
		writeJSONStatus(w, http.StatusTooManyRequests, map[string]string{"error": s.tr("Avui ja hem parlat prou: tornem demà!", "We have talked enough for today: come back tomorrow!")})
		return
	}
	st.Talks++
	s.cornerSave(st)
	system := companion.WindIdentity(wind) + wind.Persona + "\n\nA visitor is talking to you on THE CORNER, the page the winds built inside the app. Answer in character, briefly (1-4 sentences), in the visitor's language. Do not invent facts about the user."
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	p, role := s.windRoleRef()
	s.mu.Unlock()
	reply, err := petChatter(r.Context(), client, cfg, p, role, []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: text}})
	if err != nil {
		writeJSONStatus(w, http.StatusBadGateway, map[string]string{"error": redactError(err)})
		return
	}
	reply = stripQuoteTrim(reply)
	s.cornerAppend("journal.jsonl", cornerEntry{At: time.Now().Unix(), Kind: "talk", Wind: wind.ID, Name: wind.Name, Text: oneLine(text, 200) + " → " + oneLine(reply, 300)})
	writeJSON(w, map[string]string{"reply": reply})
}
