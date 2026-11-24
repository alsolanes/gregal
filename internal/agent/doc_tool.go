package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"

	"gregal/internal/gdocs"
	"gregal/internal/llm"
	"gregal/internal/ooxml"
)

// L'eina doc: els documents vius del projecte (internal/gdocs). Els canvis
// passen sense confirmació en els modes que escriuen perquè la persona els
// veu en directe i queden versionats per bloc; en els de només lectura es
// deneguen, com office_edit.

func docToolSpec() llm.ToolSpec {
	str := map[string]any{"type": "string"}
	return llm.ToolSpec{Name: "doc", Description: "Live project documents the person can see and download: a text document made of markdown blocks (Word), " +
		"or a presentation with kind \"deck\" whose blocks are slides (PowerPoint), " +
		"or a spreadsheet with kind \"sheet\" whose blocks are named sheets of cells, with Excel formulas the server calculates (Excel). " +
		"A new text document (a report, a letter, notes, «a Word») is created here, not with office_create, and your reply gives the Word link create returns. " +
		"Prefer it over office_create for a new presentation or a new spreadsheet too. Actions: " +
		"create (title, kind doc, deck or sheet, optional md for a doc: split into blocks at headings and blank lines; a deck starts empty; a sheet starts with one empty sheet named sheet; returns the download link to give the person); list; " +
		"read (id, optional range like A1:D20 for a sheet; every block with its id and version, a sheet's cells with their input and computed value); insert (id, md or for a deck slide or for a sheet a new sheet name in sheet, after = block id, or omitted for the start); " +
		"replace (id, block, md or slide); image (id, image = path of a PNG, JPEG or GIF file inside the project: copies it into the document and returns the images/<name> path to use as ![caption](images/<name>) in a block or as a slide's image); set_cells (id, block = the sheet, cells: {\"A1\": \"Sales\", \"B1\": \"1200\", \"B3\": \"=SUM(B1:B2)\"}, an empty value clears the cell); delete (id, block). A slide is {layout: title|section|content|two_columns|comparison|big_number|image|title_only|blank|closing, " +
		"title, subtitle, bullets, body, left, right, left_title, right_title, number, caption, image, table, chart: {type, categories, series: [{name, values}]}, notes}. " +
		"Edit one block or slide at a time instead of rewriting the document. " +
		"Text blocks also take ++underline++, [text]{color=#rrggbb bg=#rrggbb font=\"Name\" size=N} (any of the keys, size in points) and a line {align=center} (left, center, right or justify) right after a paragraph or heading.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"action": map[string]any{"type": "string", "enum": []string{"create", "list", "read", "insert", "replace", "image", "set_cells", "delete"}},
			"kind":   map[string]any{"type": "string", "enum": []string{gdocs.KindDoc, gdocs.KindDeck, gdocs.KindSheet}},
			"slide":  map[string]any{"type": "object", "description": "One slide of a deck"},
			"cells":  map[string]any{"type": "object", "description": "Cell name → input (a number, text or a formula starting with =)"},
			"id":     str, "title": str, "md": str, "after": str, "block": str, "sheet": str, "range": str, "image": str,
		}, "required": []string{"action"}}}
}

// docExec fa l'acció i apunta a la sessió la versió que l'agent ha vist,
// perquè el torn següent li digui només el que la persona ha canviat després.
func docExec(dir, session, argsJSON string) (string, error) {
	s, err := gdocs.Open(procDirOr(dir))
	if err != nil {
		return "", err
	}
	return DocExecIn(s, session, argsJSON)
}

// docArgs són els arguments de l'eina doc.
type docArgs struct {
	Action, ID, Title, Kind, MD, After, Block, Sheet, Range, Image string
	Slide                                                          *ooxml.SlideSpec
	Cells                                                          map[string]string
}

// DocExecIn fa l'acció de doc sobre un magatzem ja obert (els vents hi
// passen el de la carpeta personal).
func DocExecIn(s *gdocs.Store, session, argsJSON string) (string, error) {
	var a docArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "create":
		return docCreate(s, session, a)
	case "list":
		return docList(s)
	case "image":
		return docImage(s, a)
	}
	d, err := docChange(s, session, a)
	if err != nil {
		return "", err
	}
	marcaVist(s, session, d)
	return docRender(d, a.Range)
}

// docCreate crea un document del tipus demanat i torna l'enllaç de baixada.
func docCreate(s *gdocs.Store, session string, a docArgs) (string, error) {
	switch a.Kind {
	case gdocs.KindDeck:
		if strings.TrimSpace(a.MD) != "" {
			return "", fmt.Errorf("a deck starts empty: create it without md, then insert its slides one by one")
		}
		d, err := s.CreateDeck(a.Title)
		if err != nil {
			return "", err
		}
		marcaVist(s, session, d)
		return fmt.Sprintf("Created document %s (an empty deck): insert its slides one by one. Give the person this link to download it as PowerPoint: [%s](/api/docs/%s/export?format=pptx)",
			d.ID, d.Title, d.ID), nil
	case gdocs.KindSheet:
		d, err := s.CreateSheet(a.Title, a.Sheet, gdocs.AuthorAgent)
		if err != nil {
			return "", err
		}
		marcaVist(s, session, d)
		return fmt.Sprintf("Created document %s (a spreadsheet: sheet %s is block %s): fill it with set_cells. Give the person this link to download it as Excel: [%s](/api/docs/%s/export?format=xlsx)",
			d.ID, d.Blocks[0].Name, d.Blocks[0].ID, d.Title, d.ID), nil
	case "", gdocs.KindDoc:
		d, err := s.Create(a.Title, a.MD, gdocs.AuthorAgent)
		if err != nil {
			return "", err
		}
		marcaVist(s, session, d)
		return fmt.Sprintf("Created document %s with %d blocks. Give the person this link to download it as Word: [%s](/api/docs/%s/export?format=docx)",
			d.ID, len(d.Blocks), d.Title, d.ID), nil
	}
	return "", fmt.Errorf("unknown kind %q: doc, deck or sheet", a.Kind)
}

// maxDocImage és la mida més gran d'una imatge que va a un document, la
// mateixa que la ruta POST /api/docs/{id}/images.
const maxDocImage = 10 << 20

// docImage copia una imatge de l'arrel del projecte (una ruta relativa o
// absoluta, mai fora, tampoc per un enllaç) a la carpeta del document.
func docImage(s *gdocs.Store, a docArgs) (string, error) {
	p := strings.TrimSpace(a.Image)
	if p == "" {
		return "", errors.New("image: the path of a PNG, JPEG or GIF file in the project")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(s.Root, p)
	}
	root, err1 := filepath.EvalSymlinks(s.Root)
	real, err2 := filepath.EvalSymlinks(p)
	if err := errors.Join(err1, err2); err != nil {
		return "", fmt.Errorf("image %q: %w", a.Image, err)
	}
	if rel, err := filepath.Rel(root, real); err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("image %q: only files inside the project", a.Image)
	}
	if fi, err := os.Stat(real); err != nil || fi.Size() > maxDocImage {
		return "", fmt.Errorf("image %q: a file of 10 MB at most", a.Image)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		return "", err
	}
	src, err := s.AddImage(a.ID, data)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Image copied into document %s as %s: insert it as ![caption](%s) in a block, or as a slide's image.", a.ID, src, src), nil
}

func docList(s *gdocs.Store) (string, error) {
	docs, err := s.List()
	if err != nil || len(docs) == 0 {
		return "No documents in this project.", err
	}
	var b strings.Builder
	for _, d := range docs {
		fmt.Fprintf(&b, "%s · %s · %d blocks · v%d\n", d.ID, d.Title, len(d.Blocks), d.Version)
	}
	return b.String(), nil
}

// docChange fa read o una acció que canvia blocs, segons el tipus de bloc
// que porten els arguments.
func docChange(s *gdocs.Store, session string, a docArgs) (gdocs.Doc, error) {
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "read":
		return s.Get(a.ID)
	case "insert":
		if a.Slide != nil {
			return s.InsertSlide(a.ID, a.After, *a.Slide, gdocs.AuthorAgent)
		}
		if a.Sheet != "" {
			return s.InsertSheet(a.ID, a.After, a.Sheet, gdocs.AuthorAgent)
		}
		return s.Insert(a.ID, a.After, a.MD, gdocs.AuthorAgent)
	case "replace":
		if a.Slide != nil {
			return s.ReplaceSlide(a.ID, a.Block, *a.Slide, gdocs.AuthorAgent, vist(s, session, a.ID))
		}
		return s.Replace(a.ID, a.Block, a.MD, gdocs.AuthorAgent, vist(s, session, a.ID))
	case "set_cells":
		return s.SetCells(a.ID, a.Block, a.Cells, gdocs.AuthorAgent, vist(s, session, a.ID))
	case "delete":
		return s.Delete(a.ID, a.Block, gdocs.AuthorAgent, vist(s, session, a.ID))
	}
	return gdocs.Doc{}, fmt.Errorf("unknown action %q: create, list, read, insert, replace, image, set_cells or delete", a.Action)
}

// docRender és el document tal com el llegeix l'agent: cada bloc amb el seu
// id, versió i autor; d'un full, les cel·les de rng.
func docRender(d gdocs.Doc, rng string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Document %s · %s · v%d\n", d.ID, d.Title, d.Version)
	for _, blk := range d.Blocks {
		text := blk.MD
		if d.Kind == gdocs.KindSheet {
			cells, err := cellesFull(blk, rng)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "\n[%s v%d] (%s) sheet %s\n%s", blk.ID, blk.Version, blk.Author, blk.Name, cells)
			continue
		}
		if blk.Slide != nil {
			raw, err := json.Marshal(blk.Slide)
			if err != nil {
				return "", err
			}
			text = string(raw)
		}
		fmt.Fprintf(&b, "\n[%s v%d] (%s)\n%s\n", blk.ID, blk.Version, blk.Author, text)
	}
	return b.String(), nil
}

// cellesFull són les cel·les d'un full dins de rng (p. ex. A1:D20, "" =
// totes), fila per fila: l'entrada i, d'una fórmula, el valor calculat.
func cellesFull(blk gdocs.Block, rng string) (string, error) {
	c0, r0, c1, r1 := 1, 1, math.MaxInt, math.MaxInt
	if rng != "" {
		from, to, _ := strings.Cut(rng, ":")
		if to == "" {
			to = from
		}
		var err1, err2 error
		c0, r0, err1 = excelize.CellNameToCoordinates(strings.TrimSpace(from))
		c1, r1, err2 = excelize.CellNameToCoordinates(strings.TrimSpace(to))
		if err := errors.Join(err1, err2); err != nil {
			return "", fmt.Errorf("range %q: use cells like A1:D20", rng)
		}
	}
	type cella struct {
		nom  string
		c, r int
	}
	var cs []cella
	for nom := range blk.Cells {
		c, r, err := excelize.CellNameToCoordinates(nom)
		if err == nil && c >= c0 && c <= c1 && r >= r0 && r <= r1 {
			cs = append(cs, cella{nom, c, r})
		}
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].r < cs[j].r || cs[i].r == cs[j].r && cs[i].c < cs[j].c })
	var b strings.Builder
	for _, c := range cs {
		in := blk.Cells[c.nom]
		if strings.HasPrefix(strings.TrimSpace(in), "=") {
			in += " → " + blk.Values[c.nom]
		}
		fmt.Fprintf(&b, "%s = %s\n", c.nom, in)
	}
	return b.String(), nil
}

// vist és l'última versió del document id que l'agent de la sessió ha vist:
// un bloc que la persona ha canviat després no es pot trepitjar sense
// tornar-lo a llegir. Si el registre no es pot llegir, 0 vol dir que
// qualsevol canvi de la persona xoca, que és el costat segur.
func vist(s *gdocs.Store, session, id string) int {
	vistos, err := s.Seen(session)
	if err != nil {
		log.Printf("doc: versions vistes de la sessió: %v", err)
	}
	return vistos[id]
}

// marcaVist apunta la versió que l'agent acaba de veure. Si no es pot
// desar, el pitjor és que el torn següent li repeteixi canvis: no val la
// pena fer fallar l'acció, que ja s'ha fet.
func marcaVist(s *gdocs.Store, session string, d gdocs.Doc) {
	if err := s.MarkSeen(session, d.ID, d.Version); err != nil {
		log.Printf("doc: no s'ha pogut apuntar la versió vista de %s: %v", d.ID, err)
	}
}

// MarcaDocs encapçala la nota del torn amb els blocs que la persona ha
// canviat als documents que l'agent de la sessió ha llegit o escrit.
const MarcaDocs = "[documents the person changed since you last read or wrote them"

// maxNotaBloc és el que la nota ensenya del text nou de cada bloc.
const maxNotaBloc = 300

// canvisPersona són les línies de la nota per als blocs que la persona ha
// canviat a d després de la versió v.
func canvisPersona(d gdocs.Doc, v int) []string {
	var linies []string
	for _, blk := range d.Blocks {
		if blk.Version <= v || blk.Author != gdocs.AuthorUser {
			continue
		}
		que := "edited"
		if blk.Since > v {
			que = "inserted"
		}
		linies = append(linies, fmt.Sprintf("- %s %s: %s", que, blk.ID, retallaText(textNota(blk), maxNotaBloc)))
	}
	for _, blk := range d.Deleted {
		// Un bloc que l'agent no havia vist mai no cal dir-li que ja no hi és.
		if blk.Version > v && blk.Author == gdocs.AuthorUser && blk.Since <= v {
			linies = append(linies, "- deleted "+blk.ID)
		}
	}
	return linies
}

// textNota és el que la nota ensenya d'un bloc: el markdown, el títol
// d'una diapositiva o les cel·les d'un full (no se sap quines han canviat).
func textNota(blk gdocs.Block) string {
	switch {
	case blk.Slide != nil:
		return "slide «" + blk.Slide.Title + "»"
	case blk.Name != "":
		// Sense rang, cellesFull no falla.
		cells, _ := cellesFull(blk, "")
		return "sheet " + blk.Name + ": " + strings.ReplaceAll(strings.TrimSpace(cells), "\n", "; ")
	}
	return blk.MD
}

// notaDocs fa la nota del torn dels documents del projecte dir.
func notaDocs(dir, session string) (string, func()) {
	s, err := gdocs.Open(procDirOr(dir))
	if err != nil {
		return "", nil
	}
	return NotaDocsIn(s, session)
}

// NotaDocsIn fa la nota del torn amb els documents de s (els vents hi
// passen el de la seva carpeta) i torna amb què apuntar-los com a vistos,
// perquè els mateixos canvis no tornin a sortir. Qui la fa servir l'ha de
// cridar quan el model l'ha rebuda de debò: si la primera crida falla, els
// canvis han de tornar a sortir al torn següent. "" si no hi ha res a dir.
func NotaDocsIn(s *gdocs.Store, session string) (string, func()) {
	vistos, err := s.Seen(session)
	if err != nil {
		log.Printf("doc: versions vistes de la sessió: %v", err)
		return "", nil
	}
	ids := make([]string, 0, len(vistos))
	for id := range vistos {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b strings.Builder
	var dits []gdocs.Doc
	for _, id := range ids {
		v := vistos[id]
		d, err := s.Get(id)
		if err != nil {
			log.Printf("doc: %s: %v", id, err)
			continue
		}
		linies := canvisPersona(d, v)
		if len(linies) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s (%s, now v%d):\n%s\n", d.Title, d.ID, d.Version, strings.Join(linies, "\n"))
		dits = append(dits, d)
	}
	if b.Len() == 0 {
		return "", nil
	}
	return MarcaDocs + "; read a document again before editing blocks near these]\n" + b.String(), func() {
		for _, d := range dits {
			marcaVist(s, session, d)
		}
	}
}
