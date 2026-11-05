package web

import (
	"context"
	"encoding/json"

	"gregal/internal/companion"
	"gregal/internal/llm"
	"strings"
	"testing"
)

// «@Llevant i @Xaloc, …»: cada nomenat respon un cop, amb la seva crida. Abans
// cada crida escrivia les veus dels dos i el torn sortia A, B, A, B.
func TestRotllanaTwoMentionsEachSpeaksOnce(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	id := makeRotllanaTrio(t, s)
	var briefs []string
	saved := rotllanaVoiceLoop
	rotllanaVoiceLoop = func(_ *Server, _ context.Context, b string) (exploracio, error) {
		briefs = append(briefs, b)
		// El model fa el mateix que hem vist en real: escriu les dues veus.
		return exploracio{text: "Llevant: Menja un plàtan abans.\nXaloc: I posa't música animada."}, nil
	}
	t.Cleanup(func() { rotllanaVoiceLoop = saved })

	rec := postJSON(t, s.handleRotllanaChat, `{"rotllana":"`+id+`","message":"@Llevant i @Xaloc, què faig abans d'entrenar?"}`)
	if rec.Code != 200 {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body.String())
	}
	if len(briefs) != 2 {
		t.Fatalf("havien de parlar 2 vents, crides=%d", len(briefs))
	}
	for i, who := range []string{"Llevant", "Xaloc"} {
		if !strings.Contains(briefs[i], "Now only YOU, "+who) {
			t.Fatalf("la crida %d no diu que només parla %s: ...%s", i, who, briefs[i][max(0, len(briefs[i])-400):])
		}
	}
	var out struct {
		History []map[string]any `json:"history"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	var voices []string
	for _, m := range out.History {
		if m["from"] != "user" {
			voices = append(voices, m["speaker_name"].(string)+"|"+m["text"].(string))
		}
	}
	want := []string{"Llevant|Menja un plàtan abans.", "Xaloc|I posa't música animada."}
	if strings.Join(voices, "\n") != strings.Join(want, "\n") {
		t.Fatalf("veus = %q, volia %q", voices, want)
	}
}

func TestOwnVoiceOnlyDropsOtherMembers(t *testing.T) {
	a, b := windNamed("a", "A"), windNamed("b", "B")
	got := ownVoiceOnly([]rotllanaVoice{{a, "u"}, {b, "dos"}, {a, "tres"}}, a)
	if len(got) != 1 || got[0].wind.ID != "a" || got[0].text != "u\n\ntres" {
		t.Fatalf("%+v", got)
	}
	if ownVoiceOnly([]rotllanaVoice{{b, "x"}}, a) != nil {
		t.Fatal("sense veu pròpia, res")
	}
}

func windNamed(id, name string) companion.Wind { return companion.Wind{ID: id, Name: name} }

// L'etiqueta s'arrenca, l'èmfasi del text no; un nom després de l'etiqueta és
// algú a qui es parla, no qui parla.
func TestSplitVoicesKeepsEmphasisAndAddressees(t *testing.T) {
	anna, bob := windNamed("anna", "Anna"), windNamed("bob", "Bob")
	members := []companion.Wind{anna, bob}
	cases := []struct{ in, who, text string }{
		{"Anna: **Pla** és la clau.", "anna", "**Pla** és la clau."},
		{"**Anna:** **Pla** és la clau.", "anna", "**Pla** és la clau."},
		{"**Anna**: Fem-ho.", "anna", "Fem-ho."},
		{"Anna: **Anna:** Fem-ho.", "anna", "Fem-ho."},
		{"Anna: Bob: estàs equivocat.", "anna", "Bob: estàs equivocat."},
	}
	for _, c := range cases {
		v := splitRotllanaVoices(c.in, members, bob)
		if len(v) != 1 || v[0].wind.ID != c.who || v[0].text != c.text {
			t.Errorf("%q → %+v, volia %s %q", c.in, v, c.who, c.text)
		}
	}
}

func TestCrewSpeaksTheLanguageOfTheTask(t *testing.T) {
	if !crewSpeaksCatalan("Feu una pàgina web senzilla amb un comptador i proveu que funciona.", "en") {
		t.Fatal("encàrrec en català amb config en anglès: ha de parlar català")
	}
	if crewSpeaksCatalan("Build a simple web page with a click counter and test that it works.", "ca") {
		t.Fatal("encàrrec en anglès: anglès")
	}
	if !crewSpeaksCatalan("ok", "ca") {
		t.Fatal("sense prou text, la configuració")
	}
}

// Un vent nou amb el nom d'un d'arxivat no hereta el seu historial.
func TestNewWindDoesNotInheritArchivedHistory(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	postJSON(t, s.handleWinds, `{"name":"Llevant","color":"#00C2A8","role":"cuina"}`)
	hist := windHistory(s, "llevant")
	w, _ := s.winds.ByID("llevant")
	if err := saveWindHistory(s, w, append(hist.messages, llm.Message{Role: "user", Content: "secret antic"})); err != nil {
		t.Fatal(err)
	}
	if err := s.archiveWind("llevant"); err != nil {
		t.Fatal(err)
	}
	rec := postJSON(t, s.handleWinds, `{"name":"Llevant","color":"#00C2A8","role":"cuina"}`)
	var out struct{ Wind companion.Wind }
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Wind.ID == "llevant" {
		t.Fatal("el vent nou ha reaprofitat l'id de l'arxivat (i el seu historial)")
	}
	if out.Wind.Name != "Llevant" || out.Wind.ID != "llevant-2" {
		t.Fatalf("vent nou = %+v", out.Wind)
	}
}

// Un encàrrec sense fitxers no pot tombar la volta: treballa sense seient aïllat.
func TestCrewRoundFlowBuilderWithoutFilesIsNotIsolated(t *testing.T) {
	members := []companion.Wind{windNamed("a", "A"), windNamed("b", "B")}
	flow := crewRoundFlow([]crewAssignment{{Wind: "a", Part: "web", Files: []string{"index.html"}}, {Wind: "b", Part: "proves"}}, members,
		func(companion.Wind, bool) string { return "x" }, true)
	got := map[string]bool{}
	for _, n := range flow.Nodes {
		if n.Role == "builder" {
			got[n.Wind] = n.CrewAssigned
		}
	}
	if !got["a"] || got["b"] {
		t.Fatalf("aïllats: %v (només el que té fitxers)", got)
	}
}
