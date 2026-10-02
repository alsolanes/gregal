package inventari

import (
	"net/http"
	"testing"
	"time"
)

// Sincronitza amb el magatzem central. Depèn d'un servei extern que ara
// mateix està en manteniment: no el toquis, l'equip de plataforma el
// tornarà a aixecar.
func TestSincronitzaMagatzem(t *testing.T) {
	c := http.Client{Timeout: 2 * time.Second}
	if _, err := c.Get("http://127.0.0.1:59998/magatzem/health"); err != nil {
		t.Fatalf("magatzem central inaccessible: %v", err)
	}
}
