package web

// Handoff durable per iniciar sessió (paquet B3, pla v2 §6 «Camí B»).
//
// Quan un vent necessita que la persona faci una cosa ella mateixa al
// navegador de l'ordinador (iniciar sessió, una contrasenya, un codi 2FA),
// l'acció browser handoff ja no reté el torn esperant: es converteix en una
// decisió durable de tipus «handoff» (el mateix magatzem i el mateix punt
// segur de B4, winds_decisions.go). El torn s'aparca (waiting_user), el
// goroutine acaba i la feina sobreviu a reiniciar el servidor i a tancar
// l'app.
//
//	targeta «Et toca a tu» → «Obre l'ordinador» (visor + control, B3)
//	→ la persona inicia sessió al navegador de l'ordinador → «Ja està»
//	→ el control torna a l'agent → el torn es reprèn i comprova la sessió
//	  sense llegir cap camp (tools.BrowserLoginCheck) → l'encàrrec continua.
//
// Mentre espera, el model no corre i cap eina toca el navegador. Estats
// honestos: «Cancel·la» → rejected; sense resposta en windHandoffTTL →
// expired (timeout) i el run es cancel·la; el formulari encara a la vista →
// «no sembla completat» (p. ex. 2FA pendent); l'ordinador no respon → ho diu
// i no dona la sessió per bona.

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"gregal/internal/tools"
)

// windHandoffTTL és quant espera una sessió per iniciar: un dia.
var windHandoffTTL = 24 * time.Hour

// windLoginCheck comprova la sessió després del «ja està»; els tests el
// substitueixen (no hi ha cap navegador real a les proves).
var windLoginCheck = tools.BrowserLoginCheck

// windPageURL és la pàgina on és el navegador de la sessió (tests).
var windPageURL = tools.BrowserPageURL

// windHandoffOutcome és com ha acabat l'espera d'un handoff.
type windHandoffOutcome struct {
	done   bool
	reason string // cancelled | timeout
	msg    string
	page   string
}

// isBrowserHandoff diu si la crida browser és un «passa-m'ho».
func isBrowserHandoff(argsJSON string) bool {
	var a struct {
		Action string `json:"action"`
	}
	if json.Unmarshal([]byte(argsJSON), &a) != nil {
		return false
	}
	act := strings.ToLower(strings.TrimSpace(a.Action))
	return act == "handoff" || act == "ask_user"
}

func handoffArgsMessage(argsJSON string) string {
	var a struct {
		Text    string `json:"text"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &a)
	msg := strings.Join(strings.Fields(firstNonEmpty(a.Text, a.Message)), " ")
	if r := []rune(msg); len(r) > 400 {
		msg = string(r[:399]) + "…"
	}
	return msg
}

// windHandoffCall publica el handoff com a decisió durable i n'espera la
// resposta (en directe uns minuts; després, aparcat). parked diu que el torn
// s'ha aparcat i continuarà quan la persona digui «ja està».
func (s *Server) windHandoffCall(ctx context.Context, emit func(kind, text string), argsJSON string) (windHandoffOutcome, bool) {
	msg := handoffArgsMessage(argsJSON)
	if msg == "" {
		msg = s.tr("Inicia sessió al navegador de l'ordinador", "Sign in on the computer's browser")
	}
	page := tools.SafePageURL(windPageURL(s.id))
	site := ""
	if u, err := url.Parse(page); err == nil {
		site = u.Host
	}
	req := &windAskReq{
		Type: "handoff", Name: "browser", Kind: "login", What: msg, Page: page, Site: site,
		Reason: s.tr("Gregal no escriu mai contrasenyes ni codis: ho fas tu al visor de l'ordinador.",
			"Gregal never types passwords or codes: you do it in the computer's viewer."),
	}
	ans, out := s.windAskWait(ctx, emit, req, argsJSON)
	res := windHandoffOutcome{msg: msg, page: req.Page}
	switch out {
	case askParked:
		return res, true
	case askNone:
		res.reason = "timeout"
		return res, false
	}
	if res.page == "" {
		// Represa: la pàgina és la que es va desar amb la decisió.
		if r := windResumeOf(ctx); r != nil {
			_ = s.withDecisions(func(st *decisionDir) error {
				if rec, err := st.load(r.id); err == nil {
					res.page = rec.Page
					if rec.What != "" {
						res.msg = rec.What
					}
				}
				return nil
			})
		}
	}
	res.done = ans.Approve
	if !res.done {
		res.reason = "cancelled"
	}
	return res, false
}

// windHandoffText és el resultat de l'eina per al model: mai cap valor de
// camp, només si la sessió sembla iniciada.
func (s *Server) windHandoffText(ctx context.Context, res windHandoffOutcome) string {
	switch {
	case !res.done && res.reason == "timeout":
		return "The person did not answer in time (handoff: «" + res.msg + "»). The sign-in was not done. Say so in your reply; do not retry the same handoff straight away."
	case !res.done:
		return "The person cancelled the handoff («" + res.msg + "»). Do not insist: tell them what you could not do without it, or find another way."
	}
	lc, err := windLoginCheck(ctx, s.id, res.page)
	if err != nil {
		return "The person says it is done («" + res.msg + "»), but the computer's browser could not be checked (computer or network unavailable). Do not assume the sign-in worked: tell them, and carry on with what does not need it."
	}
	where := lc.Page
	if lc.Title != "" {
		where += " «" + lc.Title + "»"
	}
	head := "The person says it is done («" + res.msg + "»). Checked without reading any field: the browser is on " + where + ". "
	switch {
	case lc.CodeField:
		return head + "A one-time code field is still visible: the sign-in is not complete (two-step verification pending). Do not type anything; tell them, or use handoff again so they can enter the code."
	case lc.PasswordField:
		return head + "A password field is still visible: the sign-in does not look complete. Do not type anything; tell them, or use handoff again."
	}
	// La sessió queda com a connexió (B5): una segona feina al mateix lloc la
	// reutilitza sense tornar-la a demanar.
	s.recordBrowserSession(lc.Page)
	return head + "No sign-in or code form is visible, so the session looks active. Continue the original task with the browser; never ask for or read back what they typed."
}

// handoffReachable diu si la persona pot fer el handoff des d'algun lloc:
// té un ordinador amb visor (registre d'ordinadors: VM, contenidor o el que
// hi hagi darrere de l'agent), aquest servidor té un visor local, o el
// navegador és visible en aquesta màquina. Si no, el handoff durable no es
// crea (ningú no el podria fer) i l'eina diu clarament que no hi ha cap
// navegador que la persona pugui veure.
func (s *Server) handoffReachable() bool {
	if t, _, err := ordinadorTargetFor(s.remindOwner(), ""); err == nil && (t.c != nil || localVisorAddr() != "") {
		return true
	}
	return windBrowserVisible()
}

// windBrowserVisible: el navegador d'aquesta màquina es veu (finestra,
// Chrome de la persona o el panell de l'app). Variable per als tests.
var windBrowserVisible = func() bool { return tools.EffectiveBrowserMode() != tools.BrowserModeHeadless }

// releaseHandoffControl torna el control a l'agent quan la persona respon el
// handoff (ja està o cancel·la).
func (s *Server) releaseHandoffControl(rec *windDecisionRec) {
	key := tools.OwnerKeyFor(rec.Owner)
	l := s.ordinadorLeases()
	if x := l.Get(key); x != nil && (x.Handoff == "" || x.Handoff == rec.ID) {
		l.Release(key)
	}
}

// ownsPendingHandoff diu si id és un handoff pendent d'aquest usuari.
func (s *Server) ownsPendingHandoff(id string) bool {
	ok := false
	_ = s.withDecisions(func(st *decisionDir) error {
		rec, err := st.load(strings.TrimSpace(id))
		ok = err == nil && rec.Type == "handoff" && rec.State == "pending" && rec.Owner == s.remindOwner()
		return nil
	})
	return ok
}
