package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
)

func fakeModels(t *testing.T, ids ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"data":[` + joinIDs(ids) + `]}`))
	}))
}

func joinIDs(ids []string) string {
	var b strings.Builder
	for i, id := range ids {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"id":"` + id + `"}`)
	}
	return b.String()
}

func TestFetchModelIDs(t *testing.T) {
	srv := fakeModels(t, "b-model", "a-model")
	defer srv.Close()
	ids, err := fetchModelIDs(srv.URL+"/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "a-model" || ids[1] != "b-model" {
		t.Fatalf("ids=%v (volia ordenats)", ids)
	}
	if _, err := fetchModelIDs("http://127.0.0.1:9", ""); err == nil {
		t.Fatal("hauria de fallar amb provider caigut")
	}
}

func TestOverrideIPersistencia(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)
	bot.modelsFile = filepath.Join(t.TempDir(), "models.json")

	if got := bot.overrideFor(99, "chat"); got != "" {
		t.Fatalf("hauria de ser buit: %q", got)
	}
	bot.setOverride(99, "chat", "local-swap/Ornith-1.5-35B")
	if got := bot.overrideFor(99, "chat"); got != "local-swap/Ornith-1.5-35B" {
		t.Fatalf("override=%q", got)
	}
	// Persistit al disc (0600).
	raw, err := os.ReadFile(bot.modelsFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Ornith-1.5-35B") {
		t.Fatalf("no desat: %s", raw)
	}
	// Un bot nou amb el mateix fitxer el recupera.
	tg2 := newFakeTG(t)
	bot2, _, _ := testBot(t, tg2, llm, nil)
	bot2.modelsFile = bot.modelsFile
	bot2.loadModelsOv()
	if got := bot2.overrideFor(99, "chat"); got != "local-swap/Ornith-1.5-35B" {
		t.Fatalf("no recuperat: %q", got)
	}
	// Esborrar torna al defecte.
	bot.setOverride(99, "chat", "")
	if got := bot.overrideFor(99, "chat"); got != "" {
		t.Fatalf("no esborrat: %q", got)
	}
}

func TestRoleRefForAplicaOverride(t *testing.T) {
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)
	bot.modelsFile = filepath.Join(t.TempDir(), "models.json")

	p, r := bot.roleRefFor(99, "chat")
	if r.Model == "" || p.BaseURL == "" {
		t.Fatalf("sense override hauria de tornar el defecte: %+v", r)
	}
	defecte := r.Model

	bot.setOverride(99, "chat", "t/mt")
	_, r2 := bot.roleRefFor(99, "chat")
	if r2.Model != "mt" {
		t.Fatalf("override provider/model no aplicat: %+v", r2)
	}
	// Un altre xat no queda afectat.
	_, r3 := bot.roleRefFor(100, "chat")
	if r3.Model != defecte {
		t.Fatalf("l'override hauria de ser per xat: %+v", r3)
	}
}

func TestModelListITap(t *testing.T) {
	srv := fakeModels(t, "Ornith-1.5-35B", "Nex-2.5-mini")
	defer srv.Close()
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)
	bot.modelsFile = filepath.Join(t.TempDir(), "models.json")
	// El provider de proves dels tests es diu "t".
	for n, p := range bot.cfg.Providers {
		p.BaseURL = srv.URL + "/v1"
		bot.cfg.Providers[n] = p
	}

	ctx := context.Background()
	bot.handleMessage(ctx, msg(1234567, "/model"))
	m, ok := tg.findKB("Models")
	if !ok {
		t.Fatalf("sense llistat: %+v", tg.all())
	}
	var data string
	for _, row := range m.KB {
		for _, b := range row {
			if strings.HasPrefix(b.Data, "m:") && b.Data != "m:clear" && b.Data != "m:list" {
				data = b.Data
			}
		}
	}
	if data == "" {
		t.Fatalf("sense botons de model: %+v", m.KB)
	}
	bot.handleCallback(ctx, cb(1234567, 1234567, data))
	if _, ok := tg.find("→"); !ok {
		t.Fatalf("sense confirmació: %+v", tg.all())
	}
	st := bot.state(1234567)
	if got := bot.overrideFor(1234567, st.role); got == "" {
		t.Fatal("l'override no s'ha desat")
	}
}

func TestModelPickPerNom(t *testing.T) {
	srv := fakeModels(t, "Ornith-1.5-35B", "Nex-2.5-mini")
	defer srv.Close()
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)
	bot.modelsFile = filepath.Join(t.TempDir(), "models.json")
	for n, p := range bot.cfg.Providers {
		p.BaseURL = srv.URL + "/v1"
		bot.cfg.Providers[n] = p
	}

	ctx := context.Background()
	bot.handleMessage(ctx, msg(1234567, "/model ornith"))
	if _, ok := tg.find("→"); !ok {
		t.Fatalf("sense selecció directa: %+v", tg.all())
	}
	st := bot.state(1234567)
	if got := bot.overrideFor(1234567, st.role); !strings.Contains(got, "Ornith") {
		t.Fatalf("override=%q", got)
	}

	bot.handleMessage(ctx, msg(1234567, "/model noexisteix123"))
	if _, ok := tg.find("cap model coincideix"); !ok {
		t.Fatalf("sense error clar: %+v", tg.all())
	}
}

func TestShortModelICurrentRef(t *testing.T) {
	if shortModel("local-swap/Ornith-1.5-35B (triat)") != "Ornith-1.5-35B" {
		t.Fatal("shortModel")
	}
	if shortModel("sol") != "sol" {
		t.Fatal("shortModel sense provider")
	}
}

// El cas real: el router local respon, però Halogen (127.0.0.1:8731) és un
// provider exclusiu que ara és caigut. Abans desapareixia del teclat i
// l'usuari no veia mai el model que havia configurat. Ara hi surt marcat.
func TestModelsMergeConfigQueNoRespon(t *testing.T) {
	srv := fakeModels(t, "Ornith-local")
	defer srv.Close()
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)
	bot.cfg.Providers["t"] = config.Provider{BaseURL: srv.URL + "/v1"}
	// Porta tancada a propòsit: no hi ha res escoltant-hi.
	bot.cfg.Providers["halogen"] = config.Provider{BaseURL: "http://127.0.0.1:9/v1"}
	bot.cfg.Roles = map[string]config.Role{
		// Viu: el rol apunta a un model que la sonda SÍ que ha retornat.
		"chat": {Provider: "t", Model: "Ornith-local"},
		// Caigut: el rol apunta al model que l'usuari va configurar.
		"halogen": {Provider: "halogen", Model: "halogen-qwen3.8-flash-next"},
	}

	refs := bot.refreshModels(context.Background())

	var viu, caigut *ModelRef
	for i := range refs {
		switch refs[i].String() {
		case "t/Ornith-local":
			viu = &refs[i]
		case "halogen/halogen-qwen3.8-flash-next":
			caigut = &refs[i]
		}
	}
	if viu == nil {
		t.Fatalf("el model que respon ha de ser a la llista: %+v", refs)
	}
	if viu.Unavailable {
		t.Fatalf("un model que ha respost no es pot marcar com a no disponible: %+v", *viu)
	}
	if caigut == nil {
		t.Fatalf("el model configurat del provider caigut ha de sortir igualment: %+v", refs)
	}
	if !caigut.Unavailable {
		t.Fatalf("el model del provider caigut s'ha de marcar: %+v", *caigut)
	}
	// No s'ha de duplicar el que ja havia respost.
	n := 0
	for _, r := range refs {
		if r.String() == "t/Ornith-local" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("el model sondejat només hi pot sortir una vegada: %+v", refs)
	}
	// El teclat el mostra marcat, i el nom que es desaria és el net.
	kb := bot.modelKB(99, "chat", refs, nil)
	trobat := false
	for _, row := range kb {
		for _, b := range row {
			if strings.Contains(b.Text, "no disponible") {
				trobat = true
			}
		}
	}
	if !trobat {
		t.Fatalf("el teclat ha de marcar el model caigut: %+v", kb)
	}
	if caigut.Model != "halogen-qwen3.8-flash-next" {
		t.Fatalf("el nom desat ha de ser el net, sense el sufix: %q", caigut.Model)
	}
}

// Quan tot respon, no hi ha cap marca: el comportament no canvia.
func TestModelsSenseMarcaSiTotRespon(t *testing.T) {
	srv := fakeModels(t, "Ornith-local")
	defer srv.Close()
	tg := newFakeTG(t)
	llm := newFakeLLM(t, replyText("x"))
	bot, _, _ := testBot(t, tg, llm, nil)
	bot.cfg.Providers["t"] = config.Provider{BaseURL: srv.URL + "/v1"}
	bot.cfg.Roles = map[string]config.Role{"chat": {Provider: "t", Model: "Ornith-local"}}

	refs := bot.refreshModels(context.Background())
	if len(refs) != 1 || refs[0].Unavailable {
		t.Fatalf("tot respon: la llista ha de ser la sondejada, sense marques: %+v", refs)
	}
	if teNoDisponible(refs) {
		t.Fatalf("sense providers caiguts no hi ha d'haver cap marca: %+v", refs)
	}
}
