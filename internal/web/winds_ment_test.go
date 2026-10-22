package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// --- Memòria: extracció de fets sobre l'usuari (sense cites) ---

func stubLearnLoop(t *testing.T, out string) {
	t.Helper()
	saved := windLearnLoop
	windLearnLoop = func(_ *Server, _ context.Context, _ *llm.Client, _ *config.Config, _ config.Provider, _ config.Role, _ []llm.Message) (string, error) {
		return out, nil
	}
	t.Cleanup(func() { windLearnLoop = saved })
}

func mustWindID(t *testing.T, s *Server, id string) companion.Wind {
	t.Helper()
	w, ok := s.winds.ByID(id)
	if !ok {
		t.Fatalf("vent %s inexistent", id)
	}
	return w
}

// TestWindMemoryLearn desa gustos i fets de l'usuari sense demanar cites.
func TestWindMemoryLearn(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Llevant","color":"#29ADFF","role":"sistema"}`)
	stubLearnLoop(t, "- A l'usuari li agrada el cafè curt.\n- Viu a Girona.\n- Soroll sense valor.")

	w := mustWindID(t, s, "llevant")
	s.windMemoryLearnRevision(w, "m'agrada el cafè curt i visc a Girona", "notat!", companion.WindMemoryRevision(s.windStateDir(), "llevant"))

	mem := companion.LoadWindMemory(s.windStateDir(), "llevant")
	if !strings.Contains(mem, "cafè curt") || !strings.Contains(mem, "Girona") {
		t.Fatalf("memòria: %q", mem)
	}
}

// TestWindMemoryRevision: una edició manual posterior guanya a
// l'extracció vella (no ressuscita el que l'usuari ha esborrat).
func TestWindMemoryRevision(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Llevant","color":"#29ADFF","role":"sistema"}`)
	stubLearnLoop(t, "- Li agrada el te.")

	w := mustWindID(t, s, "llevant")
	rev := companion.WindMemoryRevision(s.windStateDir(), "llevant")
	if err := companion.ReplaceWindMemory(s.windStateDir(), "llevant", ""); err != nil {
		t.Fatal(err)
	}
	s.windMemoryLearnRevision(w, "m'agrada el te", "ok", rev)
	if mem := companion.LoadWindMemory(s.windStateDir(), "llevant"); mem != "" {
		t.Fatalf("l'edició manual havia de guanyar: %q", mem)
	}
}

// TestWindMemoriaEndpoint: GET/PUT de la memòria (mòbil, escriptori, mà).
func TestWindMemoriaEndpoint(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Llevant","color":"#29ADFF","role":"sistema"}`)

	put := httptest.NewRequest(http.MethodPut, "/api/winds/memoria", strings.NewReader(`{"wind":"llevant","memoria":"- Li agrada el cava.\n"}`))
	putRec := httptest.NewRecorder()
	s.handleWindMemoria(putRec, put)
	if putRec.Code != 200 {
		t.Fatalf("put: %d %s", putRec.Code, putRec.Body.String())
	}
	get := httptest.NewRequest(http.MethodGet, "/api/winds/memoria?wind=llevant", nil)
	getRec := httptest.NewRecorder()
	s.handleWindMemoria(getRec, get)
	if getRec.Code != 200 || !strings.Contains(getRec.Body.String(), "cava") {
		t.Fatalf("get: %d %s", getRec.Code, getRec.Body.String())
	}
	bad := httptest.NewRequest(http.MethodGet, "/api/winds/memoria?wind=fantasma", nil)
	badRec := httptest.NewRecorder()
	s.handleWindMemoria(badRec, bad)
	if badRec.Code != 400 {
		t.Fatalf("fantasma: %d", badRec.Code)
	}
}

// --- Coneixements: CRUD d'skills per vent ---

func postConeixement(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/winds/coneixements", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleWindConeixements(rec, req)
	return rec
}

// TestWindsResum: tots els vents amb rol i índex de coneixements en un cop.
type resumVent struct {
	ID           string `json:"id"`
	Nom          string `json:"nom"`
	Rol          string `json:"rol"`
	Coneixements []struct {
		Nom        string `json:"nom"`
		Titol      string `json:"titol"`
		Descripcio string `json:"descripcio"`
	} `json:"coneixements"`
}

func TestWindsResum(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Llevant","color":"#29ADFF","role":"cuina"}`)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música"}`)
	rec := postConeixement(t, s, `{"wind":"llevant","nom":"Calçots","titol":"Calçots","descripcio":"Temporada i ritual.","cos":"De gener a març."}`)
	if rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/winds/resum", nil)
	get := httptest.NewRecorder()
	s.handleWindsResum(get, req)
	if get.Code != 200 {
		t.Fatalf("resum: %d %s", get.Code, get.Body.String())
	}
	var d struct {
		Vents []resumVent `json:"vents"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &d); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(d.Vents) != 2 {
		t.Fatalf("vents: %d", len(d.Vents))
	}
	var llevant *resumVent
	for i := range d.Vents {
		if d.Vents[i].ID == "llevant" {
			llevant = &d.Vents[i]
		}
	}
	if llevant == nil || llevant.Rol != "cuina" {
		t.Fatalf("llevant: %+v", llevant)
	}
	if len(llevant.Coneixements) != 1 || llevant.Coneixements[0].Nom != "calçots" {
		t.Fatalf("coneixements: %+v", llevant.Coneixements)
	}
}

// TestWindConeixements: desar, llistar, llegir i esborrar un coneixement.
func TestWindConeixements(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música"}`)

	rec := postConeixement(t, s, `{"wind":"xaloc","nom":"Sardanes","titol":"Sardanes","descripcio":"Dansa catalana en rotllana.","cos":"La sardana es balla en rotllana comptant curts i llargs."}`)
	if rec.Code != 200 {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	// Incomplet: 400.
	rec = postConeixement(t, s, `{"wind":"xaloc","nom":"Buit","descripcio":"res","cos":""}`)
	if rec.Code != 400 {
		t.Fatalf("incomplet: %d", rec.Code)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/winds/coneixements?wind=xaloc", nil)
	getRec := httptest.NewRecorder()
	s.handleWindConeixements(getRec, get)
	if getRec.Code != 200 || !strings.Contains(getRec.Body.String(), "sardanes") {
		t.Fatalf("llistat: %d %s", getRec.Code, getRec.Body.String())
	}
	var list struct {
		Coneixements []struct {
			Nom        string `json:"nom"`
			Descripcio string `json:"descripcio"`
		} `json:"coneixements"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Coneixements) != 1 || list.Coneixements[0].Nom != "sardanes" {
		t.Fatalf("llistat: %+v", list)
	}

	one := httptest.NewRequest(http.MethodGet, "/api/winds/coneixement?wind=xaloc&nom=sardanes", nil)
	oneRec := httptest.NewRecorder()
	s.handleWindConeixement(oneRec, one)
	if oneRec.Code != 200 || !strings.Contains(oneRec.Body.String(), "curts i llargs") {
		t.Fatalf("cos: %d %s", oneRec.Code, oneRec.Body.String())
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/winds/coneixements", strings.NewReader(`{"wind":"xaloc","nom":"sardanes"}`))
	delRec := httptest.NewRecorder()
	s.handleWindConeixements(delRec, del)
	if delRec.Code != 200 {
		t.Fatalf("delete: %d %s", delRec.Code, delRec.Body.String())
	}
	one2 := httptest.NewRequest(http.MethodGet, "/api/winds/coneixement?wind=xaloc&nom=sardanes", nil)
	one2Rec := httptest.NewRecorder()
	s.handleWindConeixement(one2Rec, one2)
	if one2Rec.Code != 404 {
		t.Fatalf("esborrat però encara hi és: %d", one2Rec.Code)
	}
}

// TestWindSkillsSystem: al prompt només hi van les descripcions.
func TestWindSkillsSystem(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música"}`)
	postConeixement(t, s, `{"wind":"xaloc","nom":"Sardanes","descripcio":"Dansa catalana.","cos":"SECRET_COS_LLARG"}`)

	sys := s.windSkillsSystem("xaloc")
	if !strings.Contains(sys, "sardanes") || !strings.Contains(sys, "Dansa catalana") {
		t.Fatalf("índex: %q", sys)
	}
	if strings.Contains(sys, "SECRET_COS_LLARG") {
		t.Fatalf("el cos no pot anar al prompt: %q", sys)
	}
	if got := s.windSkillsSystem("llevant"); got != "" {
		t.Fatalf("vent sense skills: %q", got)
	}
}

// --- Router: només parla qui hi té res a dir ---

func stubRouter(t *testing.T, out string) {
	t.Helper()
	saved := rotllanaRouteLoop
	rotllanaRouteLoop = func(_ *Server, _ context.Context, _ []llm.Message) (string, error) {
		return out, nil
	}
	t.Cleanup(func() { rotllanaRouteLoop = saved })
}

func makeRotllanaTrio(t *testing.T, s *Server) string {
	t.Helper()
	postJSON(t, s.handleWinds, `{"name":"Tramuntana","color":"#29ADFF","role":"sistema"}`)
	postJSON(t, s.handleWinds, `{"name":"Xaloc","color":"#FF9F00","role":"música"}`)
	postJSON(t, s.handleWinds, `{"name":"Llevant","color":"#00C2A8","role":"cuina"}`)
	rec := postJSON(t, s.handleRotllanes, `{"name":"Tria","members":["tramuntana","xaloc","llevant"]}`)
	var saved struct {
		Rotllana Rotllana `json:"rotllana"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	return saved.Rotllana.ID
}

// TestRouterPicksExpert: el router tria l'expert i només ell parla.
func TestRouterPicksExpert(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	postConeixement(t, s, `{"wind":"llevant","nom":"Arrossos","descripcio":"Arrossos i paelles.","cos":"L'arròs bomba necessita..."}`)
	stubRouter(t, `{"parlen":["llevant"]}`)
	var briefs []string
	saved := rotllanaVoiceLoop
	rotllanaVoiceLoop = func(_ *Server, _ context.Context, b string) (exploracio, error) {
		briefs = append(briefs, b)
		return exploracio{text: "l'arròs, 18 minuts"}, nil
	}
	t.Cleanup(func() { rotllanaVoiceLoop = saved })

	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"com faig una paella?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	if len(briefs) != 1 {
		t.Fatalf("havia de parlar 1 vent, han parlat %d", len(briefs))
	}
	if !strings.Contains(rec.Body.String(), `"wind":"llevant"`) {
		t.Fatalf("havia de parlar el Llevant: %s", rec.Body.String())
	}
	// El torn de l'expert porta memòria + índex de skills al brief.
	if !strings.Contains(briefs[0], "Arrossos") {
		t.Fatalf("el brief no porta l'índex de coneixements")
	}
}

// TestRouterSilence: sense expert ni afinitat, silenci (però el missatge es desa).
func TestRouterSilence(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	stubRouter(t, `{"parlen":[]}`)
	called := false
	saved := rotllanaVoiceLoop
	rotllanaVoiceLoop = func(_ *Server, _ context.Context, _ string) (exploracio, error) {
		called = true
		return exploracio{text: "jo parlo igualment"}, nil
	}
	t.Cleanup(func() { rotllanaVoiceLoop = saved })

	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"zzzz sense sentit"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("silenci trencat: ha parlat un vent")
	}
	if !strings.Contains(rec.Body.String(), `"silenci":true`) {
		t.Fatalf("falta el marcador: %s", rec.Body.String())
	}
	get := httptest.NewRequest(http.MethodGet, "/api/winds/rotllana?rotlla="+id, nil)
	getRec := httptest.NewRecorder()
	s.handleRotllanaChat(getRec, get)
	if !strings.Contains(getRec.Body.String(), "zzzz sense sentit") {
		t.Fatalf("el missatge de l'usuari s'havia de desar: %s", getRec.Body.String())
	}
}

// TestRouterFallback: si el router diu ximpleries, el torn clàssic (un sol vent).
func TestRouterFallback(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	stubRouter(t, `això no és JSON`)
	count := 0
	saved := rotllanaVoiceLoop
	rotllanaVoiceLoop = func(_ *Server, _ context.Context, _ string) (exploracio, error) {
		count++
		return exploracio{text: "resposta clàssica"}, nil
	}
	t.Cleanup(func() { rotllanaVoiceLoop = saved })

	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"hola a tothom"}`)
	if rec.Code != 200 || count != 1 {
		t.Fatalf("fallback: %d %s (crides=%d)", rec.Code, rec.Body.String(), count)
	}
	if strings.Contains(rec.Body.String(), `"silenci":true`) {
		t.Fatalf("el fallback no pot ser silenci: %s", rec.Body.String())
	}
}
