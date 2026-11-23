package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"gregal/internal/gdocs"
	"gregal/internal/llm"
)

// L'eina doc: els documents vius del projecte (internal/gdocs). Els canvis
// passen sense confirmació en els modes que escriuen perquè la persona els
// veu en directe i queden versionats per bloc; en els de només lectura es
// deneguen, com office_edit.

func docToolSpec() llm.ToolSpec {
	str := map[string]any{"type": "string"}
	return llm.ToolSpec{Name: "doc", Description: "Live project documents made of markdown blocks, which the person can see and download as Word. " +
		"Prefer it over office_create when the person wants a document to work on with you. Actions: " +
		"create (title, optional md: split into blocks at headings and blank lines; returns the download link to give the person); list; " +
		"read (id; every block with its id and version); insert (id, md, after = block id, or omitted for the start); " +
		"replace (id, block, md); delete (id, block). Edit one block at a time instead of rewriting the document.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"action": map[string]any{"type": "string", "enum": []string{"create", "list", "read", "insert", "replace", "delete"}},
			"id":     str, "title": str, "md": str, "after": str, "block": str,
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
		Action, ID, Title, MD, After, Block string
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	var d gdocs.Doc
	var err error
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "create":
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
		d, err = s.Insert(a.ID, a.After, a.MD, gdocs.AuthorAgent)
	case "replace":
		d, err = s.Replace(a.ID, a.Block, a.MD, gdocs.AuthorAgent, vist(s, session, a.ID))
	case "delete":
		d, err = s.Delete(a.ID, a.Block, gdocs.AuthorAgent, vist(s, session, a.ID))
	default:
		return "", fmt.Errorf("unknown action %q: create, list, read, insert, replace or delete", a.Action)
	}
	if err != nil {
		return "", err
	}
	marcaVist(s, session, d)
	var b strings.Builder
	fmt.Fprintf(&b, "Document %s · %s · v%d\n", d.ID, d.Title, d.Version)
	for _, blk := range d.Blocks {
		fmt.Fprintf(&b, "\n[%s v%d] (%s)\n%s\n", blk.ID, blk.Version, blk.Author, blk.MD)
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
