package web

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // zones de Google Calendar també al backend d'Android (sense zoneinfo)
)

// Calendari de només lectura a partir d'URL iCal (l'«adreça secreta en format
// iCal» de Google Calendar, o la de qualsevol calendari). No cal Google Cloud
// ni OAuth: la persona enganxa l'URL i els vents poden veure el seu dia.
// L'analitzador cobreix el que fan servir els calendaris de debò: dies
// sencers, zones horàries, repeticions (RRULE amb INTERVAL, COUNT, UNTIL,
// BYDAY, BYMONTHDAY), excepcions (EXDATE), ocurrències modificades
// (RECURRENCE-ID) i esdeveniments cancel·lats.

type calEvent struct {
	UID      string
	Summary  string
	Location string
	Start    time.Time
	End      time.Time
	AllDay   bool
	Calendar string
	rrule    string
	exdates  map[int64]bool
	recurID  time.Time
	status   string
}

const (
	icsMaxBytes    = 8 << 20
	icsCacheFor    = 5 * time.Minute
	icsMaxInstance = 5000
)

var (
	icsCacheMu sync.Mutex
	icsCache   = map[string]icsCached{}
	// icsFetch es pot substituir als tests.
	icsFetch = fetchICS
)

type icsCached struct {
	at   time.Time
	body string
}

func fetchICS(ctx context.Context, url string) (string, error) {
	url = strings.TrimSpace(url)
	if strings.HasPrefix(strings.ToLower(url), "webcal://") {
		url = "https://" + url[len("webcal://"):]
	}
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return "", errors.New("the calendar address must start with https:// (or webcal://)")
	}
	icsCacheMu.Lock()
	if c, ok := icsCache[url]; ok && time.Since(c.at) < icsCacheFor {
		icsCacheMu.Unlock()
		return c.body, nil
	}
	icsCacheMu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Gregal calendar reader")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach the calendar: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("the calendar answered HTTP %d (is the secret address right?)", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, icsMaxBytes))
	if err != nil {
		return "", err
	}
	body := string(raw)
	if !strings.Contains(body, "BEGIN:VCALENDAR") {
		return "", errors.New("that address does not return an iCal calendar (use the secret address in iCal format)")
	}
	icsCacheMu.Lock()
	icsCache[url] = icsCached{at: time.Now(), body: body}
	icsCacheMu.Unlock()
	return body, nil
}

// icsZone és la zona del calendari (X-WR-TIMEZONE, la que posa Google).
func icsZone(body string) string {
	for _, line := range unfoldICS(body) {
		if name, _, value := icsProp(line); name == "X-WR-TIMEZONE" {
			if _, err := time.LoadLocation(strings.TrimSpace(value)); err == nil {
				return strings.TrimSpace(value)
			}
		}
		if strings.HasPrefix(line, "BEGIN:VEVENT") {
			break
		}
	}
	return ""
}

// unfoldICS junta les línies plegades (RFC 5545 §3.1).
func unfoldICS(body string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(out) > 0 {
			out[len(out)-1] += line[1:]
			continue
		}
		out = append(out, line)
	}
	return out
}

func icsUnescape(s string) string {
	r := strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`)
	return r.Replace(s)
}

// icsProp separa «NOM;PARAM=X:valor» en nom, paràmetres i valor.
func icsProp(line string) (string, map[string]string, string) {
	colon := -1
	inQuote := false
	for i, c := range line {
		if c == '"' {
			inQuote = !inQuote
		}
		if c == ':' && !inQuote {
			colon = i
			break
		}
	}
	if colon < 0 {
		return "", nil, ""
	}
	head, value := line[:colon], line[colon+1:]
	parts := strings.Split(head, ";")
	params := map[string]string{}
	for _, p := range parts[1:] {
		if k, v, ok := strings.Cut(p, "="); ok {
			params[strings.ToUpper(k)] = strings.Trim(v, `"`)
		}
	}
	return strings.ToUpper(parts[0]), params, value
}

// icsTime llegeix una data o data-hora amb la seva zona.
func icsTime(value string, params map[string]string, local *time.Location) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if params["VALUE"] == "DATE" || len(value) == 8 {
		t, err := time.ParseInLocation("20060102", value, local)
		return t, true, err
	}
	if strings.HasSuffix(value, "Z") {
		t, err := time.Parse("20060102T150405Z", value)
		return t.In(local), false, err
	}
	loc := local
	if tz := params["TZID"]; tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}
	t, err := time.ParseInLocation("20060102T150405", value, loc)
	return t.In(local), false, err
}

func icsDuration(v string) time.Duration {
	// P1D, PT1H30M, P1DT2H, -PT15M
	v = strings.TrimSpace(strings.ToUpper(v))
	neg := strings.HasPrefix(v, "-")
	v = strings.TrimLeft(v, "+-")
	v = strings.TrimPrefix(v, "P")
	var d time.Duration
	inTime := false
	num := ""
	for _, c := range v {
		switch {
		case c >= '0' && c <= '9':
			num += string(c)
		case c == 'T':
			inTime = true
		default:
			n, _ := strconv.Atoi(num)
			num = ""
			switch {
			case c == 'W':
				d += time.Duration(n) * 7 * 24 * time.Hour
			case c == 'D':
				d += time.Duration(n) * 24 * time.Hour
			case c == 'H' && inTime:
				d += time.Duration(n) * time.Hour
			case c == 'M' && inTime:
				d += time.Duration(n) * time.Minute
			case c == 'S' && inTime:
				d += time.Duration(n) * time.Second
			}
		}
	}
	if neg {
		d = -d
	}
	return d
}

// parseICS llegeix els VEVENT d'un calendari.
func parseICS(body, calendar string, local *time.Location) []calEvent {
	var out []calEvent
	var cur *calEvent
	var dur time.Duration
	hasEnd := false
	for _, line := range unfoldICS(body) {
		switch strings.ToUpper(strings.TrimSpace(line)) {
		case "BEGIN:VEVENT":
			cur = &calEvent{Calendar: calendar, exdates: map[int64]bool{}}
			dur, hasEnd = 0, false
			continue
		case "END:VEVENT":
			if cur != nil && !cur.Start.IsZero() {
				if !hasEnd {
					switch {
					case dur != 0:
						cur.End = cur.Start.Add(dur)
					case cur.AllDay:
						cur.End = cur.Start.AddDate(0, 0, 1)
					default:
						cur.End = cur.Start
					}
				}
				out = append(out, *cur)
			}
			cur = nil
			continue
		}
		if cur == nil {
			continue
		}
		name, params, value := icsProp(line)
		switch name {
		case "UID":
			cur.UID = value
		case "SUMMARY":
			cur.Summary = icsUnescape(value)
		case "LOCATION":
			cur.Location = icsUnescape(value)
		case "STATUS":
			cur.status = strings.ToUpper(value)
		case "DTSTART":
			if t, allDay, err := icsTime(value, params, local); err == nil {
				cur.Start, cur.AllDay = t, allDay
			}
		case "DTEND":
			if t, _, err := icsTime(value, params, local); err == nil {
				cur.End, hasEnd = t, true
			}
		case "DURATION":
			dur = icsDuration(value)
		case "RRULE":
			cur.rrule = value
		case "EXDATE":
			for _, v := range strings.Split(value, ",") {
				if t, _, err := icsTime(v, params, local); err == nil {
					cur.exdates[t.Unix()] = true
				}
			}
		case "RECURRENCE-ID":
			if t, _, err := icsTime(value, params, local); err == nil {
				cur.recurID = t
			}
		}
	}
	return out
}

var icsWeekdays = map[string]time.Weekday{"SU": time.Sunday, "MO": time.Monday, "TU": time.Tuesday, "WE": time.Wednesday, "TH": time.Thursday, "FR": time.Friday, "SA": time.Saturday}

type rruleSpec struct {
	freq     string
	interval int
	count    int
	until    time.Time
	byDay    []struct {
		n  int
		wd time.Weekday
	}
	byMonthDay []int
}

func parseRRule(v string, local *time.Location) rruleSpec {
	r := rruleSpec{interval: 1}
	for _, part := range strings.Split(v, ";") {
		k, val, _ := strings.Cut(part, "=")
		switch strings.ToUpper(k) {
		case "FREQ":
			r.freq = strings.ToUpper(val)
		case "INTERVAL":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				r.interval = n
			}
		case "COUNT":
			r.count, _ = strconv.Atoi(val)
		case "UNTIL":
			if t, _, err := icsTime(val, nil, local); err == nil {
				r.until = t
				if len(strings.TrimSpace(val)) == 8 {
					r.until = t.AddDate(0, 0, 1).Add(-time.Second)
				}
			}
		case "BYDAY":
			for _, d := range strings.Split(val, ",") {
				d = strings.ToUpper(strings.TrimSpace(d))
				if len(d) < 2 {
					continue
				}
				wd, ok := icsWeekdays[d[len(d)-2:]]
				if !ok {
					continue
				}
				n, _ := strconv.Atoi(d[:len(d)-2])
				r.byDay = append(r.byDay, struct {
					n  int
					wd time.Weekday
				}{n, wd})
			}
		case "BYMONTHDAY":
			for _, d := range strings.Split(val, ",") {
				if n, err := strconv.Atoi(strings.TrimSpace(d)); err == nil {
					r.byMonthDay = append(r.byMonthDay, n)
				}
			}
		}
	}
	return r
}

// nthWeekday: el n-èsim dia wd del mes (n<0 compta des del final).
func nthWeekday(year int, month time.Month, wd time.Weekday, n int, ref time.Time) (time.Time, bool) {
	loc := ref.Location()
	h, m, s := ref.Clock()
	if n > 0 {
		first := time.Date(year, month, 1, h, m, s, 0, loc)
		offset := (int(wd) - int(first.Weekday()) + 7) % 7
		d := first.AddDate(0, 0, offset+(n-1)*7)
		return d, d.Month() == month
	}
	last := time.Date(year, month+1, 0, h, m, s, 0, loc)
	offset := (int(last.Weekday()) - int(wd) + 7) % 7
	d := last.AddDate(0, 0, -offset+(n+1)*7)
	return d, d.Month() == month
}

// occurrences torna els inicis de les ocurrències fins a «to».
func occurrences(ev calEvent, to time.Time) []time.Time {
	if ev.rrule == "" {
		return []time.Time{ev.Start}
	}
	r := parseRRule(ev.rrule, ev.Start.Location())
	var out []time.Time
	emitted := 0
	add := func(t time.Time) bool {
		if t.Before(ev.Start) {
			return true
		}
		if !r.until.IsZero() && t.After(r.until) {
			return false
		}
		if r.count > 0 && emitted >= r.count {
			return false
		}
		if t.After(to) {
			return false
		}
		emitted++
		out = append(out, t)
		return len(out) < icsMaxInstance
	}
	start := ev.Start
	for i := 0; i < icsMaxInstance*4; i++ {
		var cands []time.Time
		switch r.freq {
		case "DAILY":
			cands = []time.Time{start.AddDate(0, 0, i*r.interval)}
		case "WEEKLY":
			weekStart := start.AddDate(0, 0, -((int(start.Weekday())+6)%7)+i*7*r.interval)
			if len(r.byDay) == 0 {
				cands = []time.Time{start.AddDate(0, 0, i*7*r.interval)}
			} else {
				for _, d := range r.byDay {
					cands = append(cands, weekStart.AddDate(0, 0, (int(d.wd)+6)%7))
				}
				sort.Slice(cands, func(a, b int) bool { return cands[a].Before(cands[b]) })
			}
		case "MONTHLY":
			base := time.Date(start.Year(), start.Month()+time.Month(i*r.interval), 1, start.Hour(), start.Minute(), start.Second(), 0, start.Location())
			switch {
			case len(r.byDay) > 0:
				for _, d := range r.byDay {
					n := d.n
					if n == 0 {
						n = 1
					}
					if t, ok := nthWeekday(base.Year(), base.Month(), d.wd, n, start); ok {
						cands = append(cands, t)
					}
				}
			case len(r.byMonthDay) > 0:
				for _, md := range r.byMonthDay {
					t := time.Date(base.Year(), base.Month(), md, start.Hour(), start.Minute(), start.Second(), 0, start.Location())
					if md < 0 {
						t = time.Date(base.Year(), base.Month()+1, md+1, start.Hour(), start.Minute(), start.Second(), 0, start.Location())
					}
					if t.Month() == base.Month() {
						cands = append(cands, t)
					}
				}
			default:
				t := time.Date(base.Year(), base.Month(), start.Day(), start.Hour(), start.Minute(), start.Second(), 0, start.Location())
				if t.Month() == base.Month() {
					cands = append(cands, t)
				}
			}
			sort.Slice(cands, func(a, b int) bool { return cands[a].Before(cands[b]) })
		case "YEARLY":
			t := time.Date(start.Year()+i*r.interval, start.Month(), start.Day(), start.Hour(), start.Minute(), start.Second(), 0, start.Location())
			if t.Day() == start.Day() {
				cands = []time.Time{t}
			}
		default:
			return []time.Time{ev.Start}
		}
		for _, c := range cands {
			if !add(c) {
				return out
			}
		}
		if len(cands) > 0 && cands[0].After(to) {
			break
		}
	}
	return out
}

// eventsBetween expandeix les repeticions i torna els esdeveniments que
// toquen [from, to), ordenats.
func eventsBetween(events []calEvent, from, to time.Time) []calEvent {
	overrides := map[string]calEvent{}
	for _, e := range events {
		if !e.recurID.IsZero() {
			overrides[e.UID+"|"+strconv.FormatInt(e.recurID.Unix(), 10)] = e
		}
	}
	var out []calEvent
	keep := func(e calEvent) {
		if e.status == "CANCELLED" {
			return
		}
		end := e.End
		if !end.After(e.Start) {
			end = e.Start.Add(time.Minute)
		}
		if e.Start.Before(to) && end.After(from) {
			out = append(out, e)
		}
	}
	for _, e := range events {
		if !e.recurID.IsZero() {
			keep(e)
			continue
		}
		length := e.End.Sub(e.Start)
		for _, t := range occurrences(e, to) {
			if e.exdates[t.Unix()] {
				continue
			}
			if _, replaced := overrides[e.UID+"|"+strconv.FormatInt(t.Unix(), 10)]; replaced {
				continue
			}
			inst := e
			inst.Start, inst.End = t, t.Add(length)
			keep(inst)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Start.Equal(out[j].Start) {
			return out[i].Summary < out[j].Summary
		}
		return out[i].Start.Before(out[j].Start)
	})
	return out
}

// formatEvents és el text que llegeix el vent, amb els solapaments marcats.
func formatEvents(events []calEvent) string {
	if len(events) == 0 {
		return "No events."
	}
	var b strings.Builder
	day := ""
	for i, e := range events {
		d := e.Start.Format("Mon 02 Jan 2006")
		if d != day {
			if day != "" {
				b.WriteByte('\n')
			}
			b.WriteString(d + ":\n")
			day = d
		}
		when := "all day"
		if !e.AllDay {
			when = e.Start.Format("15:04") + "–" + e.End.Format("15:04")
		}
		fmt.Fprintf(&b, "- %s %s", when, strings.TrimSpace(e.Summary))
		if e.Location != "" {
			fmt.Fprintf(&b, " (at %s)", e.Location)
		}
		if e.Calendar != "" {
			fmt.Fprintf(&b, " [%s]", e.Calendar)
		}
		if !e.AllDay {
			for j, o := range events {
				if j != i && !o.AllDay && o.Start.Before(e.End) && e.Start.Before(o.End) {
					fmt.Fprintf(&b, " ⚠ overlaps with %q", o.Summary)
					break
				}
			}
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
