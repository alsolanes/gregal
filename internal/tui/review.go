package tui

import (
	"context"
	"fmt"
	"strings"

	"gregal/internal/tools"
)

func reviewMenu(files []tools.DiffFile) *menuState {
	type ref struct {
		file tools.DiffFile
		hunk tools.DiffHunk
	}
	refs := map[string]ref{}
	items := []menuItem{}
	for fi, f := range files {
		if f.Binary {
			key := fmt.Sprintf("%d:-1", fi)
			refs[key] = ref{file: f}
			items = append(items, menuItem{text: fmt.Sprintf("%s · binari", f.Path), val: key})
			continue
		}
		for hi, h := range f.Hunks {
			key := fmt.Sprintf("%d:%d", fi, hi)
			refs[key] = ref{file: f, hunk: h}
			items = append(items, menuItem{text: fmt.Sprintf("%s · hunk %d  +%d −%d", f.Path, hi+1, h.Added, h.Removed), val: key})
		}
	}
	if len(items) == 0 {
		return nil
	}
	return &menuState{title: "revisió de canvis", items: items, run: func(m *Model, val string) (string, *menuState) {
		r := refs[val]
		return "", reviewHunkMenu(r.file, r.hunk)
	}}
}

func reviewHunkMenu(file tools.DiffFile, h tools.DiffHunk) *menuState {
	items := []menuItem{{text: "mostra el hunk", val: "show"}, {text: "conserva el canvi", val: "keep"}}
	if !file.Binary && !file.Untracked && strings.TrimSpace(h.Patch) != "" {
		items = append(items, menuItem{text: "descarta el hunk…", val: "discard"})
	}
	title := fmt.Sprintf("%s · +%d −%d", file.Path, h.Added, h.Removed)
	return &menuState{title: title, items: items, run: func(m *Model, action string) (string, *menuState) {
		switch action {
		case "show":
			m.push(reviewHunkBlock(file.Path, h))
			return "", reviewHunkMenu(file, h)
		case "keep":
			m.push(okStyle.Render("✓ conservat · " + file.Path))
			return "", nil
		case "discard":
			patch, cwd := h.Patch, m.cwd
			m.pending = &pendingOp{desc: fmt.Sprintf("descartar hunk de %s (+%d −%d)", file.Path, h.Added, h.Removed), run: func(ctx context.Context) (string, error) {
				if err := tools.GitApplyReverse(cwd, patch); err != nil {
					return "", err
				}
				return "hunk descartat", nil
			}}
			return "", nil
		}
		return "", nil
	}}
}

func reviewHunkBlock(path string, h tools.DiffHunk) string {
	var b strings.Builder
	b.WriteString(hunkStyle.Render("≋ "+path+" "+h.Header) + "\n")
	for _, l := range h.Lines {
		switch l.Kind {
		case "add":
			b.WriteString(addStyle.Render("+ "+l.Text) + "\n")
		case "del":
			b.WriteString(delStyle.Render("− "+l.Text) + "\n")
		default:
			b.WriteString(dimStyle.Render("  "+l.Text) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
