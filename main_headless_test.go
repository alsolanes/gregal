package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gregal/internal/config"
)

func TestValidaHeadless(t *testing.T) {
	cfg := &config.Config{Mode: "code"}
	for _, c := range []struct {
		output, mode string
		approve      bool
		steps        int
		cfgMode      string
		bad          string
	}{
		{"text", "", false, 0, "code", ""},
		{"json", "inspect", true, 5, "code", ""},
		{"yaml", "", false, 0, "code", "--output"},
		{"text", "cod", false, 0, "code", "--mode"},
		{"text", "", false, 500, "code", "--max-steps"},
		{"text", "", false, -1, "code", "--max-steps"},
		{"text", "autonomous", true, 0, "code", "--auto-approve"},
		// The configured mode counts too.
		{"text", "", true, 0, "autonomous", "--auto-approve"},
		{"text", "autonomous", false, 0, "code", ""},
	} {
		cfg.Mode = c.cfgMode
		err := validaHeadless(cfg, c.output, c.mode, c.approve, c.steps, "en")
		switch {
		case c.bad == "" && err != nil:
			t.Errorf("%+v: unexpected error %v", c, err)
		case c.bad != "" && (err == nil || !strings.Contains(err.Error(), c.bad)):
			t.Errorf("%+v: want an error about %s, got %v", c, c.bad, err)
		}
	}
}

// A --verify that still fails after the last round must not exit 0, and
// a provider error with --output json still prints a JSON object.
func TestRunHeadlessCodisDeSortida(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_MEMORY", t.TempDir()+"/memory.md")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"done"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"Invalid credential"}}`, http.StatusUnauthorized)
	}))
	defer bad.Close()
	cfgFor := func(url string) *config.Config {
		return &config.Config{
			Mode:      "code",
			Providers: map[string]config.Provider{"t": {BaseURL: url}},
			Roles:     map[string]config.Role{"code": {Provider: "t", Model: "m", MaxTokens: 100}},
			Verify:    config.VerifyCfg{Mode: "off"},
			Router:    config.RouterCfg{Mode: "off"},
			Agent:     config.AgentCfg{MaxSteps: 5},
		}
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if code := runHeadless(cfgFor(srv.URL), "do the thing", "json", false, 0, "", "exit 3", 1, headlessExtra{}); code != exitVerifyFailed {
		t.Fatalf("failed verification exit code = %d, want %d", code, exitVerifyFailed)
	}
	if code := runHeadless(cfgFor(srv.URL), "do the thing", "text", false, 2, "", "exit 0", 1, headlessExtra{}); code != exitOK {
		t.Fatalf("passing verification exit code = %d", code)
	}

	// Capture stdout to check the JSON error object.
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	code := runHeadless(cfgFor(bad.URL), "do the thing", "json", false, 0, "", "", 1, headlessExtra{})
	w.Close()
	os.Stdout = old
	var out strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		out.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if code != exitError {
		t.Fatalf("provider error exit code = %d", code)
	}
	if !strings.Contains(out.String(), `"error":`) || !strings.Contains(out.String(), `"tools_used": []`) {
		t.Fatalf("--output json must print a JSON object with the error:\n%s", out.String())
	}
}
