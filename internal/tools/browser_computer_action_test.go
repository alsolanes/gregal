package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"gregal/internal/identity"
)

// fakePage és un transport CDP mínim: respon el viewport, la caixa del camp
// i la captura, i apunta l'ordre de les crides.
type fakePage struct {
	mu     sync.Mutex
	field  string // JSON de jsFieldInfo
	calls  []string
	typed  string
	closed bool
}

func (f *fakePage) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method)
	p, _ := params.(map[string]any)
	val := func(v any) (json.RawMessage, error) {
		return json.Marshal(map[string]any{"result": map[string]any{"value": v}})
	}
	switch method {
	case "Runtime.evaluate":
		expr, _ := p["expression"].(string)
		switch {
		case strings.Contains(expr, "innerWidth"):
			return val(`{"w":1000,"h":500,"sx":0,"sy":0,"ow":1000,"oh":580,"scw":1000,"sch":600}`)
		case strings.Contains(expr, "getBoundingClientRect") && strings.Contains(expr, "secret"):
			return val(f.field)
		case strings.Contains(expr, "e.focus()"):
			return val(true)
		case strings.Contains(expr, `? "1" : "0"`):
			return val("1")
		}
		return val("")
	case "Page.captureScreenshot":
		return json.Marshal(map[string]any{"data": base64.StdEncoding.EncodeToString([]byte("jpeg!"))})
	case "Input.insertText":
		f.typed, _ = p["text"].(string)
	}
	return json.RawMessage(`{}`), nil
}
func (f *fakePage) Alive() bool { return !f.closed }
func (f *fakePage) Close()      { f.closed = true }

// recull connecta un receptor que guarda les accions (i el desconnecta en
// acabar el test).
func recull(t *testing.T) func() []ComputerAction {
	t.Helper()
	var mu sync.Mutex
	var got []ComputerAction
	var scopes []string
	SetComputerActionSink(func(scope string, a ComputerAction) {
		mu.Lock()
		got = append(got, a)
		scopes = append(scopes, scope)
		mu.Unlock()
	})
	t.Cleanup(func() { SetComputerActionSink(nil) })
	return func() []ComputerAction {
		mu.Lock()
		defer mu.Unlock()
		return append([]ComputerAction(nil), got...)
	}
}

func TestComputerActionEmmascaraContrasenya(t *testing.T) {
	got := recull(t)
	f := &fakePage{field: `{"x":100,"y":50,"w":200,"h":40,"secret":true,"label":"Contrasenya"}`}
	b := &Browser{t: f, actScope: "default"}
	if _, err := b.Type("#pw", "hunter2-supersecret", false); err != nil {
		t.Fatal(err)
	}
	list := got()
	if len(list) != 1 {
		t.Fatalf("accions: %d", len(list))
	}
	a := list[0]
	if a.Kind != ComputerType || !a.Masked || a.Text != MaskedText {
		t.Fatalf("no emmascarat: %+v", a)
	}
	raw, _ := json.Marshal(a)
	if strings.Contains(string(raw), "hunter2") {
		t.Fatalf("el secret surt a l'acció: %s", raw)
	}
	if a.Target != "Contrasenya" || a.Rect == nil || a.Rect[0] != 0.1 || a.Rect[2] != 0.2 {
		t.Fatalf("caixa o objectiu: %+v", a)
	}
	// El centre del camp, en fraccions del viewport.
	if a.X != 0.2 || a.Y != 0.14 {
		t.Fatalf("punt: %v,%v", a.X, a.Y)
	}
	if string(a.Shot) != "jpeg!" {
		t.Fatalf("captura: %q", a.Shot)
	}
	// S'avisa abans d'escriure.
	f.mu.Lock()
	calls := strings.Join(f.calls, ",")
	f.mu.Unlock()
	if strings.Index(calls, "Page.captureScreenshot") > strings.Index(calls, "Input.insertText") {
		t.Fatalf("l'avís ha d'anar abans de l'acció: %s", calls)
	}
}

func TestComputerActionTextNormal(t *testing.T) {
	got := recull(t)
	f := &fakePage{field: `{"x":0,"y":0,"w":100,"h":20,"secret":false,"label":"Adreça"}`}
	b := &Browser{t: f, actScope: "default"}
	if _, err := b.Type("#q", "Carrer Major 1", false); err != nil {
		t.Fatal(err)
	}
	a := got()[0]
	if a.Masked || a.Text != "Carrer Major 1" || a.Target != "Adreça" {
		t.Fatalf("%+v", a)
	}
}

// El propietari surt de la sessió: dues identitats tenen claus diferents i
// mai la del mode local.
func TestComputerActionPropietari(t *testing.T) {
	view := ComputerView{W: 800, H: 600}
	ka, kb := identity.Key("anna"), identity.Key("biel")
	a := buildComputerAction(ComputerClick, ka+"-default", view, computerPoint{x: 400, y: 300, target: "Reserva"})
	b := buildComputerAction(ComputerClick, kb+"-vent-1", view, computerPoint{x: 400, y: 300})
	l := buildComputerAction(ComputerClick, "default", view, computerPoint{x: 400, y: 300})
	if a.Owner != ka || b.Owner != kb || a.Owner == b.Owner {
		t.Fatalf("propietaris: %q %q", a.Owner, b.Owner)
	}
	if l.Owner != OrdinadorLocal {
		t.Fatalf("local: %q", l.Owner)
	}
	if a.X != 0.5 || a.Y != 0.5 || a.Target != "Reserva" {
		t.Fatalf("%+v", a)
	}
}

func TestComputerActionTallaIDesaFora(t *testing.T) {
	view := ComputerView{W: 100, H: 100}
	a := buildComputerAction(ComputerType, "default", view, computerPoint{x: -5, y: 250, text: strings.Repeat("x", 500)})
	if a.X != 0 || a.Y != 1 {
		t.Fatalf("fora del viewport: %v,%v", a.X, a.Y)
	}
	if n := len([]rune(a.Text)); n != computerTextMax+1 {
		t.Fatalf("text de %d", n)
	}
	// Un clic no porta mai text.
	c := buildComputerAction(ComputerClick, "default", view, computerPoint{text: "res"})
	if c.Text != "" {
		t.Fatalf("clic amb text: %q", c.Text)
	}
}

// Sense receptor no es fa cap feina de més (ni captures ni lectures).
func TestComputerActionSenseReceptor(t *testing.T) {
	SetComputerActionSink(nil)
	f := &fakePage{field: `{"x":0,"y":0,"w":10,"h":10,"secret":false}`}
	b := &Browser{t: f}
	if _, err := b.Type("#q", "hola", false); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.calls {
		if c == "Page.captureScreenshot" {
			t.Fatal("captura sense ningú que la vulgui")
		}
	}
}

func TestViewFromWindow(t *testing.T) {
	// Chrome maximitzat en una pantalla de 1280x800 amb 85 px de capçal.
	v := viewFromWindow(1280, 715, 0, 0, 1280, 800, 1280, 800)
	if v.Left != 0 || v.Top != 85 || v.ScreenW != 1280 {
		t.Fatalf("%+v", v)
	}
	// Finestra amb vores de 8 px.
	v = viewFromWindow(1000, 600, 100, 50, 1016, 700, 1920, 1080)
	if v.Left != 108 || v.Top != 50+100-8 {
		t.Fatalf("%+v", v)
	}
	// Headless amb una pantalla virtual més petita que la pàgina.
	v = viewFromWindow(1258, 702, 0, 0, 1280, 800, 800, 600)
	if v.ScreenW != 1258 || v.ScreenH != 702 || v.Left != 0 || v.Top != 0 {
		t.Fatalf("%+v", v)
	}
	// Headless: valors absurds → el viewport és la pantalla.
	v = viewFromWindow(800, 600, 0, 0, 0, 0, 0, 0)
	if v.ScreenW != 800 || v.ScreenH != 600 || v.Left != 0 || v.Top != 0 {
		t.Fatalf("%+v", v)
	}
}
