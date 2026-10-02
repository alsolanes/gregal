package tema

import (
	"bytes"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// El codi pintat, als dos fronts, amb els mateixos colors.
//
// El TUI el rebia de l'estil per defecte de glamour (grisos d'una altra
// marca) i el web no el pintava gens: els blocs de codi de les respostes
// eren text pla. Chroma ja era dependència (glamour l'arrossega) i aquí
// se li dona la paleta de Gregal.

// entrades tradueix la paleta a entrades de chroma. Vuit categories i
// prou: chroma en distingeix desenes, però amb més de vuit colors un bloc
// de codi deixa de llegir-se.
func entrades(t Tema) chroma.StyleEntries {
	s := t.Sintaxi
	return chroma.StyleEntries{
		chroma.Background:          s.Nom + " bg:" + t.Abisme,
		chroma.Text:                s.Nom,
		chroma.Error:               t.Red,
		chroma.Comment:             s.Comentari + " italic",
		chroma.CommentPreproc:      s.Paraula,
		chroma.Keyword:             s.Paraula + " bold",
		chroma.KeywordConstant:     s.Numero,
		chroma.KeywordType:         s.Tipus,
		chroma.Operator:            s.Operador,
		chroma.Punctuation:         s.Operador,
		chroma.Name:                s.Nom,
		chroma.NameBuiltin:         s.Tipus,
		chroma.NameClass:           s.Tipus + " bold",
		chroma.NameFunction:        s.Funcio,
		chroma.NameTag:             s.Funcio,
		chroma.NameAttribute:       s.Tipus,
		chroma.NameConstant:        s.Numero,
		chroma.NameDecorator:       s.Funcio,
		chroma.LiteralString:       s.Cadena,
		chroma.LiteralStringEscape: s.Numero,
		chroma.LiteralNumber:       s.Numero,
		chroma.GenericInserted:     t.Green,
		chroma.GenericDeleted:      t.Red,
		chroma.GenericSubheading:   t.Onada,
		chroma.GenericStrong:       "bold",
		chroma.GenericEmph:         "italic",
	}
}

var (
	muEstils sync.Mutex
	estils   = map[string]*chroma.Style{}
)

// NomEstilChroma és el nom amb què es registra l'estil. Glamour el busca
// pel nom (el seu StyleCodeBlock.Theme és una cadena), no per valor.
func NomEstilChroma(t Tema) string { return "gregal-" + t.Nom }

// EstilChroma construeix (una vegada) l'estil de chroma d'aquest tema i el
// deixa al registre global perquè glamour el trobi pel nom.
func EstilChroma(t Tema) *chroma.Style {
	nom := NomEstilChroma(t)
	muEstils.Lock()
	defer muEstils.Unlock()
	if st, ok := estils[nom]; ok {
		return st
	}
	st, err := chroma.NewStyle(nom, entrades(t))
	if err != nil {
		return styles.Fallback
	}
	styles.Register(st)
	estils[nom] = st
	return st
}

// PintaCodiANSI pinta un bloc de codi amb escapades ANSI (TUI). Amb un
// llenguatge desconegut torna el text tal qual i diu que no l'ha pintat:
// val més codi sense color que codi mal pintat.
func PintaCodiANSI(t Tema, llenguatge, codi string) (string, bool) {
	it, ok := tokens(llenguatge, codi)
	if !ok {
		return codi, false
	}
	f := formatters.Get("terminal16m")
	if f == nil {
		return codi, false
	}
	var b bytes.Buffer
	if err := f.Format(&b, EstilChroma(t), it); err != nil {
		return codi, false
	}
	return b.String(), true
}

// htmlFmt pinta amb estils en línia i sense embolcall <pre>: el posa qui
// crida, amb les seves classes. En línia i no per classes perquè el CSS
// del web no té full de sintaxi i així el mateix HTML val per al tema
// que sigui en el moment de renderitzar.
var htmlFmt = chromahtml.New(
	chromahtml.WithClasses(false),
	chromahtml.PreventSurroundingPre(true),
)

// PintaCodiHTML pinta un bloc de codi com a HTML amb estils en línia.
func PintaCodiHTML(t Tema, llenguatge, codi string) (string, bool) {
	it, ok := tokens(llenguatge, codi)
	if !ok {
		return "", false
	}
	var b bytes.Buffer
	if err := htmlFmt.Format(&b, EstilChroma(t), it); err != nil {
		return "", false
	}
	return b.String(), true
}

// tokens analitza el codi amb el lexer del llenguatge donat.
func tokens(llenguatge, codi string) (chroma.Iterator, bool) {
	lx := lexerDe(llenguatge, codi)
	if lx == nil {
		return nil, false
	}
	it, err := lx.Tokenise(nil, codi)
	if err != nil {
		return nil, false
	}
	return it, true
}

// lexerDe tria el lexer pel nom del llenguatge; sense nom, l'endevina pel
// contingut. Torna nil quan no en surt cap de fiable.
func lexerDe(llenguatge, codi string) chroma.Lexer {
	l := strings.ToLower(strings.TrimSpace(llenguatge))
	if l != "" {
		if lx := lexers.Get(l); lx != nil && lx.Config().Name != "plaintext" {
			return chroma.Coalesce(lx)
		}
		// Un llenguatge escrit que no coneixem: no endevinis. El model hi
		// posa sovint «text», «sortida» o el nom d'una eina, i endevinar
		// hi acaba pintant paraules soltes de colors sense cap sentit.
		return nil
	}
	if lx := lexers.Analyse(codi); lx != nil && lx.Config().Name != "plaintext" {
		return chroma.Coalesce(lx)
	}
	return nil
}

// PintaFitxerANSI pinta codi triant el lexer pel nom del fitxer, que és
// el que se sap en un diff (no hi ha etiqueta de llenguatge). Torna el
// text tal qual i fals quan no coneix l'extensió.
func PintaFitxerANSI(t Tema, fitxer, codi string) (string, bool) {
	lx := lexers.Match(fitxer)
	if lx == nil || lx.Config().Name == "plaintext" {
		return codi, false
	}
	it, err := chroma.Coalesce(lx).Tokenise(nil, codi)
	if err != nil {
		return codi, false
	}
	f := formatters.Get("terminal16m")
	if f == nil {
		return codi, false
	}
	var b bytes.Buffer
	if err := f.Format(&b, EstilChroma(t), it); err != nil {
		return codi, false
	}
	return b.String(), true
}
