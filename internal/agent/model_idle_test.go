package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func tornAmbLimit(s int) *Torn {
	cfg := &config.Config{}
	cfg.Agent.ModelTimeoutS = s
	return &Torn{o: OpcionsTorn{Cfg: cfg, Mode: ModeCode}}
}

// Sense cap senyal del proveïdor, el pas caduca pel límit d'inactivitat i
// ModelError ho diu així (no com un «Atura» de l'usuari).
func TestModelContextCaducaPerInactivitat(t *testing.T) {
	ctx, cancel := tornAmbLimit(1).ModelContext(context.Background())
	defer cancel()
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("el límit d'inactivitat no ha saltat")
	}
	err := ModelError(ctx, context.Canceled)
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "inactivity") {
		t.Fatalf("error inesperat: %v", err)
	}
}

// Mentre arriben deltes, el pas continua més enllà del límit.
func TestModelContextNoTallaMentreArribenDades(t *testing.T) {
	ctx, cancel := tornAmbLimit(1).ModelContext(context.Background())
	defer cancel()
	// La UI també hi posa el seu observador: s'han d'encadenar.
	vistos := 0
	ctx = llm.WithStreamProgress(ctx, func(string, int) { vistos++ })
	fn := llm.StreamProgressFrom(ctx)
	fi := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(fi) {
		fn(llm.ProgressTool, 10)
		time.Sleep(100 * time.Millisecond)
		if ctx.Err() != nil {
			t.Fatalf("tallat amb dades arribant: %v", context.Cause(ctx))
		}
	}
	if vistos == 0 {
		t.Fatal("l'observador de la UI no rep els deltes")
	}
}

func TestModelErrorRespectaCancelPerUsuari(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	ctx, cancel := tornAmbLimit(60).ModelContext(parent)
	defer cancel()
	stop()
	if err := ModelError(ctx, context.Canceled); !errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("una cancel·lació de l'usuari ha de quedar com a cancel·lació: %v", err)
	}
}
