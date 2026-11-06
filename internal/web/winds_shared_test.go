package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/config"
)

type sharedRig struct {
	aleix, carla, pere *Server
	mestral, secret    companion.Wind
	briefs             []string
}

// sharedTestRig: un servidor amb tres comptes; l'Aleix té dos vents (un dels
// quals no és al xat) i memòria personal amb un secret.
func sharedTestRig(t *testing.T) *sharedRig {
	t.Helper()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", filepath.Join(t.TempDir(), "memory.md"))
	s := goalTestServer(t)
	cfg := &config.Config{Users: map[string]config.UserCfg{
		"aleix": {Password: HashPassword("clau-aleix-1")},
		"carla": {Password: HashPassword("clau-carla-1")},
		"pere":  {Password: HashPassword("clau-pere-01")},
	}}
	s.users = NewUsers(cfg, "")
	s.users.path = filepath.Join(t.TempDir(), "tokens.json")
	h := s.Hub()
	rig := &sharedRig{
		aleix: h.SessionFor(s.users.Usuari("aleix"), "t"),
		carla: h.SessionFor(s.users.Usuari("carla"), "t"),
		pere:  h.SessionFor(s.users.Usuari("pere"), "t"),
	}
	var err error
	if rig.mestral, err = rig.aleix.winds.Save(companion.Wind{Name: "Mestral", Persona: "Parla de muntanya.", Role: "muntanya"}); err != nil {
		t.Fatal(err)
	}
	if rig.secret, err = rig.aleix.winds.Save(companion.Wind{Name: "Garbí", Persona: "PERSONA-PRIVADA-GARBI", Role: "salut"}); err != nil {
		t.Fatal(err)
	}
	if err := rig.aleix.rememberPersonal("SECRET-ALEIX: el codi de la porta és 4321"); err != nil {
		t.Fatal(err)
	}
	saved := sharedVoiceLoop
	sharedVoiceLoop = func(_ *Server, _ context.Context, brief string) (exploracio, error) {
		rig.briefs = append(rig.briefs, brief)
		return exploracio{text: "Mestral: Hola Carla, avui fa sol al Montseny."}, nil
	}
	t.Cleanup(func() { sharedVoiceLoop = saved })
	return rig
}

func sharedCall(t *testing.T, s *Server, h func(*Server, http.ResponseWriter, *http.Request), method, target, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h(s, rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (rig *sharedRig) create(t *testing.T, people ...string) string {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"name": "Excursió", "winds": []string{rig.mestral.ID}, "people": people})
	code, out := sharedCall(t, rig.aleix, (*Server).handleSharedChats, "POST", "/", string(b))
	if code != 200 {
		t.Fatalf("create: %d %v", code, out)
	}
	return out["chat"].(map[string]any)["id"].(string)
}

func listOf(t *testing.T, s *Server) (chats, invites []any) {
	t.Helper()
	code, out := sharedCall(t, s, (*Server).handleSharedChats, "GET", "/", "")
	if code != 200 {
		t.Fatalf("list: %d %v", code, out)
	}
	return out["chats"].([]any), out["invites"].([]any)
}

func TestSharedChatInviteAcceptAndTalk(t *testing.T) {
	rig := sharedTestRig(t)
	id := rig.create(t, "carla")

	chats, invites := listOf(t, rig.carla)
	if len(chats) != 0 || len(invites) != 1 {
		t.Fatalf("la Carla ha de tenir una invitació i cap xat: %v %v", chats, invites)
	}
	// Abans d'acceptar no el pot llegir ni escriure-hi.
	if code, _ := sharedCall(t, rig.carla, (*Server).handleSharedChat, "GET", "/?id="+id, ""); code != 404 {
		t.Fatalf("convidada sense acceptar llegeix: %d", code)
	}
	if code, _ := sharedCall(t, rig.carla, (*Server).handleSharedChat, "POST", "/", `{"id":"`+id+`","message":"hola"}`); code != 404 {
		t.Fatalf("convidada sense acceptar escriu: %d", code)
	}
	if code, out := sharedCall(t, rig.carla, (*Server).handleSharedAnswer, "POST", "/", `{"id":"`+id+`","accept":true}`); code != 200 || out["status"] != "accepted" {
		t.Fatalf("accept: %d %v", code, out)
	}
	chats, invites = listOf(t, rig.carla)
	if len(chats) != 1 || len(invites) != 0 {
		t.Fatalf("després d'acceptar: %v %v", chats, invites)
	}
	// La Carla escriu; respon el vent de l'Aleix.
	code, out := sharedCall(t, rig.carla, (*Server).handleSharedChat, "POST", "/", `{"id":"`+id+`","message":"Quin temps fa per anar d'excursió?"}`)
	if code != 200 {
		t.Fatalf("post: %d %v", code, out)
	}
	hist := out["history"].([]any)
	if len(hist) != 2 {
		t.Fatalf("history: %v", hist)
	}
	first, second := hist[0].(map[string]any), hist[1].(map[string]any)
	if first["from"] != "user" || first["speaker_name"] != "carla" || first["mine"] != true {
		t.Fatalf("el missatge ha de dir que és de la Carla: %v", first)
	}
	if second["speaker_id"] != rig.mestral.ID || second["text"] != "Hola Carla, avui fa sol al Montseny." {
		t.Fatalf("resposta del vent: %v", second)
	}
	// L'Aleix veu el mateix fil, amb el missatge de la Carla com a no seu.
	code, out = sharedCall(t, rig.aleix, (*Server).handleSharedChat, "GET", "/?id="+id, "")
	if code != 200 {
		t.Fatalf("owner get: %d", code)
	}
	h0 := out["history"].([]any)[0].(map[string]any)
	if h0["mine"] != false || h0["author"] != "carla" {
		t.Fatalf("per a l'Aleix el missatge és de la Carla: %v", h0)
	}
	// since: només el que és nou.
	ts := int64(second["ts"].(float64))
	if _, out = sharedCall(t, rig.aleix, (*Server).handleSharedChat, "GET", "/?id="+id+"&since="+itoa64(ts), ""); len(out["history"].([]any)) != 0 {
		t.Fatalf("since ha de filtrar: %v", out["history"])
	}
	// El vent sap que hi ha diverses persones i qui ha escrit.
	if len(rig.briefs) != 1 {
		t.Fatalf("briefs: %d", len(rig.briefs))
	}
	b := rig.briefs[0]
	for _, want := range []string{"several humans", "aleix, carla", "HUMAN carla: Quin temps", "latest message is from carla"} {
		if !strings.Contains(b, want) {
			t.Fatalf("el brief no diu %q:\n%s", want, b)
		}
	}
}

func TestSharedChatIsolation(t *testing.T) {
	rig := sharedTestRig(t)
	id := rig.create(t, "carla")
	sharedCall(t, rig.carla, (*Server).handleSharedAnswer, "POST", "/", `{"id":"`+id+`","accept":true}`)
	sharedCall(t, rig.carla, (*Server).handleSharedChat, "POST", "/", `{"id":"`+id+`","message":"Què saps de l'Aleix?"}`)

	// Ni memòria personal de l'Aleix ni la persona d'un vent que no és al xat.
	for _, b := range rig.briefs {
		if strings.Contains(b, "SECRET-ALEIX") || strings.Contains(b, "4321") {
			t.Fatal("la memòria personal del propietari ha arribat al brief del xat compartit")
		}
		if strings.Contains(b, "PERSONA-PRIVADA-GARBI") || strings.Contains(b, "Garbí") {
			t.Fatal("un vent que no és al xat ha arribat al brief")
		}
	}
	// La vista de la Carla només porta els vents del xat.
	_, out := sharedCall(t, rig.carla, (*Server).handleSharedChat, "GET", "/?id="+id, "")
	raw, _ := json.Marshal(out)
	if strings.Contains(string(raw), "Garbí") || strings.Contains(string(raw), "PERSONA") || strings.Contains(string(raw), "Parla de muntanya") {
		t.Fatalf("la vista del xat filtra dades privades: %s", raw)
	}
	// Els vents de la Carla continuen sent els seus (cap de l'Aleix).
	if list, _ := rig.carla.winds.List(); len(list) != 0 {
		t.Fatalf("la Carla veu vents de l'Aleix: %v", list)
	}
	// No pot fer servir els vents de l'Aleix per crear un xat seu.
	b, _ := json.Marshal(map[string]any{"name": "Robatori", "winds": []string{rig.secret.ID}, "people": []string{"aleix"}})
	if code, _ := sharedCall(t, rig.carla, (*Server).handleSharedChats, "POST", "/", string(b)); code != 400 {
		t.Fatalf("crear amb vents d'un altre: %d", code)
	}
	// No pot editar el xat de l'Aleix (afegir el Garbí o convidar el Pere).
	b, _ = json.Marshal(map[string]any{"id": id, "name": "X", "winds": []string{rig.secret.ID}, "people": []string{"pere"}})
	if code, _ := sharedCall(t, rig.carla, (*Server).handleSharedChats, "POST", "/", string(b)); code != 400 && code != 404 {
		t.Fatalf("editar el xat d'un altre: %d", code)
	}
	// El Pere, que no hi és convidat, no en sap res.
	chats, invites := listOf(t, rig.pere)
	if len(chats)+len(invites) != 0 {
		t.Fatalf("el Pere veu el xat: %v %v", chats, invites)
	}
	for _, c := range []struct {
		h    func(*Server, http.ResponseWriter, *http.Request)
		m, u string
		body string
	}{
		{(*Server).handleSharedChat, "GET", "/?id=" + id, ""},
		{(*Server).handleSharedChat, "POST", "/", `{"id":"` + id + `","message":"hola"}`},
		{(*Server).handleSharedAnswer, "POST", "/", `{"id":"` + id + `","accept":true}`},
		{(*Server).handleSharedLeave, "POST", "/", `{"id":"` + id + `"}`},
	} {
		if code, _ := sharedCall(t, rig.pere, c.h, c.m, c.u, c.body); code != 404 {
			t.Fatalf("el Pere %s %s: %d", c.m, c.u, code)
		}
	}
	// Un id amb camí no surt del directori.
	if code, _ := sharedCall(t, rig.carla, (*Server).handleSharedChat, "GET", "/?id=../index", ""); code != 404 {
		t.Fatalf("id amb camí: %d", code)
	}
}

func TestSharedChatDeclineLeaveDelete(t *testing.T) {
	rig := sharedTestRig(t)
	id := rig.create(t, "carla", "pere")
	if code, _ := sharedCall(t, rig.pere, (*Server).handleSharedAnswer, "POST", "/", `{"id":"`+id+`","accept":false}`); code != 200 {
		t.Fatal("decline")
	}
	if chats, invites := listOf(t, rig.pere); len(chats)+len(invites) != 0 {
		t.Fatal("rebutjat: no ha de quedar res")
	}
	// Ja no pot acceptar una invitació rebutjada.
	if code, _ := sharedCall(t, rig.pere, (*Server).handleSharedAnswer, "POST", "/", `{"id":"`+id+`","accept":true}`); code != 404 {
		t.Fatalf("acceptar després de rebutjar: %d", code)
	}
	sharedCall(t, rig.carla, (*Server).handleSharedAnswer, "POST", "/", `{"id":"`+id+`","accept":true}`)
	if code, _ := sharedCall(t, rig.carla, (*Server).handleSharedLeave, "POST", "/", `{"id":"`+id+`"}`); code != 200 {
		t.Fatal("leave")
	}
	if code, _ := sharedCall(t, rig.carla, (*Server).handleSharedChat, "GET", "/?id="+id, ""); code != 404 {
		t.Fatal("després de sortir encara llegeix")
	}
	// L'Aleix la torna a convidar: invitació nova.
	b, _ := json.Marshal(map[string]any{"id": id, "name": "Excursió", "winds": []string{rig.mestral.ID}, "people": []string{"carla"}})
	if code, _ := sharedCall(t, rig.aleix, (*Server).handleSharedChats, "POST", "/", string(b)); code != 200 {
		t.Fatal("reinvite")
	}
	if _, invites := listOf(t, rig.carla); len(invites) != 1 {
		t.Fatal("la Carla ha de tenir la invitació nova")
	}
	sharedCall(t, rig.aleix, (*Server).handleSharedChat, "POST", "/", `{"id":"`+id+`","message":"hola"}`)
	if _, err := os.Stat(rig.aleix.sharedHistoryPath(id)); err != nil {
		t.Fatal("no hi ha fil")
	}
	// El propietari en surt = l'esborra per a tothom.
	if code, out := sharedCall(t, rig.aleix, (*Server).handleSharedLeave, "POST", "/", `{"id":"`+id+`"}`); code != 200 || out["deleted"] != true {
		t.Fatalf("delete: %d %v", code, out)
	}
	if _, err := os.Stat(rig.aleix.sharedHistoryPath(id)); !os.IsNotExist(err) {
		t.Fatal("el fil ha de desaparèixer")
	}
	if _, invites := listOf(t, rig.carla); len(invites) != 0 {
		t.Fatal("la invitació d'un xat esborrat ha de marxar")
	}
}

func TestSharedChatValidation(t *testing.T) {
	rig := sharedTestRig(t)
	for _, people := range [][]string{{"ningu"}, {}, {"aleix"}} {
		b, _ := json.Marshal(map[string]any{"name": "X", "winds": []string{rig.mestral.ID}, "people": people})
		if code, _ := sharedCall(t, rig.aleix, (*Server).handleSharedChats, "POST", "/", string(b)); code != 400 {
			t.Fatalf("people %v: %d", people, code)
		}
	}
	_, out := sharedCall(t, rig.aleix, (*Server).handleSharedPeople, "GET", "/", "")
	got, _ := json.Marshal(out["people"])
	if string(got) != `["carla","pere"]` {
		t.Fatalf("people: %s", got)
	}
	// Sense compte (mode local) no n'hi ha.
	local := goalTestServer(t)
	if code, _ := sharedCall(t, local, (*Server).handleSharedChats, "GET", "/", ""); code != 404 {
		t.Fatalf("mode local: %d", code)
	}
}

func TestSharedSpeakers(t *testing.T) {
	a := companion.Wind{ID: "a", Name: "Mestral"}
	b := companion.Wind{ID: "b", Name: "Xaloc"}
	c := SharedChat{Owner: "aleix", People: []SharedPerson{{Name: "carla", Status: sharedAccepted}}}
	if got := sharedSpeakers(c, []companion.Wind{a, b}, "@carla vens dissabte?", nil); len(got) != 0 {
		t.Fatalf("un missatge per a una persona no fa parlar cap vent: %v", got)
	}
	if got := sharedSpeakers(c, []companion.Wind{a, b}, "xaloc, què en penses?", nil); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("nomenat: %v", got)
	}
	hist := []sharedMsg{{Kind: "wind", Author: "a"}}
	if got := sharedSpeakers(c, []companion.Wind{a, b}, "i ara?", hist); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("torn: %v", got)
	}
	if sharedMentions("mestralet diu", "mestral") {
		t.Fatal("paraula sencera")
	}
}

func itoa64(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
