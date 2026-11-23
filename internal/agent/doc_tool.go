package agent

import (
	"encoding/json"
	"fmt"
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

func docExec(dir, argsJSON string) (string, error) {
	var a struct {
		Action, ID, Title, MD, After, Block string
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", err
	}
	s, err := gdocs.Open(procDirOr(dir))
	if err != nil {
		return "", err
	}
	var d gdocs.Doc
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "create":
		if d, err = s.Create(a.Title, a.MD, gdocs.AuthorAgent); err != nil {
			return "", err
		}
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
		d, err = s.Replace(a.ID, a.Block, a.MD, gdocs.AuthorAgent)
	case "delete":
		d, err = s.Delete(a.ID, a.Block)
	default:
		return "", fmt.Errorf("unknown action %q: create, list, read, insert, replace or delete", a.Action)
	}
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Document %s · %s · v%d\n", d.ID, d.Title, d.Version)
	for _, blk := range d.Blocks {
		fmt.Fprintf(&b, "\n[%s v%d] (%s)\n%s\n", blk.ID, blk.Version, blk.Author, blk.MD)
	}
	return b.String(), nil
}
