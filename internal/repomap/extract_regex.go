package repomap

import (
	"regexp"
	"strings"
)

// Extracció per expressions regulars, línia a línia. Només declaracions de
// nivell superior (columna 0, o poc sagnades on el llenguatge les sagna:
// classes dins d'un namespace de C#) i els mètodes on és fàcil saber de qui
// són (Python dins d'una classe, Rust dins d'un impl). No és un parser: un
// «class Foo» dins d'un comentari de bloc pot colar-se, i a canvi no cal
// cap dependència.

type patro struct {
	re   *regexp.Regexp
	kind string // "" = el diu el grup 1 (paraula clau)
	nom  int    // grup amb el nom
}

const jsNom = `([A-Za-z_$][\w$]*)`

var patrons = map[string][]patro{
	"js": {
		{regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s*` + jsNom), "func", 1},
		{regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:abstract\s+)?class\s+` + jsNom), "class", 1},
		{regexp.MustCompile(`^(?:export\s+)?(?:declare\s+)?interface\s+` + jsNom), "interface", 1},
		{regexp.MustCompile(`^(?:export\s+)?(?:declare\s+)?type\s+` + jsNom + `\s*(?:<[^=]*>)?\s*=`), "type", 1},
		{regexp.MustCompile(`^(?:export\s+)?(?:declare\s+)?(?:const\s+)?enum\s+` + jsNom), "enum", 1},
		{regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+` + jsNom + `\s*(?::[^=]+)?=\s*(?:async\s+)?(?:function\b|\([^)]*\)\s*(?::[^=]+)?=>|[A-Za-z_$][\w$]*\s*=>)`), "func", 1},
		{regexp.MustCompile(`^export\s+(?:const|let|var)\s+` + jsNom), "var", 1},
	},
	"python": {
		{regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(`), "func", 1},
		{regexp.MustCompile(`^class\s+(\w+)`), "class", 1},
	},
	"java": {
		{regexp.MustCompile(`^\s{0,4}(?:@\w+(?:\([^)]*\))?\s+)*(?:(?:public|protected|private|abstract|final|static|sealed|non-sealed|strictfp)\s+)*(class|interface|enum|record|@interface)\s+(\w+)`), "", 2},
	},
	"csharp": {
		{regexp.MustCompile(`^\s{0,8}(?:\[[^\]]*\]\s*)*(?:(?:public|private|protected|internal|static|abstract|sealed|partial|readonly|ref|unsafe|new|file)\s+)*(class|interface|enum|struct|record)\s+(?:(?:class|struct)\s+)?(\w+)`), "", 2},
	},
	"rust": {
		{regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?(?:const\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern\s+"[^"]*"\s+)?fn\s+(\w+)`), "func", 1},
		{regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?struct\s+(\w+)`), "struct", 1},
		{regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?enum\s+(\w+)`), "enum", 1},
		{regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?(?:unsafe\s+)?trait\s+(\w+)`), "trait", 1},
		{regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?type\s+(\w+)`), "type", 1},
		{regexp.MustCompile(`^(?:pub(?:\([^)]*\))?\s+)?(?:const|static)\s+(?:mut\s+)?(\w+)\s*:`), "const", 1},
	},
}

var (
	rePyMetode   = regexp.MustCompile(`^(?: {2}| {4}|\t)(?:async\s+)?def\s+(\w+)\s*\(`)
	reRustImpl   = regexp.MustCompile(`^(?:unsafe\s+)?impl\b(?:\s*<[^{]*?>)?\s+(.+?)\s*(?:\bwhere\b.*)?\{?\s*$`)
	reRustMetode = regexp.MustCompile(`^(?: {4}|\t)(?:pub(?:\([^)]*\))?\s+)?(?:const\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern\s+"[^"]*"\s+)?fn\s+(\w+)`)
	reRustTipus  = regexp.MustCompile(`([A-Za-z_]\w*)\s*(?:<.*)?$`)
)

func extractRegex(lang string, src []byte) []Symbol {
	if lang == "ts" {
		lang = "js"
	}
	ps := patrons[lang]
	if ps == nil {
		return nil
	}
	var out []Symbol
	contenidor := "" // classe de Python o tipus de l'impl de Rust en curs
	enDoc := false   // dins d'una docstring de Python
	linies := strings.Split(string(src), "\n")
	for i, l := range linies {
		l = strings.TrimRight(l, "\r")
		if len(l) == 0 || len(l) > 1000 {
			continue
		}
		if lang == "python" {
			if n := strings.Count(l, `"""`) + strings.Count(l, `'''`); n%2 == 1 {
				enDoc = !enDoc
				continue
			}
			if enDoc {
				continue
			}
		}
		sagnat := l[0] == ' ' || l[0] == '\t'
		// Els mètodes: només dins del contenidor obert.
		if sagnat {
			if contenidor == "" {
				if lang != "java" && lang != "csharp" {
					continue
				}
			} else {
				var m []string
				switch lang {
				case "python":
					m = rePyMetode.FindStringSubmatch(l)
				case "rust":
					m = reRustMetode.FindStringSubmatch(l)
				}
				if m != nil {
					out = append(out, Symbol{Name: m[1], Kind: "method", Recv: contenidor, Line: i + 1,
						Exported: exportat(lang, l, m[1]), Sig: sigLinia(l)})
				}
				continue
			}
		} else if t := strings.TrimSpace(l); t != "}" && !strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "//") && !strings.HasPrefix(t, "@") {
			// Una línia nova a la columna 0 tanca la classe o l'impl.
			contenidor = ""
		}
		if lang == "rust" {
			if m := reRustImpl.FindStringSubmatch(l); m != nil {
				tipus := m[1]
				if k := strings.LastIndex(tipus, " for "); k >= 0 {
					tipus = tipus[k+5:]
				}
				nom := tipus
				if mm := reRustTipus.FindStringSubmatch(strings.TrimSpace(tipus)); mm != nil {
					nom = mm[1]
				}
				if j := strings.Index(nom, "<"); j >= 0 {
					nom = nom[:j]
				}
				contenidor = nom
				out = append(out, Symbol{Name: nom, Kind: "impl", Line: i + 1, Sig: sigLinia(l)})
				continue
			}
		}
		for _, p := range ps {
			m := p.re.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			kind := p.kind
			if kind == "" {
				kind = strings.TrimPrefix(m[1], "@")
			}
			s := Symbol{Name: m[p.nom], Kind: kind, Line: i + 1, Exported: exportat(lang, l, m[p.nom]), Sig: sigLinia(l)}
			out = append(out, s)
			if lang == "python" && kind == "class" {
				contenidor = s.Name
			}
			break
		}
	}
	return out
}

// exportat segueix la convenció de cada llenguatge.
func exportat(lang, linia, nom string) bool {
	t := strings.TrimSpace(linia)
	switch lang {
	case "js":
		return strings.HasPrefix(t, "export")
	case "python":
		return !strings.HasPrefix(nom, "_")
	case "java", "csharp":
		return strings.Contains(" "+t, " public ")
	case "rust":
		return strings.HasPrefix(t, "pub")
	}
	return false
}

// sigLinia és la línia de la declaració sense el cos.
func sigLinia(l string) string {
	t := strings.TrimSpace(l)
	if i := strings.Index(t, "{"); i > 0 {
		t = t[:i]
	}
	t = strings.TrimSuffix(strings.TrimSpace(t), ":")
	return curta(t)
}
