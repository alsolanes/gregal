package config

import "testing"

// El paral·lel és el comportament per defecte; off el desactiva. Si el zero
// value (camp buit) no fos «sí», cada config vella hauria canviat de
// comportament en desplegar.
func TestScoutParallelPerDefecte(t *testing.T) {
	var c Config
	if !c.ScoutParallelOn() {
		t.Fatal("sense configurar, el scout ha de córrer en paral·lel")
	}
	for _, off := range []string{"off", "false", "no", "never", " OFF "} {
		c.Agent.ScoutParallel = off
		if c.ScoutParallelOn() {
			t.Fatalf("%q hauria de desactivar el paral·lel", off)
		}
	}
	for _, on := range []string{"on", "true", "yes", "always"} {
		c.Agent.ScoutParallel = on
		if !c.ScoutParallelOn() {
			t.Fatalf("%q hauria de mantenir el paral·lel", on)
		}
	}
	var nilCfg *Config
	if nilCfg.ScoutParallelOn() {
		t.Fatal("una config nil no ha d'activar el paral·lel")
	}
}
