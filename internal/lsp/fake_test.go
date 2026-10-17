package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Un servidor LSP fals, el mateix binari de test amb GREGAL_FAKE_LSP=1:
// definició = la línia «func NOM», referències i rename = totes les
// aparicions de la paraula als .go de l'arrel, hover = «func NOM()», i un
// diagnòstic per a cada fitxer obert. Abans de respondre initialize demana
// workspace/configuration, com fan gopls i pyright.
func TestMain(m *testing.M) {
	if os.Getenv("GREGAL_FAKE_LSP") == "1" {
		fakeServer(os.Stdin, os.Stdout)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fakeServer(in io.Reader, out io.Writer) {
	r := bufio.NewReader(in)
	root := ""
	docs := map[string]string{}
	escriu := func(v any) {
		raw, _ := json.Marshal(v)
		fmt.Fprintf(out, "Content-Length: %d\r\n\r\n%s", len(raw), raw)
	}
	paraula := func(uri string, p Position) string {
		ls := strings.Split(docs[uri], "\n")
		if p.Line >= len(ls) {
			return ""
		}
		l := ls[p.Line]
		ini, fi := p.Character, p.Character
		es := func(c byte) bool {
			return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		}
		for ini > 0 && es(l[ini-1]) {
			ini--
		}
		for fi < len(l) && es(l[fi]) {
			fi++
		}
		return l[ini:fi]
	}
	aparicions := func(w string) map[string][]Range {
		res := map[string][]Range{}
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(w) + `\b`)
		fs, _ := filepath.Glob(filepath.Join(root, "*.go"))
		for _, f := range fs {
			raw, _ := os.ReadFile(f)
			for i, l := range strings.Split(string(raw), "\n") {
				for _, loc := range re.FindAllStringIndex(l, -1) {
					res[URI(f)] = append(res[URI(f)], Range{Start: Position{i, loc[0]}, End: Position{i, loc[1]}})
				}
			}
		}
		return res
	}
	for {
		n := -1
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				return
			}
			l = strings.TrimSpace(l)
			if l == "" {
				break
			}
			if k, v, ok := strings.Cut(l, ":"); ok && k == "Content-Length" {
				n, _ = strconv.Atoi(strings.TrimSpace(v))
			}
		}
		buf := make([]byte, n)
		io.ReadFull(r, buf)
		var m struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.Unmarshal(buf, &m)
		var p struct {
			RootURI      string `json:"rootUri"`
			TextDocument struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
			Position Position `json:"position"`
			NewName  string   `json:"newName"`
		}
		json.Unmarshal(m.Params, &p)
		var result any
		switch m.Method {
		case "":
			continue // resposta del client a workspace/configuration
		case "initialize":
			root = Path(p.RootURI)
			escriu(map[string]any{"jsonrpc": "2.0", "id": 999, "method": "workspace/configuration", "params": map[string]any{"items": []any{map[string]any{}}}})
			result = map[string]any{"capabilities": map[string]any{}}
		case "textDocument/didOpen", "textDocument/didChange":
			text := p.TextDocument.Text
			if len(p.ContentChanges) > 0 {
				text = p.ContentChanges[0].Text
			}
			docs[p.TextDocument.URI] = text
			escriu(map[string]any{"jsonrpc": "2.0", "method": "textDocument/publishDiagnostics", "params": map[string]any{
				"uri": p.TextDocument.URI, "diagnostics": []map[string]any{{"range": Range{}, "severity": 2, "source": "fake", "message": fmt.Sprintf("%d lines", strings.Count(text, "\n"))}},
			}})
			continue
		case "textDocument/definition":
			w := paraula(p.TextDocument.URI, p.Position)
			var locs []Location
			for uri, rs := range aparicions(w) {
				for _, rg := range rs {
					raw, _ := os.ReadFile(Path(uri))
					if strings.Contains(strings.Split(string(raw), "\n")[rg.Start.Line], "func "+w) {
						locs = append(locs, Location{URI: uri, Range: rg})
					}
				}
			}
			result = locs
		case "textDocument/references":
			var locs []Location
			for uri, rs := range aparicions(paraula(p.TextDocument.URI, p.Position)) {
				for _, rg := range rs {
					locs = append(locs, Location{URI: uri, Range: rg})
				}
			}
			result = locs
		case "textDocument/hover":
			result = map[string]any{"contents": map[string]any{"kind": "markdown", "value": "func " + paraula(p.TextDocument.URI, p.Position) + "()"}}
		case "textDocument/rename":
			ch := map[string][]TextEdit{}
			for uri, rs := range aparicions(paraula(p.TextDocument.URI, p.Position)) {
				for _, rg := range rs {
					ch[uri] = append(ch[uri], TextEdit{Range: rg, NewText: p.NewName})
				}
			}
			result = map[string]any{"changes": ch}
		case "shutdown":
			result = nil
		case "exit":
			return
		default:
			if len(m.ID) == 0 {
				continue
			}
		}
		if len(m.ID) > 0 {
			escriu(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": result})
		}
	}
}
