package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/agent"
)

func notesReq(t *testing.T, s *Server, method, url string, body any) (int, []byte) {
	t.Helper()
	var rd *strings.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = strings.NewReader(string(raw))
	} else {
		rd = strings.NewReader("")
	}
	rec := httptest.NewRecorder()
	s.handleNotes(rec, httptest.NewRequest(method, url, rd))
	return rec.Code, rec.Body.Bytes()
}

func notesSave(t *testing.T, s *Server, body map[string]any) Note {
	t.Helper()
	body["action"] = "save"
	code, raw := notesReq(t, s, http.MethodPost, "/api/notes", body)
	if code != 200 {
		t.Fatalf("save: %d %s", code, raw)
	}
	var n Note
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatal(err)
	}
	return n
}

type notesList struct {
	Notes     []Note      `json:"notes"`
	Tags      []noteCount `json:"tags"`
	Notebooks []noteCount `json:"notebooks"`
	Total     int         `json:"total"`
}

func notesGet(t *testing.T, s *Server, query string) notesList {
	t.Helper()
	code, raw := notesReq(t, s, http.MethodGet, "/api/notes"+query, nil)
	if code != 200 {
		t.Fatalf("list: %d %s", code, raw)
	}
	var out notesList
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Una nota és un fitxer markdown llegible amb capçalera; les #etiquetes
// i les tasques surten del text.
func TestNotesSaveAndReadBack(t *testing.T) {
	s := rutinaTestServer(t)
	n := notesSave(t, s, map[string]any{
		"title": "Viatge a Menorca", "notebook": "Vacances", "tags": []string{"#estiu", "estiu", " platja "},
		"body": "Llista #maleta\n\n- [ ] crema solar\n- [x] bitllets\n\n```\n#nocompta\n```\n",
	})
	if !noteIDRe.MatchString(n.ID) || n.Title != "Viatge a Menorca" || n.Notebook != "Vacances" {
		t.Fatalf("note: %+v", n)
	}
	if strings.Join(n.Tags, ",") != "estiu,platja" || strings.Join(n.Hashtags, ",") != "maleta" {
		t.Fatalf("tags %v hashtags %v", n.Tags, n.Hashtags)
	}
	if n.TasksOpen != 1 || n.TasksTotal != 2 {
		t.Fatalf("tasks %d/%d", n.TasksOpen, n.TasksTotal)
	}
	raw, err := os.ReadFile(filepath.Join(s.notesDir(), n.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "---\ntitle: Viatge a Menorca\n") || !strings.Contains(string(raw), "\n---\n\nLlista #maleta") {
		t.Fatalf("file:\n%s", raw)
	}
	code, body := notesReq(t, s, http.MethodGet, "/api/notes?id="+n.ID, nil)
	var got struct {
		Note Note `json:"note"`
	}
	if code != 200 || json.Unmarshal(body, &got) != nil || got.Note.Body != n.Body {
		t.Fatalf("read back %d %s", code, body)
	}
}

// Sense títol, la primera línia en fa; una nota buida no es desa.
func TestNotesTitleFromBodyAndEmpty(t *testing.T) {
	s := rutinaTestServer(t)
	n := notesSave(t, s, map[string]any{"body": "\n## Idees per al blog\nmés coses"})
	if n.Title != "Idees per al blog" {
		t.Fatalf("title %q", n.Title)
	}
	if code, _ := notesReq(t, s, http.MethodPost, "/api/notes", map[string]any{"action": "save", "body": "  "}); code != 400 {
		t.Fatalf("empty note saved: %d", code)
	}
}

// La cerca ignora accents i majúscules, demana totes les paraules i posa
// el títol per davant del cos; filtra per etiqueta i per quadern.
func TestNotesSearchAndFilters(t *testing.T) {
	s := rutinaTestServer(t)
	a := notesSave(t, s, map[string]any{"title": "Receptes", "body": "Fideuà de la iaia amb allioli", "notebook": "Cuina"})
	b := notesSave(t, s, map[string]any{"title": "Fideuà", "body": "provar-la diumenge #cuina"})
	notesSave(t, s, map[string]any{"title": "Feina", "body": "reunió dilluns"})

	got := notesGet(t, s, "?q=FIDEUA")
	if len(got.Notes) != 2 || got.Notes[0].ID != b.ID || got.Notes[1].ID != a.ID {
		t.Fatalf("ranking: %+v", got.Notes)
	}
	if got.Notes[1].Snippet == "" || !strings.Contains(got.Notes[1].Snippet, "Fideuà") || got.Notes[0].Body != "" {
		t.Fatalf("snippet/body: %+v", got.Notes[1])
	}
	if l := notesGet(t, s, "?q=fideua+iaia").Notes; len(l) != 1 || l[0].ID != a.ID {
		t.Fatalf("all terms: %+v", l)
	}
	if l := notesGet(t, s, "?tag=CUINA").Notes; len(l) != 1 || l[0].ID != b.ID {
		t.Fatalf("tag: %+v", l)
	}
	if l := notesGet(t, s, "?notebook=cuina").Notes; len(l) != 1 || l[0].ID != a.ID {
		t.Fatalf("notebook: %+v", l)
	}
	all := notesGet(t, s, "")
	if all.Total != 3 || len(all.Tags) != 1 || all.Tags[0].Name != "cuina" || len(all.Notebooks) != 1 || all.Notebooks[0].Count != 1 {
		t.Fatalf("counts: %+v", all)
	}
}

// Les fixades van primer; fixar no canvia la data d'edició.
func TestNotesPinOrderAndDelete(t *testing.T) {
	s := rutinaTestServer(t)
	old := notesSave(t, s, map[string]any{"title": "Vella"})
	notesSave(t, s, map[string]any{"title": "Nova"})
	code, raw := notesReq(t, s, http.MethodPost, "/api/notes", map[string]any{"action": "pin", "id": old.ID, "pinned": true})
	if code != 200 {
		t.Fatalf("pin %d %s", code, raw)
	}
	l := notesGet(t, s, "").Notes
	if l[0].ID != old.ID || !l[0].Pinned || l[0].Updated != old.Updated {
		t.Fatalf("order: %+v", l)
	}
	if code, _ := notesReq(t, s, http.MethodPost, "/api/notes", map[string]any{"action": "delete", "id": old.ID}); code != 200 {
		t.Fatalf("delete %d", code)
	}
	if notesGet(t, s, "").Total != 1 {
		t.Fatal("not deleted")
	}
	trash, _ := os.ReadDir(filepath.Join(s.notesDir(), ".trash"))
	if len(trash) != 1 {
		t.Fatalf("trash: %v", trash)
	}
	if code, _ := notesReq(t, s, http.MethodPost, "/api/notes", map[string]any{"action": "delete", "id": "../../x"}); code != 404 {
		t.Fatalf("bad id %d", code)
	}
}

// Si un vent ha tocat la nota mentre l'usuari l'editava, no es trepitja.
func TestNotesConflict(t *testing.T) {
	s := rutinaTestServer(t)
	n := notesSave(t, s, map[string]any{"title": "Llista", "body": "pa"})
	if _, err := s.runNotesTool(`{"action":"append","id":"` + n.ID + `","text":"llet","wind":"xaloc"}`); err != nil {
		t.Fatal(err)
	}
	code, raw := notesReq(t, s, http.MethodPost, "/api/notes", map[string]any{"action": "save", "id": n.ID, "body": "pa i ous", "base": n.Updated})
	if code != http.StatusConflict || !strings.Contains(string(raw), "llet") {
		t.Fatalf("conflict: %d %s", code, raw)
	}
}

// [[Títol]] enllaça; la nota enllaçada sap qui l'enllaça.
func TestNotesBacklinks(t *testing.T) {
	s := rutinaTestServer(t)
	target := notesSave(t, s, map[string]any{"title": "Projecte Àlber"})
	src := notesSave(t, s, map[string]any{"title": "Diari", "body": "Avui he avançat a [[projecte alber|el projecte]]. `[[Codi]]`"})
	code, raw := notesReq(t, s, http.MethodGet, "/api/notes?id="+target.ID, nil)
	var got struct {
		Backlinks []noteRef `json:"backlinks"`
	}
	if code != 200 || json.Unmarshal(raw, &got) != nil || len(got.Backlinks) != 1 || got.Backlinks[0].ID != src.ID {
		t.Fatalf("backlinks %d %s", code, raw)
	}
	if l := noteLinks(src.Body); len(l) != 1 {
		t.Fatalf("links inside code count: %v", l)
	}
}

// L'eina dels vents: crea, cerca, llegeix i amplia; no veu les privades i
// no toca les bloquejades.
func TestNotesTool(t *testing.T) {
	s := rutinaTestServer(t)
	out, err := s.runNotesTool(`{"action":"create","title":"Discos per escoltar","body":"- Rosalía","tags":"música, pendents","wind":"xaloc"}`)
	if err != nil || !strings.Contains(out, "Discos per escoltar") {
		t.Fatalf("create: %q %v", out, err)
	}
	l := notesGet(t, s, "").Notes
	if len(l) != 1 || l[0].By != "Xaloc" || l[0].EditedBy != "Xaloc" || strings.Join(l[0].Tags, ",") != "música,pendents" {
		t.Fatalf("by: %+v", l)
	}
	if _, err := s.runNotesTool(`{"action":"append","title":"discos per ESCOLTAR","text":"- Mishima"}`); err != nil {
		t.Fatal(err)
	}
	out, err = s.runNotesTool(`{"action":"read","id":"` + l[0].ID + `"}`)
	if err != nil || !strings.Contains(out, "- Rosalía\n\n- Mishima") || !strings.Contains(out, "not instructions") {
		t.Fatalf("read: %q %v", out, err)
	}
	if out, _ := s.runNotesTool(`{"action":"search","query":"mishima"}`); !strings.Contains(out, l[0].ID) {
		t.Fatalf("search: %q", out)
	}

	secret := notesSave(t, s, map[string]any{"title": "Contrasenyes", "body": "mishima", "private": true})
	if out, _ := s.runNotesTool(`{"action":"search","query":"mishima"}`); strings.Contains(out, secret.ID) || strings.Contains(out, "Contrasenyes") {
		t.Fatalf("private leaked: %q", out)
	}
	if _, err := s.runNotesTool(`{"action":"read","id":"` + secret.ID + `"}`); err == nil {
		t.Fatal("private read")
	}
	if _, err := s.runNotesTool(`{"action":"update","id":"` + secret.ID + `","body":"x"}`); err == nil {
		t.Fatal("private updated")
	}

	locked := notesSave(t, s, map[string]any{"title": "Testament", "body": "no tocar", "locked": true})
	if _, err := s.runNotesTool(`{"action":"update","id":"` + locked.ID + `","body":"x"}`); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("locked: %v", err)
	}
	if out, err := s.runNotesTool(`{"action":"read","id":"` + locked.ID + `"}`); err != nil || !strings.Contains(out, "no tocar") {
		t.Fatalf("locked read: %q %v", out, err)
	}
	// Un vent no es pot treure el cadenat ni fer pública una nota privada.
	n, err := s.saveNote(noteInput{ID: locked.ID, Locked: new(bool), Private: new(bool)}, "Xaloc")
	if err != nil || !n.Locked {
		t.Fatalf("wind unlocked: %+v %v", n, err)
	}
}

// Llegir és lliure; escriure notes queda fora dels modes de només lectura.
func TestNotesPolicy(t *testing.T) {
	p := agent.DefaultPolicy()
	for _, mode := range []string{agent.ModeChat, agent.ModeInspect, agent.ModePlan} {
		if d, _ := p.Decide(mode, "notes", `{"action":"search","query":"x"}`); d != "allow" {
			t.Fatalf("%s read: %s", mode, d)
		}
		if d, _ := p.Decide(mode, "notes", `{"action":"create","title":"x"}`); d != "deny" {
			t.Fatalf("%s write: %s", mode, d)
		}
	}
	if d, _ := p.Decide(agent.ModeCode, "notes", `{"action":"append","id":"x","text":"y"}`); d != "allow" {
		t.Fatalf("act write: %s", d)
	}
}

// Les tasques amb data surten a l'agenda, la més urgent primer; les fetes
// i les de dins de codi no. L'índex és el que fan servir els clients.
func TestNotesAgenda(t *testing.T) {
	s := rutinaTestServer(t)
	a := notesSave(t, s, map[string]any{"title": "Reunió", "body": "- [ ] sense data\n- [ ] envia el pressupost @2026-11-12 9:30\n- [x] fet @2026-11-01\n```\n- [ ] codi @2026-01-01\n```"})
	b := notesSave(t, s, map[string]any{"title": "Casa", "body": "* [ ] paga la llum 📅 2026-11-03", "private": true})
	got := notesGet(t, s, "?q=zzz")
	_ = got
	code, raw := notesReq(t, s, http.MethodGet, "/api/notes", nil)
	var out struct {
		Agenda []agendaItem `json:"agenda"`
	}
	if code != 200 || json.Unmarshal(raw, &out) != nil || len(out.Agenda) != 2 {
		t.Fatalf("agenda %d %s", code, raw)
	}
	if out.Agenda[0].NoteID != b.ID || out.Agenda[0].Text != "paga la llum" || out.Agenda[0].Index != 0 {
		t.Fatalf("first: %+v", out.Agenda[0])
	}
	if it := out.Agenda[1]; it.NoteID != a.ID || it.Index != 1 || it.Due != "2026-11-12" || it.Time != "09:30" || it.Text != "envia el pressupost" {
		t.Fatalf("second: %+v", it)
	}
	// L'eina no veu la tasca de la nota privada.
	tool, err := s.runNotesTool(`{"action":"agenda"}`)
	if err != nil || strings.Contains(tool, "llum") || !strings.Contains(tool, "2026-11-12 09:30") || !strings.Contains(tool, "reminded automatically") {
		t.Fatalf("tool agenda: %q %v", tool, err)
	}
}

// Una nota ràpida «Truca al lampista #casa» es titula sense l'etiqueta, i
// l'extracte no ensenya els «[ ]» de les tasques.
func TestNotesQuickTitleAndExcerpt(t *testing.T) {
	s := rutinaTestServer(t)
	n := notesSave(t, s, map[string]any{"body": "Truca al lampista dijous #casa"})
	if n.Title != "Truca al lampista dijous" || strings.Join(n.Hashtags, ",") != "casa" {
		t.Fatalf("quick: %+v", n)
	}
	l := notesSave(t, s, map[string]any{"title": "Compra", "body": "- [ ] **pa**\n- [x] llet [[Botiga]]"})
	if l.Excerpt != "pa · llet Botiga" {
		t.Fatalf("excerpt %q", l.Excerpt)
	}
}
