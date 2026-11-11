package web

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/htmlindex"
)

// Correu de només lectura per IMAP. Pensat per a Gmail amb una contrasenya
// d'aplicació (imap.gmail.com:993), però serveix per a qualsevol IMAP amb TLS.
// La bústia s'obre amb EXAMINE i els cossos es llegeixen amb BODY.PEEK: llegir
// un correu des d'un vent no el marca com a llegit ni hi canvia res. Amb Gmail
// la cerca fa servir la mateixa sintaxi que el quadre de cerca (X-GM-RAW:
// «from:anna newer_than:2d is:unread»).

const (
	imapTimeout     = 60 * time.Second
	imapMaxLiteral  = 25 << 20
	imapBodyPartial = 200000
	mailTextMax     = 6000
)

type imapClient struct {
	conn  net.Conn
	r     *bufio.Reader
	n     int
	gmail bool
}

// imapDial obre la connexió TLS (els tests el substitueixen).
var imapDial = func(host string, port int) (net.Conn, error) {
	d := &net.Dialer{Timeout: 15 * time.Second}
	return tls.DialWithDialer(d, "tcp", net.JoinHostPort(host, strconv.Itoa(port)), &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
}

type imapResp struct {
	text     string
	literals []string
}

var reIMAPLiteral = regexp.MustCompile(`\{(\d+)\+?\}$`)

func imapOpen(host string, port int) (*imapClient, error) {
	conn, err := imapDial(host, port)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %s: %v", host, err)
	}
	_ = conn.SetDeadline(time.Now().Add(imapTimeout))
	h := strings.ToLower(host)
	c := &imapClient{conn: conn, r: bufio.NewReaderSize(conn, 64<<10), gmail: strings.HasSuffix(h, "gmail.com") || strings.HasSuffix(h, "googlemail.com")}
	greet, err := c.readResp()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.HasPrefix(greet.text, "* OK") && !strings.HasPrefix(greet.text, "* PREAUTH") {
		conn.Close()
		return nil, fmt.Errorf("unexpected IMAP greeting: %s", firstLines(greet.text, 1))
	}
	return c, nil
}

func (c *imapClient) close() {
	if c == nil || c.conn == nil {
		return
	}
	_, _ = c.cmd("LOGOUT")
	_ = c.conn.Close()
}

// readResp llegeix una resposta sencera, amb els literals {N} a part.
func (c *imapClient) readResp() (imapResp, error) {
	var b strings.Builder
	var lits []string
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			return imapResp{}, fmt.Errorf("IMAP connection closed: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if m := reIMAPLiteral.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n > imapMaxLiteral {
				return imapResp{}, errors.New("IMAP message too large")
			}
			buf := make([]byte, n)
			if _, err := io.ReadFull(c.r, buf); err != nil {
				return imapResp{}, err
			}
			b.WriteString(line[:len(line)-len(m[0])])
			b.WriteString("\x00")
			lits = append(lits, string(buf))
			continue
		}
		b.WriteString(line)
		return imapResp{text: b.String(), literals: lits}, nil
	}
}

func (c *imapClient) cmd(command string) ([]imapResp, error) {
	c.n++
	tag := "g" + strconv.Itoa(c.n)
	if _, err := io.WriteString(c.conn, tag+" "+command+"\r\n"); err != nil {
		return nil, err
	}
	var out []imapResp
	for {
		r, err := c.readResp()
		if err != nil {
			return out, err
		}
		if strings.HasPrefix(r.text, tag+" ") {
			rest := strings.TrimSpace(r.text[len(tag)+1:])
			if strings.HasPrefix(strings.ToUpper(rest), "OK") {
				return out, nil
			}
			return out, fmt.Errorf("IMAP: %s", rest)
		}
		out = append(out, r)
	}
}

func imapQuote(s string) (string, error) {
	if strings.ContainsAny(s, "\r\n") {
		return "", errors.New("line breaks are not allowed")
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`, nil
}

// imapAuthError: el servidor ha rebutjat les credencials (cal tornar a
// connectar el compte, no és un error de xarxa).
type imapAuthError struct{ msg string }

func (e *imapAuthError) Error() string { return e.msg }

func (c *imapClient) login(user, pass, folder string) error {
	u, err := imapQuote(user)
	if err != nil {
		return err
	}
	p, err := imapQuote(pass)
	if err != nil {
		return err
	}
	if _, err := c.cmd("LOGIN " + u + " " + p); err != nil {
		if c.gmail {
			return &imapAuthError{msg: "Gmail rejected the login. Use an app password (Google Account → Security → 2-Step Verification → App passwords), not your normal password, and check that IMAP is on in Gmail settings"}
		}
		return &imapAuthError{msg: fmt.Sprintf("login rejected: %v", err)}
	}
	if folder == "" {
		folder = "INBOX"
	}
	f, err := imapQuote(folder)
	if err != nil {
		return err
	}
	// EXAMINE: només lectura, no toca cap marca.
	if _, err := c.cmd("EXAMINE " + f); err != nil {
		return fmt.Errorf("cannot open %s: %v", folder, err)
	}
	return nil
}

// search torna els UID que compleixen el criteri, del més nou al més vell.
func (c *imapClient) search(criteria string) ([]int, error) {
	rs, err := c.cmd("UID SEARCH " + criteria)
	if err != nil {
		return nil, err
	}
	var uids []int
	for _, r := range rs {
		if !strings.HasPrefix(strings.ToUpper(r.text), "* SEARCH") {
			continue
		}
		for _, f := range strings.Fields(r.text[len("* SEARCH"):]) {
			if n, err := strconv.Atoi(f); err == nil {
				uids = append(uids, n)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(uids)))
	return uids, nil
}

type mailSummary struct {
	UID     int
	From    string
	Subject string
	Date    time.Time
	Seen    bool
}

var (
	reFetchUID   = regexp.MustCompile(`UID (\d+)`)
	reFetchFlags = regexp.MustCompile(`FLAGS \(([^)]*)\)`)
)

var mimeWords = &mime.WordDecoder{CharsetReader: func(charset string, in io.Reader) (io.Reader, error) {
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return in, nil
	}
	return enc.NewDecoder().Reader(in), nil
}}

func decodeHeader(v string) string {
	if d, err := mimeWords.DecodeHeader(v); err == nil {
		return strings.TrimSpace(d)
	}
	return strings.TrimSpace(v)
}

func (c *imapClient) summaries(uids []int) ([]mailSummary, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	ids := make([]string, len(uids))
	for i, u := range uids {
		ids[i] = strconv.Itoa(u)
	}
	rs, err := c.cmd("UID FETCH " + strings.Join(ids, ",") + " (UID FLAGS BODY.PEEK[HEADER.FIELDS (FROM SUBJECT DATE)])")
	if err != nil {
		return nil, err
	}
	byUID := map[int]mailSummary{}
	for _, r := range rs {
		if !strings.Contains(strings.ToUpper(r.text), "FETCH") || len(r.literals) == 0 {
			continue
		}
		m := reFetchUID.FindStringSubmatch(r.text)
		if m == nil {
			continue
		}
		uid, _ := strconv.Atoi(m[1])
		s := mailSummary{UID: uid}
		if f := reFetchFlags.FindStringSubmatch(r.text); f != nil {
			s.Seen = strings.Contains(f[1], `\Seen`)
		}
		if msg, err := mail.ReadMessage(strings.NewReader(r.literals[0] + "\r\n")); err == nil {
			s.From = decodeHeader(msg.Header.Get("From"))
			s.Subject = decodeHeader(msg.Header.Get("Subject"))
			if d, err := mail.ParseDate(msg.Header.Get("Date")); err == nil {
				s.Date = d
			}
		}
		byUID[uid] = s
	}
	out := make([]mailSummary, 0, len(uids))
	for _, u := range uids {
		if s, ok := byUID[u]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

// body torna el text llegible d'un missatge (text/plain, o l'HTML net).
func (c *imapClient) body(uid int) (mailSummary, string, error) {
	rs, err := c.cmd("UID FETCH " + strconv.Itoa(uid) + " (UID FLAGS BODY.PEEK[]<0." + strconv.Itoa(imapBodyPartial) + ">)")
	if err != nil {
		return mailSummary{}, "", err
	}
	for _, r := range rs {
		if len(r.literals) == 0 || !reFetchUID.MatchString(r.text) {
			continue
		}
		raw := r.literals[0]
		msg, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			return mailSummary{}, "", fmt.Errorf("unreadable message: %v", err)
		}
		s := mailSummary{UID: uid, From: decodeHeader(msg.Header.Get("From")), Subject: decodeHeader(msg.Header.Get("Subject"))}
		if d, err := mail.ParseDate(msg.Header.Get("Date")); err == nil {
			s.Date = d
		}
		text := mailText(msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), msg.Body, 0)
		if r := []rune(text); len(r) > mailTextMax {
			text = string(r[:mailTextMax]) + "\n…(truncated)"
		}
		return s, text, nil
	}
	return mailSummary{}, "", fmt.Errorf("no message with id %d", uid)
}

var (
	reHTMLDrop  = regexp.MustCompile(`(?is)<(script|style|head)[^>]*>.*?</(script|style|head)>`)
	reHTMLBreak = regexp.MustCompile(`(?i)<(br|/p|/div|/tr|/li|/h[1-6])[^>]*>`)
	reHTMLTag   = regexp.MustCompile(`(?s)<[^>]+>`)
	reBlank     = regexp.MustCompile(`\n[ \t]*\n[\s]*`)
)

func htmlToText(s string) string {
	s = reHTMLDrop.ReplaceAllString(s, " ")
	s = reHTMLBreak.ReplaceAllString(s, "\n")
	s = reHTMLTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	return strings.TrimSpace(reBlank.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// mailText descodifica una part (i les seves subparts) i en treu el text.
func mailText(contentType, encoding string, body io.Reader, depth int) string {
	if depth > 6 {
		return ""
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = "text/plain"
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(body, params["boundary"])
		var plain, htmlText string
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			if strings.HasPrefix(strings.ToLower(p.Header.Get("Content-Disposition")), "attachment") {
				continue
			}
			t := mailText(p.Header.Get("Content-Type"), p.Header.Get("Content-Transfer-Encoding"), p, depth+1)
			ct := strings.ToLower(p.Header.Get("Content-Type"))
			switch {
			case strings.HasPrefix(ct, "text/html") && htmlText == "":
				htmlText = t
			case t != "" && plain == "":
				plain = t
			}
		}
		if plain != "" {
			return plain
		}
		return htmlText
	}
	if !strings.HasPrefix(mediaType, "text/") {
		return ""
	}
	var r io.Reader = body
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "quoted-printable":
		r = quotedprintable.NewReader(body)
	case "base64":
		raw, _ := io.ReadAll(io.LimitReader(body, imapBodyPartial))
		clean := strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, string(raw))
		dec, _ := base64.StdEncoding.DecodeString(strings.TrimRight(clean, "=") + strings.Repeat("=", (4-len(strings.TrimRight(clean, "="))%4)%4))
		r = bytes.NewReader(dec)
	}
	if cs := params["charset"]; cs != "" && !strings.EqualFold(cs, "utf-8") && !strings.EqualFold(cs, "us-ascii") {
		if enc, err := htmlindex.Get(cs); err == nil {
			r = enc.NewDecoder().Reader(r)
		}
	}
	raw, _ := io.ReadAll(io.LimitReader(r, imapBodyPartial))
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if mediaType == "text/html" {
		return htmlToText(text)
	}
	return strings.TrimSpace(text)
}

func formatSummaries(list []mailSummary, total int, what string, loc *time.Location) string {
	if len(list) == 0 {
		return "No messages " + what + "."
	}
	var b strings.Builder
	if total > len(list) {
		fmt.Fprintf(&b, "%d messages %s (newest %d):\n", total, what, len(list))
	} else {
		fmt.Fprintf(&b, "%d messages %s:\n", total, what)
	}
	for _, m := range list {
		when := "?"
		if !m.Date.IsZero() {
			when = m.Date.In(loc).Format("Mon 02 Jan 15:04")
		}
		state := ""
		if !m.Seen {
			state = " (unread)"
		}
		fmt.Fprintf(&b, "- id %d · %s · %s · %s%s\n", m.UID, when, m.From, m.Subject, state)
	}
	return strings.TrimRight(b.String(), "\n")
}
