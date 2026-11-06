package web

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

// Notes: el quadern de l'usuari, compartit amb els vents. Cada nota és un
// fitxer markdown amb capçalera YAML (notes/<id>.md a l'estat de vents del
// compte): text net i portable, com a Quaderna. Les etiquetes poden ser
// explícites o #hashtags del text; [[Títol]] enllaça una altra nota i la
// nota enllaçada en veu la referència inversa.
//
//	GET  /api/notes[?q=&tag=&notebook=] → {notes:[…], tags:[…], notebooks:[…]}
//	GET  /api/notes?id=<id>             → {note:{…}, backlinks:[{id,title}]}
//	POST /api/notes {action:"save"|"pin"|"delete", …}
//
// Els vents hi arriben amb l'eina notes (runNotesTool): llegeixen, cerquen,
// creen i amplien notes. Una nota «private» no la veuen; una de «locked» la
// poden llegir però no tocar.

const (
	notesMaxTitle    = 200
	notesMaxBody     = 200 << 10
	notesMaxNotebook = 80
	notesMaxTags     = 20
	notesMaxTag      = 40
	notesMaxCount    = 5000
)

// Note és una nota sencera (o només el resum, sense Body, a les llistes).
type Note struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Body       string   `json:"body,omitempty"`
	Notebook   string   `json:"notebook,omitempty"`
	Tags       []string `json:"tags"`     // explícites
	Hashtags   []string `json:"hashtags"` // #del text (només lectura)
	Pinned     bool     `json:"pinned"`
	Private    bool     `json:"private"`
	Locked     bool     `json:"locked"`
	NoRemind   bool     `json:"no_remind"` // sense avisos automàtics de les tasques amb data
	Created    string   `json:"created"`
	Updated    string   `json:"updated"`
	By         string   `json:"by,omitempty"`        // vent que la va crear; buit = l'usuari
	EditedBy   string   `json:"edited_by,omitempty"` // últim vent que la va tocar; buit = l'usuari
	Excerpt    string   `json:"excerpt,omitempty"`
	Snippet    string   `json:"snippet,omitempty"` // cerca: el tros on hi ha la coincidència
	TasksOpen  int      `json:"tasks_open"`
	TasksTotal int      `json:"tasks_total"`
}

// noteHead és la capçalera YAML del fitxer.
type noteHead struct {
	Title    string   `yaml:"title"`
	Notebook string   `yaml:"notebook,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
	Pinned   bool     `yaml:"pinned,omitempty"`
	Private  bool     `yaml:"private,omitempty"`
	Locked   bool     `yaml:"locked,omitempty"`
	NoRemind bool     `yaml:"no_remind,omitempty"`
	Created  string   `yaml:"created"`
	Updated  string   `yaml:"updated"`
	By       string   `yaml:"by,omitempty"`
	EditedBy string   `yaml:"edited_by,omitempty"`
}

type noteRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type noteCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

var (
	notesMu     sync.Mutex
	noteIDRe    = regexp.MustCompile(`^[a-f0-9]{16}$`)
	noteFenceRe = regexp.MustCompile("(?s)```.*?```|`[^`\n]*`")
	noteTagRe   = regexp.MustCompile(`(?:^|[\s(,;])#([\p{L}\p{N}][\p{L}\p{N}_/-]{0,39})`)
	noteLinkRe  = regexp.MustCompile(`\[\[([^\[\]\n|]{1,120})(?:\|[^\[\]\n]{0,120})?\]\]`)
	noteTaskRe  = regexp.MustCompile(`(?m)^\s*[-*+] \[([ xX])\]`)
	// noteTaskMark: el «[ ] » que queda d'una tasca després de treure la vinyeta.
	noteTaskMark = regexp.MustCompile(`^\[[ xX]\]\s*`)
	// noteInlineMarks: l'extracte és text pla, sense negretes ni [[ ]].
	noteInlineMarks = strings.NewReplacer("**", "", "__", "", "`", "", "[[", "", "]]", "", "~~", "")
	errNoteGone     = errors.New("note not found")
)

var errNoteConflict = errors.New("note changed since it was opened")

func (s *Server) notesDir() string { return filepath.Join(s.windStateDir(), "notes") }

func (s *Server) notePath(id string) string { return filepath.Join(s.notesDir(), id+".md") }

// parseNote llegeix un fitxer: capçalera YAML entre «---» i el cos.
func parseNote(id string, raw []byte) (Note, error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	var head noteHead
	body := text
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		if i := strings.Index(rest, "\n---\n"); i >= 0 {
			if err := yaml.Unmarshal([]byte(rest[:i]), &head); err != nil {
				return Note{}, err
			}
			body = rest[i+5:]
		} else if strings.HasSuffix(rest, "\n---") {
			if err := yaml.Unmarshal([]byte(strings.TrimSuffix(rest, "\n---")), &head); err != nil {
				return Note{}, err
			}
			body = ""
		}
	}
	n := Note{
		ID: id, Title: head.Title, Body: strings.TrimPrefix(body, "\n"), Notebook: head.Notebook,
		Tags: head.Tags, Pinned: head.Pinned, Private: head.Private, Locked: head.Locked, NoRemind: head.NoRemind,
		Created: head.Created, Updated: head.Updated, By: head.By, EditedBy: head.EditedBy,
	}
	if n.Title == "" {
		n.Title = noteTitleFromBody(n.Body)
	}
	return noteDerive(n), nil
}

// formatNote és el fitxer tal com es desa.
func formatNote(n Note) ([]byte, error) {
	head, err := yaml.Marshal(noteHead{
		Title: n.Title, Notebook: n.Notebook, Tags: n.Tags, Pinned: n.Pinned, Private: n.Private,
		Locked: n.Locked, NoRemind: n.NoRemind, Created: n.Created, Updated: n.Updated, By: n.By, EditedBy: n.EditedBy,
	})
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString("---\n")
	b.Write(head)
	b.WriteString("---\n\n")
	b.WriteString(n.Body)
	if n.Body != "" && !strings.HasSuffix(n.Body, "\n") {
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}

// noteDerive omple el que surt del text: #etiquetes, tasques i l'extracte.
func noteDerive(n Note) Note {
	if n.Tags == nil {
		n.Tags = []string{}
	}
	plain := noteFenceRe.ReplaceAllString(n.Body, " ")
	n.Hashtags = []string{}
	seen := map[string]bool{}
	for _, m := range noteTagRe.FindAllStringSubmatch(plain, -1) {
		t := strings.TrimRight(m[1], "/-_")
		k := foldText(t)
		if t == "" || seen[k] || isAllDigits(t) {
			continue
		}
		seen[k] = true
		n.Hashtags = append(n.Hashtags, t)
	}
	n.TasksOpen, n.TasksTotal = 0, 0
	for _, m := range noteTaskRe.FindAllStringSubmatch(plain, -1) {
		n.TasksTotal++
		if m[1] == " " {
			n.TasksOpen++
		}
	}
	n.Excerpt = noteExcerpt(n.Body, n.Title)
	return n
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// noteTitleFromBody: la primera línia amb text, sense marques.
func noteTitleFromBody(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = noteTaskMark.ReplaceAllString(strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#>-*+ ")), "")
		// «Truca al lampista #casa» → «Truca al lampista»: l'etiqueta ja hi és.
		if bare := strings.Join(strings.Fields(noteTagRe.ReplaceAllString(line, " ")), " "); bare != "" {
			line = bare
		}
		if line != "" {
			return clipRunes(line, 80)
		}
	}
	return ""
}

// noteExcerpt: les primeres línies del cos (sense el títol repetit).
func noteExcerpt(body, title string) string {
	var parts []string
	size := 0
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "```") {
			continue
		}
		clean := strings.TrimSpace(noteTaskMark.ReplaceAllString(strings.TrimSpace(strings.TrimLeft(line, "#>-*+ ")), ""))
		clean = noteInlineMarks.Replace(clean)
		if size == 0 && strings.EqualFold(clean, strings.TrimSpace(title)) {
			continue
		}
		parts = append(parts, clean)
		size += len([]rune(clean))
		if size > 160 {
			break
		}
	}
	return clipRunes(strings.Join(parts, " · "), 180)
}

func clipRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimSpace(string(r[:max-1])) + "…"
}

// foldText: minúscules i sense accents («Àlbum» i «album» casen).
func foldText(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.ToLower(out)
}

// noteLinks: els títols que la nota enllaça amb [[…]].
func noteLinks(body string) []string {
	plain := noteFenceRe.ReplaceAllString(body, " ")
	var out []string
	for _, m := range noteLinkRe.FindAllStringSubmatch(plain, -1) {
		if t := strings.TrimSpace(m[1]); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// cleanTags: sense «#», sense buits ni repetits, amb límits.
func cleanTags(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "#"))
		t = strings.Join(strings.Fields(t), "-")
		t = clipRunes(t, notesMaxTag)
		if t == "" || seen[foldText(t)] {
			continue
		}
		seen[foldText(t)] = true
		out = append(out, t)
		if len(out) == notesMaxTags {
			break
		}
	}
	return out
}

func noteNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func newNoteID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// loadNotesLocked llegeix totes les notes (cal tenir notesMu).
func (s *Server) loadNotesLocked() ([]Note, error) {
	entries, err := os.ReadDir(s.notesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Note, 0, len(entries))
	for _, e := range entries {
		id := strings.TrimSuffix(e.Name(), ".md")
		if e.IsDir() || !noteIDRe.MatchString(id) || e.Name() != id+".md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.notesDir(), e.Name()))
		if err != nil {
			continue
		}
		n, err := parseNote(id, raw)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *Server) readNoteLocked(id string) (Note, error) {
	if !noteIDRe.MatchString(id) {
		return Note{}, errNoteGone
	}
	raw, err := os.ReadFile(s.notePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return Note{}, errNoteGone
		}
		return Note{}, err
	}
	return parseNote(id, raw)
}

func (s *Server) writeNoteLocked(n Note) error {
	b, err := formatNote(n)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.notesDir(), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.notesDir(), ".note-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.notePath(n.ID))
}

// noteInput és el que pot canviar una desada (de l'usuari o d'un vent).
type noteInput struct {
	ID       string    `json:"id"`
	Title    *string   `json:"title"`
	Body     *string   `json:"body"`
	Notebook *string   `json:"notebook"`
	Tags     *[]string `json:"tags"`
	Pinned   *bool     `json:"pinned"`
	Private  *bool     `json:"private"`
	Locked   *bool     `json:"locked"`
	NoRemind *bool     `json:"no_remind"`
	Base     string    `json:"base"` // updated de la versió oberta; si ha canviat → conflicte
}

// saveNote crea (ID buit) o actualitza una nota. by és el vent que la
// toca ("" = l'usuari); un vent no pot canviar private/locked ni el pin.
func (s *Server) saveNote(in noteInput, by string) (Note, error) {
	n, err := s.saveNoteOnly(in, by)
	if err == nil {
		s.syncNoteReminders()
	}
	return n, err
}

func (s *Server) saveNoteOnly(in noteInput, by string) (Note, error) {
	notesMu.Lock()
	defer notesMu.Unlock()
	var n Note
	creating := in.ID == ""
	if creating {
		all, err := s.loadNotesLocked()
		if err != nil {
			return Note{}, err
		}
		if len(all) >= notesMaxCount {
			return Note{}, fmt.Errorf("too many notes (max %d)", notesMaxCount)
		}
		id, err := newNoteID()
		if err != nil {
			return Note{}, err
		}
		n = Note{ID: id, Created: noteNow(), By: by}
	} else {
		cur, err := s.readNoteLocked(in.ID)
		if err != nil {
			return Note{}, err
		}
		if in.Base != "" && in.Base != cur.Updated {
			return cur, errNoteConflict
		}
		n = cur
	}
	if in.Title != nil {
		n.Title = clipRunes(strings.Join(strings.Fields(*in.Title), " "), notesMaxTitle)
	}
	if in.Body != nil {
		if len(*in.Body) > notesMaxBody {
			return Note{}, fmt.Errorf("note too long (max %d KB)", notesMaxBody>>10)
		}
		n.Body = strings.ReplaceAll(*in.Body, "\r\n", "\n")
	}
	if in.Notebook != nil {
		n.Notebook = clipRunes(strings.Join(strings.Fields(*in.Notebook), " "), notesMaxNotebook)
	}
	if in.Tags != nil {
		n.Tags = cleanTags(*in.Tags)
	}
	if by == "" {
		if in.Pinned != nil {
			n.Pinned = *in.Pinned
		}
		if in.Private != nil {
			n.Private = *in.Private
		}
		if in.Locked != nil {
			n.Locked = *in.Locked
		}
		if in.NoRemind != nil {
			n.NoRemind = *in.NoRemind
		}
	}
	if strings.TrimSpace(n.Title) == "" {
		n.Title = noteTitleFromBody(n.Body)
	}
	if strings.TrimSpace(n.Title) == "" && strings.TrimSpace(n.Body) == "" {
		return Note{}, errors.New("empty note")
	}
	// Fixar, amagar o bloquejar no és editar: no canvia l'ordre ni l'autor.
	if creating || in.Title != nil || in.Body != nil || in.Notebook != nil || in.Tags != nil {
		n.EditedBy = by
		n.Updated = noteNow()
	}
	if err := s.writeNoteLocked(n); err != nil {
		return Note{}, err
	}
	return noteDerive(n), nil
}

// deleteNote la porta a notes/.trash: un error no es perd per sempre.
func (s *Server) deleteNote(id string) error {
	err := s.deleteNoteOnly(id)
	if err == nil {
		s.syncNoteReminders()
	}
	return err
}

func (s *Server) deleteNoteOnly(id string) error {
	notesMu.Lock()
	defer notesMu.Unlock()
	if _, err := s.readNoteLocked(id); err != nil {
		return err
	}
	trash := filepath.Join(s.notesDir(), ".trash")
	if err := os.MkdirAll(trash, 0o700); err != nil {
		return err
	}
	return os.Rename(s.notePath(id), filepath.Join(trash, id+"-"+time.Now().UTC().Format("20060102T150405")+".md"))
}

// noteSummary: la nota sense el cos, per a les llistes.
func noteSummary(n Note) Note {
	n.Body = ""
	return n
}

// noteHasTag mira les etiquetes explícites i les del text.
func noteHasTag(n Note, tag string) bool {
	k := foldText(strings.TrimPrefix(tag, "#"))
	for _, t := range append(append([]string{}, n.Tags...), n.Hashtags...) {
		if foldText(t) == k {
			return true
		}
	}
	return false
}

// searchNotes filtra i ordena: totes les paraules hi han de ser; el títol
// pesa més que les etiquetes i aquestes més que el cos. Sense q, les
// fixades primer i després les més noves.
func searchNotes(all []Note, q, tag, notebook string) []Note {
	terms := strings.Fields(foldText(q))
	type scored struct {
		n     Note
		score int
	}
	var hits []scored
	for _, n := range all {
		if tag != "" && !noteHasTag(n, tag) {
			continue
		}
		if notebook != "" && foldText(n.Notebook) != foldText(notebook) {
			continue
		}
		score := 0
		if len(terms) > 0 {
			title, body := foldText(n.Title), foldText(n.Body)
			tags := foldText(strings.Join(append(append([]string{}, n.Tags...), n.Hashtags...), " ") + " " + n.Notebook)
			ok := true
			for _, t := range terms {
				t = strings.TrimPrefix(t, "#")
				in := false
				if strings.Contains(title, t) {
					score += 6
					in = true
				}
				if strings.Contains(tags, t) {
					score += 3
					in = true
				}
				if c := strings.Count(body, t); c > 0 {
					score += min(c, 5)
					in = true
				}
				if !in {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			n.Snippet = noteSnippet(n.Body, terms)
		}
		hits = append(hits, scored{n, score})
	}
	sort.SliceStable(hits, func(a, b int) bool {
		x, y := hits[a], hits[b]
		if x.score != y.score {
			return x.score > y.score
		}
		if x.n.Pinned != y.n.Pinned {
			return x.n.Pinned
		}
		return x.n.Updated > y.n.Updated
	})
	out := make([]Note, len(hits))
	for i, h := range hits {
		out[i] = h.n
	}
	return out
}

// noteSnippet: unes paraules al voltant de la primera coincidència al cos.
func noteSnippet(body string, terms []string) string {
	flat := strings.Join(strings.Fields(body), " ")
	r := []rune(flat)
	folded := []rune(foldText(flat))
	if len(folded) != len(r) { // un caràcter que canvia de mida en plegar-lo
		folded = []rune(strings.ToLower(flat))
		if len(folded) != len(r) {
			return clipRunes(flat, 140)
		}
	}
	at := -1
	for _, t := range terms {
		if i := strings.Index(string(folded), strings.TrimPrefix(t, "#")); i >= 0 {
			at = len([]rune(string(folded)[:i]))
			break
		}
	}
	if at < 0 {
		return ""
	}
	from, to := max(0, at-60), min(len(r), at+100)
	out := strings.TrimSpace(string(r[from:to]))
	if from > 0 {
		out = "…" + out
	}
	if to < len(r) {
		out += "…"
	}
	return out
}

// noteCounts: etiquetes i quaderns amb quantes notes en tenen.
func noteCounts(all []Note) (tags, notebooks []noteCount) {
	tc, tn := map[string]int{}, map[string]string{}
	nc := map[string]int{}
	for _, n := range all {
		seen := map[string]bool{}
		for _, t := range append(append([]string{}, n.Tags...), n.Hashtags...) {
			k := foldText(t)
			if seen[k] {
				continue
			}
			seen[k] = true
			tc[k]++
			if _, ok := tn[k]; !ok {
				tn[k] = t
			}
		}
		if n.Notebook != "" {
			nc[n.Notebook]++
		}
	}
	tags, notebooks = []noteCount{}, []noteCount{}
	for k, c := range tc {
		tags = append(tags, noteCount{tn[k], c})
	}
	for k, c := range nc {
		notebooks = append(notebooks, noteCount{k, c})
	}
	byCount := func(l []noteCount) {
		sort.Slice(l, func(a, b int) bool {
			if l[a].Count != l[b].Count {
				return l[a].Count > l[b].Count
			}
			return foldText(l[a].Name) < foldText(l[b].Name)
		})
	}
	byCount(tags)
	byCount(notebooks)
	return
}

// findNoteByTitle: la nota amb aquest títol (sense majúscules ni accents).
func findNoteByTitle(all []Note, title string) (Note, bool) {
	k := foldText(strings.TrimSpace(title))
	for _, n := range all {
		if foldText(strings.TrimSpace(n.Title)) == k {
			return n, true
		}
	}
	return Note{}, false
}

// noteBacklinks: les notes que enllacen aquesta amb [[títol]].
func noteBacklinks(all []Note, target Note) []noteRef {
	out := []noteRef{}
	k := foldText(strings.TrimSpace(target.Title))
	if k == "" {
		return out
	}
	for _, n := range all {
		if n.ID == target.ID {
			continue
		}
		for _, l := range noteLinks(n.Body) {
			if foldText(l) == k {
				out = append(out, noteRef{n.ID, n.Title})
				break
			}
		}
	}
	return out
}

func (s *Server) handleNotes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		// Una tasca de d'aquí a dos mesos entra a la finestra dels avisos
		// (30 dies) quan arriba: obrir les notes la programa.
		if q.Get("id") == "" {
			s.syncNoteReminders()
		}
		notesMu.Lock()
		all, err := s.loadNotesLocked()
		notesMu.Unlock()
		if err != nil {
			http.Error(w, "cannot read notes", 500)
			return
		}
		if id := strings.TrimSpace(q.Get("id")); id != "" {
			for _, n := range all {
				if n.ID == id {
					writeJSON(w, map[string]any{"note": n, "backlinks": noteBacklinks(all, n)})
					return
				}
			}
			http.Error(w, "note not found", 404)
			return
		}
		found := searchNotes(all, q.Get("q"), q.Get("tag"), q.Get("notebook"))
		list := make([]Note, len(found))
		for i, n := range found {
			list[i] = noteSummary(n)
		}
		tags, notebooks := noteCounts(all)
		writeJSON(w, map[string]any{"notes": list, "tags": tags, "notebooks": notebooks, "total": len(all), "agenda": s.withRemindAt(noteAgenda(all, 60))})
	case http.MethodPost:
		var req struct {
			Action string `json:"action"`
			noteInput
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, notesMaxBody+(16<<10)))
		d.DisallowUnknownFields()
		if err := d.Decode(&req); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		switch req.Action {
		case "save", "":
			n, err := s.saveNote(req.noteInput, "")
			switch {
			case errors.Is(err, errNoteConflict):
				writeJSONStatus(w, http.StatusConflict, map[string]any{"error": err.Error(), "note": n})
			case errors.Is(err, errNoteGone):
				http.Error(w, "note not found", 404)
			case err != nil:
				http.Error(w, err.Error(), 400)
			default:
				writeJSON(w, n)
			}
		case "pin":
			if req.Pinned == nil {
				http.Error(w, "pinned required", 400)
				return
			}
			n, err := s.saveNote(noteInput{ID: req.ID, Pinned: req.Pinned}, "")
			if err != nil {
				http.Error(w, err.Error(), notesStatus(err))
				return
			}
			writeJSON(w, noteSummary(n))
		case "delete":
			if err := s.deleteNote(req.ID); err != nil {
				http.Error(w, err.Error(), notesStatus(err))
				return
			}
			writeJSON(w, map[string]any{"ok": true})
		default:
			http.Error(w, "invalid action", 400)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func notesStatus(err error) int {
	if errors.Is(err, errNoteGone) {
		return 404
	}
	return 400
}

// ---- Agenda: les tasques amb data de totes les notes ----
//
// Una tasca porta data amb «@2026-11-12» o «📅 2026-11-12» (i, si cal,
// l'hora: «@2026-11-12 09:30»). L'agenda les recull de totes les notes
// perquè res no es perdi dins d'una nota: vençudes, avui i les properes.

type agendaItem struct {
	NoteID    string `json:"note_id"`
	NoteTitle string `json:"note_title"`
	Index     int    `json:"index"` // ordre de la tasca dins la nota (per marcar-la)
	Text      string `json:"text"`
	Due       string `json:"due"`            // YYYY-MM-DD
	Time      string `json:"time,omitempty"` // HH:MM
	Done      bool   `json:"done"`
	Private   bool   `json:"private,omitempty"`
	RemindAt  string `json:"remind_at,omitempty"` // avís automàtic programat (RFC3339)
	noRemind  bool
}

var (
	noteTaskLineRe = regexp.MustCompile(`^\s*[-*+] \[([ xX])\]\s*(.*)$`)
	noteDueRe      = regexp.MustCompile(`(?:@|📅\s?)(\d{4}-\d{2}-\d{2})(?:[ T](\d{1,2}:\d{2}))?`)
)

// noteTasks: totes les tasques de la nota, en l'ordre que les compten els
// clients per marcar-les (fora dels blocs de codi).
func noteTasks(n Note) []agendaItem {
	var out []agendaItem
	inFence := false
	idx := 0
	for _, line := range strings.Split(n.Body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		}
		if inFence {
			continue
		}
		m := noteTaskLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		it := agendaItem{NoteID: n.ID, NoteTitle: n.Title, Index: idx, Done: m[1] != " ", Private: n.Private, noRemind: n.NoRemind}
		idx++
		text := m[2]
		if d := noteDueRe.FindStringSubmatch(text); d != nil {
			if _, err := time.Parse("2006-01-02", d[1]); err == nil {
				it.Due, it.Time = d[1], d[2]
				if len(it.Time) == 4 {
					it.Time = "0" + it.Time
				}
				text = strings.TrimSpace(strings.Replace(text, d[0], "", 1))
			}
		}
		it.Text = strings.Join(strings.Fields(text), " ")
		out = append(out, it)
	}
	return out
}

// noteAgenda: les tasques obertes amb data, de la més urgent a la més
// llunyana (les vençudes primer).
func noteAgenda(all []Note, limit int) []agendaItem {
	out := []agendaItem{}
	for _, n := range all {
		for _, t := range noteTasks(n) {
			if !t.Done && t.Due != "" {
				out = append(out, t)
			}
		}
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Due != out[b].Due {
			return out[a].Due < out[b].Due
		}
		return out[a].Time < out[b].Time
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
