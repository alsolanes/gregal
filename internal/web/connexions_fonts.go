package web

// API interna per a B6 (proactivitat): quines fonts té connectades una
// persona i què hi ha canviat des de l'última mirada. Determinista, sense
// model ni xarxa més enllà de llegir la font; cap secret surt d'aquí.
//
//	srcs := s.connexioSources()                       // fonts llegibles de l'owner de s
//	items, next, err := s.connexioChanges(ctx, id, cur) // canvis des de cur ("" = primera mirada)
//
// El cursor és opac i l'ha de desar qui crida (amb la responsabilitat), tant
// si notifica com si no. Amb cur == "" torna l'estat actual com a punt de
// partida (B6 decideix si el fa servir o només en guarda el cursor).
// errConnexioNoFeed: la font existeix però no té un feed de canvis (connector
// estructurat sense eines de lectura conegudes, sessió de navegador).

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"gregal/internal/connexions"
)

// connexioSource és una font connectada que B6 pot vigilar.
type connexioSource struct {
	ID            string   `json:"id"`
	Service       string   `json:"service"` // calendar-ics | mail-imap | mcp | browser
	Provider      string   `json:"provider,omitempty"`
	AccountLabel  string   `json:"accountLabel"`
	Status        string   `json:"status"`    // el darrer comprovat
	Effective     []string `json:"effective"` // read, prepare, send
	LastCheckedAt string   `json:"lastCheckedAt,omitempty"`
	// Changes: connexioChanges en sap llegir els canvis.
	Changes bool `json:"changes"`
}

// connexioItem és un element nou o canviat d'una font.
type connexioItem struct {
	Key    string    `json:"key"`    // estable dins la font (UID d'esdeveniment, UID de correu)
	Kind   string    `json:"kind"`   // event | mail
	Change string    `json:"change"` // new | changed
	Title  string    `json:"title"`
	Detail string    `json:"detail,omitempty"`
	At     time.Time `json:"at"`
}

var errConnexioNoFeed = errors.New("this source has no change feed")

// connexioSources són les fonts que la persona deixa llegir i que no ha
// desconnectat (també les que necessiten sessió: B6 ha de poder dir-ho).
func (s *Server) connexioSources() []connexioSource {
	s.loadConnections()
	out := []connexioSource{}
	for _, c := range s.connexioStore().List() {
		if c.Status == connexions.Disconnected || !c.Can(connexions.CapRead) {
			continue
		}
		out = append(out, connexioSource{
			ID: c.ID, Service: c.Service, Provider: c.Provider, AccountLabel: c.AccountLabel, Status: c.Status,
			Effective: c.Effective(), LastCheckedAt: c.LastCheckedAt,
			Changes: c.Service == connexions.ServiceICS || c.Service == connexions.ServiceIMAP,
		})
	}
	return out
}

// Finestres i límits dels canvis.
const (
	feedCalendarAhead = 30 * 24 * time.Hour
	feedCalendarBack  = 24 * time.Hour
	feedMaxSeen       = 600
	feedMaxItems      = 50
	feedMailBaseline  = 2 // dies de la primera mirada
)

type feedCursor struct {
	V    int               `json:"v"`
	Seen map[string]string `json:"s,omitempty"` // calendari: clau → empremta
	Max  int               `json:"m,omitempty"` // correu: UID més alt vist
}

func decodeFeedCursor(cur string) feedCursor {
	var c feedCursor
	if raw, err := base64.RawURLEncoding.DecodeString(cur); err == nil {
		_ = json.Unmarshal(raw, &c)
	}
	return c
}

func (c feedCursor) encode() string {
	c.V = 1
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func shortHash(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

// connexioChanges llegeix els canvis d'una font des del cursor. L'estat de la
// connexió queda apuntat amb el resultat (needs_session si la clau ja no val).
func (s *Server) connexioChanges(ctx context.Context, id, cursor string) ([]connexioItem, string, error) {
	conn, ok := s.connexioStore().Get(id)
	if !ok {
		return nil, cursor, os.ErrNotExist
	}
	if conn.Status == connexions.Disconnected || !conn.Can(connexions.CapRead) {
		return nil, cursor, fmt.Errorf("%s is not readable (disconnected or paused)", id)
	}
	switch conn.Service {
	case connexions.ServiceICS:
		return s.calendarChanges(ctx, conn, cursor)
	case connexions.ServiceIMAP:
		return s.mailChanges(conn, cursor)
	}
	return nil, cursor, errConnexioNoFeed
}

func (s *Server) calendarChanges(ctx context.Context, conn connexions.Connexio, cursor string) ([]connexioItem, string, error) {
	sec, err := s.vault().Get(s.remindOwner(), conn.AuthRef)
	if err != nil {
		s.noteConnexioUse(conn, errNeedsSession)
		return nil, cursor, errNeedsSession
	}
	body, err := icsFetch(ctx, string(sec))
	s.noteConnexioUse(conn, err)
	if err != nil {
		return nil, cursor, err
	}
	loc := s.loadConnections().zone()
	now := time.Now()
	evs := eventsBetween(parseICS(body, conn.AccountLabel, loc), now.Add(-feedCalendarBack), now.Add(feedCalendarAhead))
	sort.Slice(evs, func(i, j int) bool { return evs[i].Start.Before(evs[j].Start) })
	prev := decodeFeedCursor(cursor)
	next := feedCursor{Seen: map[string]string{}}
	var items []connexioItem
	for _, e := range evs {
		if len(next.Seen) >= feedMaxSeen {
			break
		}
		key := shortHash(e.UID, e.Start.UTC().Format(time.RFC3339))
		fp := shortHash(e.Summary, e.Location, e.Start.UTC().Format(time.RFC3339), e.End.UTC().Format(time.RFC3339))
		next.Seen[key] = fp
		old, seen := prev.Seen[key]
		if seen && old == fp {
			continue
		}
		change := "new"
		if seen {
			change = "changed"
		}
		if len(items) < feedMaxItems {
			detail := e.Start.In(loc).Format("Mon 02 Jan 15:04")
			if e.Location != "" {
				detail += " · " + e.Location
			}
			items = append(items, connexioItem{Key: key, Kind: "event", Change: change, Title: e.Summary, Detail: detail, At: e.Start})
		}
	}
	return items, next.encode(), nil
}

func (s *Server) mailChanges(conn connexions.Connexio, cursor string) ([]connexioItem, string, error) {
	cl, err := s.openMail(conn.ID)
	if err != nil {
		return nil, cursor, err
	}
	defer cl.close()
	prev := decodeFeedCursor(cursor)
	var uids []int
	if prev.Max > 0 {
		uids, err = cl.search("UID " + strconv.Itoa(prev.Max+1) + ":*")
	} else {
		uids, err = cl.search("SINCE " + time.Now().AddDate(0, 0, -feedMailBaseline).Format("02-Jan-2006"))
	}
	if err != nil {
		return nil, cursor, err
	}
	next := feedCursor{Max: prev.Max}
	var fresh []int
	for _, u := range uids {
		// «UID n:*» torna sempre el darrer encara que sigui antic.
		if u > prev.Max {
			fresh = append(fresh, u)
		}
		if u > next.Max {
			next.Max = u
		}
	}
	sort.Ints(fresh)
	if len(fresh) > feedMaxItems {
		fresh = fresh[len(fresh)-feedMaxItems:]
	}
	sums, err := cl.summaries(fresh)
	if err != nil {
		return nil, cursor, err
	}
	items := make([]connexioItem, 0, len(sums))
	for _, m := range sums {
		items = append(items, connexioItem{Key: strconv.Itoa(m.UID), Kind: "mail", Change: "new", Title: m.Subject, Detail: m.From, At: m.Date})
	}
	return items, next.encode(), nil
}
