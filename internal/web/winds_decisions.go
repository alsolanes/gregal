package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/events"
	"gregal/internal/llm"
	"gregal/internal/runs"
)

// Magatzem durable de decisions i represa dels torns aparcats (B4).
//
// Cada decisió és un fitxer <id>.json a <windStateDir>/decisions: la carpeta
// és la de l'usuari, així que un usuari mai no llegeix les d'un altre (i
// cada lectura torna a comprovar owner). Els canvis d'estat surten pel
// registre d'events de sempre (kind "decision"), amb cursor i replay.

// windResume és el punt segur d'un torn aparcat.
type windResume struct {
	Wind    string `json:"wind"`
	Thread  string `json:"thread"`
	UserMsg string `json:"user_msg,omitempty"`
	// Committed: el missatge de la persona i l'avís d'espera ja són al fil.
	Committed bool `json:"committed,omitempty"`
	// Hist acaba amb el missatge de l'assistent que demana les crides del
	// lot pendent, sense cap resultat: Loop.Repren el continua.
	Hist []llm.Message `json:"hist"`
}

// decisionDir és la vista bloquejada de la carpeta de decisions d'un usuari.
type decisionDir struct{ dir string }

var decisionLocks sync.Map // dir → *sync.Mutex

func (s *Server) decisionsPath() string { return filepath.Join(s.windStateDir(), "decisions") }

// withDecisions executa fn amb la carpeta de decisions bloquejada.
func (s *Server) withDecisions(fn func(*decisionDir) error) error {
	dir := s.decisionsPath()
	mu, _ := decisionLocks.LoadOrStore(dir, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	return fn(&decisionDir{dir: dir})
}

func validDecisionID(id string) bool {
	if !strings.HasPrefix(id, "wa-") || len(id) > 64 {
		return false
	}
	for _, r := range id[3:] {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z') {
			return false
		}
	}
	return true
}

func (d *decisionDir) load(id string) (*windDecisionRec, error) {
	if !validDecisionID(id) {
		return nil, os.ErrNotExist
	}
	raw, err := os.ReadFile(filepath.Join(d.dir, id+".json"))
	if err != nil {
		return nil, err
	}
	var rec windDecisionRec
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (d *decisionDir) save(rec *windDecisionRec) error {
	if err := os.MkdirAll(d.dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(d.dir, rec.ID+".json"), b)
}

func (d *decisionDir) list() []*windDecisionRec {
	ents, _ := os.ReadDir(d.dir)
	out := []*windDecisionRec{}
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		if rec, err := d.load(strings.TrimSuffix(name, ".json")); err == nil {
			out = append(out, rec)
		}
	}
	return out
}

// supersede marca com a superseded les pendents del mateix slot (fil, eina i
// tipus) i dona a rec la revisió següent. Torna les que ha canviat.
func (d *decisionDir) supersede(rec *windDecisionRec) []*windDecisionRec {
	if rec.Slot == "" {
		return nil
	}
	var out []*windDecisionRec
	for _, old := range d.list() {
		if old.State != "pending" || old.Slot != rec.Slot || old.Owner != rec.Owner || old.ID == rec.ID {
			continue
		}
		if old.Revision >= rec.Revision {
			rec.Revision = old.Revision + 1
		}
		old.State, old.SupersededBy, old.ResolvedAt = "superseded", rec.ID, time.Now().UnixMilli()
		if d.save(old) == nil {
			out = append(out, old)
		}
	}
	return out
}

// sweep caduca una pendent que ja no pot tenir efecte: ha passat l'hora, o
// va néixer en un altre procés sense punt de represa (ningú no l'espera).
// Torna cert si l'ha canviat (i desat).
func (d *decisionDir) sweep(rec *windDecisionRec) bool {
	if rec.State != "pending" {
		return false
	}
	now := time.Now()
	switch {
	case rec.Expires > 0 && now.UnixMilli() > rec.Expires:
		rec.StateReason = "timeout"
	case rec.Resume == nil && rec.Proc != windProc:
		rec.StateReason = "restart"
	default:
		return false
	}
	rec.State, rec.ResolvedAt = "expired", now.UnixMilli()
	_ = d.save(rec)
	return true
}

// emitDecision deixa el canvi d'estat al registre d'events (kind decision,
// payload = la decisió pública). Els clients deduplicuen per id+revision+state.
func (s *Server) emitDecision(rec *windDecisionRec) {
	if rec == nil {
		return
	}
	b, _ := json.Marshal(rec.public())
	_ = s.Hub().appendEvent(events.Event{
		SessionID: rec.Session, User: s.usuari(), RunID: int(rec.RunID),
		Kind: events.Decision, Text: rec.State, Payload: b,
	})
}

// afterDecisionChange publica una decisió que ha deixat d'estar pendent sense
// resposta (caducada, substituïda) i atura el run aparcat que l'esperava.
func (s *Server) afterDecisionChange(rec *windDecisionRec) {
	s.emitDecision(rec)
	if rec.RunID <= 0 || (rec.State != "expired" && rec.State != "superseded") {
		return
	}
	q := s.Hub().Queue()
	run, ok := q.Get(rec.RunID)
	if !ok {
		run = q.Park(runs.Run{ID: rec.RunID, Session: rec.Session, Priority: runs.PriorityBackground})
	}
	if run.State == runs.WaitingUser && q.Cancel(rec.RunID) {
		_ = s.Hub().appendEvent(events.Event{SessionID: rec.Session, User: s.usuari(), RunID: int(rec.RunID), Kind: "run_cancelled", Text: "torn cancel·lat: decisió " + rec.State})
	}
}

// markApplied apunta que l'efecte d'una decisió aprovada ja s'ha deixat fer.
func (s *Server) markApplied(id string) {
	_ = s.withDecisions(func(st *decisionDir) error {
		rec, err := st.load(id)
		if err != nil || rec.State != "approved" || rec.Applied {
			return nil
		}
		rec.Applied = true
		return st.save(rec)
	})
}

// Torn aparcable: on viu (vent i fil) i, un cop aparcat, la decisió.
type windParkKey struct{}

type windParkSpot struct {
	wind, thread, userMsg string
	runID                 int64

	mu     sync.Mutex
	hist   []llm.Message
	parked *windDecisionRec
}

func withWindPark(ctx context.Context, p *windParkSpot) context.Context {
	return context.WithValue(ctx, windParkKey{}, p)
}

func windParkOf(ctx context.Context) *windParkSpot {
	p, _ := ctx.Value(windParkKey{}).(*windParkSpot)
	return p
}

func (p *windParkSpot) setHist(h []llm.Message) {
	p.mu.Lock()
	p.hist = h
	p.mu.Unlock()
}

func (p *windParkSpot) snapshot() []llm.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]llm.Message(nil), p.hist...)
}

func (p *windParkSpot) park(rec *windDecisionRec) {
	p.mu.Lock()
	if p.parked == nil {
		p.parked = rec
	}
	p.mu.Unlock()
}

func (p *windParkSpot) parkedRec() *windDecisionRec {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.parked
}

// Represa: la decisió resolta que ha reprès aquest torn.
type windResumeKey struct{}

type windResumeCtx struct {
	id   string
	mu   sync.Mutex
	used bool
}

func withWindResume(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, windResumeKey{}, &windResumeCtx{id: id})
}

func windResumeOf(ctx context.Context) *windResumeCtx {
	r, _ := ctx.Value(windResumeKey{}).(*windResumeCtx)
	return r
}

// consumeResumedDecision: si la crida que es torna a decidir és l'efecte
// exacte de la decisió resolta, en torna la resposta una sola vegada. Una
// aprovació ja aplicada no es torna a aplicar.
func (s *Server) consumeResumedDecision(res *windResumeCtx, hash string) (windAnswer, bool) {
	res.mu.Lock()
	defer res.mu.Unlock()
	if res.used {
		return windAnswer{}, false
	}
	var ans windAnswer
	ok := false
	_ = s.withDecisions(func(st *decisionDir) error {
		rec, err := st.load(res.id)
		if err != nil || rec.ProposedEffect == nil || rec.ProposedEffect.Hash != hash || rec.Answer == nil {
			return nil
		}
		ok = true
		ans = *rec.Answer
		switch rec.State {
		case "approved":
			if rec.Type == "approve" && rec.Applied {
				ans.Approve = false // ja fet: cap segon efecte
				return nil
			}
			rec.Applied = true
			return st.save(rec)
		case "rejected":
			ans.Approve = false
		default:
			ok = false
		}
		return nil
	})
	if ok {
		res.used = true
	}
	return ans, ok
}

// parkWindRun registra a la cua el run aparcat d'un xat en directe i ho
// deixa al registre (run_waiting_user). Torna l'ID.
func (s *Server) parkWindRun(rec *windDecisionRec) int64 {
	run := s.Hub().Queue().Park(runs.Run{Session: rec.Session, Priority: runs.PriorityBackground})
	kind, text := events.RunWaitingUser, "esperant la decisió "+rec.ID
	if rec.Type == "connection" {
		// L'encàrrec espera una connexió (waiting_connection): la cua el té
		// aparcat igual, però els clients ho han de dir així.
		kind, text = "run_waiting_connection", "esperant la connexió "+rec.Site+" (decisió "+rec.ID+")"
	}
	_ = s.Hub().appendEvent(events.Event{SessionID: rec.Session, User: s.usuari(), RunID: int(run.ID), Kind: kind, Text: text})
	return run.ID
}

// decisionServer és la sessió on va néixer la decisió (la resposta pot
// entrar per una altra sessió del mateix usuari).
func (s *Server) decisionServer(session string) *Server {
	h := s.Hub()
	h.mu.RLock()
	defer h.mu.RUnlock()
	if t := h.sess[session]; t != nil && t.usuari() == s.usuari() {
		return t
	}
	return s
}

// resumeWindDecision torna a encuar el torn aparcat d'una decisió resolta.
// Només el crida qui ha fet la transició pending → resolta.
func (s *Server) resumeWindDecision(rec *windDecisionRec) {
	t := s.decisionServer(rec.Session)
	q := t.Hub().Queue()
	id := rec.RunID
	if id == 0 {
		// El procés va morir esperant en directe: el run neix ara.
		id = t.parkWindRun(rec)
		_ = t.withDecisions(func(st *decisionDir) error {
			cur, err := st.load(rec.ID)
			if err != nil {
				return err
			}
			cur.RunID = id
			rec.RunID = id
			return st.save(cur)
		})
	}
	run, ok := q.Get(id)
	if !ok {
		run = q.Park(runs.Run{ID: id, Session: rec.Session, Priority: runs.PriorityBackground})
	}
	resume := func() {
		r, ok := q.Resume(id, t.windResumeExec(rec.ID))
		if !ok {
			return
		}
		_ = t.Hub().appendEvent(events.Event{SessionID: r.Session, User: t.usuari(), RunID: int(r.ID), Kind: "run_queued", Text: "torn reprès: decisió " + rec.ID})
		t.watchRun(r)
	}
	if run.State == runs.Running || run.State == runs.Queued {
		// El torn encara s'està aparcant (la resposta ha arribat de seguida):
		// es reprèn quan deixi d'executar-se.
		goBackground(func() { <-q.Done(id); resume() })
		return
	}
	resume()
}

// windWaitingText és l'avís que queda al fil mentre el torn espera.
func (s *Server) windWaitingText(rec *windDecisionRec) string {
	if rec.Type == "question" {
		return s.tr("Necessito la teva resposta per continuar: ", "I need your answer to carry on: ") + rec.Query
	}
	if rec.Type == "connection" {
		return s.tr("Necessito que connectis «", "I need you to connect «") + rec.What + s.tr("» per continuar. Obre Connexions i toca «Connecta»; continuaré l'encàrrec tot sol.", "» to carry on. Open Connections and tap «Connect»; I will carry on with the task by myself.")
	}
	if rec.Type == "handoff" {
		return s.tr("Et toca a tu a l'ordinador: ", "Your turn on the computer: ") + rec.What +
			s.tr(" Obre l'ordinador, fes-ho tu i toca «Ja està»; continuaré l'encàrrec.", " Open the computer, do it yourself and tap «Done»; I will carry on with the task.")
	}
	return s.tr("Espero el teu permís per continuar: ", "Waiting for your permission to carry on: ") + firstNonEmpty(rec.What, rec.Name)
}

// noteParkedDecision lliga la decisió al missatge d'espera del fil.
func (s *Server) noteParkedDecision(id, messageID string) *windDecisionRec {
	var out *windDecisionRec
	_ = s.withDecisions(func(st *decisionDir) error {
		rec, err := st.load(id)
		if err != nil {
			return err
		}
		rec.MessageID = messageID
		if rec.Resume != nil {
			rec.Resume.Committed = true
		}
		out = rec
		return st.save(rec)
	})
	if out != nil {
		s.emitDecision(out)
	}
	return out
}

// appendWindThread afegeix missatges al final del fil, encara que entremig
// hi hagi hagut altres torns (la represa arriba hores després).
func (s *Server) appendWindThread(wind companion.Wind, hist *windHistoryEntry, msgs ...llm.Message) error {
	hist.mu.Lock()
	defer hist.mu.Unlock()
	history := append(append([]llm.Message(nil), hist.messages...), msgs...)
	if err := saveWindHistoryTo(s, wind, hist.name, history); err != nil {
		return err
	}
	hist.messages = podaHistorialVent(history)
	hist.gen++
	return nil
}

// windResumeExec és l'executor del run que reprèn un torn aparcat: refà el
// context del torn amb la política d'ara, continua el bucle des del punt
// segur i publica el resultat al mateix fil.
func (s *Server) windResumeExec(id string) runs.Executor {
	return func(ctx context.Context, run runs.Run) error {
		load := func() *windDecisionRec {
			var rec *windDecisionRec
			_ = s.withDecisions(func(st *decisionDir) error {
				rec, _ = st.load(id)
				return nil
			})
			return rec
		}
		rec := load()
		if rec == nil || rec.Resume == nil {
			return errors.New("decisió sense punt de represa")
		}
		wind, ok := s.winds.ByID(rec.Resume.Wind)
		if !ok {
			return fmt.Errorf("el vent %q ja no existeix", rec.Resume.Wind)
		}
		turn := windDirectTurn(s, wind.ID)
		turn.Lock()
		defer turn.Unlock()
		if rec = load(); rec == nil || (rec.State != "approved" && rec.State != "rejected") {
			return runs.ErrCancelled // substituïda o caducada mentrestant
		}
		hist := windHistoryFor(s, rec.Resume.Thread)
		_ = s.Hub().appendEvent(events.Event{SessionID: run.Session, User: s.usuari(), RunID: int(run.ID), Kind: "run_started", Text: "torn reprès: decisió " + rec.ID})

		tctx, cancel := context.WithTimeout(ctx, windTurnTimeout)
		defer cancel()
		tctx, doneTurn := s.beginWindTurn("wind", wind.ID, tctx)
		defer doneTurn()
		tctx = s.interactiveWindCtx(s.withWindSteerKey(tctx, "wind", wind.ID))
		tctx = s.windDocsCtx(agent.WithSkillsDirs(tctx, s.windSkillsDir(wind.ID)))
		spot := &windParkSpot{wind: wind.ID, thread: rec.Resume.Thread, runID: run.ID}
		tctx = withWindResume(withWindPark(tctx, spot), rec.ID)
		emit := func(kind, text string) {
			switch kind {
			case "approve_request", "question_request", "ask_resolved":
				_ = s.Hub().appendEvent(events.Event{SessionID: run.Session, User: s.usuari(), RunID: int(run.ID), Kind: kind, Text: text, Payload: json.RawMessage(text)})
			}
		}
		var userMsgs []llm.Message
		if !rec.Resume.Committed && strings.TrimSpace(rec.Resume.UserMsg) != "" {
			userMsgs = append(userMsgs, llm.Message{Role: "user", Content: rec.Resume.UserMsg, Ts: time.Now().Unix(), ID: newMessageID()})
		}
		p, role := s.windRoleRef()
		res, err := windResumeLoop(s, tctx, p, role, emit, rec.Resume.Hist)
		if errors.Is(err, errWindParked) {
			next := spot.parkedRec()
			if next == nil {
				return err
			}
			msgID := newMessageID()
			msgs := append(userMsgs, llm.Message{Role: "assistant", Content: s.windWaitingText(next), Ts: time.Now().Unix(), ID: msgID})
			if err := s.appendWindThread(wind, hist, msgs...); err != nil {
				return err
			}
			s.noteParkedDecision(next.ID, msgID)
			return runs.ErrWaitingUser
		}
		if err != nil {
			if tornAturat(err) || ctx.Err() != nil {
				return runs.ErrCancelled
			}
			return err
		}
		reply := stripQuoteTrim(res.text)
		if res.parcial && esRespostaMecanica(reply) {
			reply = fraseTornAturat(res.motiu, s.cfg.Lang())
		}
		msgID := newMessageID()
		msgs := append(userMsgs, llm.Message{Role: "assistant", Content: reply, Ts: time.Now().Unix(), ID: msgID})
		if err := s.appendWindThread(wind, hist, msgs...); err != nil {
			return err
		}
		_ = s.Hub().appendEvent(events.Event{SessionID: run.Session, User: s.usuari(), RunID: int(run.ID), Kind: "done", Text: reply})
		// Torna amb la feina feta: una novetat que apunta al missatge.
		recordNovetatMsg(s, wind, "feina:decisio", "", msgID, reply)
		return nil
	}
}

// windResumeLoop continua el bucle del vent des del punt segur. Els tests el
// substitueixen.
var windResumeLoop = func(s *Server, ctx context.Context, p config.Provider, role config.Role, emit func(kind, text string), hist []llm.Message) (exploracio, error) {
	return s.runReadOnlyLoopForRoleStream(ctx, "", windTurnSteps, p, role, emit, hist)
}

// parkedDecisionRun recupera a la cua el run aparcat d'una decisió pendent
// d'aquesta sessió (després d'un reinici la cua en memòria no el coneix).
func (s *Server) parkedDecisionRun(id int64) (runs.Run, bool) {
	found := false
	_ = s.withDecisions(func(st *decisionDir) error {
		for _, rec := range st.list() {
			if rec.RunID == id && rec.State == "pending" && rec.Session == s.id && rec.Owner == s.remindOwner() {
				found = true
				return nil
			}
		}
		return nil
	})
	if !found {
		return runs.Run{}, false
	}
	return s.Hub().Queue().Park(runs.Run{ID: id, Session: s.id, Priority: runs.PriorityBackground}), true
}

// cancelParkedDecisions caduca (cancelled) les decisions pendents del run.
func (s *Server) cancelParkedDecisions(runID int64) {
	var changed []*windDecisionRec
	_ = s.withDecisions(func(st *decisionDir) error {
		for _, rec := range st.list() {
			if rec.RunID == runID && rec.State == "pending" && rec.Owner == s.remindOwner() {
				rec.State, rec.StateReason, rec.ResolvedAt = "expired", "cancelled", time.Now().UnixMilli()
				if st.save(rec) == nil {
					changed = append(changed, rec)
				}
			}
		}
		return nil
	})
	for _, rec := range changed {
		s.emitDecision(rec)
	}
}
