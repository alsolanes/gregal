package tema

import (
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

// El markdown del terminal amb els colors de Gregal.
//
// El TUI feia servir `styles.DarkStyleConfig` tal qual: els títols sortien
// d'un blau que no és de la paleta, el codi en línia amb fons gris i els
// blocs amb els colors de chroma per defecte. I en tema clar no canviava
// res, perquè l'estil es triava una sola vegada.

func p(s string) *string { return &s }
func u(n uint) *uint     { return &n }
func b(v bool) *bool     { return &v }

// EstilGlamour és el StyleConfig d'aquest tema. Parteix del de glamour
// (que ja porta els prefixos, marges i sagnats ben posats) i en canvia
// els colors, que és l'única cosa que ha de ser nostra.
func EstilGlamour(t Tema) ansi.StyleConfig {
	st := styles.DarkStyleConfig
	if t.Clar {
		st = styles.LightStyleConfig
	}

	// Document sense marge: el TUI ja sagna la conversa pel seu compte, i
	// el marge de glamour hi sumava dues columnes que descentraven el text
	// respecte de les files d'eines.
	st.Document.Margin = u(0)
	st.Document.StylePrimitive.Color = p(t.Text)

	// Títols: arena com els de la interfície, i sense fons. El H1 de sèrie
	// ve amb fons de color i sortia com una etiqueta enmig de la resposta.
	st.Heading.StylePrimitive.Color = p(t.Arena)
	st.Heading.StylePrimitive.Bold = b(true)
	st.H1.StylePrimitive.Color = p(t.Arena)
	st.H1.StylePrimitive.BackgroundColor = nil
	st.H1.StylePrimitive.Bold = b(true)
	st.H1.Prefix = "≋ "
	st.H2.StylePrimitive.Color = p(t.Arena)
	st.H2.Prefix = "▸ "
	st.H3.StylePrimitive.Color = p(t.Onada)
	st.H3.Prefix = "· "
	st.H4.StylePrimitive.Color = p(t.Onada)
	st.H5.StylePrimitive.Color = p(t.Slate)
	st.H6.StylePrimitive.Color = p(t.Slate)

	st.Text.Color = p(t.Text)
	st.Paragraph.StylePrimitive.Color = p(t.Text)
	st.Strong.Color = p(t.Ink)
	st.Strong.Bold = b(true)
	st.Emph.Color = p(t.Text)
	st.Emph.Italic = b(true)

	// Cites i regles: el traç fi de la paleta.
	st.BlockQuote.StylePrimitive.Color = p(t.Slate)
	st.BlockQuote.IndentToken = p("│ ")
	st.HorizontalRule.Color = p(t.Algua)
	st.HorizontalRule.Format = "\n────────\n"

	// Llistes: el pic en escuma, el text normal. De sèrie tot el bloc
	// agafava el color del pic.
	st.List.StylePrimitive.Color = p(t.Text)
	st.Item.Color = p(t.Escuma)
	st.Enumeration.Color = p(t.Escuma)
	st.Task.Ticked = "✓ "
	st.Task.Unticked = "☐ "

	// Enllaços.
	st.Link.Color = p(t.Onada)
	st.Link.Underline = b(true)
	st.LinkText.Color = p(t.EscumaClara)

	// Codi en línia: escuma sobre el fons, sense el farciment de glamour
	// (amb l'espai davant i darrere, «el camp  Timeout  a  Config .» es
	// llegia com si faltessin lletres).
	st.Code.StylePrimitive.Color = p(t.EscumaClara)
	st.Code.StylePrimitive.BackgroundColor = nil
	st.Code.StylePrimitive.Prefix = ""
	st.Code.StylePrimitive.Suffix = ""

	// Blocs de codi: chroma amb la nostra paleta, i una vora esquerra
	// fina en comptes d'un fons gris a tota amplada.
	st.CodeBlock.Theme = NomEstilChroma(t)
	st.CodeBlock.Chroma = chromaGlamour(t)
	st.CodeBlock.StyleBlock.StylePrimitive.Color = p(t.Sintaxi.Nom)
	st.CodeBlock.StyleBlock.StylePrimitive.BackgroundColor = nil
	// Una barra a l'esquerra i prou: el bloc es distingeix del text sense
	// un fons gris a tota amplada, que en un terminal pesa molt.
	st.CodeBlock.Margin = u(1)
	st.CodeBlock.Indent = u(1)
	st.CodeBlock.IndentToken = p("┃ ")

	// Taules: les línies amb el traç fi, no amb el color del text.
	st.Table.StyleBlock.StylePrimitive.Color = p(t.Text)
	st.Table.CenterSeparator = p("┼")
	st.Table.ColumnSeparator = p("│")
	st.Table.RowSeparator = p("─")

	// L'estil ha d'estar registrat abans que glamour el busqui pel nom.
	EstilChroma(t)
	return st
}

// chromaGlamour és la mateixa paleta de sintaxi en el format que glamour
// fa servir per als blocs de codi.
func chromaGlamour(t Tema) *ansi.Chroma {
	s := t.Sintaxi
	col := func(c string) ansi.StylePrimitive { return ansi.StylePrimitive{Color: p(c)} }
	neg := func(c string) ansi.StylePrimitive { return ansi.StylePrimitive{Color: p(c), Bold: b(true)} }
	cur := func(c string) ansi.StylePrimitive { return ansi.StylePrimitive{Color: p(c), Italic: b(true)} }
	return &ansi.Chroma{
		Text:                col(s.Nom),
		Error:               col(t.Red),
		Comment:             cur(s.Comentari),
		CommentPreproc:      col(s.Paraula),
		Keyword:             neg(s.Paraula),
		KeywordReserved:     neg(s.Paraula),
		KeywordNamespace:    col(s.Paraula),
		KeywordType:         col(s.Tipus),
		Operator:            col(s.Operador),
		Punctuation:         col(s.Operador),
		Name:                col(s.Nom),
		NameBuiltin:         col(s.Tipus),
		NameTag:             col(s.Funcio),
		NameAttribute:       col(s.Tipus),
		NameClass:           neg(s.Tipus),
		NameConstant:        col(s.Numero),
		NameDecorator:       col(s.Funcio),
		NameException:       col(t.Red),
		NameFunction:        col(s.Funcio),
		NameOther:           col(s.Nom),
		Literal:             col(s.Numero),
		LiteralNumber:       col(s.Numero),
		LiteralDate:         col(s.Numero),
		LiteralString:       col(s.Cadena),
		LiteralStringEscape: col(s.Numero),
		GenericDeleted:      col(t.Red),
		GenericEmph:         ansi.StylePrimitive{Italic: b(true)},
		GenericInserted:     col(t.Green),
		GenericStrong:       ansi.StylePrimitive{Bold: b(true)},
		GenericSubheading:   col(t.Onada),
	}
}
