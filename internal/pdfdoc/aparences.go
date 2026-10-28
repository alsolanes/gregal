package pdfdoc

// Aparences dels camps de text amb accents.
//
// pdfcpu v0.16.3 dibuixa malament els camps MULTILÍNIA amb una font
// estàndard (Helvetica…): textFieldLines passa el text a bytes Latin-1 i
// després wrap() el recorre com a UTF-8 («for _, char := range s»), de
// manera que cada byte accentuat es torna U+FFFD i acaba escrit com un
// espai. El valor (/V) és bo, però el que es veu és «Energ tica» (vist el
// 2026-10-28 omplint una sol·licitud en català: el model va gastar dotze
// minuts intentant redibuixar-ho amb python).
//
// Arreglem el que pdfcpu ha escrit: per a cada widget amb un valor que no
// és ASCII, es recorren les cadenes «(…) Tj» de l'aparença alineant-les amb
// el valor en bytes WinAnsi, i on el valor té un caràcter accentuat i la
// cadena un espai, s'hi torna a posar. La línia s'escriu amb una font
// Helvetica (la mateixa base) amb WinAnsiEncoding, perquè els bytes es
// llegeixin com a Latin-1 sigui quina sigui la codificació de la font del
// formulari. Si l'alineació no quadra (un camp «comb», un text que pdfcpu
// ha retallat diferent), l'aparença es deixa tal com era.

import (
	"bytes"
	"context"
	"fmt"
	"regexp"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	pfont "github.com/pdfcpu/pdfcpu/pkg/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

const fontAparenca = "GregalWinAnsi"

// arreglaAparences torna el PDF amb les aparences corregides i quants
// camps ha tocat (0 = el mateix PDF).
func arreglaAparences(c context.Context, pdf []byte) ([]byte, int, error) {
	ctx, err := api.ReadContext(c, bytes.NewReader(pdf), conf())
	if err != nil {
		return pdf, 0, err
	}
	xt := ctx.XRefTable
	fets := 0
	fonts := map[string]*types.IndirectRef{}
	for nr, e := range xt.Table {
		if e == nil || e.Free {
			continue
		}
		// Els objectes dins d'object streams no es carreguen fins que
		// algú els desreferencia.
		o, err := xt.Dereference(*types.NewIndirectRef(nr, 0))
		if err != nil {
			continue
		}
		d, ok := o.(types.Dict)
		if !ok {
			continue
		}
		if st := d.NameEntry("Subtype"); st == nil || *st != "Widget" {
			continue
		}
		v := valorCamp(xt, d)
		if v == "" || esASCII(v) {
			continue
		}
		ap, err := xt.DereferenceDict(d["AP"])
		if err != nil || ap == nil {
			continue
		}
		ir, ok := ap["N"].(types.IndirectRef)
		if !ok {
			continue
		}
		sd, _, err := xt.DereferenceStreamDict(ir)
		if err != nil || sd == nil {
			continue
		}
		if err := sd.Decode(); err != nil {
			continue
		}
		nou, ok := corregeixTj(sd.Content, []byte(model.DecodeUTF8ToByte(v)))
		if !ok {
			continue
		}
		base, ok := fontDeTf(xt, sd, nou)
		if !ok {
			continue
		}
		fref := fonts[base]
		if fref == nil {
			fref, err = xt.IndRefForNewObject(types.Dict(map[string]types.Object{
				"Type": types.Name("Font"), "Subtype": types.Name("Type1"),
				"BaseFont": types.Name(base), "Encoding": types.Name("WinAnsiEncoding"),
			}))
			if err != nil {
				return pdf, 0, err
			}
			fonts[base] = fref
		}
		res, err := xt.DereferenceDict(sd.Dict["Resources"])
		if err != nil || res == nil {
			continue
		}
		fd, err := xt.DereferenceDict(res["Font"])
		if err != nil || fd == nil {
			continue
		}
		fd[fontAparenca] = *fref
		sd.Content = reTf.ReplaceAll(nou, []byte("/"+fontAparenca+" $2 Tf"))
		if err := sd.Encode(); err != nil {
			continue
		}
		xt.Table[ir.ObjectNumber.Value()].Object = *sd
		fets++
	}
	if fets == 0 {
		return pdf, 0, nil
	}
	var out bytes.Buffer
	if err := api.WriteContext(c, ctx, &out); err != nil {
		return pdf, 0, err
	}
	return out.Bytes(), fets, nil
}

var reTf = regexp.MustCompile(`/([A-Za-z0-9_.+-]+)\s+([0-9.]+)\s+Tf`)

// fontDeTf diu la font base (una de les 14 estàndard) de l'aparença, a
// partir del primer «/F n Tf». Només aquestes passen pel camí trencat.
func fontDeTf(xt *model.XRefTable, sd *types.StreamDict, content []byte) (string, bool) {
	m := reTf.FindSubmatch(content)
	if m == nil {
		return "", false
	}
	res, err := xt.DereferenceDict(sd.Dict["Resources"])
	if err != nil || res == nil {
		return "", false
	}
	fd, err := xt.DereferenceDict(res["Font"])
	if err != nil || fd == nil {
		return "", false
	}
	f, err := xt.DereferenceDict(fd[string(m[1])])
	if err != nil || f == nil {
		return "", false
	}
	bf := f.NameEntry("BaseFont")
	if bf == nil || !pfont.IsCoreFont(*bf) {
		return "", false
	}
	return *bf, true
}

// valorCamp és el /V del widget o del camp pare (text).
func valorCamp(xt *model.XRefTable, d types.Dict) string {
	for i := 0; d != nil && i < 8; i++ {
		if o, ok := d.Find("V"); ok {
			o, err := xt.Dereference(o)
			if err != nil {
				return ""
			}
			s, err := types.StringOrHexLiteral(o)
			if err != nil || s == nil {
				return ""
			}
			return *s
		}
		p, err := xt.DereferenceDict(d["Parent"])
		if err != nil {
			return ""
		}
		d = p
	}
	return ""
}

func esASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// corregeixTj alinea les cadenes «(…) Tj» amb el valor en bytes WinAnsi i
// torna a posar els caràcters accentuats que pdfcpu ha escrit com a espai.
// Torna false si no ha canviat res o si l'alineació no quadra.
func corregeixTj(content, valor []byte) ([]byte, bool) {
	var out bytes.Buffer
	i := 0 // posició al valor
	canviat := false
	k := 0
	for k < len(content) {
		if content[k] != '(' {
			out.WriteByte(content[k])
			k++
			continue
		}
		raw, fi, ok := literal(content, k)
		if !ok {
			return nil, false
		}
		s, err := types.Unescape(string(raw))
		if err != nil {
			return nil, false
		}
		// Els salts de línia del wrap es mengen espais del valor.
		// (Si la línia comença amb un accent, a la cadena hi ha un espai.)
		for len(s) > 0 && i+1 < len(valor) && valor[i] == ' ' && (s[0] != ' ' || valor[i+1] >= 0x80) {
			i++
		}
		for j := range s {
			if i >= len(valor) {
				return nil, false
			}
			switch {
			case s[j] == valor[i]:
			case s[j] == ' ' && valor[i] >= 0x80:
				s[j] = valor[i]
				canviat = true
			default:
				return nil, false
			}
			i++
		}
		e, err := types.Escape(string(s))
		if err != nil {
			return nil, false
		}
		fmt.Fprintf(&out, "(%s)", *e)
		k = fi
	}
	if !canviat {
		return nil, false
	}
	return out.Bytes(), true
}

// literal llegeix la cadena literal que comença a content[k] == '(' i torna
// el contingut cru (sense parèntesis) i la posició de després.
func literal(content []byte, k int) ([]byte, int, bool) {
	depth := 0
	for j := k; j < len(content); j++ {
		switch content[j] {
		case '\\':
			j++
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return content[k+1 : j], j + 1, true
			}
		}
	}
	return nil, 0, false
}
