package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/internal/tools"
)

// Preguntes i aprovacions dels vents (decisions durables, B4).
//
// Un torn de vent que necessita una decisió (permís per a una acció o una
// pregunta amb opcions) l'emet com a approve_request / question_request pel
// flux de la conversa (/api/winds/chat/stream i /api/winds/rotllana/stream)
// i la desa a disc, a la carpeta d'estat de l'usuari (windStateDir). Des
// d'aquí, /api/winds/pending la torna encara que el servidor s'hagi
// reiniciat, i /api/winds/answer la resol.
//
// L'espera no reté el torn tota la nit:
//
//   - Durant windAskLive el torn espera en directe (la persona sol tenir el
//     xat obert i respon de seguida).
//   - Passat aquest temps, un xat de vent s'aparca en el punt segur (totes
//     les crides del pas decidides, cap executada): queda desat l'historial
//     fins a la crida pendent, el torn passa a waiting_user i el goroutine
//     acaba. La decisió espera fins a windDecisionTTL.
//   - Quan arriba la resposta, el torn es reprèn com un run de la cua
//     compartida (mateix runId), revalidant el permís amb la política
//     d'ara, i el resultat s'afegeix al mateix fil.
//
// L'aprovació és de l'efecte concret (eina + arguments): proposedEffect.hash.
// Una petició nova del mateix fil per a la mateixa eina i tipus substitueix
// la pendent (superseded) i en puja la revisió; respondre la vella ja no
// fa res. Resoldre dues vegades una decisió només produeix un efecte.
//
// Els torns sense lloc on reprendre's (rotllanes, colles) fan com abans:
// esperen en directe fins a windAskMax i, sense resposta, l'acció no es fa.

// windAskMax és el màxim que un torn no aparcable espera una resposta.
const windAskMax = 20 * time.Minute

// windAskLive és quant espera en directe un torn aparcable abans d'alliberar
// el goroutine. Els tests el redueixen.
var windAskLive = 3 * time.Minute

// windDecisionTTL és quant temps espera una decisió aparcada: tota la nit i
// el cap de setmana. Passat, caduca (expired) i el torn no es reprèn.
var windDecisionTTL = 72 * time.Hour

// windProc identifica aquest procés: una decisió sense punt de represa que
// va néixer en un altre procés ja no té ningú esperant-la.
var windProc = newMessageID()

type windAskOpt struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// windEffect és l'efecte concret que es demana aprovar.
type windEffect struct {
	Type    string `json:"type"`           // tool_call | question
	Tool    string `json:"tool,omitempty"` // l'eina
	Kind    string `json:"kind,omitempty"` // install | download | delete | system | other
	Summary string `json:"summary"`        // l'ordre o la pregunta, curta
	// Hash lliga l'aprovació a l'eina i els arguments exactes: si canvien,
	// cal una decisió nova.
	Hash string `json:"hash"`
}

// windAskReq és la decisió pública: el que tornen /api/winds/pending i els
// events approve_request / question_request. Els camps de sempre (key, type,
// query, options, name, kind, what, reason, at, expires) no canvien de
// sentit; la resta són del contracte de decisió i són opcionals per als
// clients antics.
type windAskReq struct {
	Key     string       `json:"key"`
	Type    string       `json:"type"`
	Query   string       `json:"query,omitempty"`
	Options []windAskOpt `json:"options,omitempty"`
	Name    string       `json:"name,omitempty"` // l'eina
	Kind    string       `json:"kind,omitempty"` // install | download | delete | system | other
	What    string       `json:"what,omitempty"` // l'ordre o l'acció, curta
	// Detail: l'ordre o els arguments sencers, per a «Veure l'ordre» (What és
	// la part que importa; aquí hi ha tot el que s'executarà).
	Detail  string `json:"detail,omitempty"`
	Reason  string `json:"reason,omitempty"` // per què es pregunta
	At      int64  `json:"at"`
	Expires int64  `json:"expires"`
	// Handoff (B3): la pàgina (origen + camí, sense query) i el lloc on la
	// persona ha d'iniciar sessió des del visor.
	Page string `json:"page,omitempty"`
	Site string `json:"site,omitempty"`

	// Contracte de decisió (docs/assistant-contract.ca.md).
	ID             string      `json:"id"`
	Owner          string      `json:"owner,omitempty"`
	Wind           string      `json:"wind,omitempty"`
	RunID          int64       `json:"runId,omitempty"`
	MessageID      string      `json:"messageId,omitempty"`
	ProposedEffect *windEffect `json:"proposedEffect,omitempty"`
	Revision       int         `json:"revision"`
	State          string      `json:"state"` // decisioStates
	ExpiresAt      string      `json:"expiresAt,omitempty"`
	ResolvedAt     int64       `json:"resolvedAt,omitempty"`
	// StateReason diu per què ha caducat: timeout, cancelled, restart.
	StateReason  string `json:"stateReason,omitempty"`
	SupersededBy string `json:"supersededBy,omitempty"`
}

type windAnswer struct {
	Approve bool   `json:"approve"`
	Answer  string `json:"answer,omitempty"`
}

// windDecisionRec és el que es desa a disc: la decisió pública i el que
// cal per reprendre el torn, que mai no surt per l'API.
type windDecisionRec struct {
	windAskReq
	Session string      `json:"session,omitempty"` // clau interna de la sessió
	Slot    string      `json:"slot,omitempty"`    // fil + eina + tipus: una pendent per slot
	Proc    string      `json:"proc,omitempty"`
	Answer  *windAnswer `json:"answer,omitempty"`
	// Applied: l'efecte aprovat ja s'ha deixat executar (una sola vegada).
	Applied bool        `json:"applied,omitempty"`
	Resume  *windResume `json:"resume,omitempty"`
}

func (r *windDecisionRec) public() windAskReq { return r.windAskReq }

// windAskOutcome és com acaba una espera.
type windAskOutcome int

const (
	askNone     windAskOutcome = iota // sense resposta: no es fa res
	askAnswered                       // resposta (aprovada, rebutjada o text)
	askParked                         // torn aparcat: continuarà quan es respongui
)

// errWindParked atura el bucle en el punt segur quan una decisió s'ha
// aparcat. No és un error del torn.
var errWindParked = errors.New("torn aparcat esperant una decisió")

// Esperes en directe: id de decisió → canal. Global del procés perquè la
// resposta pot entrar per una altra sessió del mateix usuari (mòbil i
// escriptori); l'id és aleatori i la propietat la garanteix el directori.
var windLiveAsks = struct {
	sync.Mutex
	m map[string]chan windAnswer
}{m: map[string]chan windAnswer{}}

func windEffectHash(name, args string) string {
	h := sha256.Sum256([]byte(name + "\x00" + strings.TrimSpace(args)))
	return hex.EncodeToString(h[:12])
}

// windAskWait publica la decisió, la desa i n'espera la resposta. Si el torn
// és aparcable i ningú no respon en windAskLive, l'aparca (askParked).
// args són els arguments de la crida: lliguen l'aprovació a l'efecte exacte.
func (s *Server) windAskWait(ctx context.Context, emit func(kind, text string), r *windAskReq, args string) (windAnswer, windAskOutcome) {
	effect := &windEffect{Type: "tool_call", Tool: r.Name, Kind: r.Kind, Summary: r.What}
	if r.Type == "question" {
		effect = &windEffect{Type: "question", Tool: "question", Summary: r.Query}
	}
	if r.Type == "handoff" {
		effect = &windEffect{Type: "handoff", Tool: "browser", Kind: "login", Summary: r.What}
	}
	if r.Type == "connection" {
		// B5: la feina espera que la persona connecti un compte (waiting_connection).
		effect = &windEffect{Type: "connection", Tool: "connector", Kind: "connect", Summary: r.What}
	}
	effect.Hash = windEffectHash(effect.Tool, firstNonEmpty(args, effect.Summary))

	// Represa: la decisió que va aparcar aquest torn ja té resposta.
	if res := windResumeOf(ctx); res != nil {
		if ans, ok := s.consumeResumedDecision(res, effect.Hash); ok {
			return ans, askAnswered
		}
	}

	spot := windParkOf(ctx)
	now := time.Now()
	rec := &windDecisionRec{windAskReq: *r}
	rec.ID = "wa-" + strings.TrimPrefix(newMessageID(), "m-")
	rec.Key = rec.ID
	rec.Owner = s.remindOwner()
	rec.ProposedEffect = effect
	rec.Revision = 1
	rec.State = "pending"
	rec.At = now.UnixMilli()
	s.Hub() // garanteix l'id de sessió
	rec.Session = s.id
	rec.Proc = windProc
	ttl := windAskMax
	if spot != nil {
		ttl = windDecisionTTL
		if r.Type == "handoff" {
			ttl = windHandoffTTL
		}
		rec.Wind = spot.wind
		rec.RunID = spot.runID
		rec.Slot = spot.wind + "|" + spot.thread + "|" + effect.Tool + "|" + effect.Kind
		rec.Resume = &windResume{Wind: spot.wind, Thread: spot.thread, UserMsg: spot.userMsg, Hist: spot.snapshot()}
		if scope, ok := ctx.Value(interactiveWindKey{}).(windTurnScope); ok {
			rec.Resume.Workspace = scope.workspace
		}
	}
	rec.Expires = now.Add(ttl).UnixMilli()
	rec.ExpiresAt = now.Add(ttl).UTC().Format(time.RFC3339)

	ch := make(chan windAnswer, 1)
	var superseded []*windDecisionRec
	err := s.withDecisions(func(st *decisionDir) error {
		superseded = st.supersede(rec)
		if err := st.save(rec); err != nil {
			return err
		}
		windLiveAsks.Lock()
		windLiveAsks.m[rec.ID] = ch
		windLiveAsks.Unlock()
		return nil
	})
	if err != nil {
		// Sense disc no hi ha decisió durable; la persona no ho pot aprovar.
		return windAnswer{}, askNone
	}
	for _, old := range superseded {
		s.afterDecisionChange(old)
	}
	s.emitDecision(rec)
	*r = rec.public()
	if b, err := json.Marshal(r); err == nil && emit != nil {
		ev := r.Type + "_request"
		if r.Type == "handoff" || r.Type == "connection" || r.Type == "secret" {
			// Els clients que encara no coneixen handoff hi veuen una
			// aprovació (Permet = ja està, Rebutja = cancel·la).
			ev = "approve_request"
		}
		emit(ev, string(b))
	}

	live := ttl
	if spot != nil && windAskLive < live {
		live = windAskLive
	}
	timer := time.NewTimer(live)
	defer timer.Stop()
	resolved := func(answered bool) {
		if emit != nil {
			b, _ := json.Marshal(map[string]any{"key": rec.Key, "answered": answered})
			emit("ask_resolved", string(b))
		}
	}
	select {
	case ans := <-ch:
		resolved(true)
		s.markApplied(rec.ID)
		return ans, askAnswered
	case <-ctx.Done():
	case <-timer.C:
	}
	// Ja no s'espera en directe. Dins el bloqueig: una resposta que hagi
	// arribat just ara és al canal i es fa servir.
	var out windAskOutcome
	var ans windAnswer
	var changed *windDecisionRec
	_ = s.withDecisions(func(st *decisionDir) error {
		windLiveAsks.Lock()
		delete(windLiveAsks.m, rec.ID)
		windLiveAsks.Unlock()
		select {
		case ans = <-ch:
			out = askAnswered
			return nil
		default:
		}
		cur, err := st.load(rec.ID)
		if err != nil || cur.State != "pending" {
			return nil
		}
		switch {
		case ctx.Err() != nil:
			cur.State, cur.StateReason = "expired", "cancelled"
		case spot != nil:
			// Aparcar: el torn acaba i el run passa a waiting_user.
			if cur.RunID == 0 {
				cur.RunID = s.parkWindRun(cur)
			}
			spot.park(cur)
			out = askParked
			changed = cur
			return st.save(cur)
		default:
			cur.State, cur.StateReason = "expired", "timeout"
		}
		cur.ResolvedAt = time.Now().UnixMilli()
		changed = cur
		return st.save(cur)
	})
	if changed != nil {
		s.emitDecision(changed)
	}
	switch out {
	case askAnswered:
		resolved(true)
		s.markApplied(rec.ID)
	case askNone:
		resolved(false)
	}
	return ans, out
}

// windApprove demana permís per a una acció i bloqueja el torn fins que la
// persona respon. Torna el text per al model si no es fa.
func (s *Server) windApprove(ctx context.Context, emit func(kind, text string), name, kind, what, reason string) (bool, string) {
	return s.windApproveCall(ctx, emit, name, "", kind, what, reason)
}

// windApproveCall és windApprove amb els arguments de la crida, que lliguen
// l'aprovació a l'efecte exacte.
func (s *Server) windApproveCall(ctx context.Context, emit func(kind, text string), name, args, kind, what, reason string) (bool, string) {
	ans, out := s.windAskWait(ctx, emit, &windAskReq{Type: "approve", Name: name, Kind: kind, What: what, Detail: windAskArgsFull(args), Reason: reason}, args)
	switch {
	case out == askParked:
		return false, "NOT RUN YET: waiting for the user's decision."
	case out == askNone:
		return false, "NOT RUN: the user did not answer in time. Do not retry it; say what you needed and ask again in your reply."
	case !ans.Approve:
		return false, "NOT RUN: the user tapped Decline on your request (the server did not block it on its own). Do not retry it and do not look for a way around it; carry on without it or say plainly that you did not do it because they declined."
	}
	return true, ""
}

// windQuestion fa la pregunta del vent i torna el text de la resposta per al model.
func (s *Server) windQuestion(ctx context.Context, emit func(kind, text string), query string, opts []tools.QuestionOption) string {
	ans, _ := s.windQuestionCall(ctx, emit, query, opts, "")
	return ans
}

// windQuestionCall: la pregunta lligada als arguments de la crida. parked diu
// que el torn s'ha aparcat i la resposta arribarà en reprendre'l.
func (s *Server) windQuestionCall(ctx context.Context, emit func(kind, text string), query string, opts []tools.QuestionOption, args string) (string, bool) {
	req := &windAskReq{Type: "question", Query: query}
	for _, o := range opts {
		req.Options = append(req.Options, windAskOpt{Label: o.Label, Description: o.Description})
	}
	ans, out := s.windAskWait(ctx, emit, req, args)
	if out == askParked {
		return "", true
	}
	if out != askAnswered || strings.TrimSpace(ans.Answer) == "" {
		return "The user did not answer in time. Do not assume an answer: say what you need to know and carry on with what does not depend on it.", false
	}
	return "The user answered: " + strings.TrimSpace(ans.Answer), false
}

// forwardWindAsk passa al client les peticions que surten del bucle del vent.
func forwardWindAsk(emit func(string, any), kind, text string) bool {
	switch kind {
	case "approve_request", "question_request", "ask_resolved":
		emit(kind, json.RawMessage(text))
		return true
	}
	return false
}

// handleWindsPending: GET → les decisions pendents d'aquest usuari, de la més
// antiga a la més nova. Sobreviuen als reinicis.
func (s *Server) handleWindsPending(w http.ResponseWriter, r *http.Request) {
	var list []*windDecisionRec
	var changed []*windDecisionRec
	_ = s.withDecisions(func(st *decisionDir) error {
		all := st.list()
		for _, rec := range all {
			if st.sweep(rec) {
				changed = append(changed, rec)
			}
			if rec.State == "pending" && rec.Owner == s.remindOwner() {
				list = append(list, rec)
			}
		}
		return nil
	})
	for _, c := range changed {
		s.afterDecisionChange(c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].At < list[j].At })
	out := make([]windAskReq, 0, len(list))
	for _, rec := range list {
		out = append(out, rec.public())
	}
	writeJSON(w, map[string]any{"pending": out})
}

// handleWindsAnswer: POST {key, approve?, answer?, revision?} resol una
// decisió pendent. Si el torn espera en directe, continua al moment; si
// estava aparcat, es reprèn com un run. Respondre una decisió ja resolta amb
// la mateixa resposta no fa res (duplicate:true); amb una altra, 409.
func (s *Server) handleWindsAnswer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key      string `json:"key"`
		Approve  bool   `json:"approve"`
		Answer   string `json:"answer"`
		Revision int    `json:"revision"`
		// Secret: el valor del formulari segur. Va directe al magatzem
		// xifrat; la decisió només guarda que s'ha donat.
		Secret string `json:"secret"`
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	if len([]rune(req.Answer)) > 2000 {
		req.Answer = string([]rune(req.Answer)[:2000])
	}
	if req.Secret != "" {
		if len(req.Secret) > 8192 {
			http.Error(w, "secret too long", http.StatusBadRequest)
			return
		}
		ok, err := s.storeSecretAnswer(strings.TrimSpace(req.Key), req.Secret)
		req.Secret = ""
		if err != nil {
			http.Error(w, "could not save the secret", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "expired or unknown request", http.StatusNotFound)
			return
		}
		req.Approve, req.Answer = true, "saved"
	}
	ans := windAnswer{Approve: req.Approve, Answer: req.Answer}
	rec, code := s.answerDecision(strings.TrimSpace(req.Key), ans, req.Revision)
	switch code {
	case http.StatusNotFound:
		http.Error(w, "expired or unknown request", http.StatusNotFound)
		return
	case http.StatusOK:
		writeJSON(w, map[string]any{"ok": true, "duplicate": true, "state": rec.State, "id": rec.ID})
		return
	case http.StatusConflict:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "decision is no longer pending", "state": rec.State, "revision": rec.Revision, "supersededBy": rec.SupersededBy})
		return
	case http.StatusInternalServerError:
		http.Error(w, "could not save the decision", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "state": rec.State, "id": rec.ID, "runId": rec.RunID})
}

// answerDecision resol una decisió pendent d'aquest usuari i fa continuar el
// torn (en directe o reprenent l'aparcat). code: -1 = resolta ara; 200 = ja
// ho estava amb la mateixa resposta; 404, 409, 500. També el fa servir la
// connexió d'un compte (B5) per reprendre les feines que l'esperaven.
func (s *Server) answerDecision(id string, ans windAnswer, revision int) (*windDecisionRec, int) {
	var rec *windDecisionRec
	var code int
	var resume, live, sweptNow bool
	_ = s.withDecisions(func(st *decisionDir) error {
		cur, err := st.load(id)
		if err != nil || cur.Owner != s.remindOwner() {
			code = http.StatusNotFound
			return nil
		}
		rec = cur
		if st.sweep(cur) {
			sweptNow = true
		}
		if cur.State != "pending" {
			if cur.Answer != nil && *cur.Answer == ans && (cur.State == "approved" || cur.State == "rejected") {
				code = http.StatusOK // la mateixa resposta: ja està feta
			} else {
				code = http.StatusConflict
			}
			return nil
		}
		if revision != 0 && revision != cur.Revision {
			code = http.StatusConflict
			return nil
		}
		cur.State = "approved"
		if (cur.Type == "approve" || cur.Type == "handoff" || cur.Type == "connection" || cur.Type == "secret") && !ans.Approve {
			cur.State = "rejected"
		}
		cur.Answer = &ans
		cur.ResolvedAt = time.Now().UnixMilli()
		if err := st.save(cur); err != nil {
			code = http.StatusInternalServerError
			return nil
		}
		windLiveAsks.Lock()
		ch := windLiveAsks.m[cur.ID]
		delete(windLiveAsks.m, cur.ID)
		windLiveAsks.Unlock()
		if ch != nil {
			ch <- ans
			live = true
		} else if cur.Resume != nil {
			resume = true
		}
		code = -1
		return nil
	})
	if sweptNow && rec != nil {
		s.afterDecisionChange(rec)
	}
	if code != -1 {
		return rec, code
	}
	s.emitDecision(rec)
	if rec.Type == "handoff" {
		// «Ja està» o «Cancel·la»: el control torna a l'agent abans que el
		// torn es reprengui i comprovi la sessió.
		s.releaseHandoffControl(rec)
	}
	if resume && !live {
		s.resumeWindDecision(rec)
	}
	return rec, code
}

// windAskArgsHint: una pista curta (camí, ordre, adreça) de la crida d'una eina,
// perquè l'aprovació digui de què va sense mostrar tots els arguments.
// windAskArgsFull és l'argument principal sencer (l'ordre de bash, el camí),
// amb un sostre de seguretat, per ensenyar-lo plegat a la targeta.
func windAskArgsFull(argsJSON string) string {
	var m map[string]any
	if json.Unmarshal([]byte(argsJSON), &m) != nil {
		return ""
	}
	for _, k := range []string{"command", "path", "file", "dir", "url", "target", "name", "action"} {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			v = strings.TrimSpace(v)
			if r := []rune(v); len(r) > 4000 {
				v = string(r[:4000]) + "…"
			}
			return v
		}
	}
	return ""
}

func windAskArgsHint(argsJSON string) string {
	var m map[string]any
	if json.Unmarshal([]byte(argsJSON), &m) != nil {
		return ""
	}
	for _, k := range []string{"command", "path", "file", "dir", "url", "target", "name", "action"} {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			v = strings.TrimSpace(v)
			if r := []rune(v); len(r) > 100 {
				v = string(r[:100]) + "…"
			}
			return v
		}
	}
	return ""
}
