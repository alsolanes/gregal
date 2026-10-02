package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func stubRunner(t *testing.T, fn func(ctx context.Context, prompt string) (string, error)) {
	t.Helper()
	SetDelegateRunner(fn)
	t.Cleanup(func() { SetDelegateRunner(nil) })
}

func TestDelegateFusiona(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		return "resum de " + prompt, nil
	})
	out, _, err := Exec("delegate", `{"tasks":[{"prompt":"pista A"},{"prompt":"pista B"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"== subagent 1:", "resum de pista A", "== subagent 2:", "resum de pista B"} {
		if !strings.Contains(out, want) {
			t.Fatalf("fusió sense %q:\n%s", want, out)
		}
	}
}

func TestDelegateValidacio(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) { return "x", nil })
	for _, args := range []string{
		`{"tasks":[]}`,
		`{"tasks":[{"prompt":"  "}]}`,
		`{"tasks":[{"prompt":"a"},{"prompt":"b"},{"prompt":"c"},{"prompt":"d"},{"prompt":"e"}]}`,
		`no-json`,
	} {
		if _, _, err := Exec("delegate", args); err == nil {
			t.Fatalf("hauria de rebutjar: %q", args)
		}
	}
}

func TestDelegateErrorNoTrenca(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		if prompt == "mala" {
			return "", fmt.Errorf("boom")
		}
		return "bé", nil
	})
	out, _, err := Exec("delegate", `{"tasks":[{"prompt":"bona"},{"prompt":"mala"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "bé") || !strings.Contains(out, "== subagent 2: ERROR: boom") {
		t.Fatalf("fusió amb error incorrecta:\n%s", out)
	}
}

func TestDelegateParalel(t *testing.T) {
	stubRunner(t, func(ctx context.Context, prompt string) (string, error) {
		time.Sleep(300 * time.Millisecond)
		return "fet", nil
	})
	t0 := time.Now()
	_, _, err := Exec("delegate", `{"tasks":[{"prompt":"a"},{"prompt":"b"},{"prompt":"c"},{"prompt":"d"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if el := time.Since(t0); el > 900*time.Millisecond {
		t.Fatalf("4×300ms en sèrie serien 1.2s; han estat %v (no és paral·lel)", el)
	}
}

func TestDelegateSenseRunner(t *testing.T) {
	SetDelegateRunner(nil)
	if _, _, err := Exec("delegate", `{"tasks":[{"prompt":"a"}]}`); err == nil {
		t.Fatal("sense runner ha de fallar, no inventar")
	}
}

// Sense recursió: els subagents reben Specs() (natives) i delegate només
// existeix a SpecsAll() (loop pare).
func TestDelegateSenseRecursio(t *testing.T) {
	for _, s := range Specs() {
		if s.Name == "delegate" {
			t.Fatal("Specs() no pot incloure delegate (recursió)")
		}
	}
	if len(Specs()) != 24 {
		t.Fatalf("natives=%d, han de ser 24", len(Specs()))
	}
	trobat := false
	for _, s := range SpecsAll() {
		if s.Name == "delegate" {
			trobat = true
		}
	}
	if !trobat {
		t.Fatal("SpecsAll() ha d'exposar delegate al loop pare")
	}
}

func TestDelegatePermisPerDefecte(t *testing.T) {
	if d, _ := DefaultPolicy().Decide(ModeCode, "delegate", `{}`); d != "allow" {
		t.Fatalf("delegate=%s, ha de passar sol (subagents read-only)", d)
	}
	q := &Policy{Tools: map[string]string{"delegate": "deny"}}
	if d, _ := q.Decide(ModeCode, "delegate", `{}`); d != "deny" {
		t.Fatal("l'override del config ha de poder vetar delegate")
	}
}
