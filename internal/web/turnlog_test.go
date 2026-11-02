package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"gregal/internal/config"
)

func readTurnLog(t *testing.T) []turnRecord {
	t.Helper()
	f, err := os.Open(turnLogPath())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []turnRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r turnRecord
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("bad line %q: %v", sc.Text(), err)
		}
		out = append(out, r)
	}
	return out
}

func TestTurnLogRecordsTimingAndOutcome(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	role := config.Role{Provider: "zen", Model: "deepseek-v4.1-flash"}

	// Torn d'agent: el primer token i les crides surten de l'emissor.
	tl := startTurnLog("agent", "code", role)
	var seen []string
	emit := tl.emitter(func(kind string, _ any) { seen = append(seen, kind) })
	time.Sleep(5 * time.Millisecond)
	emit("token", map[string]string{"text": "Ho"})
	emit("speed", map[string]any{"prompt": 1200, "completion": 40})
	emit("speed", map[string]any{"prompt": 1300, "completion": 60})
	tl.finish(nil, false, false, nil)
	if strings.Join(seen, ",") != "token,speed,speed" {
		t.Fatalf("emitter must pass every event through: %v", seen)
	}

	// Torn de vent: passos i temps del loop; un error es resumeix.
	w := startTurnLog("wind", "espurna", role)
	w.finish(errors.New("upstream: Endpoint is unavailable"), false, false, &tempsTorn{passos: 3, model: 2 * time.Second, eines: 500 * time.Millisecond})
	startTurnLog("wind", "espurna", role).finish(context.Canceled, true, false, nil)
	startTurnLog("rotllana", "r1:espurna", role).finish(nil, false, true, &tempsTorn{passos: 1})

	recs := readTurnLog(t)
	if len(recs) != 4 {
		t.Fatalf("want 4 records, got %d", len(recs))
	}
	a := recs[0]
	if a.Kind != "agent" || a.Model != "zen/deepseek-v4.1-flash" || a.Outcome != "ok" || a.Steps != 2 || a.Prompt != 2500 || a.Completion != 100 {
		t.Fatalf("agent record: %+v", a)
	}
	if a.FirstTokenMs < 5 || a.TotalMs < a.FirstTokenMs || a.Ts == "" {
		t.Fatalf("agent timing: %+v", a)
	}
	if b := recs[1]; b.Outcome != "error" || !strings.Contains(b.Error, "Endpoint is unavailable") || b.Steps != 3 || b.ModelMs != 2000 || b.ToolsMs != 500 || b.FirstTokenMs != 0 {
		t.Fatalf("wind error record: %+v", b)
	}
	if recs[2].Outcome != "stopped" || recs[3].Outcome != "partial" || recs[3].ID != "r1:espurna" {
		t.Fatalf("outcomes: %+v %+v", recs[2], recs[3])
	}
}

func TestTurnLogRotates(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	if err := os.MkdirAll(strings.TrimSuffix(turnLogPath(), "turns.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(turnLogPath(), make([]byte, turnLogMaxBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	startTurnLog("wind", "x", config.Role{}).finish(nil, false, false, nil)
	if recs := readTurnLog(t); len(recs) != 1 {
		t.Fatalf("new file should hold only the new record, got %d", len(recs))
	}
	if _, err := os.Stat(turnLogPath() + ".1"); err != nil {
		t.Fatal("old log not kept as turns.jsonl.1")
	}
}
