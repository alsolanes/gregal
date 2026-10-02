package tools

import (
	"strings"
	"testing"
	"time"
)

// «timeout (30s)» sol no diu què fer, i el que feia el model era tornar-hi
// igual i tornar-se a menjar els trenta segons. L'error ha de portar la
// sortida d'abans del tall i la sortida del pas següent.
func TestElTimeoutDiuQueFerDespres(t *testing.T) {
	out, err := BashIn("", "echo abans-del-tall; sleep 5", 900*time.Millisecond)
	if err == nil {
		t.Fatal("una comanda de 5s amb 900ms de marge ha de fer timeout")
	}
	if !strings.Contains(err.Error(), "bash_background") {
		t.Errorf("l'error ha de dir per on sortir-se'n:\n%v", err)
	}
	if !strings.Contains(out, "abans-del-tall") {
		t.Errorf("la sortida d'abans del tall s'ha de conservar: %q", out)
	}
}

// El topall eren 30s i es quedaven curts per a la primera cosa que fa
// qualsevol agent en un projecte: compilar. Mesurat en aquest repositori
// amb la cau buida, `go build ./...` triga 36s, o sigui que la primera
// compilació d'una sessió es tallava sempre.
func TestElTopallDeBashDeixaCompilar(t *testing.T) {
	if DefaultTimeout < 60*time.Second {
		t.Errorf("amb %s no hi cap ni un build en fred (36s mesurats)", DefaultTimeout)
	}
}
