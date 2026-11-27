package vterm

import (
	"fmt"
	"html"
	"strings"
)

// Colors per defecte del terminal pintat (fons fosc, com el TUI). Els
// per defecte i els 16 bàsics surten com a variables CSS amb el color de
// sempre de reserva: la pestanya Terminal els canvia segons el tema
// (--term-fg, --term-bg, --ansi-0 … --ansi-15) i el Kanban es queda igual.
const (
	DefaultFG = "var(--term-fg,#d4d4d4)"
	DefaultBG = "var(--term-bg,#0f141a)"
)

// base16 és la paleta dels 16 colors bàsics (to semblant al de VS Code).
var base16 = [16]string{
	"var(--ansi-0,#1e1e1e)", "var(--ansi-1,#e05561)", "var(--ansi-2,#8cc265)", "var(--ansi-3,#d18f52)",
	"var(--ansi-4,#4aa5f0)", "var(--ansi-5,#c162de)", "var(--ansi-6,#42b3c2)", "var(--ansi-7,#d4d4d4)",
	"var(--ansi-8,#6b737f)", "var(--ansi-9,#ff616e)", "var(--ansi-10,#a5e075)", "var(--ansi-11,#f0a45d)",
	"var(--ansi-12,#4dc4ff)", "var(--ansi-13,#de73ff)", "var(--ansi-14,#4cd1e0)", "var(--ansi-15,#ffffff)",
}

// Hex torna el color CSS (fg diu quin per defecte fer servir).
func (c Color) Hex(fg bool) string {
	if c < 0 {
		if fg {
			return DefaultFG
		}
		return DefaultBG
	}
	if r, g, b, ok := c.IsRGB(); ok {
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}
	n := int(c)
	switch {
	case n < 16:
		return base16[n]
	case n < 232:
		n -= 16
		lv := func(v int) int {
			if v == 0 {
				return 0
			}
			return 55 + v*40
		}
		return fmt.Sprintf("#%02x%02x%02x", lv(n/36), lv((n/6)%6), lv(n%6))
	case n < 256:
		v := 8 + (n-232)*10
		return fmt.Sprintf("#%02x%02x%02x", v, v, v)
	}
	return DefaultFG
}

type style struct {
	fg, bg string
	attr   Attr
	cursor bool
}

func cellStyle(c Cell, cursor bool) style {
	fg, bg := c.FG.Hex(true), c.BG.Hex(false)
	// El negre bàsic sobre el fons per defecte quedaria invisible.
	if c.Attr&Bold != 0 && c.FG >= 0 && c.FG < 8 {
		fg = Color(c.FG + 8).Hex(true)
	}
	if c.Attr&Inverse != 0 {
		fg, bg = bg, fg
	}
	if c.Attr&Hidden != 0 {
		fg = bg
	}
	return style{fg: fg, bg: bg, attr: c.Attr &^ (Inverse | Hidden | Blink), cursor: cursor}
}

func (s style) css() string {
	var b strings.Builder
	if s.fg != DefaultFG {
		b.WriteString("color:" + s.fg + ";")
	}
	if s.bg != DefaultBG {
		b.WriteString("background:" + s.bg + ";")
	}
	if s.attr&Bold != 0 {
		b.WriteString("font-weight:700;")
	}
	if s.attr&Faint != 0 {
		b.WriteString("opacity:.6;")
	}
	if s.attr&Italic != 0 {
		b.WriteString("font-style:italic;")
	}
	switch {
	case s.attr&Underline != 0 && s.attr&Strike != 0:
		b.WriteString("text-decoration:underline line-through;")
	case s.attr&Underline != 0:
		b.WriteString("text-decoration:underline;")
	case s.attr&Strike != 0:
		b.WriteString("text-decoration:line-through;")
	}
	return b.String()
}

// LineHTML pinta una fila com a HTML (spans amb estil en línia). cursorX
// és la columna del cursor a marcar (-1 = cap). Els espais del final sense
// fons es retallen.
func LineHTML(row []Cell, cursorX int) string {
	end := len(row)
	for end > 0 && end-1 != cursorX {
		c := row[end-1]
		if (c.Ch == "" || c.Ch == " ") && c.BG < 0 && c.Attr&(Inverse|Underline|Strike) == 0 {
			end--
			continue
		}
		break
	}
	var b strings.Builder
	var run strings.Builder
	var cur style
	open := false
	flush := func() {
		if run.Len() == 0 {
			return
		}
		css := cur.css()
		cls := ""
		if cur.cursor {
			cls = ` class="cur"`
		}
		if css == "" && cls == "" {
			b.WriteString(run.String())
		} else {
			b.WriteString("<span" + cls)
			if css != "" {
				b.WriteString(` style="` + css + `"`)
			}
			b.WriteString(">" + run.String() + "</span>")
		}
		run.Reset()
	}
	for x := 0; x < end; x++ {
		c := row[x]
		if c.Width == 0 {
			continue
		}
		st := cellStyle(c, x == cursorX)
		if !open || st != cur {
			flush()
			cur, open = st, true
		}
		ch := c.Ch
		if ch == "" {
			ch = " "
		}
		run.WriteString(html.EscapeString(ch))
	}
	flush()
	return b.String()
}
