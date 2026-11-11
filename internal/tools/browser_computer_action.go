package tools

// On actua l'agent a l'ordinador (paquet B13, pla v2).
//
// Cada acció de punter de l'eina del navegador (clic, escriure, desplaçar,
// arrossegar, triar) avisa ABANS d'executar-se amb un ComputerAction: tipus,
// on (fraccions del viewport de la pàgina, 0..1), l'objectiu llegible i, si
// escriu, el text. El visor de l'ordinador (internal/web) el dibuixa a sobre
// del noVNC i en desa una captura petita per a la reproducció.
//
// Res de secrets: el text d'un camp de contrasenya, targeta o codi d'un sol
// ús no surt mai d'aquí (Masked + punts), i internal/web hi passa a més el
// redactor de secrets abans de desar res. L'esdeveniment no va mai al model:
// la sortida de l'eina no canvia.

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Tipus d'acció de ComputerAction.Kind.
const (
	ComputerClick  = "click"
	ComputerType   = "type"
	ComputerScroll = "scroll"
	ComputerDrag   = "drag"
	ComputerSelect = "select"
)

// MaskedText és el que es mostra en lloc del text d'un camp secret. Sempre
// la mateixa llargada: tampoc no se'n dedueix quants caràcters té.
const MaskedText = "••••••••"

// computerTextMax és el tros de text escrit que es mostra (no cal més per
// veure què fa i així un enganxat enorme no infla el registre).
const computerTextMax = 80

// ComputerView és el viewport de la pàgina i on cau a la pantalla de
// l'ordinador (píxels CSS). Amb Left/Top i la mida de la pantalla, el visor
// passa un punt del viewport a un punt del noVNC.
type ComputerView struct {
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	ScreenW float64 `json:"screenW,omitempty"`
	ScreenH float64 `json:"screenH,omitempty"`
	Left    float64 `json:"left,omitempty"`
	Top     float64 `json:"top,omitempty"`
}

// ComputerAction és una acció de l'agent a l'ordinador. Coordenades en
// fraccions del viewport (0..1): no depenen de la mida de la captura ni de
// la del visor.
type ComputerAction struct {
	Kind string `json:"type"`
	// Owner és la clau del propietari de l'ordinador (BrowserOwnerKey).
	Owner string   `json:"owner"`
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
	ToX   *float64 `json:"toX,omitempty"`
	ToY   *float64 `json:"toY,omitempty"`
	// Rect és la caixa de l'objectiu [x, y, w, h], també en fraccions.
	Rect   []float64    `json:"rect,omitempty"`
	Dir    string       `json:"dir,omitempty"`
	Target string       `json:"target,omitempty"`
	Text   string       `json:"text,omitempty"`
	Masked bool         `json:"masked,omitempty"`
	View   ComputerView `json:"view"`
	// Shot és una captura JPEG petita del viewport just abans de l'acció
	// (per a la reproducció). No viatja mai dins l'esdeveniment.
	Shot []byte    `json:"-"`
	At   time.Time `json:"-"`
}

var (
	computerSinkMu sync.RWMutex
	computerSink   func(scope string, a ComputerAction)
)

// SetComputerActionSink connecta qui rep les accions (internal/web, que les
// passa al registre d'esdeveniments). nil = ningú: no es calcula res.
func SetComputerActionSink(fn func(scope string, a ComputerAction)) {
	computerSinkMu.Lock()
	computerSink = fn
	computerSinkMu.Unlock()
}

func loadComputerSink() func(string, ComputerAction) {
	computerSinkMu.RLock()
	defer computerSinkMu.RUnlock()
	return computerSink
}

// computerPoint és un punt o una caixa en píxels CSS del viewport, abans de
// passar-lo a fraccions.
type computerPoint struct {
	x, y     float64
	box      []float64 // x, y, w, h (opcional)
	toX, toY float64
	hasTo    bool
	dir      string
	target   string
	text     string
	secret   bool
}

// jsComputerView llegeix el viewport i on és a la pantalla. outer-inner és
// la barra de pestanyes i d'adreces de Chrome (a dalt) i les vores.
const jsComputerView = `JSON.stringify({w: innerWidth, h: innerHeight, sx: screenX, sy: screenY, ow: outerWidth, oh: outerHeight, scw: screen.width, sch: screen.height})`

func (b *Browser) computerView() (ComputerView, bool) {
	s, err := b.evalString(jsComputerView)
	if err != nil {
		return ComputerView{}, false
	}
	var v struct {
		W, H, SX, SY, OW, OH, SCW, SCH float64
	}
	if json.Unmarshal([]byte(s), &v) != nil || v.W <= 0 || v.H <= 0 {
		return ComputerView{}, false
	}
	return viewFromWindow(v.W, v.H, v.SX, v.SY, v.OW, v.OH, v.SCW, v.SCH), true
}

// viewFromWindow situa el viewport a la pantalla. Les vores laterals són
// iguals; la de sota és com una lateral i la resta de la diferència és el
// capçal de Chrome. Sense finestra (headless) o amb valors absurds, el
// viewport ocupa tota la pantalla.
func viewFromWindow(w, h, sx, sy, ow, oh, scw, sch float64) ComputerView {
	v := ComputerView{W: w, H: h, ScreenW: scw, ScreenH: sch}
	if scw <= 0 || sch <= 0 || ow < w || oh < h {
		v.ScreenW, v.ScreenH = w, h
		return v
	}
	side := (ow - w) / 2
	v.Left = sx + side
	v.Top = sy + (oh - h) - side
	if v.Left < 0 || v.Top < 0 || v.Left+w > scw+1 || v.Top+h > sch+1 {
		// La pàgina no cap a la pantalla que diu el navegador (headless,
		// pantalla virtual): l'única referència fiable és el viewport.
		return ComputerView{W: w, H: h, ScreenW: w, ScreenH: h}
	}
	return v
}

// emitComputer avisa d'una acció abans de fer-la. Mai no falla l'acció: si
// no hi ha receptor o no es pot llegir el viewport, simplement no avisa.
func (b *Browser) emitComputer(kind string, p computerPoint) {
	sink := loadComputerSink()
	if sink == nil || b == nil {
		return
	}
	b.mu.Lock()
	scope := b.actScope
	b.mu.Unlock()
	view, ok := b.computerView()
	if !ok {
		return
	}
	a := buildComputerAction(kind, scope, view, p)
	a.Shot = b.computerShot(view)
	a.At = time.Now()
	sink(scope, a)
}

// buildComputerAction passa píxels CSS a fraccions i emmascara el text.
// Funció pura: és el que proven els tests d'emmascarament i propietari.
func buildComputerAction(kind, scope string, view ComputerView, p computerPoint) ComputerAction {
	fx := func(v float64) float64 { return clamp01(v / view.W) }
	fy := func(v float64) float64 { return clamp01(v / view.H) }
	a := ComputerAction{
		Kind: kind, Owner: BrowserOwnerKey(scope), View: view,
		X: round4(fx(p.x)), Y: round4(fy(p.y)), Dir: p.dir,
		Target: axClip(oneLineText(p.target), 60),
	}
	if len(p.box) == 4 && p.box[2] > 0 && p.box[3] > 0 {
		a.Rect = []float64{round4(fx(p.box[0])), round4(fy(p.box[1])), round4(clamp01(p.box[2] / view.W)), round4(clamp01(p.box[3] / view.H))}
	}
	if p.hasTo {
		tx, ty := round4(fx(p.toX)), round4(fy(p.toY))
		a.ToX, a.ToY = &tx, &ty
	}
	if kind == ComputerType {
		if p.secret {
			a.Text, a.Masked = MaskedText, true
		} else {
			a.Text = clipRunes(oneLineText(p.text), computerTextMax)
		}
	}
	return a
}

// computerShot és la captura de la reproducció: JPEG petit, com a molt 960
// píxels d'ample. Si falla, la reproducció ensenya només el punt.
func (b *Browser) computerShot(view ComputerView) []byte {
	scale := 1.0
	if view.W > 960 {
		scale = 960 / view.W
	}
	res, err := b.call("Page.captureScreenshot", map[string]any{
		"format": "jpeg", "quality": 50, "optimizeForSpeed": true,
		"clip": map[string]any{"x": 0, "y": 0, "width": view.W, "height": view.H, "scale": scale},
	})
	if err != nil {
		return nil
	}
	var r struct {
		Data string `json:"data"`
	}
	if json.Unmarshal(res, &r) != nil || r.Data == "" || len(r.Data) > 2<<20 {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(r.Data)
	if err != nil {
		return nil
	}
	return raw
}

// jsFieldInfo: la caixa d'un element, si és un camp secret i un nom llegible.
// Mateix criteri que jsCredential (on l'agent no escriu) més els noms
// habituals de contrasenyes i PIN.
const jsFieldInfo = `function() {
  const r = this.getBoundingClientRect();
  const t = String(this.type || '').toLowerCase();
  const ac = String(this.autocomplete || (this.getAttribute && this.getAttribute('autocomplete')) || '').toLowerCase();
  const nm = String((this.name || '') + ' ' + (this.id || '')).toLowerCase();
  const secret = t === 'password' || /cc-|one-time-code|current-password|new-password/.test(ac) || /pass|pwd|card.?num|cardnumber|cvc|cvv|iban|otp|pin\b/.test(nm);
  let label = (this.getAttribute && (this.getAttribute('aria-label') || this.getAttribute('placeholder'))) || '';
  if (!label && this.labels && this.labels[0]) label = this.labels[0].innerText || '';
  if (!label) label = (this.innerText || '').slice(0, 80);
  return JSON.stringify({x: r.left, y: r.top, w: r.width, h: r.height, secret: secret, label: String(label).trim()});
}`

type fieldInfo struct {
	X, Y, W, H float64
	Secret     bool
	Label      string
}

func parseFieldInfo(s string) (fieldInfo, bool) {
	var f fieldInfo
	if json.Unmarshal([]byte(s), &f) != nil {
		return fieldInfo{}, false
	}
	return f, true
}

func (f fieldInfo) box() []float64 { return []float64{f.X, f.Y, f.W, f.H} }

// nodeField llegeix la caixa i el tipus d'un node de l'arbre d'accessibilitat.
func (b *Browser) nodeField(node int) (fieldInfo, bool) {
	if loadComputerSink() == nil {
		return fieldInfo{}, false
	}
	s, err := b.callOnString(node, jsFieldInfo)
	if err != nil {
		return fieldInfo{}, false
	}
	return parseFieldInfo(s)
}

// selectorField és el mateix per a un selector CSS.
func (b *Browser) selectorField(sel string) (fieldInfo, bool) {
	if loadComputerSink() == nil {
		return fieldInfo{}, false
	}
	s, err := b.evalString(`(() => { const e = document.querySelector(` + jsonStr(sel) + `); if (!e) return ''; return (` + jsFieldInfo + `).call(e); })()`)
	if err != nil || s == "" {
		return fieldInfo{}, false
	}
	return parseFieldInfo(s)
}

// emitRef emet una acció sobre una ref: centre del node i la seua caixa.
func (b *Browser) emitRef(kind string, r axRef, p computerPoint) {
	if loadComputerSink() == nil {
		return
	}
	if f, ok := b.nodeField(r.node); ok {
		if p.x == 0 && p.y == 0 {
			p.x, p.y = f.X+f.W/2, f.Y+f.H/2
		}
		p.box = f.box()
		p.secret = p.secret || f.Secret
	}
	if p.target == "" {
		p.target = r.name
		if p.target == "" {
			p.target = r.role
		}
	}
	b.emitComputer(kind, p)
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

func oneLineText(s string) string { return strings.Join(strings.Fields(s), " ") }

func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
