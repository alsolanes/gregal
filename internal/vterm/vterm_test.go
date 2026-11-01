package vterm

import (
	"strings"
	"testing"
)

func feed(t *Term, s string) { _, _ = t.Write([]byte(s)) }

func TestTextAndCursor(t *testing.T) {
	vt := New(20, 5, nil)
	feed(vt, "hola\r\nmón")
	txt := vt.Text()
	if txt[0] != "hola" || txt[1] != "món" {
		t.Fatalf("text: %q", txt)
	}
	if x, y, _ := vt.Cursor(); x != 3 || y != 1 {
		t.Fatalf("cursor %d,%d", x, y)
	}
}

func TestCUPAndErase(t *testing.T) {
	vt := New(10, 3, nil)
	feed(vt, "aaaaaaaaaa\r\nbbbbbbbbbb")
	feed(vt, "\x1b[1;4H\x1b[K")
	if got := vt.Text()[0]; got != "aaa" {
		t.Fatalf("EL: %q", got)
	}
	feed(vt, "\x1b[2J")
	for _, l := range vt.Text() {
		if l != "" {
			t.Fatalf("ED 2 ha deixat %q", l)
		}
	}
}

func TestAutowrapAndScroll(t *testing.T) {
	vt := New(5, 2, nil)
	feed(vt, "12345678")
	if txt := vt.Text(); txt[0] != "12345" || txt[1] != "678" {
		t.Fatalf("wrap: %q", txt)
	}
	feed(vt, "\r\nxy")
	if txt := vt.Text(); txt[0] != "678" || txt[1] != "xy" {
		t.Fatalf("scroll: %q", txt)
	}
}

func TestScrollRegionAndInsertLines(t *testing.T) {
	vt := New(5, 4, nil)
	feed(vt, "a\r\nb\r\nc\r\nd")
	feed(vt, "\x1b[2;3r\x1b[2;1H\x1b[L")
	if txt := vt.Text(); strings.Join(txt, "|") != "a||b|d" {
		t.Fatalf("IL dins regió: %q", txt)
	}
	feed(vt, "\x1b[2;1H\x1b[M")
	if txt := vt.Text(); strings.Join(txt, "|") != "a|b||d" {
		t.Fatalf("DL dins regió: %q", txt)
	}
}

func TestAltScreenRestoresMain(t *testing.T) {
	vt := New(10, 3, nil)
	feed(vt, "principal")
	feed(vt, "\x1b[?1049h")
	if !vt.Modes().AltScreen || vt.Text()[0] != "" {
		t.Fatal("la pantalla alternativa hauria de ser buida")
	}
	feed(vt, "tui")
	feed(vt, "\x1b[?1049l")
	if vt.Text()[0] != "principal" {
		t.Fatalf("en sortir: %q", vt.Text()[0])
	}
}

func TestSGRColors(t *testing.T) {
	vt := New(10, 1, nil)
	feed(vt, "\x1b[1;31mA\x1b[38;5;208mB\x1b[38;2;1;2;3;48;2;9;9;9mC\x1b[0mD")
	row := vt.Lines()[0]
	if row[0].FG != 1 || row[0].Attr&Bold == 0 {
		t.Fatalf("A: %+v", row[0])
	}
	if row[1].FG != 208 {
		t.Fatalf("B: %+v", row[1])
	}
	if r, g, b, ok := row[2].FG.IsRGB(); !ok || r != 1 || g != 2 || b != 3 {
		t.Fatalf("C fg: %+v", row[2])
	}
	if _, _, b, ok := row[2].BG.IsRGB(); !ok || b != 9 {
		t.Fatalf("C bg: %+v", row[2])
	}
	if row[3].FG != DefaultColor || row[3].Attr != 0 {
		t.Fatalf("D: %+v", row[3])
	}
	h := LineHTML(row, -1)
	if !strings.Contains(h, "#010203") || !strings.Contains(h, "font-weight:700") {
		t.Fatalf("html: %s", h)
	}
}

func TestWideCharsAndUTF8Split(t *testing.T) {
	vt := New(10, 1, nil)
	b := []byte("界x")
	_, _ = vt.Write(b[:2])
	_, _ = vt.Write(b[2:])
	row := vt.Lines()[0]
	if row[0].Ch != "界" || row[0].Width != 2 || row[1].Width != 0 || row[2].Ch != "x" {
		t.Fatalf("ample: %+v", row[:3])
	}
	if vt.Text()[0] != "界x" {
		t.Fatalf("text %q", vt.Text()[0])
	}
}

func TestQueriesReply(t *testing.T) {
	var got []string
	vt := New(10, 3, func(b []byte) { got = append(got, string(b)) })
	feed(vt, "\x1b[2;3H\x1b[6n\x1b]11;?\x07\x1b[c")
	if len(got) != 3 || got[0] != "\x1b[2;3R" || !strings.HasPrefix(got[1], "\x1b]11;rgb:") || !strings.HasPrefix(got[2], "\x1b[?") {
		t.Fatalf("respostes: %q", got)
	}
}

func TestModesAndTitle(t *testing.T) {
	vt := New(10, 3, nil)
	feed(vt, "\x1b[?1002h\x1b[?1006h\x1b[?2004h\x1b[?1h\x1b[?25l\x1b]0;gregal\x07")
	m := vt.Modes()
	if m.Mouse != MouseDrag || !m.MouseSGR || !m.BracketedPaste || !m.AppCursor || m.CursorVisible {
		t.Fatalf("modes: %+v", m)
	}
	if vt.Title() != "gregal" {
		t.Fatalf("títol %q", vt.Title())
	}
}

func TestUnknownSequencesAreSkipped(t *testing.T) {
	vt := New(20, 2, nil)
	feed(vt, "\x1bP1$r0m\x1b\\a\x1b[>4;2mb\x1b[?2026hc\x1b(Bd\x1b[3 qe")
	if vt.Text()[0] != "abcde" {
		t.Fatalf("text %q", vt.Text()[0])
	}
}

func TestResizeKeepsContent(t *testing.T) {
	vt := New(10, 3, nil)
	feed(vt, "abc\r\ndef")
	vt.Resize(5, 2)
	if txt := vt.Text(); txt[0] != "abc" || txt[1] != "def" {
		t.Fatalf("resize: %q", txt)
	}
	vt.Resize(30, 10)
	if c, r := vt.Size(); c != 30 || r != 10 {
		t.Fatalf("mida %dx%d", c, r)
	}
}

func TestLineHTMLEscapesAndMarksCursor(t *testing.T) {
	vt := New(10, 1, nil)
	feed(vt, "<a&b>")
	h := LineHTML(vt.Lines()[0], 5)
	if !strings.Contains(h, "&lt;a&amp;b&gt;") || !strings.Contains(h, `class="cur"`) {
		t.Fatalf("html %s", h)
	}
}
