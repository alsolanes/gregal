package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBashBackgroundStopsWhenRunIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out, _, err := ExecCtx(ctx, "cancel-test", t.TempDir(), "bash_background", `{"command":"sleep 30"}`)
	if err != nil {
		t.Fatal(err)
	}
	var id string
	fields := strings.Fields(out)
	for i, field := range fields {
		if field == "id" && i+1 < len(fields) {
			id = fields[i+1]
			break
		}
	}
	if id == "" {
		t.Fatalf("no he trobat l'id a %q", out)
	}
	p := Procs().Get(id)
	if p == nil {
		t.Fatalf("procés %q desconegut", id)
	}
	cancel()
	if !p.Wait(3*time.Second) || p.IsRunning() {
		t.Fatal("bash_background hauria d'haver acabat en cancel·lar el torn")
	}
}
