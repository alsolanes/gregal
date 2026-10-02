package procs

import (
	"strings"
	"testing"
	"time"
)

// Un procés curt acaba amb codi 0 i la seva sortida es pot llegir.
func TestStartISortida(t *testing.T) {
	s := New()
	p, err := s.Start("sess", t.TempDir(), "echo hola; echo adeu >&2")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Wait(5 * time.Second) {
		t.Fatal("no ha acabat")
	}
	lines, done, exit := p.Output(0)
	if !done || exit == nil || *exit != 0 {
		t.Fatalf("done=%v exit=%v", done, exit)
	}
	var out, errs []string
	for _, l := range lines {
		if l.Stream == "err" {
			errs = append(errs, l.Text)
			continue
		}
		out = append(out, l.Text)
	}
	if strings.Join(out, "") != "hola" || strings.Join(errs, "") != "adeu" {
		t.Fatalf("out=%v err=%v", out, errs)
	}
}

// Output(from) només dona les línies noves: així la UI fa polling barat.
func TestOutputIncremental(t *testing.T) {
	s := New()
	p, _ := s.Start("sess", t.TempDir(), "echo un; echo dos; echo tres")
	p.Wait(5 * time.Second)
	all, _, _ := p.Output(0)
	if len(all) != 3 {
		t.Fatalf("%d línies", len(all))
	}
	rest, _, _ := p.Output(all[0].N)
	if len(rest) != 2 || rest[0].Text != "dos" {
		t.Fatalf("%+v", rest)
	}
}

// Un codi de sortida diferent de zero es reporta tal qual.
func TestExitCode(t *testing.T) {
	s := New()
	p, _ := s.Start("sess", t.TempDir(), "exit 3")
	p.Wait(5 * time.Second)
	_, _, exit := p.Output(0)
	if exit == nil || *exit != 3 {
		t.Fatalf("exit=%v", exit)
	}
}

// Un procés llarg es pot matar i deixa de córrer.
func TestKill(t *testing.T) {
	s := New()
	p, _ := s.Start("sess", t.TempDir(), "sleep 30")
	if !p.IsRunning() {
		t.Fatal("hauria de córrer")
	}
	if !s.Kill(p.ID) {
		t.Fatal("kill hauria de trobar-lo")
	}
	if p.IsRunning() {
		t.Fatal("hauria d'estar mort")
	}
	if s.Kill("no-existeix") {
		t.Fatal("un id inventat no s'ha de poder matar")
	}
}

// Tancar una sessió mata els seus processos i no els de les altres.
func TestKillSession(t *testing.T) {
	s := New()
	a, _ := s.Start("a", t.TempDir(), "sleep 30")
	b, _ := s.Start("b", t.TempDir(), "sleep 30")
	if n := s.KillSession("a"); n != 1 {
		t.Fatalf("morts=%d", n)
	}
	a.Wait(3 * time.Second)
	if a.IsRunning() {
		t.Fatal("a hauria d'estar mort")
	}
	if !b.IsRunning() {
		t.Fatal("b no s'havia de tocar")
	}
	s.KillSession("b")
}

// La llista filtra per sessió i posa els nous primer.
func TestList(t *testing.T) {
	s := New()
	s.Start("a", t.TempDir(), "true")
	s.Start("b", t.TempDir(), "true")
	time.Sleep(120 * time.Millisecond)
	if got := s.List("a"); len(got) != 1 || got[0].Session != "a" {
		t.Fatalf("%+v", got)
	}
	if got := s.List(""); len(got) != 2 {
		t.Fatalf("%+v", got)
	}
}

// El búfer no creix sense límit.
func TestBuferCircular(t *testing.T) {
	s := New()
	p, _ := s.Start("s", t.TempDir(), "seq 1 6000")
	p.Wait(20 * time.Second)
	lines, _, _ := p.Output(0)
	if len(lines) > MaxLines {
		t.Fatalf("%d línies desades", len(lines))
	}
	if p.total != 6000 {
		t.Fatalf("total=%d", p.total)
	}
	if !strings.Contains(p.Tail(3), "6000") {
		t.Fatalf("la cua ha de tenir el final: %q", p.Tail(3))
	}
}
