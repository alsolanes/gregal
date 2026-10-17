package lsp

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Server diu com engegar el servidor d'un llenguatge.
type Server struct {
	Lang       string
	LanguageID string     // languageId de textDocument/didOpen
	Commands   [][]string // alternatives, la primera que hi és al PATH
	Extensions []string
}

// Servers són els que Gregal sap fer servir si són instal·lats. No se'n
// baixa cap: el que no és al PATH no s'ofereix.
var Servers = []Server{
	{Lang: "go", LanguageID: "go", Commands: [][]string{{"gopls"}}, Extensions: []string{".go"}},
	{Lang: "python", LanguageID: "python", Commands: [][]string{{"pyright-langserver", "--stdio"}, {"basedpyright-langserver", "--stdio"}, {"pylsp"}}, Extensions: []string{".py", ".pyi"}},
	{Lang: "typescript", LanguageID: "typescript", Commands: [][]string{{"typescript-language-server", "--stdio"}}, Extensions: []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"}},
	{Lang: "rust", LanguageID: "rust", Commands: [][]string{{"rust-analyzer"}}, Extensions: []string{".rs"}},
	{Lang: "c", LanguageID: "cpp", Commands: [][]string{{"clangd"}}, Extensions: []string{".c", ".h", ".cc", ".cpp", ".cxx", ".hpp", ".hh"}},
}

// Custom són servidors del config (lsp.servers): substitueixen el del
// mateix llenguatge o n'afegeixen un de nou.
func Configure(custom []Server, off bool) {
	mu.Lock()
	defer mu.Unlock()
	desactivat = off
	extra = custom
	disponiblesCalc = false
}

var (
	mu              sync.Mutex
	desactivat      bool
	extra           []Server
	disponibles     []resolt
	disponiblesCalc bool
	lookPath        = exec.LookPath
)

type resolt struct {
	Server
	Command []string
}

// Disponibles són els servidors que es poden engegar (al PATH), un per
// llenguatge. Es calcula un cop: el conjunt d'eines no ha de canviar a mitja
// sessió (trencaria la caché de prefix).
func Disponibles() []resolt {
	mu.Lock()
	defer mu.Unlock()
	if desactivat {
		return nil
	}
	if disponiblesCalc {
		return disponibles
	}
	disponiblesCalc = true
	disponibles = nil
	tots := map[string]Server{}
	var ordre []string
	for _, s := range append(append([]Server{}, Servers...), extra...) {
		if _, ja := tots[s.Lang]; !ja {
			ordre = append(ordre, s.Lang)
		}
		tots[s.Lang] = s
	}
	for _, lang := range ordre {
		s := tots[lang]
		for _, c := range s.Commands {
			if len(c) == 0 {
				continue
			}
			if p, err := lookPath(c[0]); err == nil {
				disponibles = append(disponibles, resolt{Server: s, Command: append([]string{p}, c[1:]...)})
				break
			}
		}
	}
	return disponibles
}

// Llenguatges és la llista dels llenguatges amb servidor, per a la
// descripció de l'eina.
func Llenguatges() []string {
	var out []string
	for _, s := range Disponibles() {
		out = append(out, s.Lang)
	}
	sort.Strings(out)
	return out
}

func perFitxer(path string) (resolt, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	for _, s := range Disponibles() {
		for _, e := range s.Extensions {
			if e == ext {
				return s, true
			}
		}
	}
	return resolt{}, false
}

// Manager guarda un client per (arrel, llenguatge) i l'estat dels
// documents oberts.
type Manager struct {
	mu      sync.Mutex
	clients map[string]*Client
	docs    map[string]docEstat // URI normalitzada → versió i hash
	errors  map[string]error    // un servidor que no ha arrencat no es torna a provar
}

type docEstat struct {
	versio int
	hash   [32]byte
	client *Client
}

// Default és el gestor del procés.
var Default = &Manager{}

// clientPer engega (o reutilitza) el servidor del fitxer.
func (m *Manager) clientPer(ctx context.Context, root, path string) (*Client, resolt, error) {
	s, ok := perFitxer(path)
	if !ok {
		return nil, s, fmt.Errorf("no language server for %s files (available: %s)", filepath.Ext(path), strings.Join(Llenguatges(), ", "))
	}
	clau := filepath.Clean(root) + "|" + s.Lang
	m.mu.Lock()
	if m.clients == nil {
		m.clients, m.docs, m.errors = map[string]*Client{}, map[string]docEstat{}, map[string]error{}
	}
	if c := m.clients[clau]; c != nil {
		m.mu.Unlock()
		return c, s, nil
	}
	if err := m.errors[clau]; err != nil {
		m.mu.Unlock()
		return nil, s, err
	}
	m.mu.Unlock()
	sctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	c, err := Start(sctx, s.Command, root, nil)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		err = fmt.Errorf("%s did not start: %w", filepath.Base(s.Command[0]), err)
		m.errors[clau] = err
		return nil, s, err
	}
	if prev := m.clients[clau]; prev != nil { // una altra crida ha guanyat
		go c.Close()
		return prev, s, nil
	}
	m.clients[clau] = c
	return c, s, nil
}

// obre envia el contingut actual del fitxer al servidor: didOpen la primera
// vegada, didChange (text sencer) si ha canviat al disc. Torna la URI.
func (m *Manager) obre(c *Client, s resolt, path string) (string, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	text := string(raw)
	uri := URI(path)
	k := normURI(uri)
	h := sha256.Sum256(raw)
	m.mu.Lock()
	st, ja := m.docs[k]
	if ja && st.client == c && st.hash == h {
		m.mu.Unlock()
		return uri, text, nil
	}
	versio := st.versio + 1
	m.docs[k] = docEstat{versio: versio, hash: h, client: c}
	m.mu.Unlock()
	if !ja || st.client != c {
		err = c.Notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": s.LanguageID, "version": versio, "text": text,
		}})
	} else {
		err = c.Notify("textDocument/didChange", map[string]any{
			"textDocument":   map[string]any{"uri": uri, "version": versio},
			"contentChanges": []map[string]any{{"text": text}},
		})
	}
	return uri, text, err
}

// Close atura tots els servidors (en acabar el procés).
func (m *Manager) Close() {
	m.mu.Lock()
	cs := m.clients
	m.clients, m.docs, m.errors = nil, nil, nil
	m.mu.Unlock()
	var wg sync.WaitGroup
	for _, c := range cs {
		wg.Add(1)
		go func(c *Client) { defer wg.Done(); c.Close() }(c)
	}
	wg.Wait()
}
