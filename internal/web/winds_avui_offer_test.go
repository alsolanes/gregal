package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// stubOfferLoop fa de model lateral: torna reply i apunta les consignes.
func stubOfferLoop(t *testing.T, reply *string) *[]string {
	t.Helper()
	var mu sync.Mutex
	var prompts []string
	old := windOfferLoop
	windOfferLoop = func(_ *Server, _ context.Context, msgs []llm.Message) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		prompts = append(prompts, msgs[len(msgs)-1].Content)
		return *reply, nil
	}
	t.Cleanup(func() { waitBackground(); windOfferLoop = old })
	return &prompts
}

func offerCards(s *Server) []avuiCard {
	var out []avuiCard
	for _, c := range loadAvui(s) {
		if c.Source == avuiOfferSource {
			out = append(out, c)
		}
	}
	return out
}

// offerAfter fa com si acabés un torn de xat amb el vent xaloc.
func offerAfter(t *testing.T, s *Server, question, answer string) {
	t.Helper()
	w, _ := s.winds.ByID("xaloc")
	s.startWindOffer(w, question, answer)
	waitBackground()
}

const romeAnswer = "Ara el vol a Roma del 12 de desembre costa 89 € ([vols.example.org](https://vols.example.org/roma))."

func TestAvuiOfferAtMostOnePerDay(t *testing.T) {
	s := localTestServer(t)
	reply := "El preu del vol a Roma :: https://vols.example.org/roma"
	prompts := stubOfferLoop(t, &reply)

	offerAfter(t, s, "Quin temps fa avui?", "Fa sol tot el dia.")
	if len(*prompts) != 0 || len(offerCards(s)) != 0 {
		t.Fatalf("without a page to watch the model is not asked: %d %+v", len(*prompts), offerCards(s))
	}
	offerAfter(t, s, "Quant costa el vol a Roma?", romeAnswer)
	cards := offerCards(s)
	if len(*prompts) != 1 || len(cards) != 1 {
		t.Fatalf("one offer after a turn about a price: %d %+v", len(*prompts), cards)
	}
	c := cards[0]
	if c.Section != avuiFYI || c.Title != s.tr("Vols que ho vigili?", "Want me to keep an eye on it?") || c.Why != "El preu del vol a Roma" || c.Wind != "xaloc" {
		t.Fatalf("card: %+v", c)
	}
	if len(c.Actions) != 1 || !strings.Contains(c.Actions[0].Prompt, "https://vols.example.org/roma") || !strings.Contains(c.Actions[0].Prompt, "El preu del vol a Roma") {
		t.Fatalf("the action asks to watch that page: %+v", c.Actions)
	}
	if !strings.Contains((*prompts)[0], "89 €") {
		t.Fatalf("the model reads the exchange: %q", (*prompts)[0])
	}

	reply = "Les entrades del Liceu :: https://liceu.example.org/entrades"
	offerAfter(t, s, "Hi ha entrades pel Liceu?", "Encara no, mira https://liceu.example.org/entrades")
	if len(*prompts) != 1 || len(offerCards(s)) != 1 {
		t.Fatalf("at most one offer per day: %d %+v", len(*prompts), offerCards(s))
	}
}

func TestAvuiOfferNothingThatChangesNoCard(t *testing.T) {
	s := localTestServer(t)
	reply := "NONE"
	prompts := stubOfferLoop(t, &reply)
	offerAfter(t, s, "Qui va escriure el Quixot?", "Cervantes (https://ca.wikipedia.org/wiki/Quixot).")
	if len(*prompts) != 1 || len(offerCards(s)) != 0 {
		t.Fatalf("nothing that changes, no offer: %d %+v", len(*prompts), offerCards(s))
	}
}

// offerModel és el proveïdor fals del xat: respon el preu amb la font i, quan
// la persona li demana vigilar una pàgina, crida l'eina watch amb aquella URL.
func offerModel(t *testing.T) *httptest.Server {
	t.Helper()
	url := regexp.MustCompile(`https?://[^\s)]+`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "text/event-stream")
		send := func(format string, v []byte) {
			if _, err := fmt.Fprintf(w, format, v); err != nil {
				t.Error(err)
			}
		}
		say := func(text string) {
			b, _ := json.Marshal(text)
			send("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", b)
		}
		last := req.Messages[len(req.Messages)-1]
		hasWatch := false
		for _, tl := range req.Tools {
			hasWatch = hasWatch || tl.Function.Name == "watch"
		}
		msg := fmt.Sprint(last.Content)
		low := strings.ToLower(msg)
		switch {
		case last.Role == "tool":
			say("Fet: ho vigilo.")
		case last.Role == "user" && hasWatch && url.MatchString(msg) && (strings.Contains(low, "vigila") || strings.Contains(low, "watch")):
			args, _ := json.Marshal(map[string]any{"wind": "xaloc", "objective": msg, "urls": []string{url.FindString(msg)}})
			wire, _ := json.Marshal(string(args))
			send("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"w1\",\"function\":{\"name\":\"watch\",\"arguments\":%s}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", wire)
		default:
			say(romeAnswer)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAvuiOfferPromptCreatesTheWatch(t *testing.T) {
	s := localTestServer(t)
	model := offerModel(t)
	s.cfg.Providers["p"] = config.Provider{BaseURL: model.URL + "/v1"}
	s.cfg.Agent.MaxSteps = 6
	s.cwd = t.TempDir()
	wind, _ := s.winds.ByID("xaloc")
	if err := s.clearWindHistory(wind, windHistory(s, "xaloc")); err != nil {
		t.Fatal(err)
	}
	reply := "El preu del vol a Roma :: https://vols.example.org/roma"
	stubOfferLoop(t, &reply)

	windChat(t, s, "Quant costa el vol a Roma del 12 de desembre?")
	waitBackground()
	cards := offerCards(s)
	if len(cards) != 1 {
		t.Fatalf("the chat turn about a price ends in one offer: %+v", cards)
	}
	if out := windChat(t, s, cards[0].Actions[0].Prompt); !strings.Contains(out, "ho vigilo") {
		t.Fatalf("turn: %s", out)
	}
	waitBackground()
	var watches []*responsabilitat
	if err := s.withResp(func(st *respStore) error { watches = st.list(); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(watches) != 1 || watches[0].Kind != respKindWatch || len(watches[0].Sources) != 1 ||
		watches[0].Sources[0].URL != "https://vols.example.org/roma" || watches[0].Wind != "xaloc" {
		t.Fatalf("sending the offer's prompt creates the watch: %+v", watches)
	}
}

// offerYesterday fa que les ofertes d'avui siguin d'ahir (per passar el límit diari).
func offerYesterday(t *testing.T, s *Server) {
	t.Helper()
	list := loadAvui(s)
	for i := range list {
		list[i].Created -= 26 * 3600
	}
	if err := saveAvui(s, list); err != nil {
		t.Fatal(err)
	}
}

func TestAvuiOfferNeverAgainWhenDismissedOrWatched(t *testing.T) {
	s := localTestServer(t)
	reply := "El preu del vol a Roma :: https://vols.example.org/roma"
	prompts := stubOfferLoop(t, &reply)
	offerAfter(t, s, "Quant costa el vol a Roma?", romeAnswer)
	cards := offerCards(s)
	if len(cards) != 1 {
		t.Fatalf("offer: %+v", cards)
	}
	avuiPost(t, s, cards[0].ID, "dismiss")
	offerYesterday(t, s)

	offerAfter(t, s, "I ara quant costa el vol a Roma?", romeAnswer)
	if len(offerCards(s)) != 1 {
		t.Fatalf("a dismissed topic is never offered again: %+v", offerCards(s))
	}
	if !strings.Contains((*prompts)[1], "El preu del vol a Roma") {
		t.Fatalf("the model is told what not to offer again: %q", (*prompts)[1])
	}

	if _, err := s.respCreate(respInput{Wind: "xaloc", Objective: "Les entrades del Liceu", URLs: []string{"https://liceu.example.org/entrades"}}, "ui"); err != nil {
		t.Fatal(err)
	}
	reply = "Entrades noves del Liceu :: https://liceu.example.org/entrades"
	offerAfter(t, s, "Hi ha entrades pel Liceu?", "Encara no, mira https://liceu.example.org/entrades")
	if len(offerCards(s)) != 1 {
		t.Fatalf("a page already watched is never offered: %+v", offerCards(s))
	}
	if !strings.Contains((*prompts)[2], "Les entrades del Liceu") {
		t.Fatalf("the model is told what is already watched: %q", (*prompts)[2])
	}
}

func TestAvuiOfferCalmNeverRings(t *testing.T) {
	s := localTestServer(t)
	avuiSettingsPost(t, s, `{"level":"calm"}`)
	reply := "El preu del vol a Roma :: https://vols.example.org/roma"
	stubOfferLoop(t, &reply)
	offerAfter(t, s, "Quant costa el vol a Roma?", romeAnswer)
	cards := offerCards(s)
	if len(cards) != 1 || !cards[0].Held {
		t.Fatalf("in calm mode the offer waits in the tray: %+v", cards)
	}
	if n := loadNovetats(s); len(n) != 0 {
		t.Fatalf("the offer never rings: %+v", n)
	}
}
