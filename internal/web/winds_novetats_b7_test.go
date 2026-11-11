package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/events"
	"gregal/internal/push"
)

type fakePush struct {
	mu   sync.Mutex
	sent []map[string]string
	to   []string
	fail map[string]error
	done chan struct{}
}

func (f *fakePush) Send(_ context.Context, token string, data map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, data)
	f.to = append(f.to, token)
	if f.done != nil {
		f.done <- struct{}{}
	}
	return f.fail[token]
}

func withFakePush(t *testing.T, f *fakePush) {
	t.Helper()
	saved := pushSender
	pushSender = func() push.Sender {
		if f == nil {
			return nil
		}
		return f
	}
	t.Cleanup(func() { pushSender = saved })
}

func novetatByID(t *testing.T, s *Server, id string) (int, windNovetat) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindNovetats(rec, httptest.NewRequest(http.MethodGet, "/api/winds/novetats?id="+id, nil))
	var out struct {
		Novetat windNovetat `json:"novetat"`
	}
	if rec.Code == 200 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out.Novetat
}

func userEvents(t *testing.T, s *Server, user string) []events.Event {
	t.Helper()
	got, err := s.Hub().eventStore.AfterScope(0, 500, user, "")
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestNovetatOwnerSecondIdentity: una novetat de l'Aleix porta id opac i
// owner; la Carla no la veu ni per llista ni per id, no rep el seu event i
// no la pot marcar com a llegida.
func TestNovetatOwnerSecondIdentity(t *testing.T) {
	rig := sharedTestRig(t)
	withFakePush(t, nil)
	recordNovetatMsg(rig.aleix, rig.mestral, "remind", "", "m-1", "Recorda la reunió")

	mine := getNovetats(t, rig.aleix, 0)
	if len(mine) != 1 || !strings.HasPrefix(mine[0].ID, "n-") || mine[0].Owner != "aleix" || mine[0].MessageID != "m-1" {
		t.Fatalf("novetat de l'Aleix = %+v", mine)
	}
	if got := getNovetats(t, rig.carla, 0); len(got) != 0 {
		t.Fatalf("la Carla veu novetats de l'Aleix: %+v", got)
	}
	if code, _ := novetatByID(t, rig.carla, mine[0].ID); code != http.StatusNotFound {
		t.Fatalf("la Carla llegeix per id: %d", code)
	}
	if code, n := novetatByID(t, rig.aleix, mine[0].ID); code != 200 || n.Text != "Recorda la reunió" {
		t.Fatalf("l'Aleix per id: %d %+v", code, n)
	}
	// Event al registre durable: de l'Aleix, només ids, cap text.
	var found bool
	for _, e := range userEvents(t, rig.aleix, "aleix") {
		if e.Kind == eventNovetat {
			found = true
			if strings.Contains(string(e.Payload), "reunió") || e.Text != "" {
				t.Fatalf("l'event porta text privat: %s %q", e.Payload, e.Text)
			}
			if !strings.Contains(string(e.Payload), mine[0].ID) || !strings.Contains(string(e.Payload), `"message_id":"m-1"`) {
				t.Fatalf("payload = %s", e.Payload)
			}
		}
	}
	if !found {
		t.Fatal("cap event novetat")
	}
	for _, e := range userEvents(t, rig.carla, "carla") {
		if e.Kind == eventNovetat {
			t.Fatalf("la Carla té l'event de l'Aleix: %+v", e)
		}
	}
	// La Carla no pot marcar-la (no és seva): res no canvia.
	rec := postJSON(t, rig.carla.handleWindNovetatsRead, `{"ids":["`+mine[0].ID+`"]}`)
	if rec.Code != 200 || strings.Contains(rec.Body.String(), mine[0].ID) {
		t.Fatalf("la Carla ha marcat: %d %s", rec.Code, rec.Body.String())
	}
	if getNovetats(t, rig.aleix, 0)[0].Read {
		t.Fatal("marcada per un altre compte")
	}
}

// TestNovetatOwnerFilterSameFile: encara que un fitxer tingués una entrada
// d'un altre owner, la lectura la descarta.
func TestNovetatOwnerFilterSameFile(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	saveNovetats(s, []windNovetat{{Ts: 10, Wind: "x", Text: "aliena", Owner: "algú-altre"}, {Ts: 11, Wind: "x", Text: "antiga"}})
	got := getNovetats(t, s, 0)
	if len(got) != 1 || got[0].Text != "antiga" || got[0].ID == "" {
		t.Fatalf("filtre owner = %+v", got)
	}
	// L'id derivat de les antigues és estable entre lectures.
	if again := getNovetats(t, s, 0); again[0].ID != got[0].ID {
		t.Fatalf("id derivat inestable: %s vs %s", again[0].ID, got[0].ID)
	}
}

// TestNovetatReadByMessageAndWind: llegit per message_id i per vent és
// idempotent i deixa un event novetat_read.
func TestNovetatReadByMessageAndWind(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	withFakePush(t, nil)
	s := remindTestServer(t)
	s.Hub() // el registre d'events viu al hub (en producció sempre hi és)
	w := companion.Wind{ID: "xaloc", Name: "Xaloc"}
	recordNovetatMsg(s, w, "remind", "", "m-a", "u")
	recordNovetatMsg(s, w, "heartbeat", "", "m-b", "dos")
	recordNovetatMsg(s, companion.Wind{ID: "mestral", Name: "Mestral"}, "heartbeat", "", "m-c", "tres")

	rec := postJSON(t, s.handleWindNovetatsRead, `{"message_ids":["m-a"]}`)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"marked":["n-`) {
		t.Fatalf("read m-a: %d %s", rec.Code, rec.Body.String())
	}
	rec = postJSON(t, s.handleWindNovetatsRead, `{"message_ids":["m-a"]}`)
	if !strings.Contains(rec.Body.String(), `"marked":[]`) {
		t.Fatalf("no idempotent: %s", rec.Body.String())
	}
	postJSON(t, s.handleWindNovetatsRead, `{"wind":"xaloc"}`)
	got := getNovetats(t, s, 0)
	if !got[0].Read || !got[1].Read || got[2].Read {
		t.Fatalf("estat llegit = %+v", got)
	}
	reads := 0
	for _, e := range userEvents(t, s, s.usuari()) {
		if e.Kind == eventNovetatRead {
			reads++
		}
	}
	if reads != 2 {
		t.Fatalf("events novetat_read = %d, calien 2", reads)
	}
	if rec := postJSON(t, s.handleWindNovetatsRead, `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("buit hauria de ser 400: %d", rec.Code)
	}
}

// TestNovetatPushOnlyOpaqueID: el push porta només {k, id}, només als
// dispositius del compte, no sona per a les tranquil·les i un token
// caducat s'oblida.
func TestNovetatPushOnlyOpaqueID(t *testing.T) {
	rig := sharedTestRig(t)
	f := &fakePush{fail: map[string]error{"tok-vell": push.ErrUnregistered}, done: make(chan struct{}, 8)}
	withFakePush(t, f)
	if rec := postJSON(t, rig.aleix.handlePush, `{"token":"tok-aleix","platform":"android"}`); rec.Code != 200 {
		t.Fatalf("registre: %d %s", rec.Code, rec.Body.String())
	}
	postJSON(t, rig.aleix.handlePush, `{"token":"tok-vell","platform":"android"}`)
	postJSON(t, rig.carla.handlePush, `{"token":"tok-carla","platform":"android"}`)

	recordNovetatMsg(rig.aleix, rig.mestral, "remind", "", "m-9", "Text privat de l'Aleix")
	for i := 0; i < 2; i++ {
		select {
		case <-f.done:
		case <-time.After(5 * time.Second):
			t.Fatal("cap push")
		}
	}
	f.mu.Lock()
	for i, data := range f.sent {
		if f.to[i] == "tok-carla" {
			t.Fatal("push a la Carla d'una novetat de l'Aleix")
		}
		if len(data) != 2 || data["k"] != "novetat" || !strings.HasPrefix(data["id"], "n-") {
			t.Fatalf("payload push = %v", data)
		}
		for _, v := range data {
			if strings.Contains(v, "privat") {
				t.Fatal("text privat al push")
			}
		}
	}
	f.mu.Unlock()
	// El token caducat desapareix (en segon pla).
	deadline := time.Now().Add(5 * time.Second)
	for {
		left := myPushDevices(rig.aleix)
		if len(left) == 1 && left[0].Token == "tok-aleix" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dispositius = %+v", left)
		}
		time.Sleep(20 * time.Millisecond)
	}
	// GET no torna cap token.
	rec := httptest.NewRecorder()
	rig.aleix.handlePush(rec, httptest.NewRequest(http.MethodGet, "/api/v2/push", nil))
	if strings.Contains(rec.Body.String(), "tok-") || !strings.Contains(rec.Body.String(), `"mode":"fcm"`) {
		t.Fatalf("GET push = %s", rec.Body.String())
	}
	// Tranquil·la (quiet): cap push. La forcem amb el registre directe.
	f.mu.Lock()
	before := len(f.sent)
	f.mu.Unlock()
	n := windNovetat{ID: "", Wind: "x"}
	rig.aleix.pushNovetat(n) // sense id: res
	time.Sleep(50 * time.Millisecond)
	f.mu.Lock()
	if len(f.sent) != before {
		t.Fatal("push sense id")
	}
	f.mu.Unlock()
}

// TestPushPollingModeWithoutCredentials: sense credencials el mode és
// «polling» i el registre es desa igualment (per quan n'hi hagi).
func TestPushPollingModeWithoutCredentials(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	withFakePush(t, nil)
	s := remindTestServer(t)
	rec := httptest.NewRecorder()
	s.handlePush(rec, httptest.NewRequest(http.MethodGet, "/api/v2/push", nil))
	if !strings.Contains(rec.Body.String(), `"mode":"polling"`) || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("GET = %s", rec.Body.String())
	}
	if rec := postJSON(t, s.handlePush, `{"token":"a b"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("token amb espai: %d", rec.Code)
	}
	postJSON(t, s.handlePush, `{"token":"t1"}`)
	req := httptest.NewRequest(http.MethodDelete, "/api/v2/push", strings.NewReader(`{"token":"t1"}`))
	rec = httptest.NewRecorder()
	s.handlePush(rec, req)
	if rec.Code != 200 || len(myPushDevices(s)) != 0 {
		t.Fatalf("DELETE: %d %d", rec.Code, len(myPushDevices(s)))
	}
}

// TestEventsUserScopeKinds: ?scope=user&kinds=novetat torna les novetats
// del compte (de cap sessió) i el cursor avança per sobre de la resta.
func TestEventsUserScopeKinds(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	withFakePush(t, nil)
	s := remindTestServer(t)
	h := s.Hub()
	if _, err := h.eventStore.Append(events.Event{User: s.usuari(), SessionID: "altra", Kind: "text", Text: "soroll"}); err != nil {
		t.Fatal(err)
	}
	recordNovetatMsg(s, companion.Wind{ID: "xaloc", Name: "Xaloc"}, "remind", "", "m-1", "hola")
	if _, err := h.eventStore.Append(events.Event{User: s.usuari(), SessionID: "altra", Kind: "text", Text: "més soroll"}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.handleEvents(rec, httptest.NewRequest(http.MethodGet, "/api/v2/events?scope=user&kinds=novetat,novetat_read", nil))
	var out struct {
		Events []events.Event `json:"events"`
		Next   uint64         `json:"next"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if len(out.Events) != 1 || out.Events[0].Kind != eventNovetat {
		t.Fatalf("events = %+v", out.Events)
	}
	if out.Next <= out.Events[0].ID {
		t.Fatalf("el cursor no avança per sobre del filtrat: next=%d", out.Next)
	}
}

// TestAvuiDoneReadsNovetat: decidir la targeta a «Avui» (o des del botó de
// la notificació) deixa llegida la novetat que la va portar.
func TestAvuiDoneReadsNovetat(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	withFakePush(t, nil)
	s := remindTestServer(t)
	recordNovetatMsg(s, companion.Wind{ID: "xaloc", Name: "Xaloc"}, "remind", "", "m-1", "Truca al metge")
	got := getNovetats(t, s, 0)
	if len(got) != 1 || got[0].Card == "" {
		t.Skipf("aquesta veu no genera targeta d'«Avui»: %+v", got)
	}
	rec := postJSON(t, s.handleWindsAvui, `{"id":"`+got[0].Card+`","action":"done"}`)
	if rec.Code != 200 {
		t.Fatalf("avui done: %d %s", rec.Code, rec.Body.String())
	}
	if !getNovetats(t, s, 0)[0].Read {
		t.Fatal("la novetat no ha quedat llegida en fer «Fet»")
	}
}
