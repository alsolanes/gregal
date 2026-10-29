package tools

// Accions de l'eina browser i el lot (batch). Cada acció que toca la pàgina
// torna què ha fet i, al final, què ha canviat (AXChanges): el model no
// necessita una lectura sencera després de cada clic. En un lot, el diff es
// fa un sol cop al final.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// jsInstrument s'injecta a cada document (i al que ja hi ha): guarda la
// consola, els errors, les peticions fetch/XHR i els diàlegs. Els diàlegs
// (alert/confirm/prompt) bloquegen el JavaScript i, amb ell, el CDP: es
// responen sols (confirm = cancel·lar) i l'agent ho veu i pot repetir amb
// dialog accept:true.
const jsInstrument = `(() => {
  if (window.__gregal) return;
  const g = window.__gregal = {log: [], net: [], dialogs: [], answer: null, seenLog: 0, seenDlg: 0};
  const push = (a, x) => { a.push(x); if (a.length > 300) a.shift(); };
  const str = v => { try { return typeof v === 'string' ? v : JSON.stringify(v); } catch (e) { return String(v); } };
  for (const lv of ['log', 'info', 'warn', 'error', 'debug']) {
    const o = console[lv];
    console[lv] = function (...a) { try { push(g.log, {lv, m: a.map(str).join(' ').slice(0, 500)}); } catch (e) {} return o.apply(this, a); };
  }
  addEventListener('error', e => push(g.log, {lv: 'error', m: String(e.message || e) + (e.filename ? ' @ ' + e.filename + ':' + e.lineno : '')}));
  addEventListener('unhandledrejection', e => push(g.log, {lv: 'error', m: 'unhandled rejection: ' + str(e.reason && e.reason.message || e.reason)}));
  const of = window.fetch;
  if (of) window.fetch = function (i, o) {
    const t0 = Date.now(), u = String(i && i.url || i), m = (o && o.method) || (i && i.method) || 'GET';
    return of.apply(this, arguments).then(r => { push(g.net, {m, u, s: r.status, ms: Date.now() - t0}); return r; },
      e => { push(g.net, {m, u, s: 0, err: String(e), ms: Date.now() - t0}); throw e; });
  };
  const X = XMLHttpRequest.prototype, oo = X.open, os = X.send;
  X.open = function (m, u) { this.__g = {m, u: String(u)}; return oo.apply(this, arguments); };
  X.send = function () { const t0 = Date.now(), me = this; this.addEventListener('loadend', () => { if (me.__g) push(g.net, {m: me.__g.m, u: me.__g.u, s: me.status, ms: Date.now() - t0}); }); return os.apply(this, arguments); };
  const ans = (kind, msg, def) => { const a = g.answer; g.answer = null; const v = a !== null ? a : def; push(g.dialogs, {kind, msg: String(msg).slice(0, 300), v: str(v)}); return v; };
  window.alert = function (m) { ans('alert', m, undefined); };
  window.confirm = function (m) { return !!ans('confirm', m, false); };
  window.prompt = function (m, d) { const r = ans('prompt', m, null); return r === true ? (d == null ? '' : d) : r; };
})()`

// instrument posa jsInstrument a la pàgina actual i a les que vindran.
func (b *Browser) instrument() {
	_, _ = b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": jsInstrument})
	_, _ = b.eval(jsInstrument)
}

// pageNews és el que la pàgina ha dit des de l'última acció: diàlegs i
// errors de consola nous. Curt: va enganxat a cada acció.
func (b *Browser) pageNews() string {
	v, err := b.evalString(`(() => { const g = window.__gregal; if (!g) return '';
	  const d = g.dialogs.slice(g.seenDlg); g.seenDlg = g.dialogs.length;
	  const e = g.log.filter(x => x.lv === 'error').slice(-50); const fresh = g.log.length - g.seenLog; g.seenLog = g.log.length;
	  const errs = fresh > 0 ? g.log.slice(-fresh).filter(x => x.lv === 'error') : [];
	  return JSON.stringify({d, errs: errs.slice(-3).map(x => x.m), n: errs.length}); })()`)
	if err != nil || v == "" {
		return ""
	}
	var news struct {
		D []struct {
			Kind string `json:"kind"`
			Msg  string `json:"msg"`
			V    string `json:"v"`
		} `json:"d"`
		Errs []string `json:"errs"`
		N    int      `json:"n"`
	}
	if json.Unmarshal([]byte(v), &news) != nil {
		return ""
	}
	var out []string
	for _, d := range news.D {
		line := fmt.Sprintf("A %s dialog appeared: %q", d.Kind, d.Msg)
		if d.Kind == "confirm" && d.V == "false" {
			line += " → it was CANCELLED. If it must be accepted: first dialog accept:true, then repeat the action (one call: batch [{action:dialog, accept:true}, {action:click, target:…}])."
		} else if d.Kind == "confirm" && d.V == "true" {
			line += " → accepted (it confirms the action the user approved)."
		} else if d.Kind == "prompt" && d.V == "null" {
			line += " → it was dismissed. To answer it, call dialog text:\"…\" and repeat the action."
		}
		out = append(out, line)
	}
	if news.N > 0 {
		out = append(out, fmt.Sprintf("%d new console error(s): %s", news.N, strings.Join(news.Errs, " · ")))
	}
	return strings.Join(out, "\n")
}

// Console torna els últims missatges de consola.
func (b *Browser) Console(errorsOnly bool) (string, error) {
	v, err := b.evalString(`JSON.stringify((window.__gregal || {log: []}).log.slice(-60))`)
	if err != nil {
		return "", err
	}
	var logs []struct {
		Lv string `json:"lv"`
		M  string `json:"m"`
	}
	_ = json.Unmarshal([]byte(v), &logs)
	var out strings.Builder
	n := 0
	for _, l := range logs {
		if errorsOnly && l.Lv != "error" && l.Lv != "warn" {
			continue
		}
		fmt.Fprintf(&out, "[%s] %s\n", l.Lv, l.M)
		n++
	}
	if n == 0 {
		return "No console messages since Gregal attached to this page.", nil
	}
	return truncate(out.String(), MaxOutputChars), nil
}

// Network torna les peticions de la pàgina: fetch/XHR registrades i els
// recursos que han fallat (Resource Timing).
func (b *Browser) Network(failedOnly bool) (string, error) {
	v, err := b.evalString(`JSON.stringify({net: (window.__gregal || {net: []}).net.slice(-80),
	  res: performance.getEntriesByType('resource').filter(r => (r.responseStatus || 0) >= 400).slice(-30).map(r => ({u: r.name, s: r.responseStatus, t: r.initiatorType}))})`)
	if err != nil {
		return "", err
	}
	var n struct {
		Net []struct {
			M   string `json:"m"`
			U   string `json:"u"`
			S   int    `json:"s"`
			Ms  int    `json:"ms"`
			Err string `json:"err"`
		} `json:"net"`
		Res []struct {
			U string `json:"u"`
			S int    `json:"s"`
			T string `json:"t"`
		} `json:"res"`
	}
	_ = json.Unmarshal([]byte(v), &n)
	var out strings.Builder
	for _, r := range n.Net {
		if failedOnly && r.S > 0 && r.S < 400 {
			continue
		}
		st := strconv.Itoa(r.S)
		if r.S == 0 {
			st = "failed " + r.Err
		}
		fmt.Fprintf(&out, "%s %s → %s (%d ms)\n", r.M, truncateInline(r.U, 160), st, r.Ms)
	}
	for _, r := range n.Res {
		fmt.Fprintf(&out, "%s %s → %d\n", r.T, truncateInline(r.U, 160), r.S)
	}
	if out.Len() == 0 {
		return "No requests recorded (fetch/XHR since Gregal attached, failed resources).", nil
	}
	return truncate(out.String(), MaxOutputChars), nil
}

// Dialog prepara la resposta del proper diàleg (accept, o el text d'un prompt).
func (b *Browser) Dialog(accept bool, text string) (string, error) {
	val := "false"
	switch {
	case text != "":
		val = jsonStr(text)
	case accept:
		val = "true"
	}
	if _, err := b.eval(`(() => { if (window.__gregal) window.__gregal.answer = ` + val + `; return true; })()`); err != nil {
		return "", err
	}
	// Un diàleg natiu obert abans d'instrumentar bloqueja la pàgina: es respon.
	_, _ = b.call("Page.handleJavaScriptDialog", map[string]any{"accept": accept || text != "", "promptText": text})
	return "the next dialog will be answered: " + map[bool]string{true: "accept", false: "cancel"}[accept || text != ""] + ". Now repeat the action that opens it.", nil
}

// Wait espera que aparegui un text, que en desaparegui un altre, un selector
// o, sense res, que la pàgina deixi de canviar.
func (b *Browser) Wait(text, gone, selector string, secs float64) (string, error) {
	ms := int(secs * 1000)
	if ms <= 0 || ms > 30000 {
		ms = 10000
	}
	if text == "" && gone == "" && selector == "" {
		if secs > 0 {
			time.Sleep(time.Duration(ms) * time.Millisecond)
			return fmt.Sprintf("waited %.1f s", float64(ms)/1000), nil
		}
		v, err := b.evalString(fmt.Sprintf(`new Promise(res => { const t0 = Date.now(); let last = Date.now();
		  const mo = new MutationObserver(() => { last = Date.now(); }); mo.observe(document, {subtree: true, childList: true, attributes: true, characterData: true});
		  const f = () => { if (document.readyState === 'complete' && Date.now() - last > 500) { mo.disconnect(); return res('quiet:' + (Date.now() - t0)); }
		    if (Date.now() - t0 > %d) { mo.disconnect(); return res('timeout'); } setTimeout(f, 100); }; f(); })`, ms))
		if err != nil {
			return "", err
		}
		if v == "timeout" {
			return "the page kept changing; continuing anyway", nil
		}
		return "page settled after " + strings.TrimPrefix(v, "quiet:") + " ms", nil
	}
	v, err := b.evalString(fmt.Sprintf(`new Promise(res => { const T = %s, G = %s, S = %s, t0 = Date.now();
	  const f = () => { const body = document.body ? document.body.innerText : ''; let ok = true;
	    if (T && !body.includes(T)) ok = false; if (G && body.includes(G)) ok = false;
	    if (S) { try { if (!document.querySelector(S)) ok = false; } catch (e) { ok = false; } }
	    if (ok) return res('ok:' + (Date.now() - t0)); if (Date.now() - t0 > %d) return res('timeout'); setTimeout(f, 150); }; f(); })`,
		jsonStr(text), jsonStr(gone), jsonStr(selector), ms))
	if err != nil {
		return "", err
	}
	if v == "timeout" {
		return "", fmt.Errorf("browser: waited %d s and the condition did not happen", ms/1000)
	}
	return "condition met after " + strings.TrimPrefix(v, "ok:") + " ms", nil
}

// Scroll fa rodar la roda del ratolí a sobre d'un punt (o al centre).
func (b *Browser) Scroll(x, y float64, direction string, amount int) (string, error) {
	if amount <= 0 {
		amount = 3
	}
	if x == 0 && y == 0 {
		x, y = 400, 300
		if raw, err := b.call("Page.getLayoutMetrics", nil); err == nil {
			var m struct {
				V struct {
					W float64 `json:"clientWidth"`
					H float64 `json:"clientHeight"`
				} `json:"cssLayoutViewport"`
			}
			if json.Unmarshal(raw, &m) == nil && m.V.W > 0 {
				x, y = m.V.W/2, m.V.H/2
			}
		}
	}
	dx, dy := 0.0, 0.0
	step := float64(amount) * 120
	switch direction {
	case "up":
		dy = -step
	case "left":
		dx = -step
	case "right":
		dx = step
	default:
		direction = "down"
		dy = step
	}
	if _, err := b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseWheel", "x": x, "y": y, "deltaX": dx, "deltaY": dy}); err != nil {
		return "", err
	}
	time.Sleep(350 * time.Millisecond)
	return fmt.Sprintf("scrolled %s %d", direction, amount), nil
}

// ClickXY clica en coordenades d'una captura (per a llenços: Sheets, mapes,
// jocs). Les coordenades són píxels de la captura: es passen a CSS.
func (b *Browser) ClickXY(x, y float64, button string, count int) (string, error) {
	b.mu.Lock()
	scale := b.shotScale
	b.mu.Unlock()
	if scale <= 0 {
		scale = 1
	}
	if err := b.mouse(x/scale, y/scale, button, count, 0); err != nil {
		return "", err
	}
	return fmt.Sprintf("clicked at (%.0f, %.0f)", x, y), nil
}

// ScreenshotRegion fa la captura (o un tros ampliat) i recorda l'escala
// perquè click_xy pugui fer servir les coordenades de la imatge.
func (b *Browser) ScreenshotRegion(region []float64) (string, string, error) {
	dpr := 1.0
	if v, err := b.evalString("String(window.devicePixelRatio || 1)"); err == nil {
		if f, e := strconv.ParseFloat(v, 64); e == nil && f > 0 {
			dpr = f
		}
	}
	params := map[string]any{"format": "jpeg", "quality": 70}
	note := ""
	if len(region) == 4 && region[2] > region[0] && region[3] > region[1] {
		params["clip"] = map[string]any{"x": region[0] / dpr, "y": region[1] / dpr, "width": (region[2] - region[0]) / dpr, "height": (region[3] - region[1]) / dpr, "scale": 2}
		note = " (zoomed region; click_xy still uses full-screenshot coordinates)"
	} else {
		b.mu.Lock()
		b.shotScale = dpr
		b.mu.Unlock()
	}
	res, err := b.call("Page.captureScreenshot", params)
	if err != nil {
		return "", "", err
	}
	var r struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(res, &r); err != nil || r.Data == "" {
		return "", "", fmt.Errorf("browser: captura buida")
	}
	if len(r.Data)*3/4 > MaxImageBytes {
		return "", "", fmt.Errorf("browser: captura massa gran")
	}
	return "data:image/jpeg;base64," + r.Data, note, nil
}

// ---- tecles ----

var keyNames = map[string]struct {
	code string
	vk   int
	text string
}{
	"Enter": {"Enter", 13, "\r"}, "Tab": {"Tab", 9, "\t"}, "Escape": {"Escape", 27, ""},
	"Backspace": {"Backspace", 8, ""}, "Delete": {"Delete", 46, ""}, "Space": {"Space", 32, " "},
	"ArrowDown": {"ArrowDown", 40, ""}, "ArrowUp": {"ArrowUp", 38, ""}, "ArrowLeft": {"ArrowLeft", 37, ""},
	"ArrowRight": {"ArrowRight", 39, ""}, "PageDown": {"PageDown", 34, ""}, "PageUp": {"PageUp", 33, ""},
	"Home": {"Home", 36, ""}, "End": {"End", 35, ""},
}

// pressCombo prem una tecla o combinació: «Enter», «Control+a», «Shift+Tab»,
// «Meta+Enter», «F5», «x».
func (b *Browser) pressCombo(combo string) error {
	parts := strings.Split(combo, "+")
	if strings.HasSuffix(combo, "++") || combo == "+" {
		parts = append(strings.Split(strings.TrimSuffix(combo, "++"), "+"), "+")
	}
	mods := 0
	key := parts[len(parts)-1]
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "alt", "option":
			mods |= 1
		case "control", "ctrl":
			mods |= 2
		case "meta", "cmd", "command", "win":
			mods |= 4
		case "shift":
			mods |= 8
		default:
			return fmt.Errorf("browser: modificador %q desconegut (Alt, Control, Meta, Shift)", m)
		}
	}
	code, vk, text := "", 0, ""
	if k, ok := keyNames[key]; ok {
		code, vk, text = k.code, k.vk, k.text
	} else if n, err := strconv.Atoi(strings.TrimPrefix(key, "F")); err == nil && strings.HasPrefix(key, "F") && n >= 1 && n <= 12 {
		code, vk = key, 111+n
	} else if r := []rune(key); len(r) == 1 {
		text = key
		up := strings.ToUpper(key)
		switch {
		case up[0] >= 'A' && up[0] <= 'Z':
			code, vk = "Key"+up, int(up[0])
		case up[0] >= '0' && up[0] <= '9':
			code, vk = "Digit"+up, int(up[0])
		}
	} else {
		return fmt.Errorf("browser: tecla %q no suportada (Enter, Tab, Escape, Backspace, Delete, Arrow*, Page*, Home, End, F1-F12, una lletra; amb Control+/Shift+/Alt+/Meta+)", key)
	}
	if mods&(2|4) != 0 {
		text = "" // amb Control/Meta és una drecera, no text
	}
	down := map[string]any{"type": "keyDown", "key": key, "code": code, "windowsVirtualKeyCode": vk, "nativeVirtualKeyCode": vk, "modifiers": mods}
	if text != "" {
		down["text"] = text
		down["unmodifiedText"] = text
	}
	if _, err := b.call("Input.dispatchKeyEvent", down); err != nil {
		return err
	}
	_, err := b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": key, "code": code, "windowsVirtualKeyCode": vk, "nativeVirtualKeyCode": vk, "modifiers": mods})
	time.Sleep(250 * time.Millisecond)
	_ = b.esperaCarrega()
	return err
}

// ---- despatx ----

// browserArgs llegeix els arguments d'una crida (o d'un pas d'un lot).
type browserArgs map[string]any

func (a browserArgs) str(k string) string {
	if v, ok := a[k].(string); ok {
		return strings.TrimSpace(v)
	}
	if v, ok := a[k].(float64); ok {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

func (a browserArgs) num(k string) float64 {
	switch v := a[k].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f
	}
	return 0
}

func (a browserArgs) boolean(k string) bool {
	switch v := a[k].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func (a browserArgs) has(k string) bool { _, ok := a[k]; return ok }

func (a browserArgs) target() string {
	return firstNonEmpty(a.str("target"), a.str("ref"), a.str("selector"), a.str("element"))
}

// Accions de lectura: no canvien res a la pàgina.
var browserReadActions = map[string]bool{
	"open": true, "goto": true, "navigate": true, "read": true, "snapshot": true, "text": true,
	"screenshot": true, "back": true, "forward": true, "reload": true, "where": true, "url": true,
	"close": true, "wait": true, "console": true, "network": true, "tabs": true, "tab_new": true,
	"tab_select": true, "tab_close": true, "scroll": true, "hover": true,
}

// BrowserActionSegura diu si una acció només mira (obre, llegeix, captura,
// espera, desplaça) o si actua sobre la pàgina (clic, escriure, triar,
// pujar fitxers, JavaScript). Un lot és segur si tots els passos ho són.
func BrowserActionSegura(action string) bool {
	return browserReadActions[strings.ToLower(strings.TrimSpace(action))]
}

// BrowserCallSegura és BrowserActionSegura mirant també els passos d'un lot.
func BrowserCallSegura(args map[string]any) bool {
	a := browserArgs(args)
	act := strings.ToLower(a.str("action"))
	if act != "batch" {
		return BrowserActionSegura(act)
	}
	for _, s := range batchSteps(args) {
		if !BrowserActionSegura(browserArgs(s).str("action")) {
			return false
		}
	}
	return true
}

func batchSteps(args map[string]any) []map[string]any {
	raw, _ := args["steps"].([]any)
	var out []map[string]any
	for _, s := range raw {
		if m, ok := s.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// MaxBatchSteps és el màxim de passos d'un lot.
const MaxBatchSteps = 15

// browserDo executa una acció. quiet: sense el diff final (dins d'un lot).
func browserDo(b *Browser, action string, a browserArgs, quiet bool) (string, []string, error) {
	changes := func(msg string) (string, []string, error) {
		if quiet {
			return msg, nil, nil
		}
		return b.withNews(msg + "\n" + b.afterAction()), nil, nil
	}
	switch action {
	case "batch":
		return b.batch(a)
	case "open", "goto", "navigate":
		if err := b.Navigate(a.str("url")); err != nil {
			return "", nil, err
		}
		b.instrument()
		if quiet {
			return "opened " + b.On(), nil, nil
		}
		out, err := b.read(a)
		return b.withNews(out), nil, err
	case "read", "snapshot":
		out, err := b.read(a)
		return b.withNews(out), nil, err
	case "text":
		out, err := b.Snapshot(WebFetchOpts{MaxChars: int(a.num("max_chars")), Find: a.str("find"), Raw: a.boolean("raw")})
		if i := strings.Index(out, "\n\nElements interactius"); i >= 0 {
			out = out[:i]
		}
		return out, nil, err
	case "click":
		t := a.target()
		if t == "" && (a.has("x") || a.has("y")) {
			msg, err := b.ClickXY(a.num("x"), a.num("y"), a.str("button"), int(a.num("count")))
			if err != nil {
				return "", nil, err
			}
			return changes(msg)
		}
		before := b.tabIDs()
		var msg string
		var err error
		if r, ok := b.refNode(t); ok {
			// Un clic irreversible només arriba aquí si l'usuari l'ha
			// confirmat (BrowserGate): el «n'estàs segur?» de la pàgina és
			// la mateixa decisió i s'accepta sol.
			if IrreversibleName(r.name) {
				_, _ = b.eval(`(() => { if (window.__gregal) window.__gregal.answer = true; return true; })()`)
			}
			msg, err = b.ClickRef(r, a.str("button"), clickCount(a), modifierMask(a.str("modifiers")))
			if err == nil {
				time.Sleep(300 * time.Millisecond)
				_ = b.esperaCarrega()
			}
			if IrreversibleName(r.name) {
				_, _ = b.eval(`(() => { if (window.__gregal) window.__gregal.answer = null; return true; })()`)
			}
		} else {
			msg, err = b.Click(t)
			msg = strings.TrimSuffix(msg, ". Fes read per veure la pàgina nova.")
		}
		if err != nil {
			return "", nil, err
		}
		if t := b.followNewTab(before); t != "" {
			msg += "\n" + t
		}
		return changes(msg)
	case "type", "fill":
		t := a.target()
		clear := !a.has("clear") || a.boolean("clear")
		var msg string
		var err error
		if r, ok := b.refNode(t); ok {
			msg, err = b.TypeRef(r, a.str("text"), clear, a.boolean("enter") || a.boolean("submit"))
		} else {
			msg, err = b.Type(t, a.str("text"), a.boolean("enter") || a.boolean("submit"))
		}
		if err != nil {
			return "", nil, err
		}
		return changes(msg)
	case "select":
		r, ok := b.refNode(a.target())
		if !ok {
			return "", nil, fmt.Errorf("browser: select needs a ref from read (target:\"e12\")")
		}
		msg, err := b.SelectRef(r, firstNonEmpty(a.str("option"), a.str("value"), a.str("text")))
		if err != nil {
			return "", nil, err
		}
		return changes(msg)
	case "hover":
		r, ok := b.refNode(a.target())
		if !ok {
			return "", nil, fmt.Errorf("browser: hover needs a ref from read")
		}
		msg, err := b.HoverRef(r)
		if err != nil {
			return "", nil, err
		}
		time.Sleep(250 * time.Millisecond)
		return changes(msg)
	case "drag":
		from, ok1 := b.refNode(firstNonEmpty(a.str("from"), a.target()))
		to, ok2 := b.refNode(a.str("to"))
		if !ok1 || !ok2 {
			return "", nil, fmt.Errorf("browser: drag needs from:\"eN\" and to:\"eM\" refs")
		}
		msg, err := b.DragRef(from, to)
		if err != nil {
			return "", nil, err
		}
		return changes(msg)
	case "upload":
		r, ok := b.refNode(a.target())
		if !ok {
			return "", nil, fmt.Errorf("browser: upload needs the ref of the file input")
		}
		paths, err := uploadPaths(a)
		if err != nil {
			return "", nil, err
		}
		msg, err := b.UploadRef(r, paths)
		if err != nil {
			return "", nil, err
		}
		return changes(msg)
	case "scroll":
		x, y := 0.0, 0.0
		if r, ok := b.refNode(a.target()); ok {
			x, y, _ = b.nodePoint(r.node)
		}
		msg, err := b.Scroll(x, y, strings.ToLower(a.str("direction")), int(a.num("amount")))
		if err != nil {
			return "", nil, err
		}
		return changes(msg)
	case "press", "key":
		k := firstNonEmpty(a.str("key"), a.str("text"))
		if err := b.pressCombo(k); err != nil {
			return "", nil, err
		}
		return changes("pressed " + k)
	case "wait":
		msg, err := b.Wait(a.str("text"), a.str("gone"), a.str("selector"), a.num("seconds"))
		if err != nil {
			return "", nil, err
		}
		return changes(msg)
	case "screenshot":
		var region []float64
		if rs, ok := a["region"].([]any); ok {
			for _, v := range rs {
				if f, ok := v.(float64); ok {
					region = append(region, f)
				}
			}
		}
		img, note, err := b.ScreenshotRegion(region)
		if err != nil {
			return "", nil, err
		}
		return "screenshot attached" + note + " · " + b.On() + ". For canvas apps, click x/y uses this image's pixels.", []string{img}, nil
	case "back", "forward", "reload":
		var err error
		switch action {
		case "back":
			err = b.Back()
		case "forward":
			_, err = b.eval("history.forward()")
		default:
			_, err = b.call("Page.reload", map[string]any{})
		}
		if err != nil {
			return "", nil, err
		}
		time.Sleep(300 * time.Millisecond)
		_ = b.esperaCarrega()
		b.instrument()
		return changes(action)
	case "dialog":
		msg, err := b.Dialog(a.boolean("accept"), a.str("text"))
		return msg, nil, err
	case "console":
		out, err := b.Console(a.boolean("errors"))
		return out, nil, err
	case "network":
		out, err := b.Network(a.boolean("failed"))
		return out, nil, err
	case "eval", "js":
		out, err := b.Eval(a.str("js"))
		return out, nil, err
	case "where", "url":
		return b.On(), nil, nil
	case "tabs":
		out, err := b.Tabs()
		return out, nil, err
	case "tab_new":
		if err := b.NewTab(a.str("url")); err != nil {
			return "", nil, err
		}
		if quiet {
			return "new tab · " + b.On(), nil, nil
		}
		out, err := b.read(a)
		return out, nil, err
	case "tab_select":
		if err := b.SelectTab(a.str("tab")); err != nil {
			return "", nil, err
		}
		if quiet {
			return "tab · " + b.On(), nil, nil
		}
		out, err := b.read(a)
		return out, nil, err
	case "tab_close":
		msg, err := b.CloseTab(a.str("tab"))
		return msg, nil, err
	}
	return "", nil, fmt.Errorf("browser: unknown action %q (open, read, text, click, type, select, hover, drag, upload, scroll, press, wait, screenshot, back, forward, reload, dialog, console, network, tabs, tab_new, tab_select, tab_close, eval, batch, close)", action)
}

func clickCount(a browserArgs) int {
	if a.boolean("double") {
		return 2
	}
	return int(a.num("count"))
}

func modifierMask(s string) int {
	m := 0
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == '+' || r == ',' || r == ' ' }) {
		switch strings.ToLower(p) {
		case "alt":
			m |= 1
		case "control", "ctrl":
			m |= 2
		case "meta", "cmd":
			m |= 4
		case "shift":
			m |= 8
		}
	}
	return m
}

// uploadPaths resol els fitxers a pujar (relatius a la carpeta del projecte).
func uploadPaths(a browserArgs) ([]string, error) {
	var raw []string
	if p := firstNonEmpty(a.str("path"), a.str("file")); p != "" {
		raw = append(raw, p)
	}
	if ps, ok := a["paths"].([]any); ok {
		for _, p := range ps {
			if s, ok := p.(string); ok && strings.TrimSpace(s) != "" {
				raw = append(raw, strings.TrimSpace(s))
			}
		}
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("browser: upload needs path (or paths)")
	}
	dir := a.str("_dir")
	var out []string
	for _, p := range raw {
		if !filepath.IsAbs(p) && dir != "" {
			p = filepath.Join(dir, p)
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		if st, err := os.Stat(abs); err != nil || st.IsDir() {
			return nil, fmt.Errorf("browser: upload: %s is not a file", p)
		}
		out = append(out, abs)
	}
	return out, nil
}

// read és la lectura: arbre d'accessibilitat, o la de sempre si no n'hi ha.
func (b *Browser) read(a browserArgs) (string, error) {
	scope := 0
	if n, ok := axRefOf(a.str("scope")); ok {
		scope = n
	}
	if out, ok, err := b.AXRead(a.str("find"), scope); ok || err != nil {
		return out, err
	}
	b.mu.Lock()
	if b.ax != nil {
		b.ax.active = false
	}
	b.mu.Unlock()
	return b.Snapshot(WebFetchOpts{MaxChars: int(a.num("max_chars")), Find: a.str("find"), Raw: a.boolean("raw")})
}

// afterAction és el que s'enganxa a una acció: els canvis de la pàgina.
func (b *Browser) afterAction() string {
	if c := b.AXChanges(); c != "" {
		return c
	}
	return "Now on " + b.On() + ". Read the page to see it."
}

// withNews hi afegeix diàlegs i errors nous de la pàgina.
func (b *Browser) withNews(out string) string {
	if n := b.pageNews(); n != "" {
		return n + "\n\n" + out
	}
	return out
}

// batch executa diversos passos seguits en una sola crida (un sol torn del
// model). S'atura al primer error i diu fins on ha arribat; al final, un
// sol diff de la pàgina.
func (b *Browser) batch(a browserArgs) (string, []string, error) {
	steps := batchSteps(a)
	if len(steps) == 0 {
		return "", nil, fmt.Errorf("browser: batch needs steps: [{action:…}, …]")
	}
	if len(steps) > MaxBatchSteps {
		return "", nil, fmt.Errorf("browser: batch has %d steps (maximum %d)", len(steps), MaxBatchSteps)
	}
	var out []string
	var imgs []string
	for i, s := range steps {
		sa := browserArgs(s)
		if dir := a.str("_dir"); dir != "" && !sa.has("_dir") {
			sa["_dir"] = dir
		}
		act := strings.ToLower(sa.str("action"))
		if act == "batch" || act == "close" {
			return "", nil, fmt.Errorf("browser: step %d: %s is not allowed inside a batch", i+1, act)
		}
		msg, im, err := browserDo(b, act, sa, true)
		if err != nil {
			out = append(out, fmt.Sprintf("%d. %s → ERROR: %v (batch stopped; %d step(s) not run)", i+1, act, err, len(steps)-i-1))
			return b.withNews(strings.Join(out, "\n") + "\n\n" + b.afterAction()), imgs, nil
		}
		imgs = append(imgs, im...)
		out = append(out, fmt.Sprintf("%d. %s", i+1, truncateInline(msg, 300)))
	}
	return b.withNews(strings.Join(out, "\n") + "\n\n" + b.afterAction()), imgs, nil
}
