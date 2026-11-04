package web

import (
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// El contingut desat d'una veu d'una rotllana NO ha de portar el prefix
// «Nom: »: la identitat viatja per speaker_id/speaker_name i la UI ja posa
// el nom. Si el contingut el porta, surt «Fredolic: Fredolic: …» al xat.
func TestRotllanaVoiceContentHasNoNamePrefix(t *testing.T) {
	s, _ := companionTestServer(t)
	a, _ := s.winds.Save(companion.Wind{Name: "Fredolic", Role: "Builder"})
	b, _ := s.winds.Save(companion.Wind{Name: "Tramuntana", Role: "Reviewer"})
	members := []companion.Wind{a, b}
	reply := "Fredolic: comprova el HID.\n\nTramuntana: és el Modern Standby."
	voices := splitRotllanaVoices(reply, members, a)
	if len(voices) != 2 {
		t.Fatalf("veus = %d, esperava 2", len(voices))
	}
	for _, v := range voices {
		if strings.HasPrefix(v.text, v.wind.Name+":") {
			t.Fatalf("el text de %s encara porta el prefix: %q", v.wind.Name, v.text)
		}
		if strings.Contains(v.text, v.wind.Name+": ") && strings.HasPrefix(strings.TrimSpace(v.text), v.wind.Name) {
			t.Fatalf("prefix repetit a %q", v.text)
		}
	}
	// I el JSON que rep el mòbil tampoc no ha de duplicar el nom.
	rot, err := s.saveRotllana(Rotllana{Name: "test-prefix", Members: []string{a.ID, b.ID}})
	if err != nil {
		t.Fatal(err)
	}
	conversation := []llm.Message{}
	for _, v := range voices {
		conversation = append(conversation, llm.Message{Role: "assistant", Content: v.text, SpeakerID: v.wind.ID, SpeakerName: v.wind.Name, Ts: time.Now().Unix()})
	}
	if err := saveRotllanaHistory(s, rot.ID, conversation); err != nil {
		t.Fatal(err)
	}
	rows := s.rotllanaHistoryJSON(conversation)
	for i, row := range rows {
		content, _ := row["text"].(string)
		name, _ := row["speaker_name"].(string)
		if name != "" && strings.HasPrefix(strings.TrimSpace(content), name+":") {
			t.Fatalf("fila %d: el contingut repeteix el nom %q: %q", i, name, content)
		}
	}
}

// El model pot escriure el prefix amb èmfasi («**Nom:**»): la veu es
// reconeix igual i el text no es queda amb el nom ni amb èmfasi penjat.
func TestRotllanaSplitVeusAmbEmfasi(t *testing.T) {
	A := companion.Wind{ID: "mestral-qa", Name: "Mestral QA"}
	B := companion.Wind{ID: "marinada-qa", Name: "Marinada QA"}
	members := []companion.Wind{A, B}
	vs := splitRotllanaVoices("**Mestral QA:** toc, toc.", members, A)
	if len(vs) != 1 || vs[0].wind.ID != A.ID || vs[0].text != "toc, toc." {
		t.Fatalf("èmfasi al prefix: %+v", vs)
	}
	// Un «**Nom:**» que el model escriu repetint el seu propi prefix es
	// col·lapsa en una sola veu neta, sense el nom dins del text.
	vs = splitRotllanaVoices("Marinada QA: **Marinada QA:** al gra", members, A)
	if len(vs) != 1 || vs[0].wind.ID != B.ID || vs[0].text != "al gra" {
		t.Fatalf("prefix amb nom dins: %+v", vs)
	}
	// «*Nom:*» amb un sol astèris també.
	vs = splitRotllanaVoices("*Marinada QA:* curta", members, A)
	if len(vs) != 1 || vs[0].text != "curta" {
		t.Fatalf("èmfasi simple: %+v", vs)
	}
}
