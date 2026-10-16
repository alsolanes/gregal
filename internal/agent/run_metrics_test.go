package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gregal/internal/llm"
)

// Les mètriques del banc (P1) surten del torn: un pas amb una edició que
// falla, una que passa, una eina inexistent i arguments que no són JSON;
// un altre amb la crida escrita al text; i la resposta final.
func TestRunNonInteractiveMetriquesBanc(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("hola món\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	arg := func(m map[string]string) string {
		inner, _ := json.Marshal(m)
		out, _ := json.Marshal(string(inner))
		return string(out)
	}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		n++
		switch n {
		case 1:
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":"","tool_calls":[`+
				`{"id":"c1","type":"function","function":{"name":"edit","arguments":%s}},`+
				`{"id":"c2","type":"function","function":{"name":"edit","arguments":%s}},`+
				`{"id":"c3","type":"function","function":{"name":"volar","arguments":"{}"}},`+
				`{"id":"c4","type":"function","function":{"name":"read","arguments":"{path: a.txt"}}`+
				`]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1000,"completion_tokens":20}}`,
				arg(map[string]string{"path": f, "old_string": "no hi és", "new_string": "x"}),
				arg(map[string]string{"path": f, "old_string": "hola", "new_string": "adéu"}))
		case 2:
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"<tool_call>{\"name\":\"read\",\"arguments\":{\"path\":\"a.txt\"}}</tool_call>"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1500,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":1000}}}`)
		default:
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"fet"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1600,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":1500}}}`)
		}
	}))
	defer srv.Close()
	res, err := RunNonInteractiveIn(context.Background(), llm.New(), testCfg(srv.URL), "edita a.txt", "code", 8, true, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.EditCalls != 2 || res.EditFailed != 1 {
		t.Fatalf("edicions %d/%d, volia 1 fallada de 2: %+v", res.EditFailed, res.EditCalls, res.Trace)
	}
	if res.ToolCallInvalid != 3 {
		t.Fatalf("tool_call_invalid=%d, volia 3 (eina inexistent, JSON dolent, crida al text)", res.ToolCallInvalid)
	}
	if res.LLMCalls != 3 || res.CachedTokens != 2500 || res.UpTokens != 4100 {
		t.Fatalf("usage: %+v", res)
	}
	if res.DoomHits != 0 || res.Compactions != 0 {
		t.Fatalf("doom=%d compactions=%d", res.DoomHits, res.Compactions)
	}
}

func TestCridesInvalides(t *testing.T) {
	specs := []llm.ToolSpec{{Name: "read"}, {Name: "bash"}}
	call := func(name, args string) llm.ToolCall {
		var c llm.ToolCall
		c.Function.Name, c.Function.Arguments = name, args
		return c
	}
	cases := []struct {
		content string
		calls   []llm.ToolCall
		want    int
	}{
		{"fet", nil, 0},
		{"<function=read><parameter=path>x</parameter></function>", nil, 1},
		{"", []llm.ToolCall{call("read", `{"path":"x"}`), call("bash", "")}, 0},
		{"", []llm.ToolCall{call("llegeix", `{}`), call("read", `{"path":`), call("bash", `{"command":"ls"}`)}, 2},
	}
	for i, c := range cases {
		if got := CridesInvalides(c.content, c.calls, specs); got != c.want {
			t.Errorf("cas %d: %d, volia %d", i, got, c.want)
		}
	}
}
