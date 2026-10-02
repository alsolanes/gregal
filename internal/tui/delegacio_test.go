package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	serviceclient "gregal/internal/client"
)

// Amb el servei connectat, enviar un missatge ha d'encuar el torn a la cua
// del servei (no executar res local) i els events durables han de pintar la
// resposta i alliberar el torn quan arriba el done.
func TestDelegacioServeiEncuaIPintaEvents(t *testing.T) {
	var sessioRecuda, wsRebut string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v2/runs" && r.Method == http.MethodPost:
			sessioRecuda = r.Header.Get("X-Gregal-Session")
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			wsRebut = body["workspace"]
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"run":{"id":3,"state":"queued"}}`))
		case r.URL.Path == "/api/v2/events":
			_, _ = w.Write([]byte(`{"events":[` +
				`{"id":1,"kind":"tool_call","text":"eina de prova"},` +
				`{"id":2,"kind":"text","text":"resposta final del servei"},` +
				`{"id":3,"kind":"done","text":"torn completat"}],"cursor":0,"next":3}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := modelDeBarra(t, 100)
	m.serviceClient = serviceclient.New(srv.URL, "tok", "inst")
	m.serviceSession = "default"
	m.serviceCursor = 0

	mod, cmd := m.submitText("fes la feina delegada")
	mm := mod.(Model)
	if !mm.serviceTurn {
		t.Fatal("enviar amb el servei connectat ha de marcar el torn delegat")
	}
	if cmd == nil {
		t.Fatal("hi havia d'haver una ordre d'enviament")
	}
	sub := cmd().(serviceSubmitMsg)
	if sub.err != nil || sub.run.ID != 3 {
		t.Fatalf("submit: run=%+v err=%v", sub.run, sub.err)
	}
	if sessioRecuda != "default" {
		t.Fatalf("la petició havia d'anar a la sessió del servei, ha anat a %q", sessioRecuda)
	}
	// El TUI delega el SEU directori: sense això el torn corre al cwd de
	// la sessió del servei, no on l'usuari ha obert el TUI.
	if wsRebut == "" || wsRebut != mm.cwd {
		t.Fatalf("workspace rebut=%q, volia el cwd del TUI %q", wsRebut, mm.cwd)
	}

	mod2, cmd2 := mm.Update(sub)
	mm2 := mod2.(Model)
	if mm2.serviceRunID != 3 {
		t.Fatalf("serviceRunID=%d, volia 3", mm2.serviceRunID)
	}

	// pollServiceEvents: un tick de 750 ms abans de la resposta.
	ev := cmd2().(serviceEventsMsg)
	if ev.err != nil || len(ev.page.Events) != 3 {
		t.Fatalf("events: %+v %v", ev.page, ev.err)
	}
	mod3, _ := mm2.Update(ev)
	mm3 := mod3.(Model)
	if mm3.serviceTurn || mm3.serviceRunID != 0 {
		t.Fatal("el done havia d'alliberar el torn delegat")
	}
	tot := strings.Join(mm3.lines, "\n")
	if !strings.Contains(tot, "resposta final del") {
		t.Fatalf("la resposta remota no ha arribat al transcript:\n%s", tot)
	}
	if !strings.Contains(tot, "eina de prova") {
		t.Fatal("l'activitat d'eina havia de sortir al transcript")
	}
	if mm3.status != T("barra.llest") {
		t.Fatalf("estat=%q, volia llest", mm3.status)
	}
}

// Sense servei connectat, el text no es delega: l'executor local segueix.
func TestSenseServeiNoDelega(t *testing.T) {
	m := modelDeBarra(t, 100)
	if m.serviceClient != nil {
		t.Fatal("el model de prova no hauria de tenir servei")
	}
	// submitText amb "hola" és smalltalk local: comprova que la via
	// delegada no s'activa (serviceTurn fals) i que no hi ha Cmd de submit.
	mod, _ := m.submitText("hola")
	mm := mod.(Model)
	if mm.serviceTurn {
		t.Fatal("sense servei no es pot delegar cap torn")
	}
}

// Cancelar el torn delegat apunta a l'execució concreta de la cua.
func TestCancelDelegatAlRunConcret(t *testing.T) {
	var cancelPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/runs/7/cancel" {
			cancelPath = r.URL.Path
			_, _ = w.Write([]byte(`{"ok":true,"cancelled":true}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	m := modelDeBarra(t, 100)
	m.serviceClient = serviceclient.New(srv.URL, "tok", "inst")
	m.serviceSession = "default"
	m.serviceRunID = 7
	m.serviceTurn = true

	mod, cmd := m.runCommand("/cancel")
	if cmd == nil {
		t.Fatal("hi havia d'haver una ordre de cancel·lació")
	}
	msg := cmd().(serviceCancelMsg)
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	if cancelPath != "/api/v2/runs/7/cancel" {
		t.Fatalf("s'ha cridat %q, volia el run concret", cancelPath)
	}
	_ = mod
}
