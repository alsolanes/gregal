package google

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Tool és una acció que l'agent pot fer amb el compte. Read: només llegeix;
// la resta (Act) necessiten el permís «send» de la connexió.
type Tool struct {
	Name        string
	Read        bool
	Description string
	Arguments   string // esquema JSON curt, per al model
	Scope       string // l'àmbit que cal haver concedit
}

// Tools són les accions que ofereix un compte de Google.
var Tools = []Tool{
	{"calendar_events", true, "Events of the primary calendar between two dates (RFC 3339 or YYYY-MM-DD; default: the next 7 days). Optional text query.",
		`{"from":"2026-12-01","to":"2026-12-08","query":"","max":25}`, "https://www.googleapis.com/auth/calendar.events"},
	{"gmail_search", true, "Search the mailbox with Gmail syntax (from:, subject:, is:unread, newer_than:2d…). Returns id, date, from, subject and snippet.",
		`{"query":"is:unread newer_than:1d","max":15}`, "https://www.googleapis.com/auth/gmail.readonly"},
	{"gmail_read", true, "Read one message by id (from gmail_search): headers and plain-text body.",
		`{"id":"18c…"}`, "https://www.googleapis.com/auth/gmail.readonly"},
	{"drive_search", true, "Search Google Drive files by name or content. Returns name, type, date and link.",
		`{"query":"pressupost 2026","max":15}`, "https://www.googleapis.com/auth/drive.readonly"},
	{"calendar_create", false, "Create an event in the primary calendar. start/end RFC 3339 (or YYYY-MM-DD for all-day).",
		`{"summary":"Dentista","start":"2026-12-03T10:00:00+01:00","end":"2026-12-03T10:30:00+01:00","location":"","description":""}`, "https://www.googleapis.com/auth/calendar.events"},
	{"gmail_draft", false, "Prepare a draft (never sends: the user sends it from Gmail). to, subject and plain-text body.",
		`{"to":"anna@example.com","subject":"…","body":"…"}`, "https://www.googleapis.com/auth/gmail.compose"},
}

// FindTool torna l'acció pel nom.
func FindTool(name string) (Tool, bool) {
	for _, t := range Tools {
		if t.Name == strings.TrimSpace(name) {
			return t, true
		}
	}
	return Tool{}, false
}

func clamp(n, def, max int) int {
	if n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func parseWhen(s string, def time.Time) (time.Time, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return def, false, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, false, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, true, nil
	}
	return def, false, fmt.Errorf("unreadable date %q (use RFC 3339 or YYYY-MM-DD)", s)
}

// Run executa una acció amb els arguments JSON i en torna el text per al model.
func (c *Client) Run(ctx context.Context, name, args string) (string, error) {
	if strings.TrimSpace(args) == "" {
		args = "{}"
	}
	switch name {
	case "calendar_events":
		var a struct {
			From, To, Query string
			Max             int
		}
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return "", errors.New("unreadable arguments")
		}
		now := time.Now()
		from, _, err := parseWhen(a.From, now)
		if err != nil {
			return "", err
		}
		to, _, err := parseWhen(a.To, from.AddDate(0, 0, 7))
		if err != nil {
			return "", err
		}
		return c.calendarEvents(ctx, from, to, a.Query, clamp(a.Max, 25, 100))
	case "gmail_search":
		var a struct {
			Query string
			Max   int
		}
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return "", errors.New("unreadable arguments")
		}
		return c.gmailSearch(ctx, a.Query, clamp(a.Max, 15, 30))
	case "gmail_read":
		var a struct{ ID string }
		if err := json.Unmarshal([]byte(args), &a); err != nil || strings.TrimSpace(a.ID) == "" {
			return "", errors.New("gmail_read needs an id from gmail_search")
		}
		return c.gmailRead(ctx, a.ID)
	case "drive_search":
		var a struct {
			Query string
			Max   int
		}
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return "", errors.New("unreadable arguments")
		}
		return c.driveSearch(ctx, a.Query, clamp(a.Max, 15, 50))
	case "calendar_create":
		var a struct{ Summary, Start, End, Location, Description string }
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return "", errors.New("unreadable arguments")
		}
		return c.calendarCreate(ctx, a.Summary, a.Start, a.End, a.Location, a.Description)
	case "gmail_draft":
		var a struct{ To, Subject, Body string }
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return "", errors.New("unreadable arguments")
		}
		return c.gmailDraft(ctx, a.To, a.Subject, a.Body)
	}
	return "", fmt.Errorf("unknown Google action %q", name)
}

type eventTime struct {
	DateTime string `json:"dateTime,omitempty"`
	Date     string `json:"date,omitempty"`
}

func (e eventTime) String() string {
	if e.DateTime != "" {
		if t, err := time.Parse(time.RFC3339, e.DateTime); err == nil {
			return t.Local().Format("Mon 2006-01-02 15:04")
		}
		return e.DateTime
	}
	return e.Date + " (all day)"
}

func (c *Client) calendarEvents(ctx context.Context, from, to time.Time, query string, max int) (string, error) {
	q := url.Values{"timeMin": {from.Format(time.RFC3339)}, "timeMax": {to.Format(time.RFC3339)},
		"singleEvents": {"true"}, "orderBy": {"startTime"}, "maxResults": {fmt.Sprint(max)}}
	if strings.TrimSpace(query) != "" {
		q.Set("q", query)
	}
	var res struct {
		Items []struct {
			Summary, Location, Status string
			Start, End                eventTime
			Attendees                 []struct{ Email string }
		}
	}
	if err := c.call(ctx, http.MethodGet, apiBase+"/calendar/v3/calendars/primary/events?"+q.Encode(), nil, &res); err != nil {
		return "", err
	}
	if len(res.Items) == 0 {
		return "No events between " + from.Format("2006-01-02") + " and " + to.Format("2006-01-02") + ".", nil
	}
	var b strings.Builder
	for _, e := range res.Items {
		if e.Status == "cancelled" {
			continue
		}
		fmt.Fprintf(&b, "- %s → %s · %s", e.Start, e.End, firstNonEmpty(e.Summary, "(no title)"))
		if e.Location != "" {
			b.WriteString(" · " + e.Location)
		}
		if n := len(e.Attendees); n > 0 {
			fmt.Fprintf(&b, " · %d attendees", n)
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

type gmailHeader struct{ Name, Value string }

type gmailPart struct {
	MimeType string
	Headers  []gmailHeader
	Body     struct{ Data string }
	Parts    []gmailPart
}

func header(h []gmailHeader, name string) string {
	for _, x := range h {
		if strings.EqualFold(x.Name, name) {
			return x.Value
		}
	}
	return ""
}

func (c *Client) gmailSearch(ctx context.Context, query string, max int) (string, error) {
	q := url.Values{"maxResults": {fmt.Sprint(max)}}
	if strings.TrimSpace(query) != "" {
		q.Set("q", query)
	}
	var list struct {
		Messages []struct{ ID string }
	}
	if err := c.call(ctx, http.MethodGet, gmailBase+"/gmail/v1/users/me/messages?"+q.Encode(), nil, &list); err != nil {
		return "", err
	}
	if len(list.Messages) == 0 {
		return "No messages match.", nil
	}
	var b strings.Builder
	for _, m := range list.Messages {
		var msg struct {
			ID, Snippet string
			LabelIDs    []string `json:"labelIds"`
			Payload     gmailPart
		}
		u := gmailBase + "/gmail/v1/users/me/messages/" + url.PathEscape(m.ID) + "?format=metadata&metadataHeaders=From&metadataHeaders=Subject&metadataHeaders=Date"
		if err := c.call(ctx, http.MethodGet, u, nil, &msg); err != nil {
			return "", err
		}
		unread := ""
		for _, l := range msg.LabelIDs {
			if l == "UNREAD" {
				unread = " · unread"
			}
		}
		fmt.Fprintf(&b, "- id %s · %s · from %s · %s%s\n  %s\n", msg.ID, header(msg.Payload.Headers, "Date"),
			header(msg.Payload.Headers, "From"), firstNonEmpty(header(msg.Payload.Headers, "Subject"), "(no subject)"), unread, msg.Snippet)
	}
	return b.String(), nil
}

func plainText(p gmailPart) string {
	if strings.HasPrefix(p.MimeType, "text/plain") && p.Body.Data != "" {
		if b, err := base64.URLEncoding.DecodeString(p.Body.Data); err == nil {
			return string(b)
		}
		if b, err := base64.RawURLEncoding.DecodeString(p.Body.Data); err == nil {
			return string(b)
		}
	}
	for _, sub := range p.Parts {
		if t := plainText(sub); t != "" {
			return t
		}
	}
	return ""
}

func (c *Client) gmailRead(ctx context.Context, id string) (string, error) {
	var msg struct {
		Snippet string
		Payload gmailPart
	}
	if err := c.call(ctx, http.MethodGet, gmailBase+"/gmail/v1/users/me/messages/"+url.PathEscape(strings.TrimSpace(id))+"?format=full", nil, &msg); err != nil {
		return "", err
	}
	h := msg.Payload.Headers
	body := strings.TrimSpace(plainText(msg.Payload))
	if body == "" {
		body = msg.Snippet + "\n(no plain-text part; this is the snippet)"
	}
	if r := []rune(body); len(r) > 8000 {
		body = string(r[:8000]) + "\n… (truncated)"
	}
	return fmt.Sprintf("From: %s\nTo: %s\nDate: %s\nSubject: %s\n\n%s", header(h, "From"), header(h, "To"), header(h, "Date"), header(h, "Subject"), body), nil
}

func (c *Client) driveSearch(ctx context.Context, query string, max int) (string, error) {
	q := "trashed = false"
	if s := strings.TrimSpace(query); s != "" {
		esc := strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `'`, `\'`)
		q += " and (name contains '" + esc + "' or fullText contains '" + esc + "')"
	}
	v := url.Values{"q": {q}, "pageSize": {fmt.Sprint(max)}, "orderBy": {"modifiedTime desc"},
		"fields": {"files(id,name,mimeType,modifiedTime,webViewLink)"}}
	var res struct {
		Files []struct{ ID, Name, MimeType, ModifiedTime, WebViewLink string }
	}
	if err := c.call(ctx, http.MethodGet, apiBase+"/drive/v3/files?"+v.Encode(), nil, &res); err != nil {
		return "", err
	}
	if len(res.Files) == 0 {
		return "No files match.", nil
	}
	var b strings.Builder
	for _, f := range res.Files {
		kind := strings.TrimPrefix(f.MimeType, "application/vnd.google-apps.")
		fmt.Fprintf(&b, "- %s · %s · %s · %s\n", f.Name, kind, f.ModifiedTime, f.WebViewLink)
	}
	return b.String(), nil
}

func (c *Client) calendarCreate(ctx context.Context, summary, start, end, location, description string) (string, error) {
	if strings.TrimSpace(summary) == "" || strings.TrimSpace(start) == "" {
		return "", errors.New("calendar_create needs at least summary and start")
	}
	s, allDay, err := parseWhen(start, time.Time{})
	if err != nil {
		return "", err
	}
	e, _, err := parseWhen(end, s.Add(time.Hour))
	if err != nil {
		return "", err
	}
	ev := map[string]any{"summary": summary}
	if location != "" {
		ev["location"] = location
	}
	if description != "" {
		ev["description"] = description
	}
	if allDay {
		if !e.After(s) {
			e = s.AddDate(0, 0, 1)
		}
		ev["start"], ev["end"] = eventTime{Date: s.Format("2006-01-02")}, eventTime{Date: e.Format("2006-01-02")}
	} else {
		ev["start"], ev["end"] = eventTime{DateTime: s.Format(time.RFC3339)}, eventTime{DateTime: e.Format(time.RFC3339)}
	}
	var out struct{ HTMLLink string }
	if err := c.call(ctx, http.MethodPost, apiBase+"/calendar/v3/calendars/primary/events", ev, &out); err != nil {
		return "", err
	}
	return "Event created: " + summary + " · " + out.HTMLLink, nil
}

func (c *Client) gmailDraft(ctx context.Context, to, subject, body string) (string, error) {
	if strings.TrimSpace(to) == "" {
		return "", errors.New("gmail_draft needs to")
	}
	if strings.ContainsAny(to+subject, "\r\n") {
		return "", errors.New("to and subject must be a single line")
	}
	raw := "To: " + to + "\r\nSubject: " + mime.QEncoding.Encode("utf-8", subject) +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + body
	req := map[string]any{"message": map[string]string{"raw": base64.RawURLEncoding.EncodeToString([]byte(raw))}}
	var out struct{ ID string }
	if err := c.call(ctx, http.MethodPost, gmailBase+"/gmail/v1/users/me/drafts", req, &out); err != nil {
		return "", err
	}
	return "Draft saved in Gmail (not sent): «" + subject + "» to " + to + ". The user reviews and sends it from Gmail.", nil
}
