package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// El TUI era fosc i prou. En un terminal de fons clar, els grisos apagats
// de la paleta fosca (faint és #87978B) queden pràcticament il·legibles
// sobre blanc, i l'escuma i l'arena tampoc no hi passen contrast.
//
// El tema clar no inventa cap color: tots surten de la taula «Paleta
// clara» de docs/identitat.md, que és la mateixa que fa servir la finestra.
func TestElTemaClarSurtDelDocument(t *testing.T) {
	doc := paletaDoc(t)
	t.Cleanup(func() { SetTema("fosc") })
	SetTema("clar")
	per := map[string]*lipgloss.Color{
		"escumaClar": &escuma, "arenaClar": &arena, "onadaClar": &onada,
		"fonsClar": &fons, "nightClar": &night, "slateClar": &slate,
		"greenClar": &green, "redClar": &red, "lilaClar": &lila,
		"lilaFonsClar": &lilaFons, "faintClar": &faint,
		"textClar": &textPrinc, "inkClar": &textFort,
	}
	for nom, c := range per {
		want, ok := doc[nom]
		if !ok {
			t.Errorf("%s no és a docs/identitat.md", nom)
			continue
		}
		if got := strings.ToUpper(string(*c)); got != want {
			t.Errorf("en tema clar, %s hauria de ser %s i és %s", nom, want, got)
		}
	}
}

// Canviar de tema ha de refer TOTS els estils. Si algun es quedés amb el
// valor de la declaració, en tema clar sortiria un color del fosc enmig
// —que és exactament l'error que ja he comès dues vegades: calcular una
// cosa en carregar el paquet i que no segueixi el canvi.
func TestCanviarDeTemaRefaTotsElsEstils(t *testing.T) {
	t.Cleanup(func() { SetTema("fosc") })
	SetTema("fosc")
	foscos := instantania()
	SetTema("clar")
	clars := instantania()
	for nom, f := range foscos {
		if clars[nom] == f {
			t.Errorf("%s no canvia amb el tema (%s als dos)", nom, f)
		}
	}
}

// instantania és el color de primer pla de cada estil que en té un de
// dependent del tema.
func instantania() map[string]string {
	e := map[string]lipgloss.Style{
		"logoStyle": logoStyle, "dimStyle": dimStyle, "faintStyle": faintStyle,
		"waveStyle": waveStyle, "userStyle": userStyle, "assistantStyle": assistantStyle,
		"systemStyle": systemStyle, "okStyle": okStyle, "badStyle": badStyle,
		"warnStyle": warnStyle, "roleStyle": roleStyle, "projectStyle": projectStyle,
		"toolRailStyle": toolRailStyle, "toolNameStyle": toolNameStyle,
		"composerGoal": composerGoal, "spinnerStyle": spinnerStyle,
		"addStyle": addStyle, "delStyle": delStyle, "hunkStyle": hunkStyle,
		"menuHL": menuHL, "userTextStyle": userTextStyle,
	}
	out := map[string]string{}
	for nom, st := range e {
		out[nom] = fmt.Sprint(st.GetForeground()) + "|" + fmt.Sprint(st.GetBackground())
	}
	return out
}

// I el tema per defecte continua sent el fosc: qui no en digui res no ha
// de notar cap canvi.
func TestPerDefecteElTemaEsFosc(t *testing.T) {
	t.Cleanup(func() { SetTema("fosc") })
	SetTema("clar")
	SetTema("qualsevol cosa que no sigui clar")
	if got := strings.ToUpper(string(fons)); got != "#171A19" {
		t.Errorf("el defecte ha de ser el fosc, i fons és %s", got)
	}
}
