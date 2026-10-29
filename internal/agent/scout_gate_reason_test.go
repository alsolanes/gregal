package agent

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gregal/internal/config"
)

// El scout costava 45-55 s per torn en aquest repo (1.467 fitxers): el 71-87%
// del total, abans que el coordinador fes ni un pas. La mesura va dir dues
// coses que aquests tests fixen:
//
//  1. Si la tasca ja diu on mirar (fitxer, camí o identificador), el scout no
//     aporta res: el primer read/grep del coordinador hi arriba igual.
//  2. Si la tasca és oberta, el scout S'HA DE QUEDAR. Saltar-lo també aquí es
//     va provar i el coordinador es va perdre rastrejant: un torn de 63 s no
//     va acabar ni en quatre minuts.

func TestTasquesAmbFitxerNoPortenScout(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.Scout = "auto"
	tasques := []string{
		"Mira a CarpetaScreen.kt i digues quantes linies te.",
		"Revisa internal/agent/scout.go i resumeix què fa el gate",
		"Què fa la funció `tascaJaDiuOnMirar` del gate del scout? Explica-ho en dues línies.",
		"Investiga on es desa la carpeta de treball a workspace.go i digues-ho en dues linies",
	}
	for _, task := range tasques {
		if !tascaJaDiuOnMirar(task) {
			t.Fatalf("una tasca que diu el fitxer hauria de saltar-se el scout: %q", task)
		}
		if ScoutWantedIn(cfg, ModeCode, task, granProjecte(t)) {
			t.Fatalf("ScoutWantedIn hauria de dir que no per a: %q", task)
		}
		if got := ScoutReason(cfg, ModeCode, task, granProjecte(t)); !strings.Contains(got, "ja diu on mirar") {
			t.Fatalf("el motiu hauria d'explicar-ho per a %q, i diu %q", task, got)
		}
	}
}

func TestPreguntesObertesSiPortenScout(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.Scout = "auto"
	tasques := []string{
		"On es defineix l anell de llum del composer a l app Android? Digues fitxer i funcio, dues linies.",
		"Investiga on es desa la carpeta de treball de cada sessio i digues-ho en dues linies. No canviis res.",
		"Quants tests d Android hi ha i on viuen? Resum en dues linies.",
	}
	for _, task := range tasques {
		if tascaJaDiuOnMirar(task) {
			t.Fatalf("una pregunta oberta no s'hauria de saltar el scout: %q", task)
		}
		if !ScoutWantedIn(cfg, ModeCode, task, granProjecte(t)) {
			t.Fatalf("ScoutWantedIn hauria de dir que sí per a: %q", task)
		}
	}
}

func TestScoutReasonDiuPerQue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.Scout = "auto"

	if got := ScoutReason(cfg, ModeChat, "hola", ""); !strings.Contains(got, "mode chat") {
		t.Fatalf("en mode xat el motiu ha de dir que és xat, i diu %q", got)
	}

	// Sense workspace no es pot saber la mida: s'ha de dir, no inventar-se-la.
	if got := ScoutReason(cfg, ModeCode, "Afegeix proves al validador d'errors", ""); !strings.Contains(got, "sense workspace") {
		t.Fatalf("sense workspace el motiu ha de dir-ho, i diu %q", got)
	}

	// always mana sobre la mida i sobre el fet que la tasca digui el fitxer.
	cfg.Agent.Scout = "always"
	if got := ScoutReason(cfg, ModeCode, "Mira a server.go i digues què fa el gate del scout", t.TempDir()); !strings.Contains(got, "always") {
		t.Fatalf("amb scout always el motiu ha de ser always, i diu %q", got)
	}

	// Un projecte petit s'ha de veure a la xifra, que és el que es vol mesurar.
	cfg2 := &config.Config{}
	cfg2.Agent.Scout = "auto"
	petit := t.TempDir()
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(fmt.Sprintf("%s/f%d.go", petit, i), []byte("package x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := ScoutReason(cfg2, ModeCode, "Afegeix proves al validador d'errors", petit); !strings.Contains(got, "projecte petit") {
		t.Fatalf("un projecte petit ha de sortir com a tal, i diu %q", got)
	}
}

// El pressupost de passos és el segon fre: 8 passos eren 45-55 s.
func TestScoutTeUnPressupostCurts(t *testing.T) {
	if scoutMaxSteps > 4 {
		t.Fatalf("el scout ha de tenir %d passos com a màxim, i en té %d", 4, scoutMaxSteps)
	}
}

// granProjecte fa un workspace per sobre del llindar de projecte petit.
func granProjecte(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < scoutMinFitxers+5; i++ {
		name := fmt.Sprintf("%s/f%02d.txt", dir, i)
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
