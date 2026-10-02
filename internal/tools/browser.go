package tools

// Navegador controlat per l'agent, sense extensions ni serveis externs:
// s'engega el Chrome o l'Edge de la màquina amb el port de depuració
// (Chrome DevTools Protocol) i un perfil propi del gregal, i s'hi parla per
// websocket (golang.org/x/net/websocket, que ja era dependència).
//
// Què hi guanya l'agent respecte de web_fetch: pàgines que només existeixen
// després d'executar JavaScript, aplicacions locals que està desenvolupant
// (localhost:3000), llocs on l'usuari ja ha iniciat sessió al perfil del
// gregal, i poder fer clic, escriure i mirar (captura) el resultat. El
// perfil viu a ~/.config/gregal/browser: l'usuari s'hi identifica un cop i
// queda; mai es toca el perfil personal del navegador.
//
// El model veu la pàgina com la veu a web_fetch (contingut principal en
// markdown) més una llista numerada d'elements interactius [n] (enllaços,
// botons, camps). click i type accepten aquest número, un selector CSS o
// un text visible. Res d'imatges tret que demani screenshot.
//
// Una sola finestra i una sola pestanya activa per procés: prou per a un
// agent que fa una cosa darrere l'altra, i sense el pes de gestionar
// pestanyes. La finestra és visible per defecte (l'usuari veu què passa);
// GREGAL_BROWSER_HEADLESS=1 l'amaga.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// BrowserTimeout és el màxim per acció (navegar, avaluar).
const BrowserTimeout = 30 * time.Second

// Browser és la connexió viva amb el navegador (una per procés).
type Browser struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	port    int
	ws      *websocket.Conn
	nextID  int
	pending map[int]chan cdpMsg
	pageWS  string
	dead    chan struct{}
	// refs són els elements de l'última lectura ([n] → selector únic).
	refs map[int]string
	// launched diu si el procés l'hem engegat nosaltres (i el matarem).
	launched bool
	// actx: context del pas en marxa; call el consulta per tallar-se.
	actx context.Context
}

type cdpMsg struct {
	ID     int             `json:"id"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

var (
	browserMu   sync.Mutex
	browserInst *Browser
	// browserExec i browserAttach permeten als tests apuntar a un servidor
	// CDP fals en comptes d'engegar un navegador.
	browserAttach = "" // "host:port" d'un CDP ja en marxa (GREGAL_BROWSER_CDP)
)

// BrowserExecutable troba el navegador: GREGAL_BROWSER, o els llocs
// habituals d'Edge i Chrome segons el sistema.
func BrowserExecutable() string {
	if v := strings.TrimSpace(os.Getenv("GREGAL_BROWSER")); v != "" {
		return v
	}
	var cands []string
	switch runtime.GOOS {
	case "windows":
		pf, pf86, local := os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")
		cands = []string{
			filepath.Join(pf, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(pf86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(local, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(pf86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(pf, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(pf, "Chromium", "Application", "chrome.exe"),
		}
	case "darwin":
		cands = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}
	default:
		for _, n := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			if p, err := exec.LookPath(n); err == nil {
				return p
			}
		}
	}
	for _, c := range cands {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// browserProfileDir és el perfil propi del gregal.
func browserProfileDir() string {
	if v := strings.TrimSpace(os.Getenv("GREGAL_BROWSER_PROFILE")); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gregal", "browser")
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// GetBrowser torna el navegador viu, engegant-lo si cal.
func GetBrowser() (*Browser, error) {
	browserMu.Lock()
	defer browserMu.Unlock()
	if browserInst != nil && browserInst.alive() {
		return browserInst, nil
	}
	b := &Browser{pending: map[int]chan cdpMsg{}, refs: map[int]string{}}
	attach := browserAttach
	if attach == "" {
		attach = strings.TrimSpace(os.Getenv("GREGAL_BROWSER_CDP"))
	}
	if attach != "" {
		host, portS, err := net.SplitHostPort(attach)
		if err != nil {
			return nil, fmt.Errorf("browser: GREGAL_BROWSER_CDP ha de ser host:port: %v", err)
		}
		fmt.Sscanf(portS, "%d", &b.port)
		_ = host
	} else {
		exe := BrowserExecutable()
		if exe == "" {
			return nil, fmt.Errorf("browser: no trobo Chrome ni Edge (defineix GREGAL_BROWSER amb la ruta de l'executable)")
		}
		port, err := freePort()
		if err != nil {
			return nil, err
		}
		b.port = port
		args := []string{
			fmt.Sprintf("--remote-debugging-port=%d", port),
			"--user-data-dir=" + browserProfileDir(),
			"--no-first-run", "--no-default-browser-check", "--disable-sync",
			"--disable-background-networking", "--disable-features=Translate,MediaRouter",
			"--window-size=1280,900", "--remote-allow-origins=*",
		}
		if os.Getenv("GREGAL_BROWSER_HEADLESS") == "1" {
			args = append(args, "--headless=new")
		}
		args = append(args, "about:blank")
		b.cmd = exec.Command(exe, args...)
		b.cmd.Stdout, b.cmd.Stderr = io.Discard, io.Discard
		if err := b.cmd.Start(); err != nil {
			return nil, fmt.Errorf("browser: no s'ha pogut engegar %s: %v", filepath.Base(exe), err)
		}
		b.launched = true
	}
	if err := b.esperaCDP(); err != nil {
		b.closeIntern()
		return nil, err
	}
	if err := b.connectaPagina(); err != nil {
		b.closeIntern()
		return nil, err
	}
	browserInst = b
	return b, nil
}

func (b *Browser) alive() bool {
	if b == nil || b.ws == nil {
		return false
	}
	select {
	case <-b.dead:
		return false
	default:
		return true
	}
}

func (b *Browser) httpBase() string { return fmt.Sprintf("http://127.0.0.1:%d", b.port) }

// cdpHTTP és el client per als GET del CDP: amb topall, perquè un Chrome
// zombi que accepta la connexió i no respon penja http.Get per sempre
// (mesurat: 15 min de pas «browser» sense cap error).
var cdpHTTP = &http.Client{Timeout: 10 * time.Second}

// esperaCDP espera que el port de depuració respongui (arrencada).
func (b *Browser) esperaCDP() error {
	dl := time.Now().Add(15 * time.Second)
	for time.Now().Before(dl) {
		resp, err := cdpHTTP.Get(b.httpBase() + "/json/version")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("browser: el navegador no respon al port de depuració %d", b.port)
}

// connectaPagina obre (o reutilitza) una pestanya i s'hi connecta.
func (b *Browser) connectaPagina() error {
	type tab struct {
		Type string `json:"type"`
		WS   string `json:"webSocketDebuggerUrl"`
		URL  string `json:"url"`
	}
	var tabs []tab
	if resp, err := cdpHTTP.Get(b.httpBase() + "/json/list"); err == nil {
		json.NewDecoder(resp.Body).Decode(&tabs)
		resp.Body.Close()
	}
	var ws string
	for _, t := range tabs {
		if t.Type == "page" && t.WS != "" {
			ws = t.WS
			break
		}
	}
	if ws == "" {
		req, _ := http.NewRequest("PUT", b.httpBase()+"/json/new?about:blank", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("browser: no puc obrir pestanya: %v", err)
		}
		var t tab
		json.NewDecoder(resp.Body).Decode(&t)
		resp.Body.Close()
		ws = t.WS
	}
	if ws == "" {
		return fmt.Errorf("browser: cap pestanya amb depuració")
	}
	conn, err := websocket.Dial(ws, "", b.httpBase())
	if err != nil {
		return fmt.Errorf("browser: websocket: %v", err)
	}
	b.ws = conn
	b.pageWS = ws
	b.dead = make(chan struct{})
	go b.llegeix()
	if _, err := b.call("Page.enable", nil); err != nil {
		return err
	}
	_, _ = b.call("Runtime.enable", nil)
	return nil
}

// llegeix reparteix les respostes per id; els esdeveniments s'ignoren.
func (b *Browser) llegeix() {
	defer close(b.dead)
	for {
		var raw []byte
		if err := websocket.Message.Receive(b.ws, &raw); err != nil {
			b.mu.Lock()
			for id, ch := range b.pending {
				close(ch)
				delete(b.pending, id)
			}
			b.mu.Unlock()
			return
		}
		var m cdpMsg
		if json.Unmarshal(raw, &m) != nil || m.ID == 0 {
			continue
		}
		b.mu.Lock()
		ch, ok := b.pending[m.ID]
		if ok {
			delete(b.pending, m.ID)
		}
		b.mu.Unlock()
		if ok {
			ch <- m
		}
	}
}

// call envia una ordre CDP i n'espera el resultat. El temps d'espera és el
// mínim entre BrowserTimeout i el deadline del context (cancel·lació amb
// /stop o topall del pas): una ordre que no arriba a resposta mai ha de
// deixar el pas viu més enllà del qui l'ha cridat.
func (b *Browser) callCtx(ctx context.Context, method string, params any) (json.RawMessage, error) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	ch := make(chan cdpMsg, 1)
	b.pending[id] = ch
	b.mu.Unlock()
	req := map[string]any{"id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	raw, _ := json.Marshal(req)
	if err := websocket.Message.Send(b.ws, string(raw)); err != nil {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return nil, fmt.Errorf("browser: envia %s: %v", method, err)
	}
	espera := BrowserTimeout
	b.mu.Lock()
	actx := b.actx
	b.mu.Unlock()
	if actx != nil {
		ctx = actx
	}
	if dl, ok := ctx.Deadline(); ok {
		if fins := time.Until(dl); fins < espera {
			espera = fins
		}
	}
	timer := time.NewTimer(espera)
	defer timer.Stop()
	select {
	case m, ok := <-ch:
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		if !ok {
			return nil, fmt.Errorf("browser: connexió tancada")
		}
		if m.Error != nil {
			return nil, fmt.Errorf("browser: %s: %s", method, m.Error.Message)
		}
		return m.Result, nil
	case <-timer.C:
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return nil, fmt.Errorf("browser: %s sense resposta en %s", method, espera.Round(time.Second))
	case <-ctx.Done():
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return nil, fmt.Errorf("browser: %s aturat: %v", method, ctx.Err())
	}
}

// call és callCtx amb el context de fons (els usos interns del navegador).
func (b *Browser) call(method string, params any) (json.RawMessage, error) {
	return b.callCtx(context.Background(), method, params)
}

// eval avalua JavaScript a la pàgina i torna el valor (JSON).
func (b *Browser) eval(js string) (json.RawMessage, error) {
	res, err := b.call("Runtime.evaluate", map[string]any{
		"expression": js, "returnByValue": true, "awaitPromise": true,
	})
	if err != nil {
		return nil, err
	}
	var r struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text      string `json:"text"`
			Exception *struct {
				Description string `json:"description"`
			} `json:"exception"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(res, &r); err != nil {
		return nil, err
	}
	if r.ExceptionDetails != nil {
		msg := r.ExceptionDetails.Text
		if r.ExceptionDetails.Exception != nil && r.ExceptionDetails.Exception.Description != "" {
			msg = r.ExceptionDetails.Exception.Description
		}
		return nil, fmt.Errorf("browser: JavaScript: %s", truncateInline(msg, 200))
	}
	return r.Result.Value, nil
}

func (b *Browser) evalString(js string) (string, error) {
	v, err := b.eval(js)
	if err != nil {
		return "", err
	}
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s, nil
	}
	return string(v), nil
}

// Close tanca la connexió i, si l'hem engegat nosaltres, el navegador.
func (b *Browser) Close() {
	if b == nil {
		return
	}
	b.closeIntern()
	browserMu.Lock()
	if browserInst == b {
		browserInst = nil
	}
	browserMu.Unlock()
}

// closeIntern tanca sense tocar browserMu. GetBrowser el crida en els seus
// camins d'error MENTRE encara té browserMu agafat: amb Close() hi havia un
// deadlock (Lock dues vegades al mateix mutex) i qualsevol fallada
// d'arrencada del navegador penjava el pas per sempre.
func (b *Browser) closeIntern() {
	if b.ws != nil {
		b.ws.Close()
	}
	if b.launched && b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
		_, _ = b.cmd.Process.Wait()
	}
}

// CloseBrowser tanca el navegador de l'agent si és obert.
func CloseBrowser() {
	browserMu.Lock()
	b := browserInst
	browserMu.Unlock()
	if b != nil {
		b.Close()
	}
}

// ---- accions ----

// Navigate obre una URL i espera que carregui.
func (b *Browser) Navigate(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("browser: cal una URL http(s) completa")
	}
	if _, err := b.call("Page.navigate", map[string]any{"url": u.String()}); err != nil {
		return err
	}
	return b.esperaCarrega()
}

// esperaCarrega espera document.readyState == complete (amb topall) i un
// respir perquè les SPA pintin.
func (b *Browser) esperaCarrega() error {
	dl := time.Now().Add(BrowserTimeout)
	for time.Now().Before(dl) {
		st, err := b.evalString("document.readyState")
		if err == nil && st == "complete" {
			time.Sleep(300 * time.Millisecond)
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil // la pàgina va lenta: es llegeix el que hi hagi
}

// jsSnapshot marca els elements interactius amb data-gregal-ref i torna
// l'HTML de la pàgina més la llista d'elements. És l'única «vista» que té
// el model: text llegible + [n] per actuar.
const jsSnapshot = `(() => {
  const sel = 'a[href], button, input, textarea, select, [role=button], [role=link], [role=tab], [role=menuitem], [contenteditable=true], summary';
  const vis = e => { const r = e.getBoundingClientRect(); const s = getComputedStyle(e); return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && s.display !== 'none'; };
  const els = Array.from(document.querySelectorAll(sel)).filter(vis);
  const items = [];
  let n = 0;
  for (const e of els) {
    if (n >= 80) break;
    n++;
    e.setAttribute('data-gregal-ref', String(n));
    const tag = e.tagName.toLowerCase();
    let text = (e.innerText || e.value || e.getAttribute('aria-label') || e.getAttribute('placeholder') || e.getAttribute('title') || e.getAttribute('alt') || '').trim().replace(/\s+/g, ' ');
    if (text.length > 80) text = text.slice(0, 79) + '…';
    let kind = tag;
    if (tag === 'input') kind = 'input:' + (e.type || 'text');
    if (tag === 'a') { const h = e.getAttribute('href') || ''; if (h && !h.startsWith('#') && !h.startsWith('javascript')) { try { text += '  → ' + new URL(h, location.href).href; } catch (x) {} } }
    items.push('[' + n + '] ' + kind + (text ? ' ' + JSON.stringify(text) : ''));
  }
  return JSON.stringify({ title: document.title, url: location.href, html: document.documentElement.outerHTML, items });
})()`

// Snapshot llegeix la pàgina: capçalera, contingut principal en markdown
// i elements interactius numerats.
func (b *Browser) Snapshot(o WebFetchOpts) (string, error) {
	raw, err := b.evalString(jsSnapshot)
	if err != nil {
		return "", err
	}
	var snap struct {
		Title string   `json:"title"`
		URL   string   `json:"url"`
		HTML  string   `json:"html"`
		Items []string `json:"items"`
	}
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		return "", fmt.Errorf("browser: lectura il·legible: %v", err)
	}
	b.mu.Lock()
	b.refs = map[int]string{}
	for i := range snap.Items {
		b.refs[i+1] = fmt.Sprintf("[data-gregal-ref=%q]", fmt.Sprint(i+1))
	}
	b.mu.Unlock()
	if o.MaxChars <= 0 || o.MaxChars > 30000 {
		o.MaxChars = MaxOutputChars
	}
	base, _ := url.Parse(snap.URL)
	var body string
	whole := false
	if o.Raw {
		body = htmlToText(snap.HTML)
		whole = true
	} else {
		doc := ReadableHTML(snap.HTML, base)
		body = doc.Markdown
		whole = doc.Whole
	}
	p := &pagina{URL: snap.URL, Title: snap.Title, Body: body, Words: comptaParaules(body), Whole: whole}
	out := p.render(WebFetchOpts{MaxChars: o.MaxChars, Find: o.Find})
	if len(snap.Items) > 0 {
		var it strings.Builder
		it.WriteString("\n\nElements interactius (usa el número amb click/type):\n")
		for _, s := range snap.Items {
			it.WriteString(s + "\n")
		}
		out += truncate(it.String(), 6000)
	}
	return out, nil
}

// resolTarget converteix [n], un selector CSS o un text visible en un
// selector que la pàgina entén (amb el text es marca l'element).
func (b *Browser) resolTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("browser: cal ref, selector o text")
	}
	var n int
	if _, err := fmt.Sscanf(strings.Trim(target, "[]"), "%d", &n); err == nil && fmt.Sprint(n) == strings.Trim(target, "[]") {
		b.mu.Lock()
		sel, ok := b.refs[n]
		b.mu.Unlock()
		if !ok {
			return "", fmt.Errorf("browser: no hi ha cap element [%d]: llegeix la pàgina primer (read)", n)
		}
		return sel, nil
	}
	// Selector CSS vàlid?
	okSel, _ := b.evalString(fmt.Sprintf(`(() => { try { return document.querySelector(%s) ? "1" : "0"; } catch (e) { return "x"; } })()`, jsonStr(target)))
	if okSel == "1" {
		return target, nil
	}
	// Text visible (botó, enllaç, etiqueta): es marca amb data-gregal-ref=text.
	found, _ := b.evalString(fmt.Sprintf(`(() => {
	  const t = %s.toLowerCase();
	  const sel = 'a, button, input, textarea, select, label, [role=button], [role=link], [role=tab], [role=menuitem], summary, li, span, div';
	  const vis = e => { const r = e.getBoundingClientRect(); return r.width > 0 && r.height > 0; };
	  let best = null;
	  for (const e of document.querySelectorAll(sel)) {
	    if (!vis(e)) continue;
	    const txt = ((e.innerText || e.value || e.getAttribute('aria-label') || e.getAttribute('placeholder') || '') + '').trim().toLowerCase();
	    if (!txt) continue;
	    if (txt === t) { best = e; break; }
	    if (!best && txt.includes(t) && txt.length < t.length + 40) best = e;
	  }
	  if (!best) return "";
	  if (best.tagName === 'LABEL' && best.control) best = best.control;
	  best.setAttribute('data-gregal-ref', 'text');
	  return "1";
	})()`, jsonStr(target)))
	if found == "1" {
		return `[data-gregal-ref="text"]`, nil
	}
	return "", fmt.Errorf("browser: no trobo %q (ni ref, ni selector, ni text visible). Fes read i usa un [n]", target)
}

func jsonStr(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

// Click fa clic a un element (scroll + click per JS + esdeveniment real).
func (b *Browser) Click(target string) (string, error) {
	sel, err := b.resolTarget(target)
	if err != nil {
		return "", err
	}
	res, err := b.evalString(fmt.Sprintf(`(() => {
	  const e = document.querySelector(%s);
	  if (!e) return "no hi és";
	  e.scrollIntoView({block: 'center'});
	  const r = e.getBoundingClientRect();
	  return JSON.stringify({x: r.left + r.width / 2, y: r.top + r.height / 2, tag: e.tagName, text: (e.innerText || e.value || '').trim().slice(0, 60)});
	})()`, jsonStr(sel)))
	if err != nil {
		return "", err
	}
	var pos struct {
		X, Y float64
		Tag  string
		Text string
	}
	if json.Unmarshal([]byte(res), &pos) != nil {
		return "", fmt.Errorf("browser: %s", res)
	}
	abansURL, _ := b.evalString("location.href")
	for _, tipus := range []string{"mousePressed", "mouseReleased"} {
		if _, err := b.call("Input.dispatchMouseEvent", map[string]any{"type": tipus, "x": pos.X, "y": pos.Y, "button": "left", "clickCount": 1}); err != nil {
			// Recanvi: clic per JS.
			if _, e2 := b.eval(fmt.Sprintf(`document.querySelector(%s).click()`, jsonStr(sel))); e2 != nil {
				return "", err
			}
			break
		}
	}
	time.Sleep(400 * time.Millisecond)
	_ = b.esperaCarrega()
	despres, _ := b.evalString("location.href")
	titol, _ := b.evalString("document.title")
	msg := fmt.Sprintf("clic a %s %q", strings.ToLower(pos.Tag), pos.Text)
	if despres != abansURL {
		msg += " → ara a " + despres
	}
	if titol != "" {
		msg += " · " + titol
	}
	return msg + ". Fes read per veure la pàgina nova.", nil
}

// Type escriu text en un camp (el buida abans) i, si cal, prem Enter.
func (b *Browser) Type(target, text string, enter bool) (string, error) {
	sel, err := b.resolTarget(target)
	if err != nil {
		return "", err
	}
	if _, err := b.eval(fmt.Sprintf(`(() => {
	  const e = document.querySelector(%s);
	  if (!e) throw new Error("no hi és");
	  e.scrollIntoView({block: 'center'});
	  e.focus();
	  if ('value' in e) { e.value = ''; e.dispatchEvent(new Event('input', {bubbles: true})); }
	  else if (e.isContentEditable) { e.textContent = ''; }
	  return true;
	})()`, jsonStr(sel))); err != nil {
		return "", err
	}
	if _, err := b.call("Input.insertText", map[string]any{"text": text}); err != nil {
		return "", err
	}
	if enter {
		if err := b.Press("Enter"); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("escrit %q", truncateInline(text, 60)) + map[bool]string{true: " + Enter", false: ""}[enter], nil
}

var keyCodes = map[string]struct {
	code string
	vk   int
	text string
}{
	"Enter":     {"Enter", 13, "\r"},
	"Tab":       {"Tab", 9, "\t"},
	"Escape":    {"Escape", 27, ""},
	"Backspace": {"Backspace", 8, ""},
	"ArrowDown": {"ArrowDown", 40, ""},
	"ArrowUp":   {"ArrowUp", 38, ""},
	"PageDown":  {"PageDown", 34, ""},
	"PageUp":    {"PageUp", 33, ""},
	"Space":     {"Space", 32, " "},
}

// Press prem una tecla (Enter, Tab, Escape, ArrowDown…).
func (b *Browser) Press(key string) error {
	k, ok := keyCodes[key]
	if !ok {
		return fmt.Errorf("browser: tecla %q no suportada (Enter, Tab, Escape, Backspace, ArrowUp/Down, PageUp/Down, Space)", key)
	}
	down := map[string]any{"type": "keyDown", "key": key, "code": k.code, "windowsVirtualKeyCode": k.vk, "nativeVirtualKeyCode": k.vk}
	if k.text != "" {
		down["text"] = k.text
		down["unmodifiedText"] = k.text
	}
	if _, err := b.call("Input.dispatchKeyEvent", down); err != nil {
		return err
	}
	_, err := b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": key, "code": k.code, "windowsVirtualKeyCode": k.vk, "nativeVirtualKeyCode": k.vk})
	time.Sleep(300 * time.Millisecond)
	_ = b.esperaCarrega()
	return err
}

// Screenshot torna una captura PNG com a data URL (per adjuntar al model).
func (b *Browser) Screenshot() (string, error) {
	res, err := b.call("Page.captureScreenshot", map[string]any{"format": "jpeg", "quality": 70})
	if err != nil {
		return "", err
	}
	var r struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(res, &r); err != nil || r.Data == "" {
		return "", fmt.Errorf("browser: captura buida")
	}
	if raw, err := base64.StdEncoding.DecodeString(r.Data); err == nil && len(raw) > MaxImageBytes {
		return "", fmt.Errorf("browser: captura massa gran (%d bytes)", len(raw))
	}
	return "data:image/jpeg;base64," + r.Data, nil
}

// Back torna enrere a l'historial.
func (b *Browser) Back() error {
	if _, err := b.eval("history.back()"); err != nil {
		return err
	}
	time.Sleep(300 * time.Millisecond)
	return b.esperaCarrega()
}

// Eval executa JavaScript arbitrari i torna el resultat (retallat).
func (b *Browser) Eval(js string) (string, error) {
	v, err := b.eval(js)
	if err != nil {
		return "", err
	}
	var s string
	if json.Unmarshal(v, &s) != nil {
		s = string(v)
	}
	if s == "" || s == "null" {
		s = "(sense valor)"
	}
	return truncate(s, MaxOutputChars), nil
}

// On és on som ara (url · títol).
func (b *Browser) On() string {
	u, _ := b.evalString("location.href")
	t, _ := b.evalString("document.title")
	return strings.TrimSpace(u + " · " + t)
}

// BrowserAction executa una acció de l'eina `browser` i torna text (i
// imatges per a screenshot). És l'entrada que fa servir l'agent.
//
// Cada acció té un topall dur (BrowserActionTimeout): el navegador és una
// eina viva (Chrome + CDP) i una acció que no torna bloqueja el pas sencer.
// Sense això, un «read» contra una pàgina que no carrega deixa el torn
// penjat fins que algú el mati (mesurat: 15 min d'espera silenciosa).
const BrowserActionTimeout = 60 * time.Second

func BrowserAction(ctx context.Context, action string, args map[string]any) (string, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	str := func(k string) string {
		if v, ok := args[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	num := func(k string) int {
		switch v := args[k].(type) {
		case float64:
			return int(v)
		case int:
			return v
		}
		return 0
	}
	boolean := func(k string) bool {
		v, _ := args[k].(bool)
		return v
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "close" {
		CloseBrowser()
		return "navegador tancat", nil, nil
	}
	// Topall dur per acció: l'arrencada del navegador (fins a ~15 s) corre
	// fora; les accions sobre la pàgina queden limitades. El context del
	// torn (cancel·lat amb /stop) també talla aquí.
	actx, cancel := context.WithTimeout(ctx, BrowserActionTimeout)
	defer cancel()
	ctx = actx
	b, err := GetBrowser()
	if err != nil {
		return "", nil, err
	}
	// El context del pas viu al browser: totes les crides CDP posteriors
	// (navigate, read, click…) queden limitades pel topall del pas.
	b.mu.Lock()
	b.actx = ctx
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.actx = nil
		b.mu.Unlock()
	}()
	switch action {
	case "open", "goto", "navigate":
		if err := b.Navigate(str("url")); err != nil {
			return "", nil, err
		}
		out, err := b.Snapshot(WebFetchOpts{MaxChars: num("max_chars"), Find: str("find"), Raw: boolean("raw")})
		return out, nil, err
	case "read", "snapshot":
		out, err := b.Snapshot(WebFetchOpts{MaxChars: num("max_chars"), Find: str("find"), Raw: boolean("raw")})
		return out, nil, err
	case "click":
		out, err := b.Click(firstNonEmpty(str("target"), str("ref"), str("selector"), str("text")))
		return out, nil, err
	case "type", "fill":
		out, err := b.Type(firstNonEmpty(str("target"), str("ref"), str("selector")), str("text"), boolean("enter"))
		return out, nil, err
	case "press", "key":
		k := firstNonEmpty(str("key"), str("text"))
		if err := b.Press(k); err != nil {
			return "", nil, err
		}
		return "premuda " + k + " · " + b.On(), nil, nil
	case "screenshot":
		img, err := b.Screenshot()
		if err != nil {
			return "", nil, err
		}
		return "captura adjunta · " + b.On(), []string{img}, nil
	case "back":
		if err := b.Back(); err != nil {
			return "", nil, err
		}
		return "enrere · " + b.On(), nil, nil
	case "eval", "js":
		out, err := b.Eval(str("js"))
		return out, nil, err
	case "where", "url":
		return b.On(), nil, nil
	}
	return "", nil, fmt.Errorf("browser: acció %q desconeguda (open, read, click, type, press, screenshot, back, eval, close)", action)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// BrowserActionSegura diu si una acció només mira (obre, llegeix, captura)
// o si actua sobre la pàgina (clic, escriure, JavaScript).
func BrowserActionSegura(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "open", "goto", "navigate", "read", "snapshot", "screenshot", "back", "where", "url", "close":
		return true
	}
	return false
}
