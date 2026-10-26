package tools

// Exa: cercador i lector per a agents, pel seu servei MCP públic (el mateix
// que fa servir OpenCode per a la seva eina websearch). La cerca és
// semàntica i cada resultat ja porta el text de la pàgina; la lectura surt
// del seu índex, que té pàgines que bloquegen els agents (Stack Overflow,
// Medium). Sense clau és gratuït amb un límit diari per IP; EXA_API_KEY (o
// la clau «exa» del magatzem de claus) l'aixeca. Quan el límit s'acaba,
// Exa respon 429 i el motor queda en pausa (web_pausa.go): la cerca passa
// als altres motors sense que l'agent ho hagi de saber.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// exaEndpoint és una var perquè els tests el puguin redirigir; buit
// desactiva Exa.
var exaEndpoint = "https://mcp.exa.ai/mcp"

var (
	exaKeyMu       sync.Mutex
	exaKeyFallback string
)

// SetExaAPIKey fixa la clau desada al magatzem de claus (auth.yaml, «exa»).
// La variable d'entorn EXA_API_KEY, si hi és, mana.
func SetExaAPIKey(k string) {
	exaKeyMu.Lock()
	exaKeyFallback = strings.TrimSpace(k)
	exaKeyMu.Unlock()
}

func exaKey() string {
	if k := strings.TrimSpace(os.Getenv("EXA_API_KEY")); k != "" {
		return k
	}
	exaKeyMu.Lock()
	defer exaKeyMu.Unlock()
	return exaKeyFallback
}

// exaCall crida una eina del servidor MCP d'Exa (tools/call, sense sessió)
// i en torna el text. El límit gratuït es torna com a bloqueig amb el temps
// d'espera que diu Exa.
func exaCall(ctx context.Context, tool string, args map[string]any) (string, error) {
	if exaEndpoint == "" {
		return "", fmt.Errorf("Exa desactivat")
	}
	payload, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	req, err := http.NewRequestWithContext(ctx, "POST", exaEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", "gregal")
	// La clau va per capçalera, no a la URL: els errors de xarxa de Go
	// porten la URL sencera i acabarien al registre i davant del model.
	if k := exaKey(); k != "" {
		req.Header.Set("x-api-key", k)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2_000_000))
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", &bloqueig{motiu: "límit gratuït d'Exa (amb EXA_API_KEY no n'hi ha)", espera: retryAfter(resp.Header.Get("Retry-After"))}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var msg struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if !mcpDecode(body, &msg) {
		return "", fmt.Errorf("resposta il·legible d'Exa")
	}
	if msg.Error != nil {
		if exaLimit(msg.Error.Message) {
			return "", &bloqueig{motiu: "límit gratuït d'Exa (amb EXA_API_KEY no n'hi ha)"}
		}
		return "", fmt.Errorf("Exa: %s", truncateInline(collapseWS(msg.Error.Message), 120))
	}
	var parts []string
	for _, c := range msg.Result.Content {
		if strings.TrimSpace(c.Text) != "" {
			parts = append(parts, c.Text)
		}
	}
	text := strings.Join(parts, "\n\n")
	if msg.Result.IsError {
		if exaLimit(text) {
			return "", &bloqueig{motiu: "límit gratuït d'Exa (amb EXA_API_KEY no n'hi ha)"}
		}
		return "", fmt.Errorf("Exa: %s", truncateInline(collapseWS(text), 120))
	}
	return text, nil
}

// mcpDecode llegeix una resposta JSON-RPC, tant si ve sola com dins d'un
// flux SSE («data: {...}»).
func mcpDecode(body []byte, v any) bool {
	s := strings.TrimSpace(string(body))
	if strings.HasPrefix(s, "{") && json.Unmarshal([]byte(s), v) == nil {
		return true
	}
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ln), "data:"))
		if strings.HasPrefix(ln, "{") && json.Unmarshal([]byte(ln), v) == nil {
			return true
		}
	}
	return false
}

func exaLimit(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "rate limit") || strings.Contains(l, "dashboard.exa.ai/api-keys")
}

// retryAfter llegeix els segons d'espera d'un Retry-After (0 si no n'hi ha).
func retryAfter(v string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return 0
	}
	return time.Duration(n) * time.Second
}

// searchExa cerca amb web_search_exa. Cada bloc del text és
// «Title:/URL:/Published:/Author:/Highlights:» separat per «---».
func searchExa(ctx context.Context, query string) ([]SearchHit, error) {
	text, err := exaCall(ctx, "web_search_exa", map[string]any{"query": query, "numResults": 8})
	if err != nil {
		return nil, err
	}
	return parseExaSearch(text), nil
}

func parseExaSearch(text string) []SearchHit {
	// Un «---» dins del text d'una pàgina (una ratlla en markdown) no és
	// un resultat nou: només ho és el que comença per «Title:».
	var blocks []string
	for _, frag := range strings.Split(text, "\n---\n") {
		if len(blocks) > 0 && !strings.HasPrefix(strings.TrimSpace(frag), "Title:") {
			blocks[len(blocks)-1] += "\n---\n" + frag
			continue
		}
		blocks = append(blocks, frag)
	}
	var hits []SearchHit
	for _, block := range blocks {
		h := SearchHit{}
		var cos []string
		dinsCos := false
		for _, ln := range strings.Split(block, "\n") {
			if dinsCos {
				cos = append(cos, ln)
				continue
			}
			camp, valor, ok := strings.Cut(ln, ":")
			valor = strings.TrimSpace(valor)
			if !ok {
				continue
			}
			switch strings.TrimSpace(camp) {
			case "Title":
				h.Title = exaNA(valor)
			case "URL":
				h.URL = valor
			case "Published":
				if d := exaNA(valor); len(d) >= 10 {
					h.Date = d[:10]
				}
			case "Highlights", "Text", "Summary":
				dinsCos = true
				if valor != "" {
					cos = append(cos, valor)
				}
			}
		}
		if !strings.HasPrefix(h.URL, "http") {
			continue
		}
		h.Snippet = strings.TrimSpace(strings.Join(cos, "\n"))
		hits = append(hits, h)
	}
	return hits
}

func exaNA(s string) string {
	if s == "N/A" {
		return ""
	}
	return s
}

// webFetchExa llegeix una pàgina des d'Exa (web_fetch_exa). El text comença
// per «# Títol», «URL:» i potser «Published:»/«Author:»; després, el cos en
// markdown.
func webFetchExa(ctx context.Context, target string) (*pagina, error) {
	text, err := exaCall(ctx, "web_fetch_exa", map[string]any{"urls": []string{target}, "maxCharacters": 30000})
	if err != nil {
		return nil, err
	}
	p := &pagina{URL: target, Whole: true}
	lines := strings.Split(strings.TrimSpace(text), "\n")
	i := 0
	if i < len(lines) && strings.HasPrefix(lines[i], "# ") {
		p.Title = strings.TrimSpace(lines[i][2:])
		i++
	}
	for capcalera := true; capcalera && i < len(lines); {
		camp, valor, _ := strings.Cut(lines[i], ":")
		valor = exaNA(strings.TrimSpace(valor))
		switch strings.TrimSpace(camp) {
		case "URL":
		case "Published":
			if len(valor) >= 10 {
				p.Published = valor[:10]
			}
		case "Author":
			p.Byline = valor
		default:
			capcalera = false
			continue
		}
		i++
	}
	body := strings.TrimSpace(strings.Join(lines[i:], "\n"))
	// Moltes pàgines repeteixen el títol com a primera línia del cos.
	body = strings.TrimSpace(strings.TrimPrefix(body, p.Title))
	if comptaParaules(body) < 20 || isInterstitialTitle(p.Title) {
		return nil, fmt.Errorf("Exa no en té text útil")
	}
	p.Body = body
	p.Words = comptaParaules(body)
	return p, nil
}
