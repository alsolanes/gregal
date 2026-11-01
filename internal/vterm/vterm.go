// Package vterm és un emulador de terminal VT/xterm petit: rep la sortida
// d'un procés (la del TUI de Gregal dins d'un pseudoterminal) i manté la
// pantalla com una graella de cel·les amb colors. El mode Kanban del web el
// fa servir per pintar cada targeta sense cap emulador al navegador: el
// servidor sap què hi ha a cada terminal i n'envia les línies ja pintades.
//
// Cobreix el que fan servir bubbletea, lipgloss i ConPTY: moviments de
// cursor, esborrats, regions de scroll, inserir/esborrar línies i
// caràcters, SGR (16, 256 i color directe), pantalla alternativa, modes
// del ratolí i de l'enganxat, i les consultes habituals (DSR, DA, OSC
// 10/11). La resta es descarta sense trencar el flux.
package vterm

import (
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// Color: -1 és el color per defecte; 0-255 la paleta; RGB amb el bit 24.
type Color int32

const DefaultColor Color = -1

const rgbFlag = 1 << 24

// RGB construeix un color directe.
func RGB(r, g, b uint8) Color { return Color(rgbFlag | int32(r)<<16 | int32(g)<<8 | int32(b)) }

// IsRGB diu si és un color directe i en torna les components.
func (c Color) IsRGB() (r, g, b uint8, ok bool) {
	if c < 0 || c&rgbFlag == 0 {
		return 0, 0, 0, false
	}
	return uint8(c >> 16), uint8(c >> 8), uint8(c), true
}

// Attr són els atributs de text.
type Attr uint16

const (
	Bold Attr = 1 << iota
	Faint
	Italic
	Underline
	Blink
	Inverse
	Hidden
	Strike
)

// Cell és una posició de la pantalla. Ch buit és un espai; Width 0 marca
// la segona meitat d'un caràcter ample.
type Cell struct {
	Ch    string
	FG    Color
	BG    Color
	Attr  Attr
	Width int8
}

func blank(pen Cell) Cell { return Cell{FG: DefaultColor, BG: pen.BG, Width: 1} }

// Mouse: quins events demana l'aplicació.
type MouseMode int

const (
	MouseOff    MouseMode = 0
	MouseClick  MouseMode = 1000
	MouseDrag   MouseMode = 1002
	MouseMotion MouseMode = 1003
)

// Modes són els modes privats que importen a qui envia l'entrada.
type Modes struct {
	AppCursor      bool
	BracketedPaste bool
	Mouse          MouseMode
	MouseSGR       bool
	CursorVisible  bool
	AltScreen      bool
}

type cursor struct {
	x, y     int
	pen      Cell
	wrapNext bool
}

const (
	stGround = iota
	stEscape
	stCSI
	stOSC
	stString  // DCS, SOS, PM, APC: s'ignoren fins a ST
	stCharset // ESC ( X i companyia: un byte més
)

// Term és l'emulador. És segur per a ús concurrent.
type Term struct {
	mu         sync.Mutex
	cols, rows int
	main, alt  [][]Cell
	screen     *[][]Cell
	cur        cursor
	savedMain  cursor
	savedAlt   cursor
	top, bot   int
	autowrap   bool
	modes      Modes
	title      string
	lastPrint  string
	seq        uint64
	dirty      []bool
	reply      func([]byte)

	state    int
	buf      []byte // paràmetres CSI o text OSC
	escInStr bool
	utf      []byte
}

// New crea un terminal de cols×rows. reply rep les respostes a les
// consultes (posició del cursor, atributs del dispositiu, colors): s'han
// d'escriure a l'entrada del procés. Pot ser nil.
func New(cols, rows int, reply func([]byte)) *Term {
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	t := &Term{cols: cols, rows: rows, autowrap: true, reply: reply}
	t.cur.pen = Cell{FG: DefaultColor, BG: DefaultColor, Width: 1}
	t.main = newGrid(cols, rows)
	t.alt = newGrid(cols, rows)
	t.screen = &t.main
	t.top, t.bot = 0, rows-1
	t.modes.CursorVisible = true
	t.dirty = make([]bool, rows)
	t.touchAll()
	return t
}

func newGrid(cols, rows int) [][]Cell {
	g := make([][]Cell, rows)
	for i := range g {
		g[i] = newRow(cols, DefaultColor)
	}
	return g
}

func newRow(cols int, bg Color) []Cell {
	r := make([]Cell, cols)
	for i := range r {
		r[i] = Cell{FG: DefaultColor, BG: bg, Width: 1}
	}
	return r
}

// Size torna columnes i files.
func (t *Term) Size() (int, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cols, t.rows
}

// Seq canvia cada cop que la pantalla canvia.
func (t *Term) Seq() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.seq
}

// Title és l'últim títol que ha posat l'aplicació (OSC 0/2).
func (t *Term) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.title
}

// Modes torna els modes actuals.
func (t *Term) Modes() Modes {
	t.mu.Lock()
	defer t.mu.Unlock()
	m := t.modes
	m.AltScreen = t.screen == &t.alt
	return m
}

// Cursor torna la posició del cursor i si es veu.
func (t *Term) Cursor() (x, y int, visible bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cur.x, t.cur.y, t.modes.CursorVisible
}

// Lines torna una còpia de la pantalla activa.
func (t *Term) Lines() [][]Cell {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([][]Cell, t.rows)
	for i, r := range *t.screen {
		out[i] = append([]Cell(nil), r...)
	}
	return out
}

// Text torna la pantalla com a text pla (una línia per fila, sense espais
// al final).
func (t *Term) Text() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, t.rows)
	for i, r := range *t.screen {
		out[i] = rowText(r)
	}
	return out
}

func rowText(r []Cell) string {
	var b strings.Builder
	for _, c := range r {
		if c.Width == 0 {
			continue
		}
		if c.Ch == "" {
			b.WriteByte(' ')
		} else {
			b.WriteString(c.Ch)
		}
	}
	return strings.TrimRight(b.String(), " ")
}

// TakeDirty torna quines files han canviat des de l'últim cop i les neteja.
func (t *Term) TakeDirty() []bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := t.dirty
	t.dirty = make([]bool, t.rows)
	return out
}

func (t *Term) touch(y int) {
	if y >= 0 && y < len(t.dirty) {
		t.dirty[y] = true
	}
	t.seq++
}

func (t *Term) touchRange(a, b int) {
	for y := a; y <= b; y++ {
		t.touch(y)
	}
}

func (t *Term) touchAll() { t.touchRange(0, t.rows-1) }

// Resize canvia la mida sense refluir: el contingut es queda a dalt a
// l'esquerra i el que no hi cap es perd (el TUI es repinta sencer igualment).
func (t *Term) Resize(cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if cols < 1 || rows < 1 || (cols == t.cols && rows == t.rows) {
		return
	}
	t.main = resizeGrid(t.main, cols, rows)
	t.alt = resizeGrid(t.alt, cols, rows)
	if t.screen != &t.main {
		t.screen = &t.alt
	}
	t.cols, t.rows = cols, rows
	t.top, t.bot = 0, rows-1
	t.cur.x = min(t.cur.x, cols-1)
	t.cur.y = min(t.cur.y, rows-1)
	t.cur.wrapNext = false
	t.dirty = make([]bool, rows)
	t.touchAll()
}

func resizeGrid(g [][]Cell, cols, rows int) [][]Cell {
	out := make([][]Cell, rows)
	for y := 0; y < rows; y++ {
		r := newRow(cols, DefaultColor)
		if y < len(g) {
			copy(r, g[y])
			if cols < len(g[y]) && cols > 0 && r[cols-1].Width == 2 {
				r[cols-1] = Cell{FG: DefaultColor, BG: DefaultColor, Width: 1}
			}
		}
		out[y] = r
	}
	return out
}

// Write processa la sortida del procés.
func (t *Term) Write(p []byte) (int, error) {
	t.mu.Lock()
	var replies [][]byte
	for _, b := range p {
		if r := t.step(b); r != nil {
			replies = append(replies, r)
		}
	}
	reply := t.reply
	t.mu.Unlock()
	// Fora del lock: qui respon escriu al pseudoterminal i pot bloquejar.
	if reply != nil {
		for _, r := range replies {
			reply(r)
		}
	}
	return len(p), nil
}

func (t *Term) step(b byte) []byte {
	switch t.state {
	case stGround:
		return t.ground(b)
	case stEscape:
		return t.escape(b)
	case stCSI:
		switch {
		case b == 0x1b:
			t.state, t.buf = stEscape, t.buf[:0]
		case b == 0x18 || b == 0x1a:
			t.state = stGround
		case b < 0x20:
			t.control(b)
		case b >= 0x40 && b <= 0x7e:
			t.state = stGround
			return t.csi(string(t.buf), b)
		default:
			if len(t.buf) < 256 {
				t.buf = append(t.buf, b)
			}
		}
	case stOSC:
		switch {
		case b == 0x07:
			t.state = stGround
			return t.osc(string(t.buf), "\x07")
		case t.escInStr:
			t.escInStr = false
			t.state = stGround
			if b == '\\' {
				return t.osc(string(t.buf), "\x1b\\")
			}
		case b == 0x1b:
			t.escInStr = true
		default:
			if len(t.buf) < 4096 {
				t.buf = append(t.buf, b)
			}
		}
	case stString:
		switch {
		case b == 0x07:
			t.state = stGround
		case t.escInStr:
			t.escInStr = false
			if b == '\\' {
				t.state = stGround
			}
		case b == 0x1b:
			t.escInStr = true
		}
	case stCharset:
		t.state = stGround
	}
	return nil
}

func (t *Term) ground(b byte) []byte {
	if len(t.utf) > 0 || b >= 0x80 {
		if b < 0x80 || (len(t.utf) == 0 && !utf8.RuneStart(b)) {
			// Seqüència trencada: es pinta el reemplaçament i es continua.
			t.utf = t.utf[:0]
			t.print("�")
			if b >= 0x80 {
				return nil
			}
		} else {
			t.utf = append(t.utf, b)
			if !utf8.FullRune(t.utf) {
				if len(t.utf) >= 4 {
					t.utf = t.utf[:0]
					t.print("�")
				}
				return nil
			}
			r, _ := utf8.DecodeRune(t.utf)
			t.utf = t.utf[:0]
			t.printRune(r)
			return nil
		}
	}
	if b == 0x1b {
		t.state = stEscape
		return nil
	}
	if b < 0x20 || b == 0x7f {
		t.control(b)
		return nil
	}
	t.printRune(rune(b))
	return nil
}

func (t *Term) control(b byte) {
	switch b {
	case '\r':
		t.cur.x, t.cur.wrapNext = 0, false
	case '\n', '\v', '\f':
		t.index()
	case '\b':
		if t.cur.x > 0 {
			t.cur.x--
		}
		t.cur.wrapNext = false
	case '\t':
		t.cur.x = min((t.cur.x/8+1)*8, t.cols-1)
		t.cur.wrapNext = false
	}
}

func (t *Term) escape(b byte) []byte {
	t.state = stGround
	switch b {
	case '[':
		t.state, t.buf = stCSI, t.buf[:0]
	case ']':
		t.state, t.buf, t.escInStr = stOSC, t.buf[:0], false
	case 'P', 'X', '^', '_':
		t.state, t.escInStr = stString, false
	case '(', ')', '*', '+', '-', '.', '/', '#', '%', ' ':
		t.state = stCharset
	case '7':
		t.saveCursor()
	case '8':
		t.restoreCursor()
	case 'D':
		t.index()
	case 'E':
		t.cur.x = 0
		t.index()
	case 'M':
		t.reverseIndex()
	case 'c':
		t.reset()
	}
	return nil
}

func (t *Term) reset() {
	t.main = newGrid(t.cols, t.rows)
	t.alt = newGrid(t.cols, t.rows)
	t.screen = &t.main
	t.cur = cursor{pen: Cell{FG: DefaultColor, BG: DefaultColor, Width: 1}}
	t.top, t.bot = 0, t.rows-1
	t.autowrap = true
	t.modes = Modes{CursorVisible: true}
	t.touchAll()
}

func (t *Term) saveCursor() {
	if t.screen == &t.alt {
		t.savedAlt = t.cur
	} else {
		t.savedMain = t.cur
	}
}

func (t *Term) restoreCursor() {
	c := t.savedMain
	if t.screen == &t.alt {
		c = t.savedAlt
	}
	if c.pen.Width == 0 {
		c.pen = Cell{FG: DefaultColor, BG: DefaultColor, Width: 1}
	}
	c.x, c.y = min(c.x, t.cols-1), min(c.y, t.rows-1)
	t.cur = c
}

func (t *Term) index() {
	t.cur.wrapNext = false
	if t.cur.y == t.bot {
		t.scrollUp(t.top, t.bot, 1)
	} else if t.cur.y < t.rows-1 {
		t.cur.y++
	}
}

func (t *Term) reverseIndex() {
	t.cur.wrapNext = false
	if t.cur.y == t.top {
		t.scrollDown(t.top, t.bot, 1)
	} else if t.cur.y > 0 {
		t.cur.y--
	}
}

func (t *Term) scrollUp(top, bot, n int) {
	g := *t.screen
	n = min(n, bot-top+1)
	if n <= 0 {
		return
	}
	copy(g[top:bot+1], g[top+n:bot+1])
	for y := bot - n + 1; y <= bot; y++ {
		g[y] = newRow(t.cols, t.cur.pen.BG)
	}
	t.touchRange(top, bot)
}

func (t *Term) scrollDown(top, bot, n int) {
	g := *t.screen
	n = min(n, bot-top+1)
	if n <= 0 {
		return
	}
	copy(g[top+n:bot+1], g[top:bot+1-n])
	for y := top; y < top+n; y++ {
		g[y] = newRow(t.cols, t.cur.pen.BG)
	}
	t.touchRange(top, bot)
}

func (t *Term) printRune(r rune) {
	w := runewidth.RuneWidth(r)
	if w == 0 {
		// Combinant: s'enganxa al caràcter anterior.
		x := t.cur.x
		if !t.cur.wrapNext && x > 0 {
			x--
		}
		row := (*t.screen)[t.cur.y]
		if x < len(row) && row[x].Width == 0 && x > 0 {
			x--
		}
		if x < len(row) {
			if row[x].Ch == "" {
				row[x].Ch = " "
			}
			row[x].Ch += string(r)
			t.touch(t.cur.y)
		}
		return
	}
	t.put(string(r), w)
}

func (t *Term) print(s string) { t.put(s, 1) }

func (t *Term) put(s string, w int) {
	if w > t.cols {
		return
	}
	if t.cur.wrapNext {
		t.cur.wrapNext = false
		if t.autowrap {
			t.cur.x = 0
			t.index()
		}
	}
	if t.cur.x+w > t.cols {
		if t.autowrap {
			t.cur.x = 0
			t.index()
		} else {
			t.cur.x = t.cols - w
		}
	}
	row := (*t.screen)[t.cur.y]
	t.clearWide(row, t.cur.x)
	if w == 2 {
		t.clearWide(row, t.cur.x+1)
	}
	c := t.cur.pen
	c.Ch, c.Width = s, int8(w)
	row[t.cur.x] = c
	if w == 2 {
		cont := t.cur.pen
		cont.Ch, cont.Width = "", 0
		row[t.cur.x+1] = cont
	}
	t.lastPrint = s
	t.touch(t.cur.y)
	t.cur.x += w
	if t.cur.x >= t.cols {
		t.cur.x = t.cols - 1
		t.cur.wrapNext = true
	}
}

// clearWide trenca un caràcter ample si se n'escriu una meitat a sobre.
func (t *Term) clearWide(row []Cell, x int) {
	if x < 0 || x >= len(row) {
		return
	}
	switch row[x].Width {
	case 0:
		if x > 0 {
			row[x-1] = Cell{FG: DefaultColor, BG: row[x-1].BG, Width: 1}
		}
	case 2:
		if x+1 < len(row) {
			row[x+1] = Cell{FG: DefaultColor, BG: row[x+1].BG, Width: 1}
		}
	}
}

// params separa els paràmetres CSI. Els subparàmetres (38:2::r:g:b) queden
// dins del mateix grup.
func parseParams(s string) [][]int {
	if s == "" {
		return nil
	}
	var out [][]int
	for _, grp := range strings.Split(s, ";") {
		var g []int
		for _, sub := range strings.Split(grp, ":") {
			n, err := strconv.Atoi(sub)
			if err != nil {
				n = -1 // buit = per defecte
			}
			g = append(g, n)
		}
		out = append(out, g)
	}
	return out
}

func param(ps [][]int, i, def int) int {
	if i >= len(ps) || len(ps[i]) == 0 || ps[i][0] < 0 {
		return def
	}
	return ps[i][0]
}

// paramN és com param però amb 0 tractat com el valor per defecte (1),
// com fan els moviments de cursor.
func paramN(ps [][]int, i int) int {
	n := param(ps, i, 1)
	if n < 1 {
		return 1
	}
	return n
}

func (t *Term) csi(raw string, final byte) []byte {
	priv := byte(0)
	if raw != "" && (raw[0] == '?' || raw[0] == '>' || raw[0] == '<' || raw[0] == '=') {
		priv, raw = raw[0], raw[1:]
	}
	inter := ""
	for len(raw) > 0 && raw[len(raw)-1] >= 0x20 && raw[len(raw)-1] <= 0x2f {
		inter = string(raw[len(raw)-1]) + inter
		raw = raw[:len(raw)-1]
	}
	if inter != "" {
		// DECSCUSR (forma del cursor), DECSTR… no canvien la pantalla.
		if inter == "!" && final == 'p' {
			t.softReset()
		}
		return nil
	}
	ps := parseParams(raw)
	if priv == '?' {
		switch final {
		case 'h':
			t.decset(ps, true)
		case 'l':
			t.decset(ps, false)
		}
		return nil
	}
	if priv == '>' {
		if final == 'c' {
			return []byte("\x1b[>0;10;1c")
		}
		return nil
	}
	if priv != 0 {
		return nil
	}
	c := &t.cur
	switch final {
	case '@':
		t.insertChars(paramN(ps, 0))
	case 'A':
		c.y = max(c.y-paramN(ps, 0), t.marginTop())
		c.wrapNext = false
	case 'B', 'e':
		c.y = min(c.y+paramN(ps, 0), t.marginBot())
		c.wrapNext = false
	case 'C', 'a':
		c.x = min(c.x+paramN(ps, 0), t.cols-1)
		c.wrapNext = false
	case 'D':
		c.x = max(c.x-paramN(ps, 0), 0)
		c.wrapNext = false
	case 'E':
		c.y = min(c.y+paramN(ps, 0), t.marginBot())
		c.x, c.wrapNext = 0, false
	case 'F':
		c.y = max(c.y-paramN(ps, 0), t.marginTop())
		c.x, c.wrapNext = 0, false
	case 'G', '`':
		c.x = clampInt(paramN(ps, 0)-1, 0, t.cols-1)
		c.wrapNext = false
	case 'd':
		c.y = clampInt(paramN(ps, 0)-1, 0, t.rows-1)
		c.wrapNext = false
	case 'H', 'f':
		c.y = clampInt(paramN(ps, 0)-1, 0, t.rows-1)
		c.x = clampInt(paramN(ps, 1)-1, 0, t.cols-1)
		c.wrapNext = false
	case 'J':
		t.eraseDisplay(param(ps, 0, 0))
	case 'K':
		t.eraseLine(param(ps, 0, 0))
	case 'L':
		if c.y >= t.top && c.y <= t.bot {
			t.scrollDown(c.y, t.bot, paramN(ps, 0))
			c.x, c.wrapNext = 0, false
		}
	case 'M':
		if c.y >= t.top && c.y <= t.bot {
			t.scrollUp(c.y, t.bot, paramN(ps, 0))
			c.x, c.wrapNext = 0, false
		}
	case 'P':
		t.deleteChars(paramN(ps, 0))
	case 'X':
		t.eraseChars(paramN(ps, 0))
	case 'S':
		t.scrollUp(t.top, t.bot, paramN(ps, 0))
	case 'T':
		if len(ps) <= 1 {
			t.scrollDown(t.top, t.bot, paramN(ps, 0))
		}
	case 'b':
		if t.lastPrint != "" {
			n := min(paramN(ps, 0), t.cols*t.rows)
			w := max(runewidth.StringWidth(t.lastPrint), 1)
			for i := 0; i < n; i++ {
				t.put(t.lastPrint, w)
			}
		}
	case 'Z':
		for n := paramN(ps, 0); n > 0 && c.x > 0; n-- {
			c.x = ((c.x - 1) / 8) * 8
		}
	case 'm':
		t.sgr(ps)
	case 'r':
		top := paramN(ps, 0) - 1
		bot := param(ps, 1, t.rows)
		if bot < 1 || bot > t.rows {
			bot = t.rows
		}
		bot--
		if top < bot {
			t.top, t.bot = top, bot
			c.x, c.y, c.wrapNext = 0, 0, false
		}
	case 's':
		t.saveCursor()
	case 'u':
		t.restoreCursor()
	case 'n':
		switch param(ps, 0, 0) {
		case 5:
			return []byte("\x1b[0n")
		case 6:
			return []byte("\x1b[" + strconv.Itoa(c.y+1) + ";" + strconv.Itoa(c.x+1) + "R")
		}
	case 'c':
		if param(ps, 0, 0) == 0 {
			return []byte("\x1b[?62;22c")
		}
	}
	return nil
}

func (t *Term) softReset() {
	t.cur.pen = Cell{FG: DefaultColor, BG: DefaultColor, Width: 1}
	t.top, t.bot = 0, t.rows-1
	t.autowrap = true
	t.modes.CursorVisible = true
	t.modes.AppCursor = false
}

func (t *Term) marginTop() int {
	if t.cur.y >= t.top {
		return t.top
	}
	return 0
}

func (t *Term) marginBot() int {
	if t.cur.y <= t.bot {
		return t.bot
	}
	return t.rows - 1
}

func clampInt(v, lo, hi int) int { return max(lo, min(v, hi)) }

func (t *Term) eraseCells(y, from, to int) {
	if y < 0 || y >= t.rows {
		return
	}
	row := (*t.screen)[y]
	from, to = max(from, 0), min(to, t.cols)
	for x := from; x < to; x++ {
		row[x] = blank(t.cur.pen)
	}
	// Un ample partit pels límits queda net.
	if from > 0 && from < t.cols && row[from-1].Width == 2 {
		row[from-1] = blank(t.cur.pen)
	}
	if to < t.cols && row[to].Width == 0 {
		row[to] = blank(t.cur.pen)
	}
	t.touch(y)
}

func (t *Term) eraseDisplay(mode int) {
	c := t.cur
	switch mode {
	case 0:
		t.eraseCells(c.y, c.x, t.cols)
		for y := c.y + 1; y < t.rows; y++ {
			t.eraseCells(y, 0, t.cols)
		}
	case 1:
		for y := 0; y < c.y; y++ {
			t.eraseCells(y, 0, t.cols)
		}
		t.eraseCells(c.y, 0, c.x+1)
	case 2, 3:
		for y := 0; y < t.rows; y++ {
			t.eraseCells(y, 0, t.cols)
		}
	}
}

func (t *Term) eraseLine(mode int) {
	c := t.cur
	switch mode {
	case 0:
		t.eraseCells(c.y, c.x, t.cols)
	case 1:
		t.eraseCells(c.y, 0, c.x+1)
	case 2:
		t.eraseCells(c.y, 0, t.cols)
	}
	t.cur.wrapNext = false
}

func (t *Term) eraseChars(n int) {
	t.eraseCells(t.cur.y, t.cur.x, t.cur.x+n)
	t.cur.wrapNext = false
}

func (t *Term) insertChars(n int) {
	row := (*t.screen)[t.cur.y]
	x := t.cur.x
	n = min(n, t.cols-x)
	copy(row[x+n:], row[x:t.cols-n])
	for i := x; i < x+n; i++ {
		row[i] = blank(t.cur.pen)
	}
	t.cur.wrapNext = false
	t.touch(t.cur.y)
}

func (t *Term) deleteChars(n int) {
	row := (*t.screen)[t.cur.y]
	x := t.cur.x
	n = min(n, t.cols-x)
	copy(row[x:], row[x+n:])
	for i := t.cols - n; i < t.cols; i++ {
		row[i] = blank(t.cur.pen)
	}
	t.cur.wrapNext = false
	t.touch(t.cur.y)
}

func (t *Term) decset(ps [][]int, on bool) {
	for i := range ps {
		switch param(ps, i, 0) {
		case 1:
			t.modes.AppCursor = on
		case 7:
			t.autowrap = on
		case 25:
			t.modes.CursorVisible = on
			t.seq++
		case 47, 1047:
			t.altScreen(on, false)
		case 1048:
			if on {
				t.saveCursor()
			} else {
				t.restoreCursor()
			}
		case 1049:
			t.altScreen(on, true)
		case 1000:
			t.setMouse(MouseClick, on)
		case 1002:
			t.setMouse(MouseDrag, on)
		case 1003:
			t.setMouse(MouseMotion, on)
		case 1006:
			t.modes.MouseSGR = on
		case 2004:
			t.modes.BracketedPaste = on
		}
	}
}

func (t *Term) setMouse(m MouseMode, on bool) {
	if on {
		t.modes.Mouse = m
	} else if t.modes.Mouse == m {
		t.modes.Mouse = MouseOff
	}
}

func (t *Term) altScreen(on, saveCursor bool) {
	if on == (t.screen == &t.alt) {
		return
	}
	if on {
		if saveCursor {
			t.savedMain = t.cur
		}
		t.alt = newGrid(t.cols, t.rows)
		t.screen = &t.alt
	} else {
		t.screen = &t.main
		if saveCursor {
			t.restoreCursor()
		}
	}
	t.top, t.bot = 0, t.rows-1
	t.touchAll()
}

func (t *Term) sgr(ps [][]int) {
	p := &t.cur.pen
	if len(ps) == 0 {
		ps = [][]int{{0}}
	}
	for i := 0; i < len(ps); i++ {
		g := ps[i]
		n := g[0]
		if n < 0 {
			n = 0
		}
		switch {
		case n == 0:
			p.FG, p.BG, p.Attr = DefaultColor, DefaultColor, 0
		case n == 1:
			p.Attr |= Bold
		case n == 2:
			p.Attr |= Faint
		case n == 3:
			p.Attr |= Italic
		case n == 4:
			if len(g) > 1 && g[1] == 0 {
				p.Attr &^= Underline
			} else {
				p.Attr |= Underline
			}
		case n == 5 || n == 6:
			p.Attr |= Blink
		case n == 7:
			p.Attr |= Inverse
		case n == 8:
			p.Attr |= Hidden
		case n == 9:
			p.Attr |= Strike
		case n == 21:
			p.Attr |= Underline
		case n == 22:
			p.Attr &^= Bold | Faint
		case n == 23:
			p.Attr &^= Italic
		case n == 24:
			p.Attr &^= Underline
		case n == 25:
			p.Attr &^= Blink
		case n == 27:
			p.Attr &^= Inverse
		case n == 28:
			p.Attr &^= Hidden
		case n == 29:
			p.Attr &^= Strike
		case n >= 30 && n <= 37:
			p.FG = Color(n - 30)
		case n == 39:
			p.FG = DefaultColor
		case n >= 40 && n <= 47:
			p.BG = Color(n - 40)
		case n == 49:
			p.BG = DefaultColor
		case n >= 90 && n <= 97:
			p.FG = Color(n - 90 + 8)
		case n >= 100 && n <= 107:
			p.BG = Color(n - 100 + 8)
		case n == 38 || n == 48 || n == 58:
			var col Color
			var ok bool
			if len(g) > 1 {
				col, ok = extColor(g[1:])
			} else {
				var used int
				col, ok, used = extColorSemi(ps[i+1:])
				i += used
			}
			if ok && n == 38 {
				p.FG = col
			} else if ok && n == 48 {
				p.BG = col
			}
		}
	}
}

// extColor llegeix 5:n o 2:[cs]:r:g:b (forma amb dos punts).
func extColor(g []int) (Color, bool) {
	if len(g) == 0 {
		return 0, false
	}
	switch g[0] {
	case 5:
		if len(g) >= 2 && g[1] >= 0 && g[1] <= 255 {
			return Color(g[1]), true
		}
	case 2:
		v := g[1:]
		if len(v) == 4 { // amb l'espai de color
			v = v[1:]
		}
		if len(v) >= 3 {
			return RGB(uint8(max(v[0], 0)), uint8(max(v[1], 0)), uint8(max(v[2], 0))), true
		}
	}
	return 0, false
}

// extColorSemi llegeix 5;n o 2;r;g;b (forma amb punts i coma) i diu
// quants paràmetres ha consumit.
func extColorSemi(rest [][]int) (Color, bool, int) {
	at := func(i int) int {
		if i < len(rest) && len(rest[i]) > 0 {
			return max(rest[i][0], 0)
		}
		return -1
	}
	switch at(0) {
	case 5:
		if n := at(1); n >= 0 && n <= 255 {
			return Color(n), true, 2
		}
		return 0, false, min(2, len(rest))
	case 2:
		r, g, b := at(1), at(2), at(3)
		if r >= 0 && g >= 0 && b >= 0 {
			return RGB(uint8(r), uint8(g), uint8(b)), true, 4
		}
		return 0, false, min(4, len(rest))
	}
	return 0, false, 0
}

func (t *Term) osc(s, term string) []byte {
	num, arg, _ := strings.Cut(s, ";")
	switch num {
	case "0", "2":
		if len(arg) > 200 {
			arg = arg[:200]
		}
		t.title = arg
		t.seq++
	case "10":
		if arg == "?" {
			return []byte("\x1b]10;rgb:d4d4/d4d4/d4d4" + term)
		}
	case "11":
		if arg == "?" {
			return []byte("\x1b]11;rgb:0f0f/1414/1a1a" + term)
		}
	}
	return nil
}
