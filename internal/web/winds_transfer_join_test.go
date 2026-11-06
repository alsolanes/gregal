package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/llm"
	"gregal/internal/skills"
)

func transferTestJoin(t *testing.T, s *Server, id, into string, b transferBundle) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"transferId": id, "bundle": b, "into": into})
	rec := httptest.NewRecorder()
	s.handleWindsTransfer(rec, httptest.NewRequest(http.MethodPost, "/api/winds/transfer", bytes.NewReader(raw)))
	return rec
}

// Amb `into`, la conversa del mòbil s'afegeix al vent del servidor que es diu
// igual: cap vent nou, el que ja sabia es manté i un reintent no duplica res.
func TestWindTransferJoinsNamesake(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	hub := testHubWithUsers(t)
	phone := hub.SessionFor(&User{Name: "phone"}, "one")
	server := hub.SessionFor(&User{Name: "server"}, "one")
	pw, _ := phone.winds.Save(companion.Wind{Name: "Garbí", Color: "#112233", Persona: "Esports", Style: "pso"})
	_ = companion.ReplaceWindMemory(phone.windStateDir(), pw.ID, "- Juga a pàdel.\n- Viu a Girona.\n")
	_ = phone.saveWindSkill(pw.ID, skills.Skill{Nom: "rutina", Titol: "Rutina", Descripcio: "Rutina d'exercici", Cos: "3 dies."})
	_ = saveWindHistory(phone, pw, []llm.Message{{Role: "user", Content: "Quin esport faig?", Ts: 200}, {Role: "assistant", Content: "Natació.", Ts: 201}})

	sw, _ := server.winds.Save(companion.Wind{Name: "Garbí", Color: "#abcdef", Persona: "Esports del servidor"})
	_ = companion.ReplaceWindMemory(server.windStateDir(), sw.ID, "- Viu a Girona.\n")
	_ = saveWindHistory(server, sw, []llm.Message{{Role: "user", Content: "Com va el Barça?", Ts: 100}, {Role: "assistant", Content: "Va guanyar.", Ts: 101}})
	list0, _ := server.winds.List()
	before := len(list0)

	b, err := phone.exportTransfer("wind", pw.ID, transferConsent{Memory: true, Skills: true})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // el segon és un reintent idèntic
		rec := transferTestJoin(t, server, "join-request-01", sw.ID, b)
		if rec.Code != 200 {
			t.Fatalf("join %d: %d %s", i, rec.Code, rec.Body.String())
		}
		var res transferResult
		_ = json.Unmarshal(rec.Body.Bytes(), &res)
		if res.ID != sw.ID || res.VerifyPath != "/api/winds/chat?wind="+sw.ID {
			t.Fatalf("result: %+v", res)
		}
	}
	if list1, _ := server.winds.List(); len(list1) != before {
		n := len(list1)
		t.Fatalf("no s'ha de crear cap vent: %d → %d", before, n)
	}
	h, _ := server.transferHistory("wind", sw.ID)
	var got []string
	for _, m := range h {
		got = append(got, m.Content)
	}
	if strings.Join(got, "|") != "Com va el Barça?|Va guanyar.|Quin esport faig?|Natació." {
		t.Fatalf("historial: %v", got)
	}
	mem := companion.LoadWindMemory(server.windStateDir(), sw.ID)
	if strings.Count(mem, "Girona") != 1 || !strings.Contains(mem, "pàdel") {
		t.Fatalf("memòria: %q", mem)
	}
	if p, _ := server.winds.ByID(sw.ID); p.Persona != "Esports del servidor" {
		t.Fatalf("el perfil del servidor no es toca: %q", p.Persona)
	}
	found := false
	for _, sk := range server.windSkills(sw.ID) {
		found = found || sk.Nom == "rutina"
	}
	if !found {
		t.Fatal("la skill marcada s'ha d'afegir")
	}
	// Un vent que no existeix: error, mai una còpia d'amagat.
	if rec := transferTestJoin(t, server, "join-request-02", "no-such-wind", b); rec.Code != 409 {
		t.Fatalf("destí inexistent: %d", rec.Code)
	}
}
