package web

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
)

// fakeGmail és un servidor IMAP mínim com el de Gmail. Desa les ordres per
// comprovar que mai no se n'envia cap que canviï la bústia.
type fakeGmail struct {
	mu   sync.Mutex
	cmds []string
	pass string
}

const gmailMsg101 = "From: =?UTF-8?Q?Anna_Puig?= <anna@example.com>\r\nSubject: =?UTF-8?B?UmV1bmnDsyBkZSBkaWxsdW5z?=\r\nDate: Tue, 03 Nov 2026 08:12:00 +0100\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"b1\"\r\n\r\n--b1\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\nHola! Podem moure la reuni=C3=B3 a les 10?\r\n--b1\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n<p>Hola!</p>\r\n--b1--\r\n"

const gmailMsg103 = "From: Banc <avisos@banc.example>\r\nSubject: Factura\r\nDate: Mon, 02 Nov 2026 19:00:00 +0100\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\nPGh0bWw+PGJvZHk+PHA+TGEgdm9zdHJhIGZhY3R1cmEgamEgw6lzIGRpc3BvbmlibGU8L3A+PHN0eWxlPnB7fTwvc3R5bGU+PC9ib2R5PjwvaHRtbD4=\r\n"

func headerOf(msg string) string {
	h, _, _ := strings.Cut(msg, "\r\n\r\n")
	var keep []string
	for _, l := range strings.Split(h, "\r\n") {
		if strings.HasPrefix(l, "From:") || strings.HasPrefix(l, "Subject:") || strings.HasPrefix(l, "Date:") {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\r\n") + "\r\n\r\n"
}

func (g *fakeGmail) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := func(s string) { _, _ = conn.Write([]byte(s)) }
	w("* OK Gimap ready\r\n")
	msgs := map[string]string{"101": gmailMsg101, "103": gmailMsg103}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		tag, cmd, _ := strings.Cut(line, " ")
		g.mu.Lock()
		g.cmds = append(g.cmds, cmd)
		g.mu.Unlock()
		up := strings.ToUpper(cmd)
		switch {
		case strings.HasPrefix(up, "LOGIN"):
			if strings.Contains(cmd, `"`+g.pass+`"`) {
				w(tag + " OK logged in\r\n")
			} else {
				w(tag + " NO [AUTHENTICATIONFAILED] Invalid credentials (Failure)\r\n")
			}
		case strings.HasPrefix(up, "EXAMINE"):
			w("* 3 EXISTS\r\n" + tag + " OK [READ-ONLY] INBOX selected\r\n")
		case up == "UID SEARCH UNSEEN":
			w("* SEARCH 101 103\r\n" + tag + " OK\r\n")
		case strings.HasPrefix(up, "UID SEARCH X-GM-RAW"):
			w("* SEARCH 103\r\n" + tag + " OK\r\n")
		case strings.HasPrefix(up, "UID FETCH") && strings.Contains(up, "HEADER.FIELDS"):
			ids := strings.Split(strings.Fields(cmd)[2], ",")
			for i, id := range ids {
				h := headerOf(msgs[id])
				w(fmt.Sprintf("* %d FETCH (UID %s FLAGS () BODY[HEADER.FIELDS (FROM SUBJECT DATE)] {%d}\r\n%s)\r\n", i+1, id, len(h), h))
			}
			w(tag + " OK\r\n")
		case strings.HasPrefix(up, "UID FETCH"):
			id := strings.Fields(cmd)[2]
			m := msgs[id]
			w(fmt.Sprintf("* 1 FETCH (UID %s FLAGS () BODY[]<0> {%d}\r\n%s)\r\n", id, len(m), m))
			w(tag + " OK\r\n")
		case up == "LOGOUT":
			w("* BYE\r\n" + tag + " OK\r\n")
			return
		default:
			w(tag + " BAD unknown\r\n")
		}
	}
}

func useFakeGmail(t *testing.T, pass string) *fakeGmail {
	g := &fakeGmail{pass: pass}
	old := imapDial
	imapDial = func(host string, port int) (net.Conn, error) {
		a, b := net.Pipe()
		go g.serve(b)
		return a, nil
	}
	t.Cleanup(func() { imapDial = old })
	return g
}

func useFakeICS(t *testing.T) {
	old := icsFetch
	icsFetch = func(_ context.Context, url string) (string, error) { return googleICS, nil }
	t.Cleanup(func() { icsFetch = old })
}

func connPost(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	s.handleConnections(w, httptest.NewRequest(http.MethodPost, "/api/connections", strings.NewReader(body)))
	return w
}

func TestGmailAndGoogleCalendarForWinds(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	g := useFakeGmail(t, "abcdefghijklmnop")
	useFakeICS(t)
	h := testHubWithUsers(t)
	s := h.def

	if w := connPost(t, s, `{"mail":{"user":"aleix@gmail.com","password":"wrong-pass"}}`); w.Code != 400 || !strings.Contains(w.Body.String(), "app password") {
		t.Fatalf("a wrong Gmail password must explain app passwords: %d %s", w.Code, w.Body.String())
	}
	// Google mostra la contrasenya d'aplicació en grups de 4: els espais fora.
	if w := connPost(t, s, `{"mail":{"user":"aleix@gmail.com","password":"abcd efgh ijkl mnop"}}`); w.Code != 200 {
		t.Fatalf("save gmail: %d %s", w.Code, w.Body.String())
	}
	if w := connPost(t, s, `{"add_calendar":{"name":"Personal","url":"https://calendar.google.com/calendar/ical/x%40gmail.com/private-secret123/basic.ics"}}`); w.Code != 200 {
		t.Fatalf("add calendar: %d %s", w.Code, w.Body.String())
	}
	get := httptest.NewRecorder()
	s.handleConnections(get, httptest.NewRequest(http.MethodGet, "/api/connections", nil))
	if body := get.Body.String(); strings.Contains(body, "abcdefghijklmnop") || strings.Contains(body, "private-secret123") || !strings.Contains(body, `"has_password":true`) {
		t.Fatalf("GET must not reveal the app password or the secret calendar address: %s", body)
	}

	out, err := s.runMailTool(`{"action":"unread"}`)
	if err != nil || !strings.Contains(out, "2 messages unread") || !strings.Contains(out, "Anna Puig <anna@example.com> · Reunió de dilluns (unread)") {
		t.Fatalf("unread: %v\n%s", err, out)
	}
	out, err = s.runMailTool(`{"action":"read","id":101}`)
	if err != nil || !strings.Contains(out, "Podem moure la reunió a les 10?") || !strings.Contains(out, "UNTRUSTED EMAIL CONTENT") {
		t.Fatalf("read quoted-printable: %v\n%s", err, out)
	}
	out, err = s.runMailTool(`{"action":"read","id":103}`)
	if err != nil || !strings.Contains(out, "La vostra factura ja és disponible") || strings.Contains(out, "<p>") || strings.Contains(out, "p{}") {
		t.Fatalf("read base64 html: %v\n%s", err, out)
	}
	if out, err = s.runMailTool(`{"action":"search","query":"from:banc newer_than:7d"}`); err != nil || !strings.Contains(out, "Factura") {
		t.Fatalf("gmail search: %v\n%s", err, out)
	}
	g.mu.Lock()
	for _, c := range g.cmds {
		u := strings.ToUpper(c)
		if strings.HasPrefix(u, "SELECT") || strings.Contains(u, "STORE") || strings.Contains(u, "EXPUNGE") || strings.Contains(u, "COPY") || (strings.Contains(u, "FETCH") && !strings.Contains(u, "PEEK")) {
			t.Fatalf("mail must stay read-only, sent: %s", c)
		}
	}
	g.mu.Unlock()

	out, err = s.runCalendarTool(context.Background(), `{"action":"range","from":"2026-11-03","to":"2026-11-03"}`)
	if err != nil || !strings.Contains(out, "Reunió, equip") || !strings.Contains(out, "overlaps") || !strings.Contains(out, "[Personal]") {
		t.Fatalf("calendar: %v\n%s", err, out)
	}
	if b := s.windConnectionsBrief(); !strings.Contains(b, "calendar tool") || !strings.Contains(b, "mail tool") {
		t.Fatalf("brief: %q", b)
	}
}

func TestMorningBriefingRoutine(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	s := h.def
	t.Cleanup(func() { _ = s.jobs.Delete(briefingJobID("")) })
	wind, err := s.winds.Save(companion.Wind{Name: "Gregal", Color: "#C1B4DE", Role: "Organització"})
	if err != nil {
		t.Fatal(err)
	}
	if w := connPost(t, s, `{"briefing":{"enabled":true,"time":"7:5","wind":"`+wind.ID+`"}}`); w.Code != 400 {
		t.Fatalf("bad time must fail: %d", w.Code)
	}
	if w := connPost(t, s, `{"briefing":{"enabled":true,"time":"07:45","wind":"`+wind.ID+`"}}`); w.Code != 200 {
		t.Fatalf("enable briefing: %d %s", w.Code, w.Body.String())
	}
	list, _ := s.jobs.List()
	var found bool
	for _, j := range list {
		if j.ID == briefingJobID("") {
			found = j.Kind == "daily" && j.Time == "07:45" && j.Wind == wind.ID && strings.Contains(j.Prompt, "MORNING BRIEFING")
		}
	}
	if !found {
		t.Fatalf("briefing routine not created: %+v", list)
	}
	if w := connPost(t, s, `{"briefing":{"enabled":false}}`); w.Code != 200 || s.jobs.Has(briefingJobID("")) {
		t.Fatalf("disable briefing: %d", w.Code)
	}
}

// Els servidors solen anar en UTC: l'hora és la del calendari de Google de
// la persona, que es desa en afegir-lo.
func TestCalendarUsesTheCalendarTimezone(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	useFakeICS(t)
	h := testHubWithUsers(t)
	s := h.def
	if w := connPost(t, s, `{"add_calendar":{"name":"Personal","url":"https://calendar.google.com/x/basic.ics"}}`); w.Code != 200 {
		t.Fatalf("add: %d %s", w.Code, w.Body.String())
	}
	if tz := s.loadConnections().Timezone; tz != "Europe/Madrid" {
		t.Fatalf("timezone = %q", tz)
	}
	out, err := s.runCalendarTool(context.Background(), `{"action":"range","from":"2026-11-03","to":"2026-11-03"}`)
	if err != nil || !strings.Contains(out, "09:00–10:00 Reunió, equip") || !strings.Contains(out, "Europe/Madrid") {
		t.Fatalf("times must be the calendar's local time:\n%s %v", out, err)
	}
}

// Activar el resum a mig matí no l'ha d'enviar de seguida: el primer arriba
// a la propera hora triada.
func TestBriefingDoesNotFireWhenEnabled(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	h := testHubWithUsers(t)
	s := h.def
	t.Cleanup(func() { _ = s.jobs.Delete(briefingJobID("")) })
	wind, err := s.winds.Save(companion.Wind{Name: "Gregal", Color: "#C1B4DE", Role: "Organització"})
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour).Format("15:04")
	if w := connPost(t, s, `{"briefing":{"enabled":true,"time":"`+past+`","wind":"`+wind.ID+`"}}`); w.Code != 200 {
		t.Fatalf("enable: %d %s", w.Code, w.Body.String())
	}
	list, _ := s.jobs.List()
	for _, j := range list {
		if j.ID == briefingJobID("") {
			if jobs.Due(j, time.Now()) {
				t.Fatal("the briefing must wait for the next slot, not fire on enabling")
			}
			if next := jobs.NextDue(j, time.Now()); next.Sub(time.Now()) < 20*time.Hour {
				t.Fatalf("next briefing %v should be tomorrow", next)
			}
			return
		}
	}
	t.Fatal("briefing job missing")
}
