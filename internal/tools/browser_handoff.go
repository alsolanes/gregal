package tools

// «Passa-m'ho»: l'agent necessita que l'usuari faci una cosa ell mateix al
// navegador (iniciar sessió, escriure una contrasenya o un codi, resoldre
// un CAPTCHA, triar un compte, acceptar galetes a la seva manera) i espera
// que digui «Fet». El Gregal no escriu mai credencials (ErrCredentialField):
// aquesta és la manera de continuar la feina sense que les vegi.
//
//   - app: el panell de l'app mostra l'avís amb «Fet» i «Cancel·la» (pont
//     /handoff, desktop/browser-bridge.js).
//   - window i chrome: el navegador és visible; s'hi posa una barra a dalt
//     de la pàgina (dins d'un shadow DOM tancat) i es torna a posar si la
//     pàgina navega mentre l'usuari hi treballa.
//   - headless: ningú no veu el navegador; l'eina ho diu i proposa
//     alternatives.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Temps que l'agent espera l'usuari: per defecte 10 minuts, màxim 30.
const (
	HandoffDefaultWait = 10 * time.Minute
	HandoffMaxWait     = 30 * time.Minute
)

// handoffWait és l'espera demanada (seconds), dins de marges.
func handoffWait(a browserArgs) time.Duration {
	s := a.num("seconds")
	if s <= 0 {
		return HandoffDefaultWait
	}
	d := time.Duration(s * float64(time.Second))
	if d < 15*time.Second {
		d = 15 * time.Second
	}
	if d > HandoffMaxWait {
		d = HandoffMaxWait
	}
	return d
}

// handoffMessage és el que llegeix l'usuari (una línia, curta).
func handoffMessage(a browserArgs) string {
	msg := strings.Join(strings.Fields(firstNonEmpty(a.str("text"), a.str("message"))), " ")
	if len([]rune(msg)) > 400 {
		msg = string([]rune(msg)[:399]) + "…"
	}
	return msg
}

// Handoff demana a l'usuari que faci msg i espera que acabi. Torna si l'ha
// fet (done) i el motiu si no («cancelled», «timeout»).
func (b *Browser) Handoff(msg string, wait time.Duration) (done bool, reason string, err error) {
	if msg == "" {
		return false, "", fmt.Errorf("browser: handoff needs text: what the user should do (e.g. \"Sign in to GitHub, then press Done\")")
	}
	ctx := b.ctx()
	b.mu.Lock()
	t, mode := b.t, b.mode
	b.mu.Unlock()
	if at, ok := t.(*appTransport); ok {
		return at.Handoff(ctx, msg, wait)
	}
	if mode == BrowserModeHeadless {
		return false, "", fmt.Errorf("browser: handoff needs a browser the user can see, and this one is headless. Ask the user in the chat instead, or retry with chrome:true (their own Chrome, already signed in) or with browser.mode: window")
	}
	return b.handoffOverlay(ctx, msg, wait)
}

// jsHandoffOverlay posa (o manté) la barra i torna l'estat: "waiting",
// "done" o "cancelled". Viu en un shadow DOM tancat: la pàgina no la pot
// llegir ni restilitzar.
const jsHandoffOverlay = `((msg, labels) => {
  const w = window;
  if (w.__gregalHandoff && w.__gregalHandoff.state !== 'waiting') return w.__gregalHandoff.state;
  if (w.__gregalHandoff && document.documentElement.contains(w.__gregalHandoff.host)) return 'waiting';
  const host = document.createElement('div');
  host.setAttribute('style', 'all:initial;position:fixed;top:0;left:0;right:0;z-index:2147483647');
  const root = host.attachShadow({mode: 'closed'});
  root.innerHTML = '<style>.b{display:flex;gap:12px;align-items:center;padding:10px 16px;background:#1f2440;color:#f3f5fa;font:14px system-ui,sans-serif;box-shadow:0 6px 24px #0006;border-bottom:2px solid #8193ff}.m{flex:1}.m b{color:#a3b0ff;margin-right:6px}button{font:600 13px system-ui,sans-serif;border-radius:8px;padding:7px 14px;cursor:pointer;border:1px solid #8193ff}.ok{background:#8193ff;color:#0e1015}.no{background:transparent;color:#f3f5fa;border-color:#ffffff44}</style>'
    + '<div class="b" role="alert"><span class="m"><b></b><span></span></span><button class="no" type="button"></button><button class="ok" type="button"></button></div>';
  root.querySelector('b').textContent = labels.title;
  root.querySelector('.m span').textContent = msg;
  const ok = root.querySelector('.ok'), no = root.querySelector('.no');
  ok.textContent = labels.done; no.textContent = labels.cancel;
  const end = state => { w.__gregalHandoff.state = state; host.remove(); };
  ok.onclick = () => end('done');
  no.onclick = () => end('cancelled');
  w.__gregalHandoff = {state: 'waiting', host};
  (document.body || document.documentElement).appendChild(host);
  return 'waiting';
})`

const jsHandoffClear = `(() => { const h = window.__gregalHandoff; if (h) { try { h.host.remove(); } catch (e) {} delete window.__gregalHandoff; } return true; })()`

// handoffOverlay fa el «passa-m'ho» al mateix navegador (finestra o Chrome
// de l'usuari): barra a la pàgina i consulta cada mig segon.
func (b *Browser) handoffOverlay(ctx context.Context, msg string, wait time.Duration) (bool, string, error) {
	labels, _ := json.Marshal(map[string]string{"title": "Gregal is waiting for you:", "done": "Done, continue", "cancel": "Cancel"})
	m, _ := json.Marshal(msg)
	js := jsHandoffOverlay + "(" + string(m) + "," + string(labels) + ")"
	deadline := time.Now().Add(wait)
	defer func() { _, _ = b.eval(jsHandoffClear) }()
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return false, "", fmt.Errorf("browser: handoff stopped: %v", ctx.Err())
		}
		// Durant una navegació l'avaluació pot fallar: es torna a provar.
		if st, err := b.evalString(js); err == nil && (st == "done" || st == "cancelled") {
			return st == "done", st, nil
		}
		select {
		case <-ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
	return false, "timeout", nil
}

// Handoff demana el «passa-m'ho» al panell de l'app i n'espera la resposta.
func (t *appTransport) Handoff(ctx context.Context, msg string, wait time.Duration) (bool, string, error) {
	out, err := t.post(ctx, "/handoff", map[string]any{"message": msg, "seconds": int(wait / time.Second)})
	if err != nil {
		if ctx.Err() != nil {
			return false, "", fmt.Errorf("browser: handoff stopped: %v", ctx.Err())
		}
		return false, "", err
	}
	var r struct {
		Done   bool   `json:"done"`
		Reason string `json:"reason"`
	}
	if json.Unmarshal(out, &r) != nil {
		return false, "", fmt.Errorf("browser: handoff: invalid bridge reply")
	}
	return r.Done, r.Reason, nil
}

// browserHandoff és l'acció: espera l'usuari i torna la pàgina tal com
// l'ha deixada (ha pogut navegar, iniciar sessió, canviar de pestanya…).
func browserHandoff(b *Browser, a browserArgs) (string, []string, error) {
	msg := handoffMessage(a)
	done, reason, err := b.Handoff(msg, handoffWait(a))
	if err != nil {
		return "", nil, err
	}
	var head string
	switch {
	case done:
		head = "The user says it is done: «" + msg + "». Check the page below before you continue; never ask for or read back what they typed."
	case reason == "timeout":
		head = "The user did not answer in time (handoff: «" + msg + "»). Ask them in the chat what to do; do not retry the same handoff straight away."
	default:
		head = "The user cancelled the handoff («" + msg + "»). Do not insist: tell them what you could not do, or find another way."
	}
	b.mu.Lock()
	b.ax = nil
	b.mu.Unlock()
	_ = b.esperaCarrega()
	b.instrument()
	page, rerr := b.read(a)
	if rerr != nil {
		return head + "\n\nNow on " + b.On(), nil, nil
	}
	return head + "\n\n" + page, nil, nil
}
