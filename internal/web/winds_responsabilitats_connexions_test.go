package web

import (
	"context"
	"strings"
	"testing"
	"time"
)

func icsWith(events ...string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" + strings.Join(events, "") + "END:VCALENDAR\r\n"
}

func icsEvent(uid, summary string, at time.Time) string {
	f := func(t time.Time) string { return t.UTC().Format("20060102T150405Z") }
	return "BEGIN:VEVENT\r\nUID:" + uid + "\r\nSUMMARY:" + summary + "\r\nDTSTART:" + f(at) + "\r\nDTEND:" + f(at.Add(time.Hour)) + "\r\nEND:VEVENT\r\n"
}

// TestRespConnectionSourceCalendar: una responsabilitat sobre un calendari
// connectat (B5) fixa el punt de partida a la primera mirada, no diu res
// sense canvis i porta només l'esdeveniment nou quan n'hi ha.
func TestRespConnectionSourceCalendar(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	tomorrow := time.Now().Add(26 * time.Hour).Truncate(time.Hour)
	body := icsWith(icsEvent("a@x", "Dentista", tomorrow))
	old := icsFetch
	icsFetch = func(context.Context, string) (string, error) { return body, nil }
	t.Cleanup(func() { icsFetch = old })

	s := hubTestServer(t)
	if w := connPost(t, s, `{"add_calendar":{"name":"Personal","url":"https://calendar.example.org/private-cal-secret.ics"}}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	srcs := s.connexioSources()
	if len(srcs) != 1 {
		t.Fatalf("fonts: %+v", srcs)
	}
	r := &responsabilitat{ID: "r1"}
	src, err := s.respSourceFor(r, respSource{Kind: "connection", Ref: srcs[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	first, err := src.Check(ctx, respCursor{})
	if err != nil || first.Changed || first.Cursor.Feed == "" || first.Cursor.Hash == "" {
		t.Fatalf("primera mirada: %v %+v", err, first)
	}
	same, err := src.Check(ctx, first.Cursor)
	if err != nil || same.Changed {
		t.Fatalf("sense canvis no és notícia: %v %+v", err, same)
	}

	body = icsWith(icsEvent("a@x", "Dentista", tomorrow), icsEvent("b@x", "Sopar amb la Marta", tomorrow.Add(3*time.Hour)))
	news, err := src.Check(ctx, same.Cursor)
	if err != nil || !news.Changed || !strings.Contains(news.Excerpt, "Sopar amb la Marta") || strings.Contains(news.Excerpt, "Dentista") {
		t.Fatalf("esdeveniment nou: %v %+v", err, news)
	}
	assertNoSecrets(t, "extracte", news.Excerpt)
	assertNoSecrets(t, "cursor", news.Cursor.Feed)

	// Una connexió que no és d'aquest compte (o inventada) no es pot vigilar.
	if _, err := s.respSourceFor(r, respSource{Kind: "connection", Ref: "con-altre"}); err == nil {
		t.Fatal("una connexió aliena no pot ser font")
	}
	if _, err := s.respSourceFor(r, respSource{Kind: "connection", Ref: "x"}); err == nil {
		t.Fatal("cal un id con-…")
	}
}
