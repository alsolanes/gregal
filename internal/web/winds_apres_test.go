package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// «Ho recordaré» no va mai al model; Què sap de tu ho ensenya i ho oblida.

func TestMemoryNoticeStaysOutOfTheModel(t *testing.T) {
	hist := []llm.Message{
		{Role: "user", Content: "Fes-ho curt"},
		{Role: "assistant", Content: "D'acord."},
		{Role: "assistant", Kind: memoryKind, Content: "Prefereix respostes curtes"},
	}
	for _, m := range windChatTurns(hist) {
		if strings.Contains(m.Content, "Prefereix respostes curtes") {
			t.Fatal("l'avís de memòria no pot anar al model")
		}
	}
	rows := petHistoryJSON(hist)
	if rows[2]["kind"] != memoryKind {
		t.Fatalf("history = %+v", rows[2])
	}
}

func TestApresListsAndForgets(t *testing.T) {
	s := goalTestServer(t)
	list, _ := s.winds.List()
	if len(list) == 0 {
		t.Skip("cap vent")
	}
	w := list[0]
	if err := companion.AppendWindMemory(s.windStateDir(), w.ID, []string{"Li agrada el cafè curt", "Viu a Girona"}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleWindApres(rec, httptest.NewRequest(http.MethodGet, "/api/winds/apres", nil))
	if !strings.Contains(rec.Body.String(), "Li agrada el cafè curt") {
		t.Fatalf("GET = %s", rec.Body)
	}
	rec = httptest.NewRecorder()
	s.handleWindApres(rec, httptest.NewRequest(http.MethodPost, "/api/winds/apres", strings.NewReader(`{"wind":"`+w.ID+`","all":["Li agrada el cafè curt"]}`)))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"forgot":1`) {
		t.Fatalf("POST = %d %s", rec.Code, rec.Body)
	}
	notes := companion.WindMemoryNotes(s.windStateDir(), w.ID)
	if len(notes) != 1 || notes[0] != "Viu a Girona" {
		t.Fatalf("notes = %q", notes)
	}
}
