package knowledge

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Piece és un fragment indexable amb la seva ubicació.
type Piece struct {
	Loc  string
	Text string
}

type chunkLine struct {
	text string
	no   int
}

// Chunk parteix les seccions en fragments d'uns size caràcters, amb
// overlap caràcters repetits del final de l'anterior perquè una idea
// partida per la meitat es trobi igualment. Talla preferentment en línies
// en blanc, després en salts de línia i, si una línia sola ja és massa
// llarga, entre paraules. Quan la secció té línies, la ubicació hi afegeix
// el rang («lines 12–40»).
func Chunk(secs []Section, size, overlap int) []Piece {
	if size < 200 {
		size = 200
	}
	if overlap < 0 || overlap >= size/2 {
		overlap = size / 8
	}
	var out []Piece
	for _, sec := range secs {
		lines := strings.Split(strings.TrimRight(sec.Text, "\n"), "\n")
		var all []chunkLine
		for i, l := range lines {
			no := 0
			if sec.Line > 0 {
				no = sec.Line + i
			}
			for _, part := range splitLong(l, size) {
				all = append(all, chunkLine{part, no})
			}
		}
		var buf []chunkLine
		bufLen := 0
		fresh := false // hi ha línies noves des del darrer tros emès
		emit := func() {
			if len(buf) == 0 || !fresh {
				return
			}
			fresh = false
			var sb strings.Builder
			for i, l := range buf {
				if i > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(l.text)
			}
			text := strings.TrimSpace(sb.String())
			if text == "" {
				return
			}
			loc := sec.Loc
			if sec.Line > 0 {
				first, last := buf[0].no, buf[len(buf)-1].no
				rng := fmt.Sprintf("lines %d–%d", first, last)
				if first == last {
					rng = fmt.Sprintf("line %d", first)
				}
				if loc != "" {
					loc += " · " + rng
				} else {
					loc = rng
				}
			}
			out = append(out, Piece{Loc: loc, Text: text})
		}
		for _, l := range all {
			blank := strings.TrimSpace(l.text) == ""
			if bufLen > 0 && fresh && (bufLen+len(l.text)+1 > size || (blank && bufLen > size*3/5)) {
				emit()
				// Solapament: les darreres línies de l'anterior.
				var keep []chunkLine
				kl := 0
				for i := len(buf) - 1; i >= 0 && overlap > 0; i-- {
					if kl+len(buf[i].text)+1 > overlap {
						break
					}
					keep = append([]chunkLine{buf[i]}, keep...)
					kl += len(buf[i].text) + 1
				}
				buf, bufLen = keep, kl
				if blank {
					continue
				}
			}
			if blank && bufLen == 0 {
				continue
			}
			buf = append(buf, l)
			bufLen += len(l.text) + 1
			if !blank {
				fresh = true
			}
		}
		emit()
	}
	return out
}

// splitLong parteix una línia més llarga que size entre paraules.
func splitLong(s string, size int) []string {
	if len(s) <= size {
		return []string{s}
	}
	var out []string
	var cur strings.Builder
	for _, w := range strings.Fields(s) {
		for len(w) > size {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
			cut := size
			for cut > 0 && !utf8.RuneStart(w[cut]) {
				cut--
			}
			if cut == 0 {
				cut = size
			}
			out = append(out, w[:cut])
			w = w[cut:]
		}
		if cur.Len() > 0 && cur.Len()+1+len(w) > size {
			out = append(out, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
