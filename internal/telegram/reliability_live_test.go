package telegram

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// Runs the production Telegram driver + actual Qwen + real tools. Delivery
// goes to the local test sink, never the user's Telegram or saved sessions.
func TestReliabilityQwenLiveTelegramDriver(t *testing.T) {
	if os.Getenv("GREGAL_RELIABILITY_LIVE") != "1" {
		t.Skip("opt-in live Qwen agent test")
	}
	cfg, _, err := config.Load("/home/aleix/.config/gregal/config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	role := cfg.Roles["code"]
	role.FallbackProvider = ""
	role.FallbackModel = ""
	cfg.Roles["code"] = role
	cfg.Agent.MaxSteps = 8
	cfg.Agent.ModelTimeoutS = 90
	tg := newFakeTG(t)
	stub := newFakeLLM(t, replyText("unused"))
	bot, dir, _ := testBot(t, tg, stub, nil)
	bot.cfg = cfg
	bot.client = llm.New()
	cfg.ConfigureClient(bot.client)
	st := bot.state(1234567)
	st.role = "code"
	st.rolePinned = true
	st.mode = "code"
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	bot.turn(ctx, 1234567, st, "Treballa només al directori actual. Crea add.py amb add(a,b) i test_add.py amb unittest per add(2,3)==5 i add(-2,1)==-1. Executa python3 -m unittest -v i informa del resultat. No deleguis ni toquis serveis; usa eines reals.")
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	for _, name := range []string{"add.py", "test_add.py"} {
		if _, err := os.Stat(filepath.Join(bot.cwd, name)); err != nil {
			t.Fatal(err)
		}
	}
	out, err := exec.CommandContext(ctx, "python3", "-m", "unittest", "discover", "-s", bot.cwd, "-v").CombinedOutput()
	if err != nil {
		t.Fatalf("artifact tests: %v\n%s", err, out)
	}
	t.Logf("independent real artifact tests:\n%s", out)
	saved, err := session.Load(dir, st.name)
	if err != nil {
		t.Fatal(err)
	}
	tools := 0
	for _, m := range saved.Convo {
		if m.Role == "tool" {
			tools++
		}
	}
	if tools == 0 {
		t.Fatal("real Telegram driver lost tool evidence")
	}
	if _, ok := tg.find("OK"); !ok {
		t.Fatalf("missing visible answer at Telegram sink: %+v", tg.all())
	}
	t.Logf("Qwen via real Telegram driver: %d persisted tool results; local delivery verified", tools)
}
