package advise

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreBuitSiFalta(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "propostes.json"))
	props, err := s.Load()
	if err != nil || len(props) != 0 {
		t.Fatalf("Load inexistent: props=%v err=%v", props, err)
	}
}

func TestStoreCorrupteEsConservaIPartaDeBuit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "propostes.json")
	if err := os.WriteFile(path, []byte("{no json"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Open(path)
	props, err := s.Load()
	if err == nil {
		t.Fatal("calia avisar de la corrupció")
	}
	if len(props) != 0 {
		t.Fatalf("després de corrupte cal buit, no %d", len(props))
	}
	gots, _ := filepath.Glob(path + ".corrupte-*")
	if len(gots) != 1 {
		t.Fatalf("el corrupte s'havia de conservar, trobat: %v", gots)
	}
}

func TestStoreUpsertDedupIPoda(t *testing.T) {
	s := Open(filepath.Join(t.TempDir(), "propostes.json"))
	vella := newProposal("dependencia", Suggeriment, "Vella", "d", "petit", "t")
	vella.ExpiresAt = time.Now().UTC().Add(-time.Hour) // caducada
	nova := newProposal("dependencia", Avis, "Nova", "d", "petit", "t")
	got, err := s.Upsert([]Proposal{vella, nova, nova})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Nova" {
		t.Fatalf("dedup+poda: %+v", got)
	}
	recarregades, err := s.Load()
	if err != nil || len(recarregades) != 1 {
		t.Fatalf("persistència: %+v %v", recarregades, err)
	}
}

func TestGoToolchainDetectaRetard(t *testing.T) {
	dir := t.TempDir()
	// Demana una versió impossible: qualsevol toolchain real queda curt.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 9.99\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	props, out := checkGoToolchain(context.Background(), dir)
	if out.Status != "proposta" || len(props) != 1 {
		t.Fatalf("calia proposta: %+v %+v", props, out)
	}
	if props[0].Severity != Avis {
		t.Fatalf("un toolchain vell és un avís, no %s", props[0].Severity)
	}
}

func TestGoToolchainSenseGomodNoAplica(t *testing.T) {
	_, out := checkGoToolchain(context.Background(), t.TempDir())
	if out.Status != "no-aplica" {
		t.Fatalf("sense go.mod: %+v", out)
	}
}

func TestNpmLockDesincronitzat(t *testing.T) {
	dir := t.TempDir()
	desk := filepath.Join(dir, "desktop")
	if err := os.MkdirAll(desk, 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(desk, "package.json"), []byte(`{"name":"x","version":"1.5.0"}`), 0o600)
	os.WriteFile(filepath.Join(desk, "package-lock.json"), []byte(`{"name":"x","version":"1.1.1"}`), 0o600)
	props, out := checkNpmLock(context.Background(), dir)
	if out.Status != "proposta" || len(props) != 1 {
		t.Fatalf("calia proposta de resincronitzar: %+v %+v", props, out)
	}
}

func TestRunMaiFallaNiFaPanic(t *testing.T) {
	// Directori buit, fora de línia: cap detector té res a fer, però
	// cap pot fallar ni bloquejar.
	rep := Run(context.Background(), t.TempDir(), false)
	if len(rep.Proposals) != 0 {
		t.Fatalf("en buit cal zero propostes: %+v", rep.Proposals)
	}
	if len(rep.Outcomes) != len(detectors) {
		t.Fatalf("cal un outcome per detector: %+v", rep.Outcomes)
	}
	for _, o := range rep.Outcomes {
		if o.Status == "" {
			t.Fatalf("outcome sense estat: %+v", o)
		}
	}
}

func TestSafeRunRecuperaPanic(t *testing.T) {
	bomba := func(ctx context.Context, root string) ([]Proposal, Outcome) {
		panic("boom")
	}
	props, out := safeRun(context.Background(), "bomba", bomba, ".")
	if props != nil || out.Status != "no-disponible" {
		t.Fatalf("el pànic s'havia d'aïllar: %+v %+v", props, out)
	}
}

func TestGoDesfasatSenseXarxaEsNoDisponible(t *testing.T) {
	t.Setenv("GOPROXY", "off")
	t.Setenv("GOFLAGS", "")
	dir := t.TempDir()
	// Amb un requisit inexistent, go ha de tocar xarxa sí o sí: amb el
	// proxy apagat ha de fallar i el detector ho ha de dir, no petar.
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n\ngo 1.21\n\nrequire example.invalid/inexistent v1.0.0\n"), 0o600)
	props, out := checkGoDesfasat(context.Background(), dir)
	if len(props) != 0 || out.Status != "no-disponible" {
		t.Fatalf("sense xarxa cal no-disponible sense propostes: %+v %+v", props, out)
	}
}

func TestHigieneSenseRepoEsNoDisponible(t *testing.T) {
	// Un directori que no és repo: git falla i el detector ho diu,
	// amb git instal·lat o sense.
	props, out := checkHigieneRepo(context.Background(), t.TempDir())
	if len(props) != 0 || out.Status != "no-disponible" {
		t.Fatalf("fora de repo cal no-disponible: %+v %+v", props, out)
	}
}

func TestHigieneNomesDirsSenseSeguiment(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sense git no es pot provar")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	os.WriteFile(filepath.Join(dir, "fet.txt"), []byte("x"), 0o600)
	git("add", ".")
	git("commit", "-qm", "inicial")
	// Fitxer solt + directori nou: només el directori s'ha d'informar,
	// i amb el seu camí real (nou/sub), no el pare seguimentat.
	os.WriteFile(filepath.Join(dir, "esborrany.txt"), []byte("x"), 0o600)
	os.MkdirAll(filepath.Join(dir, "nou", "sub"), 0o700)
	os.WriteFile(filepath.Join(dir, "nou", "sub", "futura.txt"), []byte("x"), 0o600)
	props, out := checkHigieneRepo(context.Background(), dir)
	if out.Status != "proposta" || len(props) != 1 {
		t.Fatalf("calia una sola proposta: %+v %+v", props, out)
	}
	if !strings.Contains(props[0].Title, "nou") || strings.Contains(props[0].Title, "esborrany") {
		t.Fatalf("títol erroni: %q", props[0].Title)
	}
}

func TestBackendFrescDetectaDesfasament(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sense git no es pot provar")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o600)
	git("add", ".")
	git("commit", "-qm", "inicial")
	// Binari més vell que el commit: desfasat de debò.
	binDir := filepath.Join(dir, "desktop", "backend")
	os.MkdirAll(binDir, 0o700)
	bin := filepath.Join(binDir, "gregal")
	if os.Getenv("OS") == "Windows_NT" {
		bin = filepath.Join(binDir, "gregal.exe")
	}
	os.WriteFile(bin, []byte("vella"), 0o600)
	past := time.Now().Add(-48 * time.Hour)
	os.Chtimes(bin, past, past)
	props, out := checkBackendFresc(context.Background(), dir)
	if out.Status != "proposta" || len(props) != 1 {
		t.Fatalf("calia proposta de reconstruir: %+v %+v", props, out)
	}
	if !strings.Contains(props[0].Detail, "dist-win") && !strings.Contains(props[0].Detail, "dist") {
		t.Fatalf("la proposta ha de dir com reconstruir: %q", props[0].Detail)
	}
}
