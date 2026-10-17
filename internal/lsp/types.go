package lsp

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Position és 0-based, amb la columna en unitats UTF-16 (el defecte del
// protocol).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// URI d'un camí local (file:///C:/x a Windows, file:///x a la resta).
func URI(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// Path d'una URI file://.
func Path(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return strings.TrimPrefix(uri, "file://")
	}
	p := u.Path
	if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}

// normURI fa comparables les URI que envien servidors diferents (lletra
// d'unitat en minúscula, «%3A» en lloc de «:»).
func normURI(uri string) string {
	return strings.ToLower(filepath.ToSlash(Path(uri)))
}

// ColUTF16 converteix una columna en runes (0-based) a unitats UTF-16.
func ColUTF16(line string, runes int) int {
	n := 0
	for i, r := range []rune(line) {
		if i >= runes {
			break
		}
		n += len(utf16.Encode([]rune{r}))
	}
	return n
}

// ColRunes és la inversa: unitats UTF-16 → runes.
func ColRunes(line string, u16 int) int {
	n, runes := 0, 0
	for _, r := range line {
		if n >= u16 {
			break
		}
		n += len(utf16.Encode([]rune{r}))
		runes++
	}
	return runes
}

// offset és la posició en bytes d'una Position dins del text.
func offset(text string, p Position) int {
	off := 0
	for i := 0; i < p.Line; i++ {
		j := strings.IndexByte(text[off:], '\n')
		if j < 0 {
			return len(text)
		}
		off += j + 1
	}
	fi := strings.IndexByte(text[off:], '\n')
	linia := text[off:]
	if fi >= 0 {
		linia = text[off : off+fi]
	}
	linia = strings.TrimSuffix(linia, "\r")
	runes := ColRunes(linia, p.Character)
	b := 0
	for i := 0; i < runes && b < len(linia); i++ {
		_, w := utf8.DecodeRuneInString(linia[b:])
		b += w
	}
	return off + b
}
