package web

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Un run que no dona senyals de vida s'ha de tallar: si el proveïdor penja,
// no podem esperar els 45 minuts.
func TestVigilaEstancTallaaSenseProgres(t *testing.T) {
	ctx, _, atura := vigilaEstanc(context.Background(), 80*time.Millisecond)
	defer atura()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("el gos guardià no ha tallat un run sense progrés")
	}
	if !errors.Is(context.Cause(ctx), errTeamEstancat) {
		t.Fatalf("el motiu hauria de ser estancament, i és %v", context.Cause(ctx))
	}
}

// Mentre arribin senyals de progrés, el run ha de poder durar molt més que el
// llindar d'estancament: una feina llarga i productiva no mor pel rellotge.
func TestVigilaEstancDeixaViureUnaFeinaLlarga(t *testing.T) {
	ctx, toc, atura := vigilaEstanc(context.Background(), 120*time.Millisecond)
	defer atura()
	fi := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(fi) {
		toc()
		time.Sleep(20 * time.Millisecond)
	}
	if ctx.Err() != nil {
		t.Fatalf("no havia de tallar-se mentre hi ha progrés: %v", context.Cause(ctx))
	}
}

func TestTeamFailureReasonClassifica(t *testing.T) {
	estancat, cancel := context.WithCancelCause(context.Background())
	cancel(errTeamEstancat)
	if got := teamFailureReason(nil, estancat); got != "stalled" {
		t.Fatalf("estancat: got %q", got)
	}
	vencut, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel2()
	if got := teamFailureReason(context.DeadlineExceeded, vencut); got != "timeout" {
		t.Fatalf("temps: got %q", got)
	}
	if got := teamFailureReason(context.Canceled, context.Background()); got != "" {
		t.Fatalf("altres: got %q", got)
	}
	// Un estancament és una fallada, mai una cancel·lació de l'usuari.
	if got := teamErrorKind(context.Canceled, estancat); got != "failed" {
		t.Fatalf("estancat s'ha de reportar com a failed: got %q", got)
	}
}

func TestCrewPotReprendreNomesTempsOEstancament(t *testing.T) {
	inici := time.Now()
	if !crewPotReprendre(errConst("timeout"), inici) {
		t.Fatal("un temps esgotat s'ha de poder reprendre")
	}
	if !crewPotReprendre(errConst("stalled"), inici) {
		t.Fatal("un model encallat s'ha de poder reprendre")
	}
	if crewPotReprendre(errConst("failed"), inici) {
		t.Fatal("una fallada qualsevol no s'ha de reintentar a cegues")
	}
	if crewPotReprendre(nil, inici) {
		t.Fatal("sense error no hi ha res a reprendre")
	}
	if crewPotReprendre(errConst("timeout"), time.Now().Add(-2*teamRunTimeout)) {
		t.Fatal("fora de pressupost no s'ha de reintentar")
	}
}

func TestFitxersTocatsTrobaLaFeinaFeta(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Els sistemes de fitxers del contenidor poden arrodonir la data al segon:
	// es fixa explícitament què és vell i què és nou.
	antic := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "src", "a.txt"), antic, antic); err != nil {
		t.Fatal(err)
	}
	vell := filepath.Join(dir, "vell.txt")
	if err := os.WriteFile(vell, []byte("v"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(vell, antic, antic); err != nil {
		t.Fatal(err)
	}
	inici := time.Now().Add(-time.Minute)
	if err := os.WriteFile(filepath.Join(dir, "src", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := fitxersTocats(dir, inici)
	if len(got) != 1 || got[0] != filepath.Join("src", "b.txt") {
		t.Fatalf("fitxers tocats = %v", got)
	}
	if fitxersTocats("", inici) != nil {
		t.Fatal("sense workspace no hi ha res a llistar")
	}
}

// El missatge d'una feina que no s'acaba ha de dir què ha passat i què pot fer
// l'usuari: un «no s'ha pogut» a seques no serveix de res.
func TestCrewFailureMessageExplicaIPlantejaSortides(t *testing.T) {
	ca := func(caText, _ string) string { return caText }
	for _, motiu := range []string{"timeout", "stalled", "failed"} {
		msg := crewFailureMessage(ca, errConst(motiu), 12*time.Minute, []string{"src/a.kt"})
		if !strings.Contains(msg, "src/a.kt") {
			t.Fatalf("%s: el missatge ha de dir quina feina s'ha fet: %q", motiu, msg)
		}
		if !strings.Contains(msg, "tornar a engegar la colla") {
			t.Fatalf("%s: el missatge ha de dir què pot fer l'usuari: %q", motiu, msg)
		}
	}
	larg := crewFailureMessage(ca, errConst("timeout"), 47*time.Minute, nil)
	if !strings.Contains(larg, "47") {
		t.Fatalf("el temps ha de sortir al missatge: %q", larg)
	}
}
