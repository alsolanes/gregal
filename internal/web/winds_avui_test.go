package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
)

func TestAvuiQuietHoursWrapMidnight(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 10, h, m, 0, 0, time.UTC) }
	for _, tc := range []struct {
		t    time.Time
		want bool
	}{{at(23, 0), true}, {at(7, 59), true}, {at(8, 0), false}, {at(13, 0), false}, {at(22, 0), true}} {
		if got := inQuietHours(tc.t, "22:00", "08:00"); got != tc.want {
			t.Fatalf("%s: got %v", tc.t.Format("15:04"), got)
		}
	}
	if inQuietHours(at(13, 0), "13:00", "13:00") {
		t.Fatal("an empty window is never quiet")
	}
}

func TestAvuiBriefBlockBecomesCardsAndLeavesTheChatClean(t *testing.T) {
	out := "Bon dia! Avui tens dentista a les 12.\n\n```avui\nnow :: Dentista a les 12 :: Surt a les 11:30\nfyi :: **Plou** a la tarda :: Agafa paraigua\nahead :: Sopar divendres :: Compra el vi\nnonsense line\n```"
	items, clean := parseAvuiBlock(out)
	if len(items) != 3 || items[0][0] != avuiNow || items[1][1] != "Plou a la tarda" || items[2][0] != avuiAhead {
		t.Fatalf("items: %+v", items)
	}
	if strings.Contains(clean, "```") || !strings.HasPrefix(clean, "Bon dia!") {
		t.Fatalf("the chat must only get the message: %q", clean)
	}
}

func TestAvuiVoicesBecomeCardsOpenLoopsComeBackAndFeedbackTeaches(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	w := companion.Wind{ID: "garbi", Name: "Garbí", Role: "esports"}
	// Una veu del cor: va a «Per si de cas» amb accions per defecte.
	recordNovetat(s, w, "heartbeat", "El Barça juga dissabte a les 21:00. Es veu a Movistar.")
	// Un recordatori: «Ara toca».
	recordNovetat(s, w, "remind", "Truca al dentista")
	view := s.avuiView()
	now, fyi := view["now"].([]avuiCard), view["fyi"].([]avuiCard)
	if len(now) != 1 || now[0].Title != "Truca al dentista" || len(fyi) != 1 || len(fyi[0].Actions) == 0 {
		t.Fatalf("now=%+v fyi=%+v", now, fyi)
	}
	// La targeta de la novetat queda lligada (l'app hi posa els botons).
	if n := loadNovetats(s); len(n) != 2 || n[1].Card == "" {
		t.Fatalf("novetats must carry the card id: %+v", n)
	}
	// Un recordatori obert d'ahir torna avui, amb els dies que fa.
	avuiMu.Lock()
	list := loadAvui(s)
	for i := range list {
		if list[i].Source == "remind" {
			list[i].Created = startOfToday(s).Add(-20 * time.Hour).Unix()
		}
	}
	_ = saveAvui(s, list)
	avuiMu.Unlock()
	if now := s.avuiView()["now"].([]avuiCard); len(now) != 1 || now[0].Days != 1 {
		t.Fatalf("an open card must come back with its age: %+v", now)
	}
	if !strings.Contains(s.avuiBriefExtras(""), "Truca al dentista") {
		t.Fatal("the morning brief must bring back what is still open")
	}
	// «No m'interessa» la treu i el vent ho té en compte.
	act := func(id, action string) {
		rec := httptest.NewRecorder()
		s.handleWindsAvui(rec, httptest.NewRequest(http.MethodPost, "/api/winds/avui", strings.NewReader(`{"id":"`+id+`","action":"`+action+`"}`)))
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", action, rec.Code, rec.Body.String())
		}
	}
	act(fyi[0].ID, "notuseful")
	if len(s.avuiView()["fyi"].([]avuiCard)) != 0 {
		t.Fatal("a card marked not interesting leaves the tray")
	}
	if fb := s.avuiFeedbackBrief("garbi"); !strings.Contains(fb, "not interesting") || !strings.Contains(fb, "Barça") {
		t.Fatalf("feedback must reach the wind: %q", fb)
	}
	act(now[0].ID, "done")
	v := s.avuiView()
	if len(v["now"].([]avuiCard)) != 0 || len(v["done"].([]avuiCard)) != 1 {
		t.Fatalf("done goes to done: %+v", v)
	}
}

func TestAvuiQuietLevelKeepsThePhoneSilent(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	rec := httptest.NewRecorder()
	s.handleWindsAvuiSettings(rec, httptest.NewRequest(http.MethodPost, "/api/winds/avui/settings", strings.NewReader(`{"level":"calm","tune":"més esport"}`)))
	var st avuiSettings
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if st.Level != "calm" || st.Tune != "més esport" {
		t.Fatalf("settings: %+v", st)
	}
	// Canviar les hores no esborra el que li vas demanar al resum.
	rec = httptest.NewRecorder()
	s.handleWindsAvuiSettings(rec, httptest.NewRequest(http.MethodPost, "/api/winds/avui/settings", strings.NewReader(`{"quiet_from":"23:00","quiet_to":"07:30"}`)))
	if got := s.avuiSettings(); got.Tune != "més esport" || got.QuietFrom != "23:00" {
		t.Fatalf("settings after hours: %+v", got)
	}
	w := companion.Wind{ID: "xaloc", Name: "Xaloc"}
	recordNovetat(s, w, "heartbeat", "Disc nou dels teus.")
	recordNovetat(s, w, "remind", "Compra entrades")
	n := loadNovetats(s)
	if len(n) != 2 || !n[0].Quiet || n[1].Quiet {
		t.Fatalf("calm: the watch waits in the tray, the reminder rings: %+v", n)
	}
}
