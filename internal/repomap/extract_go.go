package repomap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxSig és la llargada màxima d'una signatura (en runes).
const maxSig = 140

// extractGo llegeix un fitxer Go amb go/parser: funcions, mètodes (amb el
// receptor), tipus, constants i variables de nivell superior. Amb errors de
// sintaxi aprofita el que el parser hagi pogut llegir.
func extractGo(rel string, src []byte) []Symbol {
	fset := token.NewFileSet()
	f, _ := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
	if f == nil {
		return nil
	}
	tros := func(a, z token.Pos) string {
		if !a.IsValid() || !z.IsValid() {
			return ""
		}
		i, j := fset.Position(a).Offset, fset.Position(z).Offset
		if i < 0 || j > len(src) || i >= j {
			return ""
		}
		return curta(string(src[i:j]))
	}
	var out []Symbol
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Name == nil || d.Name.Name == "_" {
				continue
			}
			s := Symbol{Name: d.Name.Name, Kind: "func", Line: fset.Position(d.Name.Pos()).Line, Exported: d.Name.IsExported()}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				s.Kind = "method"
				s.Recv = nomReceptor(d.Recv.List[0].Type)
			}
			fi := d.Type.End()
			if d.Body != nil {
				fi = d.Body.Lbrace
			}
			s.Sig = tros(d.Pos(), fi)
			out = append(out, s)
		case *ast.GenDecl:
			for _, sp := range d.Specs {
				switch sp := sp.(type) {
				case *ast.TypeSpec:
					s := Symbol{Name: sp.Name.Name, Kind: "type", Line: fset.Position(sp.Name.Pos()).Line, Exported: sp.Name.IsExported()}
					switch sp.Type.(type) {
					case *ast.StructType:
						s.Kind = "struct"
						s.Sig = "type " + sp.Name.Name + " struct"
					case *ast.InterfaceType:
						s.Kind = "interface"
						s.Sig = "type " + sp.Name.Name + " interface"
					default:
						s.Sig = "type " + tros(sp.Pos(), sp.End())
					}
					out = append(out, s)
				case *ast.ValueSpec:
					kind := "var"
					if d.Tok == token.CONST {
						kind = "const"
					}
					sig := kind + " " + tros(sp.Pos(), sp.End())
					for _, n := range sp.Names {
						if n.Name == "_" {
							continue
						}
						out = append(out, Symbol{Name: n.Name, Kind: kind, Line: fset.Position(n.Pos()).Line, Exported: n.IsExported(), Sig: sig})
					}
				}
			}
		}
	}
	return out
}

// nomReceptor treu el nom del tipus del receptor: «*Policy» → «Policy»,
// «List[T]» → «List».
func nomReceptor(e ast.Expr) string {
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.IndexListExpr:
			e = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
		}
	}
}

// curta plega els espais i talla la signatura a maxSig runes.
func curta(s string) string {
	s = strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "{"))
	if utf8.RuneCountInString(s) > maxSig {
		r := []rune(s)
		s = string(r[:maxSig-1]) + "…"
	}
	return s
}
