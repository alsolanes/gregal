package tema

import (
	"fmt"
	"strconv"
	"strings"
)

// El :root del web, generat des de la mateixa paleta que el TUI.
//
// Els tokens CSS eren una tercera còpia dels colors, escrita a mà a
// index.html. Un test els comparava amb docs/identitat.md perquè no
// divergissin, i feia la seva feina: en pujar el contrast del gris més
// apagat del tema clar, el test va saltar de seguida. Però vigilar tres
// còpies no és el mateix que tenir-ne una: ara el bloc es genera aquí i
// el test només comprova que el fitxer el porti al dia.

// rgb parteix un hex "#RRGGBB" en els seus components decimals.
func rgb(hex string) (int, int, int) {
	h := strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(h) != 6 {
		return 0, 0, 0
	}
	v := func(s string) int {
		n, _ := strconv.ParseInt(s, 16, 32)
		return int(n)
	}
	return v(h[0:2]), v(h[2:4]), v(h[4:6])
}

// vel és un color de la paleta amb transparència, en el format curt que
// fa servir el CSS del web: rgba(127,212,193,.10).
func vel(hex string, alfa string) string {
	r, g, b := rgb(hex)
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", r, g, b, alfa)
}

// tint és el color amb què es fan els vels d'una superfície: blanc sobre
// el fosc, i el text fort sobre el clar. Escrits a mà amb blanc, al tema
// clar eren invisibles.
func tint(t Tema) string {
	if t.Clar {
		return t.Ink
	}
	return "#FFFFFF"
}

// CSSColors són els tokens de color d'un tema, en l'ordre en què
// s'escriuen al full.
func CSSColors(t Tema) []string {
	lineA, lineB := ".10", ".20"
	if t.Clar {
		// Sobre blanc, el 10% del fosc no es veu.
		lineA, lineB = ".14", ".28"
	}
	velA, velB := ".05", ".08"
	return []string{
		"--bg:" + t.Fons,
		"--sidebar:" + t.Abisme,
		"--panel:" + t.Abisme,
		"--surface:" + t.Night,
		"--surface-hi:" + t.SuperficieAlta,
		"--line:" + vel(t.Escuma, lineA),
		"--line-hi:" + vel(t.Escuma, lineB),
		"--ink:" + t.Ink,
		"--text:" + t.Text,
		"--muted:" + t.Slate,
		"--faint:" + t.Faint,
		"--accent:" + t.Escuma,
		"--accent-hi:" + t.EscumaClara,
		"--green:" + t.Green,
		"--amber:" + t.Arena,
		"--red:" + t.Red,
		"--onada:" + t.Onada,
		"--lila:" + t.Lila,
		"--lila-fons:" + t.LilaFons,
		"--hover:" + vel(tint(t), velA),
		"--hover-alt:" + vel(tint(t), velB),
		"--raised:" + vel(tint(t), ".02"),
		"--edge:" + vel(tint(t), ".12"),
		"--accent-soft:" + vel(t.Escuma, ".10"),
		"--accent-soft-hi:" + vel(t.Escuma, ".20"),
		"--ok-soft:" + vel(t.Green, ".12"),
		"--ombra:" + ombra(t),
		"--amber-soft:" + vel(t.Arena, ".14"),
		"--red-soft:" + vel(t.Red, ".12"),
		// Colors del codi: els mateixos que pinta chroma al TUI, perquè
		// el CSS en pugui fer servir algun (vores, fons dels blocs).
		"--codi-fons:" + t.Abisme,
		"--codi-vora:" + vel(t.Escuma, lineB),
	}
}

func ombra(t Tema) string {
	if t.Clar {
		return vel(t.Ink, ".16")
	}
	return "rgba(0,0,0,.45)"
}

// CSSRoot escriu els blocs :root (el fosc de base i un per tema amb
// data-theme) tal com van a index.html. La tipografia i la densitat no
// hi són: les governa la persona des de Preferències.
func CSSRoot() string {
	var b strings.Builder
	b.WriteString("/* GENERAT des d'internal/tema (go test ./internal/web -run TestCSSAlDia -update).\n")
	b.WriteString("   No l'editis a mà: canvia la paleta a internal/tema/tema.go. */\n")
	for i, t := range Tots() {
		attr := t.Nom
		if t.Nom == "fosc" {
			attr = ""
		} else if t.Nom == "clar" {
			attr = "light"
		}
		if attr == "" {
			b.WriteString(":root {\n")
		} else {
			b.WriteString(":root[data-theme=\"" + attr + "\"] {\n")
		}
		b.WriteString(fileta(CSSColors(t)))
		if i == 0 {
			b.WriteString("  --mono:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,\"Liberation Mono\",monospace;\n")
			b.WriteString("  /* Escala tipogràfica: cinc mides i prou, res per sota d'11.5px.\n")
			b.WriteString("     --fs és la del cos i la mou Preferències; la resta hi van lligades. */\n")
			b.WriteString("  --t1:11.5px; --t2:12.5px; --t3:14px; --t4:16px; --t5:26px;\n")
			b.WriteString("  --lh:1.55; --lh-titol:1.3; --llegible:720px;\n")
			b.WriteString("  --fs:14px; --gap:20px; --pad:24px;\n")
		}
		b.WriteString("}\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// fileta reparteix els tokens en línies de tres, que és com estaven
// escrits a mà i com es llegeix millor un diff.
func fileta(toks []string) string {
	var b strings.Builder
	for i := 0; i < len(toks); i += 3 {
		fi := i + 3
		if fi > len(toks) {
			fi = len(toks)
		}
		b.WriteString("  " + strings.Join(toks[i:fi], "; ") + ";\n")
	}
	return b.String()
}
