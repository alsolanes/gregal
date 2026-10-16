package llm

import "testing"

func TestProfileFamilies(t *testing.T) {
	cases := map[string]string{
		"Qwen3-Coder-30B-A3B-Instruct": "qwen3-coder",
		"qwen2.5-coder:7b":             "qwen2.5-coder",
		"qwen3:8b":                     "qwen3",
		"deepseek-r1:14b":              "deepseek-r1",
		"gpt-oss-20b":                  "gpt-oss",
		"devstral-small-2507":          "devstral",
		"gemma3:27b":                   "gemma",
		"llama3.1:8b":                  "llama",
		"some-unknown-model":           "",
	}
	for m, want := range cases {
		if got := ProfileFor(m).Family; got != want {
			t.Errorf("%s: %q, volia %q", m, got, want)
		}
	}
}

func TestModelSizeAndCompact(t *testing.T) {
	if s := ModelSizeB("qwen2.5-coder:7b"); s != 7 {
		t.Errorf("mida %v", s)
	}
	if s := ModelSizeB("Qwen3-30B-A3B"); s != 30 {
		t.Errorf("mida MoE %v", s)
	}
	if !CompactFor("http://localhost:11434/v1", "qwen2.5-coder:7b", 0) {
		t.Error("7B local hauria de ser compacte")
	}
	if CompactFor("https://api.example.com/v1", "qwen2.5-coder:7b", 8192) {
		t.Error("els remots no es compacten")
	}
	if !CompactFor("http://192.168.1.20:8080/v1", "big-model", 16384) {
		t.Error("finestra petita local hauria de ser compacte")
	}
}

func TestApplyProfileOnlyLocal(t *testing.T) {
	r := applyProfile("http://127.0.0.1:8080/v1", chatRequest{Model: "qwen3:8b"})
	if r.Temperature != 0.6 || r.TopK != 20 || r.TopP == nil || r.PresencePenalty == nil {
		t.Fatalf("local qwen3: %+v", r)
	}
	r = applyProfile("https://opencode.ai/zen/go/v1", chatRequest{Model: "qwen3:8b"})
	if r.Temperature != 0 || r.TopK != 0 {
		t.Fatalf("remot no s'ha de tocar: %+v", r)
	}
	r = applyProfile("http://localhost:1234/v1", chatRequest{Model: "devstral", Temperature: 0.4})
	if r.Temperature != 0.4 {
		t.Fatal("la temperatura del rol mana")
	}
}
