package knowledge

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"gregal/internal/tools"
)

// Section és un tros de document amb la seva ubicació: capçalera, pàgina,
// full, diapositiva... Line és la primera línia (1-based) quan el document
// és text pla, per poder dir «línies 12–40»; 0 si no en té.
type Section struct {
	Loc  string
	Text string
	Line int
}

var (
	// ErrSkip: el fitxer no és per indexar (binari, imatge...) i no cal
	// ni dir-ho a la llista de documents.
	ErrSkip = errors.New("not indexable")
	// ErrUnsupported: és un document però el format no se sap llegir.
	ErrUnsupported = errors.New("unsupported format")
)

var textExts = map[string]bool{
	".txt": true, ".text": true, ".rst": true, ".adoc": true, ".asciidoc": true, ".org": true, ".tex": true,
	".csv": true, ".tsv": true, ".json": true, ".jsonl": true, ".yaml": true, ".yml": true, ".toml": true,
	".ini": true, ".cfg": true, ".conf": true, ".xml": true, ".log": true, ".env.example": true, ".properties": true,
}

var codeExts = map[string]bool{
	".go": true, ".py": true, ".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".jsx": true,
	".java": true, ".kt": true, ".kts": true, ".c": true, ".h": true, ".cc": true, ".cpp": true, ".hpp": true,
	".cs": true, ".rs": true, ".rb": true, ".php": true, ".swift": true, ".scala": true, ".sh": true, ".bash": true,
	".zsh": true, ".ps1": true, ".bat": true, ".cmd": true, ".sql": true, ".lua": true, ".r": true, ".m": true,
	".vue": true, ".svelte": true, ".css": true, ".scss": true, ".less": true, ".dart": true, ".ex": true, ".exs": true,
	".erl": true, ".hs": true, ".clj": true, ".fs": true, ".vb": true, ".pl": true, ".gradle": true, ".proto": true,
	".graphql": true, ".tf": true, ".dockerfile": true, ".mk": true,
}

// unsupportedDocs són formats de document que la gent hi posarà i que no
// es llegeixen: surten a la llista com a «no suportat» en comptes de
// desaparèixer sense dir res.
var unsupportedDocs = map[string]bool{
	".doc": true, ".xls": true, ".ppt": true, ".rtf": true, ".odt": true, ".ods": true, ".odp": true,
	".epub": true, ".pages": true, ".numbers": true, ".key": true, ".msg": true, ".eml": true,
}

// skipExts no s'intenten llegir mai.
var skipExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".bmp": true, ".ico": true, ".svg": true,
	".tif": true, ".tiff": true, ".heic": true, ".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".mp4": true,
	".mov": true, ".avi": true, ".mkv": true, ".webm": true, ".zip": true, ".gz": true, ".tgz": true, ".7z": true,
	".rar": true, ".tar": true, ".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true, ".o": true,
	".a": true, ".class": true, ".jar": true, ".pyc": true, ".wasm": true, ".gob": true, ".db": true, ".sqlite": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".lock": true, ".sum": true, ".map": true,
}

// KindOf classifica un fitxer per l'extensió: markdown, text, code, html,
// pdf, docx, xlsx, pptx, unsupported o "" (desconegut: es mira el
// contingut).
func KindOf(path string) string {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case ext == ".md" || ext == ".markdown" || ext == ".mdx":
		return "markdown"
	case ext == ".html" || ext == ".htm" || ext == ".xhtml":
		return "html"
	case ext == ".pdf":
		return "pdf"
	case ext == ".docx" || ext == ".xlsx" || ext == ".pptx":
		return ext[1:]
	case unsupportedDocs[ext]:
		return "unsupported"
	case skipExts[ext]:
		return "skip"
	case textExts[ext]:
		return "text"
	case codeExts[ext]:
		return "code"
	case base == "dockerfile" || base == "makefile" || base == "readme" || base == "license" || base == "agents.md":
		return "text"
	}
	return ""
}

// Extract llegeix un fitxer i en torna el tipus i les seccions.
func Extract(path string) (string, []Section, error) {
	kind := KindOf(path)
	switch kind {
	case "skip":
		return kind, nil, ErrSkip
	case "unsupported":
		return kind, nil, fmt.Errorf("%w: %s", ErrUnsupported, strings.ToLower(filepath.Ext(path)))
	case "docx", "xlsx", "pptx":
		out, err := tools.OfficeReadFull(path)
		if err != nil {
			return kind, nil, err
		}
		return kind, officeSections(kind, out), nil
	case "pdf":
		raw, err := os.ReadFile(path)
		if err != nil {
			return kind, nil, err
		}
		secs, err := ExtractPDF(raw)
		return kind, secs, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return kind, nil, err
	}
	if kind == "" {
		// Desconegut: només si sembla text (UTF-8, sense NUL).
		probe := raw
		if len(probe) > 8192 {
			probe = probe[:8192]
		}
		if bytes.IndexByte(probe, 0) >= 0 || !utf8.Valid(trimPartialRune(probe)) {
			return kind, nil, ErrSkip
		}
		kind = "text"
	} else if bytes.IndexByte(firstN(raw, 8192), 0) >= 0 {
		return kind, nil, ErrSkip
	}
	text := normalizeText(raw)
	switch kind {
	case "html":
		doc := tools.ReadableHTML(text, nil)
		md := doc.Markdown
		if strings.TrimSpace(doc.Title) != "" && !strings.HasPrefix(strings.TrimSpace(md), "# ") {
			md = "# " + strings.TrimSpace(doc.Title) + "\n\n" + md
		}
		return kind, markdownSections(md, false), nil
	case "markdown":
		return kind, markdownSections(text, true), nil
	}
	return kind, []Section{{Text: text, Line: 1}}, nil
}

func firstN(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}
	return b
}

// trimPartialRune treu una seqüència UTF-8 tallada al final de la mostra.
func trimPartialRune(b []byte) []byte {
	for i := 0; i < 3 && len(b) > 0; i++ {
		r, size := utf8.DecodeLastRune(b)
		if r != utf8.RuneError || size != 1 {
			break
		}
		b = b[:len(b)-1]
	}
	return b
}

func normalizeText(raw []byte) string {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	s := string(raw)
	if !utf8.ValidString(s) {
		// Probablement Latin-1 / Windows-1252: cada byte és un caràcter.
		r := make([]rune, len(raw))
		for i, b := range raw {
			r[i] = rune(b)
		}
		s = string(r)
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

var mdHeading = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)

// markdownSections parteix per capçaleres (fora dels blocs de codi). La
// ubicació és el camí de capçaleres («Instal·lació › Windows»).
func markdownSections(text string, lines bool) []Section {
	var out []Section
	var stack []string
	var cur strings.Builder
	curLoc, curLine := "", 1
	inFence := false
	flush := func() {
		if strings.TrimSpace(cur.String()) != "" {
			s := Section{Loc: curLoc, Text: cur.String()}
			if lines {
				s.Line = curLine
			}
			out = append(out, s)
		}
		cur.Reset()
	}
	for i, ln := range strings.Split(text, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
		}
		if !inFence {
			if m := mdHeading.FindStringSubmatch(ln); m != nil {
				flush()
				level := len(m[1])
				for len(stack) >= level {
					stack = stack[:len(stack)-1]
				}
				for len(stack) < level-1 {
					stack = append(stack, "")
				}
				stack = append(stack, strings.TrimSpace(m[2]))
				var parts []string
				for _, p := range stack {
					if p != "" {
						parts = append(parts, p)
					}
				}
				curLoc, curLine = strings.Join(parts, " › "), i+1
			}
		}
		cur.WriteString(ln)
		cur.WriteByte('\n')
	}
	flush()
	return out
}

var (
	officeStyle   = regexp.MustCompile(`^\[([^\]\n]{1,48})\] (.*)$`)
	headingStyle  = regexp.MustCompile(`(?i)^(heading|title|subtitle|ttulo|t[ií]tulo|ttol|t[ií]tol|berschrift|titre|kop|rubrik)\s*\d*$`)
	slideMarker   = regexp.MustCompile(`^— Diapo (\d+) —$`)
	sheetMarker   = regexp.MustCompile(`^\[FULL (.*)\]$`)
	officeTrimmed = regexp.MustCompile(`^… \(`)
)

// officeSections converteix la sortida d'OfficeReadFull en seccions:
// diapositives, fulls o capçaleres de Word.
func officeSections(kind, text string) []Section {
	var out []Section
	var cur strings.Builder
	loc := ""
	flush := func() {
		if strings.TrimSpace(cur.String()) != "" {
			out = append(out, Section{Loc: loc, Text: cur.String()})
		}
		cur.Reset()
	}
	for _, ln := range strings.Split(text, "\n") {
		t := strings.TrimSpace(ln)
		if officeTrimmed.MatchString(t) {
			continue
		}
		switch kind {
		case "pptx":
			if m := slideMarker.FindStringSubmatch(t); m != nil {
				flush()
				loc = "slide " + m[1]
				continue
			}
		case "xlsx":
			if m := sheetMarker.FindStringSubmatch(t); m != nil {
				flush()
				loc = "sheet " + m[1]
				continue
			}
		case "docx":
			if m := officeStyle.FindStringSubmatch(t); m != nil {
				if headingStyle.MatchString(strings.TrimSpace(m[1])) {
					flush()
					loc = strings.TrimSpace(m[2])
				}
				t = m[2]
			}
		}
		cur.WriteString(t)
		cur.WriteByte('\n')
	}
	flush()
	return out
}
