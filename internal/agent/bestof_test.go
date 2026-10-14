package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// modelFalsBestOf: cada model escriu sol.txt amb un contingut propi i
// després respon «fet». m → «uno», m2 → «dos».
func modelFalsBestOf(t *testing.T) (*httptest.Server, *sync.Map) {
	t.Helper()
	vistos := &sync.Map{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		vistos.Store(req.Model, true)
		w.Header().Set("Content-Type", "application/json")
		for _, m := range req.Messages {
			if m.Role == "tool" {
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fet"},"finish_reason":"stop"}]}`)
				return
			}
		}
		text := "uno\n"
		if req.Model == "m2" {
			text = "dos\n"
		}
		args, _ := json.Marshal(map[string]string{"path": "sol.txt", "content": text})
		argsJSON, _ := json.Marshal(string(args))
		fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"w1","type":"function","function":{"name":"write","arguments":%s}}]},"finish_reason":"tool_calls"}]}`, argsJSON)
	}))
	t.Cleanup(srv.Close)
	return srv, vistos
}

func TestBestOfWorktreesVerificaPuntuaIAplica(t *testing.T) {
	repo := gitRepoProva(t)
	defer tools.Active.Rewind() // el journal global no ha de quedar brut
	srv, vistos := modelFalsBestOf(t)
	cfg := testCfg(srv.URL)
	var mu sync.Mutex
	updates := 0
	res, err := RunBestOf(context.Background(), llm.New(), cfg, BestOfOpts{
		ID: "t1", Task: "escriu la solució a sol.txt", N: 2, Models: []string{"", "t/m2"}, Dir: repo,
		Verify: []string{"git grep -q --untracked uno"},
		Score: func(ctx context.Context, task, diff, checks string) (int, string, error) {
			if strings.Contains(diff, "+uno") {
				return 8, "correct", nil
			}
			return 3, "wrong value", nil
		},
		OnUpdate: func(BestOfAttempt) { mu.Lock(); updates++; mu.Unlock() },
	})
	if err != nil {
		t.Fatalf("RunBestOf: %v", err)
	}
	if len(res.Attempts) != 2 {
		t.Fatalf("dos intents: %+v", res.Attempts)
	}
	if _, ok := vistos.Load("m2"); !ok {
		t.Fatal("l'intent 2 havia de fer servir el model m2")
	}
	a1, a2 := res.Attempts[0], res.Attempts[1]
	for _, a := range res.Attempts {
		if a.State != BestOfDone || a.Error != "" {
			t.Fatalf("intent %d: %+v", a.Index, a)
		}
		if len(a.Files) != 1 || a.Files[0] != "sol.txt" || a.Added != 1 {
			t.Fatalf("diff de l'intent %d: %+v", a.Index, a)
		}
		if _, err := os.Stat(filepath.Join(a.Path, "sol.txt")); err != nil {
			t.Fatalf("l'intent %d havia d'escriure al seu worktree: %v", a.Index, err)
		}
	}
	if a1.ChecksPassed == nil || !*a1.ChecksPassed || a2.ChecksPassed == nil || *a2.ChecksPassed {
		t.Fatalf("checks: #1=%v #2=%v (%+v)", a1.ChecksPassed, a2.ChecksPassed, a2.Checks)
	}
	if a1.Score == nil || *a1.Score != 8 || a2.Score == nil || *a2.Score != 3 {
		t.Fatalf("notes: %v %v", a1.Score, a2.Score)
	}
	if a2.Model != "t/m2" || a1.Model != "t/m" {
		t.Fatalf("models: %q %q", a1.Model, a2.Model)
	}
	if res.Recommended != 1 {
		t.Fatalf("recomanat: %d", res.Recommended)
	}
	if updates < 6 {
		t.Fatalf("OnUpdate s'havia de cridar a cada canvi d'estat: %d", updates)
	}
	// El repo principal no es toca fins que s'aplica.
	if _, err := os.Stat(filepath.Join(repo, "sol.txt")); err == nil {
		t.Fatal("el repo principal no s'havia de tocar abans d'aplicar")
	}
	if !strings.Contains(RenderBestOf(res), "* #1") {
		t.Fatalf("la taula marca el recomanat:\n%s", RenderBestOf(res))
	}

	msg, removed, err := BestOfApply(repo, res.Attempts, 1, false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repo, "sol.txt"))
	if err != nil || strings.TrimSpace(string(raw)) != "uno" {
		t.Fatalf("l'intent 1 havia d'arribar al repo principal: %q %v (%s)", raw, err, msg)
	}
	if len(removed) != 2 {
		t.Fatalf("s'havien de treure els dos worktrees: %v", removed)
	}
	list, _ := tools.WorktreeList(repo)
	if len(list) != 0 {
		t.Fatalf("no han de quedar worktrees: %+v", list)
	}
}

func TestBestOfValida(t *testing.T) {
	repo := gitRepoProva(t)
	cfg := testCfg("http://127.0.0.1:1")
	if _, err := RunBestOf(context.Background(), nil, cfg, BestOfOpts{Task: "x", N: 5, Dir: repo}); err == nil {
		t.Fatal("N=5 és massa")
	}
	if _, err := RunBestOf(context.Background(), nil, cfg, BestOfOpts{Task: "x", N: 2, Dir: repo, Models: []string{"desconegut/m"}}); err == nil {
		t.Fatal("un provider desconegut ha de fallar abans de crear res")
	}
	if list, _ := tools.WorktreeList(repo); len(list) != 0 {
		t.Fatalf("una validació fallida no deixa worktrees: %+v", list)
	}
	// Run injectat: una fallada d'un intent no tomba l'altre, i BestOfDiscard ho neteja tot.
	res, err := RunBestOf(context.Background(), nil, cfg, BestOfOpts{ID: "t2", Task: "x", N: 2, Dir: repo,
		Run: func(ctx context.Context, _ *config.Config, o RunOpts) (RunResult, error) {
			if strings.HasSuffix(o.Dir, "-2") {
				return RunResult{}, fmt.Errorf("model caigut")
			}
			return RunResult{Answer: "ok"}, os.WriteFile(filepath.Join(o.Dir, "f.txt"), []byte("x\n"), 0o644)
		}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Attempts[0].State != BestOfDone || res.Attempts[1].State != BestOfFailed || res.Attempts[1].Error != "model caigut" {
		t.Fatalf("estats: %+v", res.Attempts)
	}
	if res.Recommended != 1 {
		t.Fatalf("recomanat: %d", res.Recommended)
	}
	if removed := BestOfDiscard(repo, res.Attempts); len(removed) != 2 {
		t.Fatalf("discard: %v", removed)
	}
}

func TestDiffStats(t *testing.T) {
	files, add, del := DiffStats("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1 +1,2 @@\n-a\n+b\n+c\ndiff --git a/n.txt b/n.txt\n+++ b/n.txt\n+z\n")
	if len(files) != 2 || files[0] != "x.go" || add != 3 || del != 1 {
		t.Fatalf("%v %d %d", files, add, del)
	}
}
