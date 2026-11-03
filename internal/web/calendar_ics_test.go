package web

import (
	"strings"
	"testing"
	"time"
)

// Un calendari com els que exporta Google Calendar: zona horària, dia
// sencer, setmanal amb BYDAY i excepció, una ocurrència moguda, mensual per
// «primer dilluns» i un de cancel·lat.
const googleICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Google Inc//Google Calendar 70.9054//EN\r\nX-WR-TIMEZONE:Europe/Madrid\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;TZID=Europe/Madrid:20261103T090000\r\nDTEND;TZID=Europe/Madrid:20261103T100000\r\nUID:a@google.com\r\nSUMMARY:Reunió\\, equip\r\nLOCATION:Sala 2\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;VALUE=DATE:20261104\r\nDTEND;VALUE=DATE:20261105\r\nUID:b@google.com\r\nSUMMARY:Aniversari Marta\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;TZID=Europe/Madrid:20261102T183000\r\nDTEND;TZID=Europe/Madrid:20261102T193000\r\nRRULE:FREQ=WEEKLY;BYDAY=MO,WE;UNTIL=20261130T225959Z\r\nEXDATE;TZID=Europe/Madrid:20261109T183000\r\nUID:c@google.com\r\nSUMMARY:Ioga\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;TZID=Europe/Madrid:20261111T200000\r\nDTEND;TZID=Europe/Madrid:20261111T210000\r\nRECURRENCE-ID;TZID=Europe/Madrid:20261111T183000\r\nUID:c@google.com\r\nSUMMARY:Ioga (tard)\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART:20261102T080000Z\r\nDTEND:20261102T083000Z\r\nRRULE:FREQ=MONTHLY;BYDAY=1MO\r\nUID:d@google.com\r\nSUMMARY:Factures\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;TZID=Europe/Madrid:20261103T093000\r\nDTEND;TZID=Europe/Madrid:20261103T094500\r\nUID:e@google.com\r\nSTATUS:CANCELLED\r\nSUMMARY:Cancel·lada\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nDTSTART;TZID=Europe/Madrid:20261103T094500\r\nDTEND;TZID=Europe/Madrid:20261103T103000\r\nUID:f@google.com\r\nSUMMARY:Trucada amb un nom molt llarg que Google plega en dues\r\n  línies\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestICSGoogleCalendar(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Skip("no tzdata")
	}
	evs := parseICS(googleICS, "Personal", loc)
	from := time.Date(2026, 11, 2, 0, 0, 0, 0, loc)
	got := eventsBetween(evs, from, from.AddDate(0, 0, 15))
	var names []string
	for _, e := range got {
		names = append(names, e.Start.Format("02 15:04")+" "+e.Summary)
	}
	want := []string{
		"02 09:00 Factures",
		"02 18:30 Ioga",
		"03 09:00 Reunió, equip",
		"03 09:45 Trucada amb un nom molt llarg que Google plega en dues línies",
		"04 00:00 Aniversari Marta",
		"04 18:30 Ioga",
		"11 20:00 Ioga (tard)",
		"16 18:30 Ioga",
	}
	if strings.Join(names, "\n") != strings.Join(want, "\n") {
		t.Fatalf("events:\n%s\nwant:\n%s", strings.Join(names, "\n"), strings.Join(want, "\n"))
	}
	text := formatEvents(eventsBetween(evs, time.Date(2026, 11, 3, 0, 0, 0, 0, loc), time.Date(2026, 11, 4, 0, 0, 0, 0, loc)))
	if !strings.Contains(text, "09:00–10:00 Reunió, equip (at Sala 2) [Personal] ⚠ overlaps") || strings.Contains(text, "Cancel·lada") {
		t.Fatalf("day text:\n%s", text)
	}
	if !strings.Contains(formatEvents(eventsBetween(evs, time.Date(2026, 11, 4, 0, 0, 0, 0, loc), time.Date(2026, 11, 5, 0, 0, 0, 0, loc))), "all day Aniversari Marta") {
		t.Fatal("all-day event")
	}
}

func TestICSMonthlyAndCount(t *testing.T) {
	loc := time.UTC
	ev := calEvent{Start: time.Date(2026, 1, 31, 10, 0, 0, 0, loc), End: time.Date(2026, 1, 31, 11, 0, 0, 0, loc), rrule: "FREQ=MONTHLY;BYMONTHDAY=-1;COUNT=3", exdates: map[int64]bool{}}
	occ := occurrences(ev, time.Date(2027, 1, 1, 0, 0, 0, 0, loc))
	if len(occ) != 3 || occ[1].Day() != 28 || occ[2].Day() != 31 {
		t.Fatalf("last day of month x3: %v", occ)
	}
	d := calEvent{Start: time.Date(2026, 11, 1, 7, 0, 0, 0, loc), rrule: "FREQ=DAILY;INTERVAL=2", exdates: map[int64]bool{}}
	if n := len(occurrences(d, time.Date(2026, 11, 10, 0, 0, 0, 0, loc))); n != 5 {
		t.Fatalf("every 2 days in 9 days: %d", n)
	}
}
