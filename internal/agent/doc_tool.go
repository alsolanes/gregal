package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
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
		"replace (id, block, md or slide); set_cells (id, block = the sheet, cells: {\"A1\": \"Sales\", \"B1\": \"1200\", \"B3\": \"=SUM(B1:B2)\"}, an empty value clears the cell); delete (id, block). A slide is {layout: title|section|content|two_columns|comparison|big_number|image|title_only|blank|closing, " +
		"title, subtitle, bullets, body, left, right, left_title, right_title, number, caption, image, table, chart: {type, categories, series: [{name, values}]}, notes}. " +
		"Edit one block or slide at a time instead of rewriting the document.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"action": map[string]any{"type": "string", "enum": []string{"create", "list", "read", "insert", "replace", "set_cells", "delete"}},
			"kind":   map[string]any{"type": "string", "enum": []string{gdocs.KindDoc, gdocs.KindDeck, gdocs.KindSheet}},
			"slide":  map[string]any{"type": "object", "description": "One slide of a deck"},
			"cells":  map[string]any{"type": "object", "description": "Cell name → input (a number, text or a formula starting with =)"},
			"id":     str, "title": str, "md": str, "after": str, "block": str, "sheet": str, "range": str,
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

// DocExecIn fa l'acció de doc sobre un magatzem ja obert (els vents hi
// passen el de la carpeta personal).
func DocExecIn(s *gdocs.Store, session, argsJSON string) (string, error) {
	var a struct {
		Action, ID, Title, Kind, MD, After, Block, Sheet, Range string
		Slide                                                   *ooxml.SlideSpec
		Cells                                                   map[string]string
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	var d gdocs.Doc
	var err error
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "create":
		if a.Kind == gdocs.KindDeck {
			if strings.TrimSpace(a.MD) != "" {
				return "", fmt.Errorf("a deck starts empty: create it without md, then insert its slides one by one")
			}
			if d, err = s.CreateDeck(a.Title); err != nil {
				return "", err
			}
			marcaVist(s, session, d)
			return fmt.Sprintf("Created document %s (an empty deck): insert its slides one by one. Give the person this link to download it as PowerPoint: [%s](/api/docs/%s/export?format=pptx)",
				d.ID, d.Title, d.ID), nil
		}
		if a.Kind == gdocs.KindSheet {
			if d, err = s.CreateSheet(a.Title, a.Sheet, gdocs.AuthorAgent); err != nil {
				return "", err
			}
			marcaVist(s, session, d)
			return fmt.Sprintf("Created document %s (a spreadsheet: sheet %s is block %s): fill it with set_cells. Give the person this link to download it as Excel: [%s](/api/docs/%s/export?format=xlsx)",
				d.ID, d.Blocks[0].Name, d.Blocks[0].ID, d.Title, d.ID), nil
		}
		if a.Kind != "" && a.Kind != gdocs.KindDoc {
			return "", fmt.Errorf("unknown kind %q: doc, deck or sheet", a.Kind)
		}
		if d, err = s.Create(a.Title, a.MD, gdocs.AuthorAgent); err != nil {
			return "", err
		}
		marcaVist(s, session, d)
		return fmt.Sprintf("Created document %s with %d blocks. Give the person this link to download it as Word: [%s](/api/docs/%s/export?format=docx)",
			d.ID, len(d.Blocks), d.Title, d.ID), nil
	case "list":
		docs, err := s.List()
		if err != nil || len(docs) == 0 {
			return "No documents in this project.", err
		}
		var b strings.Builder
		for _, d := range docs {
			fmt.Fprintf(&b, "%s · %s · %d blocks · v%d\n", d.ID, d.Title, len(d.Blocks), d.Version)
		}
		return b.String(), nil
	case "read":
		d, err = s.Get(a.ID)
	case "insert":
		if a.Slide != nil {
			d, err = s.InsertSlide(a.ID, a.After, *a.Slide, gdocs.AuthorAgent)
		} else if a.Sheet != "" {
			d, err = s.InsertSheet(a.ID, a.After, a.Sheet, gdocs.AuthorAgent)
		} else {
			d, err = s.Insert(a.ID, a.After, a.MD, gdocs.AuthorAgent)
		}
	case "replace":
		if a.Slide != nil {
			d, err = s.ReplaceSlide(a.ID, a.Block, *a.Slide, gdocs.AuthorAgent, vist(s, session, a.ID))
		} else {
			d, err = s.Replace(a.ID, a.Block, a.MD, gdocs.AuthorAgent, vist(s, session, a.ID))
		}
	case "set_cells":
		d, err = s.SetCells(a.ID, a.Block, a.Cells, gdocs.AuthorAgent, vist(s, session, a.ID))
	case "delete":
		d, err = s.Delete(a.ID, a.Block, gdocs.AuthorAgent, vist(s, session, a.ID))
	default:
		return "", fmt.Errorf("unknown action %q: create, list, read, insert, replace, set_cells or delete", a.Action)
	}
	if err != nil {
		return "", err
	}
	marcaVist(s, session, d)
	var b strings.Builder
	fmt.Fprintf(&b, "Document %s · %s · v%d\n", d.ID, d.Title, d.Version)
	for _, blk := range d.Blocks {
		text := blk.MD
		if d.Kind == gdocs.KindSheet {
			if text, err = cellesFull(blk, a.Range); err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "\n[%s v%d] (%s) sheet %s\n%s", blk.ID, blk.Version, blk.Author, blk.Name, text)
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
		linies = append(linies, fmt.Sprintf("- %s %s: %s", que, blk.ID, retallaText(blk.MD, maxNotaBloc)))
	}
	for _, blk := range d.Deleted {
		// Un bloc que l'agent no havia vist mai no cal dir-li que ja no hi és.
		if blk.Version > v && blk.Author == gdocs.AuthorUser && blk.Since <= v {
			linies = append(linies, "- deleted "+blk.ID)
		}
	}
	return linies
}

// notaDocs fa la nota del torn i apunta els documents com a vistos, perquè
// els mateixos canvis no tornin a sortir. "" si no hi ha res a dir.
func notaDocs(dir, session string) string {
	s, err := gdocs.Open(procDirOr(dir))
	if err != nil {
		return ""
	}
	vistos, err := s.Seen(session)
	if err != nil {
		log.Printf("doc: versions vistes de la sessió: %v", err)
		return ""
	}
	ids := make([]string, 0, len(vistos))
	for id := range vistos {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b strings.Builder
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
		marcaVist(s, session, d)
	}
	if b.Len() == 0 {
		return ""
	}
	return MarcaDocs + "; read a document again before editing blocks near these]\n" + b.String()
}
