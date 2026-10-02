package tui

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Fitxers dorats: la vista sencera de sis pantalles, en dos temes i tres
// amplades, desada tal com es pinta (amb els codis de color). Un canvi
// d'estil que no es volia surt aquí abans que a la pantalla de ningú.
//
// Es regeneren amb `go test ./internal/tui -run TestPantallesDorades
// -update` (o GREGAL_GOLDEN_UPDATE=1) i es revisen al diff del commit.

var actualitzaDorats = flag.Bool("update", false, "reescriu els fitxers dorats de testdata/golden")

// comprovaGolden compara `got` amb testdata/golden/<nom>.golden. Els
// salts de línia es normalitzen: el repositori és CRLF a Windows.
func comprovaGolden(t *testing.T, nom, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", nom+".golden")
	got = strings.ReplaceAll(got, "\r\n", "\n")
	if *actualitzaDorats || os.Getenv("GREGAL_GOLDEN_UPDATE") != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("falta %s: regenera'l amb -update", path)
	}
	want := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if want == got {
		return
	}
	fw, fg := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(fw) || i < len(fg); i++ {
		var a, b string
		if i < len(fw) {
			a = fw[i]
		}
		if i < len(fg) {
			b = fg[i]
		}
		if a != b {
			t.Fatalf("%s: la fila %d no coincideix\n  dorat: %q\n  ara:   %q\n(si el canvi és volgut, -update)", nom, i+1, stripANSI(a), stripANSI(b))
		}
	}
	t.Fatalf("%s: difereix del dorat (mateixes files, codis diferents); si el canvi és volgut, -update", nom)
}

// Sis pantalles × dos temes × tres amplades, pintades amb colors de debò
// (el perfil es força: en un test no hi ha terminal i lipgloss ho
// pintaria tot pla).
func TestPantallesDorades(t *testing.T) {
	// Els goldens protegeixen també la paleta ANSI. NO_COLOR és una opció
	// legítima de l'aplicació, però si queda heretada de l'entorn del runner
	// SetTema força perfil ASCII després del TrueColor d'aquest test i fa que
	// tots els snapshots perdin els ANSI. El harness fixa explícitament el
	// contracte visual que està comprovant.
	t.Setenv("NO_COLOR", "")
	perfil := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(perfil)
		temaGolden = "fosc"
		SetTema("fosc")
	})
	for _, nomTema := range []string{"fosc", "clar"} {
		temaGolden = nomTema
		for _, ample := range []int{80, 120, 160} {
			for _, p := range pantalles {
				nom := fmt.Sprintf("%s-%s-%d", p.nom, nomTema, ample)
				t.Run(nom, func(t *testing.T) {
					m := modelGolden(t, ample, 40)
					p.munta(t, &m)
					v := m.View()
					if h := lipgloss.Height(v); h != 40 {
						t.Fatalf("la vista fa %d files, no 40", h)
					}
					comprovaGolden(t, nom, v)
				})
			}
		}
	}
}

// Res es mou en repòs: amb les animacions apagades (el defecte), dos
// View() seguits amb el comptador de frames avançat són idèntics, byte a
// byte. Una eina on es passen hores no pot tenir res que es mogui sol.
func TestResEsMouEnRepos(t *testing.T) {
	for _, ample := range []int{80, 120, 160} {
		m := modelGolden(t, ample, 40)
		omplePantallaTorn(t, &m)
		abans := m.View()
		m.spin += 7
		despres := m.View()
		if abans == despres {
			continue
		}
		fa, fd := strings.Split(abans, "\n"), strings.Split(despres, "\n")
		for i := range fa {
			if i < len(fd) && fa[i] != fd[i] {
				t.Fatalf("a %d columnes la fila %d es mou en repòs:\n%q\n%q", ample, i+1, stripANSI(fa[i]), stripANSI(fd[i]))
			}
		}
		t.Fatalf("a %d columnes la vista canvia en repòs", ample)
	}
}
