package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// Un vent acabat de crear no surt com a «sense ús»: encara no has tingut temps
// de fer-lo servir. Un de fa més de 7 dies, o un de desat abans que hi hagués
// data de creació, sí.
func TestTidyUnusedSkipsNewWinds(t *testing.T) {
	s := remindTestServer(t)
	if _, err := s.winds.Save(companion.Wind{Name: "Marinada"}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if err := s.winds.Restore(companion.Wind{ID: "garbi", Name: "Garbí", Created: old}); err != nil {
		t.Fatal(err)
	}
	if err := s.winds.Restore(companion.Wind{ID: "llebeig", Name: "Llebeig"}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleWindTidy(rec, httptest.NewRequest(http.MethodGet, "/api/winds/tidy", nil))
	var rep tidyReport
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatalf("%v: %s", err, rec.Body.String())
	}
	var ids []string
	for _, w := range rep.Unused {
		ids = append(ids, w.ID)
	}
	if strings.Join(ids, ",") != "garbi,llebeig" {
		t.Fatalf("sense ús: %v, volia garbi i llebeig (Xaloc i Marinada són nous)", ids)
	}
}

func tidySeedThread(t *testing.T, s *Server, windID, text string) {
	t.Helper()
	if _, err := session.SaveSession(s.sessDir(), session.Session{Name: "company-" + windID, Title: "Company — x", Convo: []llm.Message{
		{Role: "user", Content: text}, {Role: "assistant", Content: "D'acord."},
	}}); err != nil {
		t.Fatal(err)
	}
}

// Dues Marinades i un «fdsaf»: «Posa ordre» les troba; ajuntar-les deixa
// una sola Marinada amb la memòria de les dues, totes les converses, les
// rutines i els grups; l'altra queda arxivada (amb còpia), no esborrada.
func TestTidyFindsDuplicatesAndMergesEverything(t *testing.T) {
	s := remindTestServer(t)
	a, err := s.winds.Save(companion.Wind{Name: "Marinada", Role: "Compres i comparacions"})
	if err != nil {
		t.Fatal(err)
	}
	// Com els duplicats de debò: un altre id (w-…) amb el mateix nom.
	b, err := s.winds.SaveNew(companion.Wind{Name: "marinada", Role: "calma i benestar"}, "w-0123456789abcdef01234567")
	if err != nil {
		t.Fatal(err)
	}
	junk, _ := s.winds.Save(companion.Wind{Name: "fdsaf"})
	_ = companion.ReplaceWindMemory(s.windStateDir(), a.ID, "- Li agraden els preus concrets")
	_ = companion.ReplaceWindMemory(s.windStateDir(), b.ID, "- Fa ioga els dimarts")
	tidySeedThread(t, s, a.ID, "Compara'm mòbils")
	tidySeedThread(t, s, a.ID+"-s1", "segona conversa de la A")
	tidySeedThread(t, s, b.ID, "Com respiro millor?")
	if _, err := s.jobs.Save(jobs.Job{ID: "rut-b", Name: "Respira", Kind: "daily", Wind: b.ID, Prompt: "Recorda'm respirar", Time: "09:00"}); err != nil {
		t.Fatal(err)
	}
	rot, err := s.saveRotllana(Rotllana{Name: "Benestar", Members: []string{b.ID, junk.ID}})
	if err != nil {
		t.Fatal(err)
	}

	rep, err := s.windTidyReport()
	if err != nil {
		t.Fatal(err)
	}
	var g *tidyGroup
	for i := range rep.Groups {
		if rep.Groups[i].Reason == "same_name" {
			g = &rep.Groups[i]
		}
	}
	if g == nil || len(g.Winds) != 2 || g.Keep != a.ID {
		t.Fatalf("havia de trobar les dues Marinades i quedar-se la que té més: %+v", rep.Groups)
	}
	if len(rep.Test) != 1 || rep.Test[0].ID != junk.ID {
		t.Fatalf("«fdsaf» és de prova: %+v", rep.Test)
	}

	res, err := s.windMerge(a.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Archived || res.Threads != 1 || res.Jobs != 1 || res.Groups != 1 || !res.Memory {
		t.Fatalf("resultat: %+v", res)
	}
	if _, ok := s.winds.ByID(b.ID); ok {
		t.Fatal("la Marinada absorbida havia de quedar arxivada")
	}
	mem := companion.LoadWindMemory(s.windStateDir(), a.ID)
	if !strings.Contains(mem, "preus concrets") || !strings.Contains(mem, "ioga") {
		t.Fatalf("memòria ajuntada: %q", mem)
	}
	threads := s.windSessionList(a.ID)
	if len(threads) != 3 {
		t.Fatalf("totes les converses a la que es queda: %+v", threads)
	}
	if len(s.windSessionList(b.ID)) != 0 {
		t.Fatal("cap conversa ha de quedar a nom de l'absorbida")
	}
	j, _ := s.jobs.List()
	for _, x := range j {
		if x.ID == "rut-b" && x.Wind != a.ID {
			t.Fatalf("la rutina havia de passar a la que es queda: %+v", x)
		}
	}
	got, _ := s.rotllanaByID(rot.ID)
	if !containsStr(got.Members, a.ID) || containsStr(got.Members, b.ID) {
		t.Fatalf("grup: %+v", got.Members)
	}
	// Arxivada vol dir recuperable: és a la biblioteca.
	lib, _ := s.windLibrary()
	found := false
	for _, it := range lib {
		if it.Kind == "archived" && it.Wind.ID == b.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("la Marinada absorbida havia de quedar a la biblioteca, recuperable")
	}
	// Gregal no es pot absorbir.
	if _, err := s.windMerge(a.ID, "gregal"); err == nil {
		t.Fatal("Gregal no s'ha de poder ajuntar a un altre")
	}
}

func TestTidyLooksTest(t *testing.T) {
	for _, n := range []string{"fdsaf", "asdf", "qwer", "x", "Test", "zxcv", "brrr"} {
		if !tidyLooksTest(n) {
			t.Errorf("%q havia de semblar de prova", n)
		}
	}
	for _, n := range []string{"Marinada", "Garbí", "Mist", "Rime", "Dew", "Glint", "Ponent"} {
		if tidyLooksTest(n) {
			t.Errorf("%q és un nom de debò", n)
		}
	}
}
