package agent

import (
	"context"
	"testing"

	"gregal/internal/tools"
)

// Compactar una sessió oblida les seves lectures, no les de les altres
// sessions del mateix procés (abans OblidaTotsSeguiments les esborrava totes).
func TestOblidaLecturesNomesDeLaSessio(t *testing.T) {
	a, b := tools.SeguimentDe("sessio-a-oblida"), tools.SeguimentDe("sessio-b-oblida")
	oblidaLectures(WithSeguimentSessio(context.Background(), "sessio-a-oblida"))
	if tools.SeguimentDe("sessio-a-oblida") == a {
		t.Fatal("el seguiment de la sessió compactada s'ha d'oblidar")
	}
	if tools.SeguimentDe("sessio-b-oblida") != b {
		t.Fatal("el seguiment d'una altra sessió no es toca")
	}
}
