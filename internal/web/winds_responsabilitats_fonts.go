package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/internal/events"
	"gregal/internal/jobs"
	"gregal/internal/ordinador"
	"gregal/internal/runs"
	"gregal/internal/tools"
)

// Fonts de les responsabilitats (B6). Cada font sap fer una comprovació
// determinista a partir del seu cursor anterior: cap model, cap avís. Només
// quan torna Changed hi ha informació nova per preparar.

// Source és una font vigilada. Check torna el cursor nou (sempre, encara que
// no hagi canviat res) i, si hi ha canvi, un extracte del que és nou. La
// primera lectura (cursor buit) és la línia de base: no és un canvi.
type Source interface {
	Check(ctx context.Context, prev respCursor) (respObservation, error)
}

type respObservation struct {
	Cursor  respCursor
	Changed bool
	// Excerpt és el que és nou, en text pla. És dada d'una font externa:
	// entra a la preparació dins d'un bloc marcat, mai com a instrucció.
	Excerpt string
}

// errRespNeedComputer: la font necessita l'ordinador personal i s'ha demanat
// d'encendre'l. La comprovació s'aparca a waiting_computer.
var errRespNeedComputer = errors.New("cal l'ordinador personal: s'està encenent")

// respConnectionSource és el punt on s'endollen les connexions de B5: quan
// B5 exposi la llista de connexions d'un compte, aquí es construeix la font
// (p. ex. correu o calendari connectat) a partir de src.Ref. Sense B5, una
// font «connection» diu clarament que encara no està disponible.
var respConnectionSource func(s *Server, owner string, src respSource) (Source, error)

func (s *Server) respSourceFor(r *responsabilitat, src respSource) (Source, error) {
	switch src.Kind {
	case "web":
		return respWebSource{url: src.URL}, nil
	case "agenda":
		return respAgendaSource{s: s}, nil
	case "page":
		return respPageSource{s: s, url: src.URL, owner: respOwner(s), reason: "task:" + r.ID}, nil
	case "connection":
		if respConnectionSource == nil {
			return nil, fmt.Errorf("les connexions encara no estan disponibles en aquest servidor")
		}
		return respConnectionSource(s, respOwner(s), src)
	}
	return nil, fmt.Errorf("font no comprovable: %s", src.Kind)
}

// --- diferències de text ---

// respTextObservation compara el text nou amb la instantània anterior. Amb
// onlyAdded, només les línies noves compten com a canvi (l'agenda: el que
// surt de la finestra pel pas del temps no és notícia).
func respTextObservation(prev respCursor, text string, onlyAdded bool) respObservation {
	text = strings.TrimSpace(text)
	c := prev
	c.Hash = respHash(text)
	c.Snapshot = trimRunes(text, respSnapshot)
	if prev.Hash == "" {
		return respObservation{Cursor: c}
	}
	if prev.Hash == c.Hash {
		return respObservation{Cursor: c}
	}
	added, removed := respLineDiff(prev.Snapshot, text)
	if onlyAdded && len(added) == 0 {
		return respObservation{Cursor: c}
	}
	var b strings.Builder
	if len(added) > 0 {
		b.WriteString("New or changed:\n" + strings.Join(added, "\n"))
	}
	if len(removed) > 0 && !onlyAdded {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("No longer there:\n" + strings.Join(removed, "\n"))
	}
	if b.Len() == 0 {
		// Mateixes línies en un altre ordre: no és informació nova.
		return respObservation{Cursor: c}
	}
	return respObservation{Cursor: c, Changed: true, Excerpt: trimRunes(b.String(), respMaxExcerpt)}
}

func respLineDiff(oldText, newText string) (added, removed []string) {
	set := func(t string) map[string]bool {
		m := map[string]bool{}
		for _, l := range strings.Split(t, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				m[l] = true
			}
		}
		return m
	}
	o, n := set(oldText), set(newText)
	for _, l := range strings.Split(newText, "\n") {
		if l = strings.TrimSpace(l); l != "" && !o[l] && !containsStr(added, l) {
			added = append(added, l)
		}
	}
	for _, l := range strings.Split(oldText, "\n") {
		if l = strings.TrimSpace(l); l != "" && !n[l] && !containsStr(removed, l) {
			removed = append(removed, l)
		}
	}
	return added, removed
}

// --- web pública ---

// respAllowLocal deixa llegir hosts locals (només als tests).
var respAllowLocal = false

const respWebMax = 2 << 20

func respCheckURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("cal una URL http(s) completa")
	}
	if u.User != nil {
		return fmt.Errorf("la URL no pot portar credencials")
	}
	if !respAllowLocal && tools.WebBlockedHost(u.Host) {
		return fmt.Errorf("host local o reservat: no es pot vigilar")
	}
	return nil
}

var respHTTP = &http.Client{
	Timeout: 20 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("massa redireccions")
		}
		return respCheckURL(req.URL.String())
	},
}

type respWebSource struct{ url string }

func (src respWebSource) Check(ctx context.Context, prev respCursor) (respObservation, error) {
	if err := respCheckURL(src.url); err != nil {
		return respObservation{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.url, nil)
	if err != nil {
		return respObservation{}, err
	}
	req.Header.Set("User-Agent", "Gregal/1 (+responsabilitats)")
	req.Header.Set("Accept", "text/html,text/plain,application/json;q=0.9,*/*;q=0.5")
	// Petició condicional: si el servidor diu 304, ni tan sols es baixa.
	if prev.Hash != "" {
		if prev.ETag != "" {
			req.Header.Set("If-None-Match", prev.ETag)
		}
		if prev.LastModified != "" {
			req.Header.Set("If-Modified-Since", prev.LastModified)
		}
	}
	resp, err := respHTTP.Do(req)
	if err != nil {
		return respObservation{}, fmt.Errorf("no s'ha pogut llegir: %v", errors.Unwrap(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified && prev.Hash != "" {
		return respObservation{Cursor: prev}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return respObservation{}, fmt.Errorf("la pàgina ha respost %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, respWebMax))
	if err != nil {
		return respObservation{}, err
	}
	text := string(body)
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "html") || (!strings.Contains(ct, "json") && strings.Contains(strings.ToLower(text[:min(len(text), 512)]), "<html")) {
		text = tools.HTMLToText(text)
	}
	o := respTextObservation(prev, text, false)
	o.Cursor.ETag = resp.Header.Get("ETag")
	o.Cursor.LastModified = resp.Header.Get("Last-Modified")
	return o, nil
}

// --- agenda: recordatoris i calendari del compte ---

type respAgendaSource struct{ s *Server }

const respAgendaAhead = 7 * 24 * time.Hour

func (src respAgendaSource) Check(ctx context.Context, prev respCursor) (respObservation, error) {
	s := src.s
	var lines []string
	if s.jobs != nil {
		list, err := s.jobs.List()
		if err == nil {
			for _, j := range list {
				if !strings.HasPrefix(j.ID, remindJobPrefix) || !j.Enabled || j.NextAt == nil || !s.respOwnsJob(j) {
					continue
				}
				lines = append(lines, j.NextAt.Local().Format("2006-01-02 15:04")+" · reminder · "+strings.TrimSpace(firstNonEmpty(j.Prompt, j.Task)))
			}
		}
	}
	now := time.Now()
	evs, problems := s.calendarEvents(ctx, now, now.Add(respAgendaAhead))
	for _, e := range evs {
		l := e.Start.Format("2006-01-02 15:04") + " · calendar · " + strings.TrimSpace(e.Summary)
		if loc := strings.TrimSpace(e.Location); loc != "" {
			l += " · " + loc
		}
		lines = append(lines, l)
	}
	if len(lines) == 0 && len(problems) > 0 {
		return respObservation{}, fmt.Errorf("calendari: %s", problems[0])
	}
	sort.Strings(lines)
	return respTextObservation(prev, strings.Join(lines, "\n"), true), nil
}

// respOwnsJob: el recordatori és d'un vent d'aquest compte.
func (s *Server) respOwnsJob(j jobs.Job) bool {
	if j.Wind != "" || j.WindOwner != "" {
		return j.WindOwner == respOwner(s)
	}
	wid, ok := remindJobWind(j.ID)
	if !ok || s.winds == nil {
		return false
	}
	_, mine := s.winds.ByID(wid)
	return mine
}

// --- pàgina llegida amb el navegador de l'ordinador personal ---

// respComputerDriver és la part de l'ordinador personal que fa servir B6.
// Els tests hi posen un doble.
type respComputerDriver interface {
	Health(ctx context.Context, owner string) (string, error)
	Wake(ctx context.Context, owner, reason string) error
	Exec(ctx context.Context, owner string, req ordinador.ExecRequest) (ordinador.ExecResponse, error)
}

// respComputer torna l'ordinador de l'amfitrió (nil sense GREGAL_ORDINADORS).
var respComputer = func() respComputerDriver {
	m, _ := ordinadorHost()
	if m == nil {
		return nil
	}
	return hostComputer{m}
}

type hostComputer struct{ m *ordinador.Manager }

func (h hostComputer) Health(ctx context.Context, owner string) (string, error) {
	c, err := h.m.For(owner)
	if err != nil {
		return "", err
	}
	return h.m.Status(ctx, c).Health, nil
}

func (h hostComputer) Wake(ctx context.Context, owner, reason string) error {
	c, err := h.m.For(owner)
	if err != nil {
		return err
	}
	return h.m.Wake(ctx, c, reason)
}

func (h hostComputer) Exec(ctx context.Context, owner string, req ordinador.ExecRequest) (ordinador.ExecResponse, error) {
	return h.m.Exec(ctx, owner, req)
}

type respPageSource struct {
	s      *Server
	url    string
	owner  string
	reason string
}

func (src respPageSource) Check(ctx context.Context, prev respCursor) (respObservation, error) {
	drv := respComputer()
	if drv == nil {
		return respObservation{}, fmt.Errorf("aquest servidor no té ordinador personal: aquesta font no es pot llegir")
	}
	health, err := drv.Health(ctx, src.owner)
	if err != nil {
		return respObservation{}, fmt.Errorf("no tens ordinador personal: %v", err)
	}
	switch health {
	case "running":
	case "starting":
		return respObservation{}, errRespNeedComputer
	default:
		// Desat o aturat: s'encén per a aquesta feina i la comprovació espera.
		if err := drv.Wake(ctx, src.owner, src.reason); err != nil {
			return respObservation{}, fmt.Errorf("l'ordinador no s'ha pogut encendre: %v", err)
		}
		return respObservation{}, errRespNeedComputer
	}
	session := "resp-" + respHash(src.owner, src.url)
	open, _ := json.Marshal(map[string]any{"action": "open", "url": src.url})
	if _, err := drv.Exec(ctx, src.owner, ordinador.ExecRequest{Session: session, Tool: "browser", Args: string(open)}); err != nil {
		return respObservation{}, fmt.Errorf("el navegador de l'ordinador no ha obert la pàgina: %v", err)
	}
	read, _ := json.Marshal(map[string]any{"action": "text"})
	out, err := drv.Exec(ctx, src.owner, ordinador.ExecRequest{Session: session, Tool: "browser", Args: string(read)})
	if err != nil {
		return respObservation{}, fmt.Errorf("el navegador de l'ordinador no ha llegit la pàgina: %v", err)
	}
	return respTextObservation(prev, out.Output, false), nil
}

// --- waiting_computer: la comprovació aparcada fins que l'ordinador respon ---

// Una comprovació que necessita l'ordinador desat no bloqueja cap goroutine:
// el run queda a waiting_computer i un sol vigilant per procés revisa les
// esperes cada respComputerPoll. Quan l'ordinador diu running, el run es
// reprèn amb el mateix ID (torna a comprovar les fonts: els cursors ja
// desats fan que sigui idempotent). Si passa el termini sense arrencar, el
// run acaba fallit amb un motiu clar i la responsabilitat ho apunta.

var (
	respComputerPoll    = 10 * time.Second
	respComputerTimeout = 5 * time.Minute
)

type computerWaiter struct {
	s        *Server
	runID    int64
	respID   string
	owner    string
	jobID    string
	deadline time.Time
}

var computerWaits = struct {
	sync.Mutex
	m       map[int64]*computerWaiter
	started bool
}{m: map[int64]*computerWaiter{}}

// forgetComputerWait treu una espera (cancel·lada o pausada).
func forgetComputerWait(runID int64) {
	computerWaits.Lock()
	delete(computerWaits.m, runID)
	computerWaits.Unlock()
}

func (s *Server) respParkForComputer(r *responsabilitat, queued runs.Run) {
	now := time.Now()
	deadline := now.Add(respComputerTimeout)
	_, _ = s.respUpdate(r.ID, func(cur *responsabilitat) error {
		cur.Waiting = &respWait{RunID: queued.ID, Since: respStamp(now), Deadline: respStamp(deadline)}
		return nil
	})
	computerWaits.Lock()
	computerWaits.m[queued.ID] = &computerWaiter{s: s, runID: queued.ID, respID: r.ID, owner: respOwner(s), jobID: r.ID, deadline: deadline}
	start := !computerWaits.started
	computerWaits.started = true
	computerWaits.Unlock()
	if start {
		go func() {
			t := time.NewTicker(respComputerPoll)
			defer t.Stop()
			for range t.C {
				respComputerTick(time.Now())
			}
		}()
	}
}

// respComputerTick revisa les esperes: reprèn les que ja tenen ordinador i
// tanca les que han passat el termini. Els tests el criden directament.
func respComputerTick(now time.Time) {
	computerWaits.Lock()
	list := make([]*computerWaiter, 0, len(computerWaits.m))
	for _, w := range computerWaits.m {
		list = append(list, w)
	}
	computerWaits.Unlock()
	drv := respComputer()
	for _, w := range list {
		q := w.s.Hub().Queue()
		if run, ok := q.Get(w.runID); !ok || run.State != runs.WaitingComputer {
			forgetComputerWait(w.runID)
			continue
		}
		health := ""
		if drv != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			health, _ = drv.Health(ctx, w.owner)
			cancel()
		}
		switch {
		case health == "running":
			forgetComputerWait(w.runID)
			w.s.respResumeComputer(w)
		case !now.Before(w.deadline):
			forgetComputerWait(w.runID)
			w.s.respComputerGaveUp(w, health)
		}
	}
}

func (s *Server) respResumeComputer(w *computerWaiter) {
	job, ok := s.respFindJobSafe(w.jobID)
	if !ok {
		s.Hub().Queue().Cancel(w.runID)
		return
	}
	exec := func(ctx context.Context, run runs.Run) error {
		s.appendRunEvent(run, "run_started", "l'ordinador ja respon: es reprèn la comprovació")
		_, err := s.executeResponsibility(ctx, job, run)
		return err
	}
	run, ok := s.Hub().Queue().Resume(w.runID, exec)
	if !ok {
		return
	}
	s.appendRunEvent(run, "run_queued", "comprovació represa")
	s.watchRun(run)
}

func (s *Server) respComputerGaveUp(w *computerWaiter, health string) {
	why := "l'ordinador no ha arrencat a temps"
	if health != "" {
		why += " (estat: " + health + ")"
	}
	if s.Hub().Queue().Fail(w.runID, why) {
		_ = s.Hub().appendEvent(events.Event{User: s.usuari(), RunID: int(w.runID), Kind: "run_failed", Text: "torn fallit: " + why})
	}
	_, _ = s.respUpdate(w.respID, func(cur *responsabilitat) error {
		cur.Waiting = nil
		cur.addLog(respAct{At: respStamp(time.Now()), Kind: "computer_unavailable", RunID: w.runID,
			Note: why + "; es tornarà a provar a la propera comprovació"})
		return nil
	})
}
