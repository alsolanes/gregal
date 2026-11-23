package web

// Terminal de la pestanya Terminal del panell lateral: un shell de debò
// (PowerShell a Windows, $SHELL a la resta) dins d'un pseudoterminal, un per
// sessió. Serveix per al que Gregal no pot fer sol: logins que obren el
// navegador, contrasenyes de sudo, instal·ladors que pregunten. És el
// shell de la persona, no una eina de l'agent: el que hi escriu no passa
// per la llista d'ordres denegades. La pantalla es pinta amb el mateix
// emulador i el mateix SSE que les targetes del Kanban.

import (
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"gregal/internal/pty"
	"gregal/internal/vterm"
)

type shellTerm struct {
	p    pty.PTY
	vt   *vterm.Term
	done chan struct{} // es tanca quan el shell ha acabat
	exit int           // vàlid després de done
}

// shellTerms va per clau de sessió (s.id): tancar la pestanya se l'endú.
var shellTerms = struct {
	sync.Mutex
	m map[string]*shellTerm
}{m: map[string]*shellTerm{}}

// shellTerm és el terminal viu de la sessió, o nil.
func (s *Server) shellTerm() *shellTerm {
	shellTerms.Lock()
	defer shellTerms.Unlock()
	return shellTerms.m[s.id]
}

// openShellTerm torna el terminal de la sessió amb la mida demanada, i
// l'engega a la carpeta de treball si no n'hi ha cap de viu.
func (s *Server) openShellTerm(cols, rows int) (*shellTerm, error) {
	shellTerms.Lock()
	defer shellTerms.Unlock()
	if t := shellTerms.m[s.id]; t != nil {
		t.vt.Resize(cols, rows)
		return t, t.p.Resize(cols, rows)
	}
	path, args, err := shellCommand()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	dir := s.cwd
	s.mu.Unlock()
	t := &shellTerm{done: make(chan struct{})}
	// Les respostes de l'emulador (posició del cursor, colors) tornen al
	// shell per l'entrada, com ho faria un terminal de debò.
	t.vt = vterm.New(cols, rows, func(r []byte) {
		if t.p != nil {
			_, _ = t.p.Write(r)
		}
	})
	t.p, err = pty.Start(pty.Spec{Path: path, Args: args, Dir: dir, Env: shellEnv(), Cols: cols, Rows: rows})
	if err != nil {
		return nil, err
	}
	shellTerms.m[s.id] = t
	key := s.id
	go func() {
		defer recoverGo("terminal")
		buf := make([]byte, 32*1024)
		for {
			n, err := t.p.Read(buf)
			if n > 0 {
				_, _ = t.vt.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer recoverGo("terminal")
		t.exit, _ = t.p.Wait()
		_ = t.p.Close()
		shellTerms.Lock()
		if shellTerms.m[key] == t {
			delete(shellTerms.m, key)
		}
		shellTerms.Unlock()
		close(t.done)
	}()
	return t, nil
}

// closeShellTerm atura el terminal d'una sessió (en tancar-la).
func closeShellTerm(key string) {
	shellTerms.Lock()
	t := shellTerms.m[key]
	delete(shellTerms.m, key)
	shellTerms.Unlock()
	if t != nil {
		_ = t.p.Kill()
	}
}

func shellCommand() (string, []string, error) {
	if runtime.GOOS == "windows" {
		path, err := exec.LookPath("pwsh.exe")
		if err != nil {
			path, err = exec.LookPath("powershell.exe")
		}
		return path, []string{"-NoLogo"}, err
	}
	sh := os.Getenv("SHELL")
	if sh == "" {
		sh = "/bin/sh"
	}
	path, err := exec.LookPath(sh)
	return path, nil, err
}

// shellEnv és l'entorn del servidor amb un terminal de 256 colors.
func shellEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(k) {
		case "TERM", "COLORTERM", "NO_COLOR":
			continue
		}
		env = append(env, kv)
	}
	return append(env, "TERM=xterm-256color", "COLORTERM=truecolor")
}

// terminalAllowed: amb comptes, només l'administrador (el shell no queda
// tancat a les arrels de l'usuari). Els POST han de ser JSON: sense comptes
// no hi ha token, i un formulari d'una altra web podria fer un POST «simple»
// (text/plain, sense preflight) a localhost.
func terminalAllowed(w http.ResponseWriter, r *http.Request) bool {
	if u := userFrom(r); u != nil && !u.Admin {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": "the terminal needs an administrator"})
		return false
	}
	if r.Method == http.MethodGet {
		return true
	}
	if r.Method != http.MethodPost {
		writeJSONStatus(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST"})
		return false
	}
	if ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); ct != "application/json" {
		writeJSONStatus(w, http.StatusUnsupportedMediaType, map[string]string{"error": "application/json"})
		return false
	}
	return true
}

// handleTerminal: POST {cols, rows} obre el terminal de la sessió (o hi
// torna) i l'ajusta a la mida del panell.
func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request) {
	if !terminalAllowed(w, r) {
		return
	}
	var req struct {
		Cols int `json:"cols"`
		Rows int `json:"rows"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req) != nil || req.Cols < 20 || req.Rows < 5 || req.Cols > 400 || req.Rows > 200 {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "cols 20-400 and rows 5-200"})
		return
	}
	if _, err := s.openShellTerm(req.Cols, req.Rows); err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "cols": req.Cols, "rows": req.Rows})
}

// handleTerminalInput: POST {data}: tecles ja codificades com les enviaria
// un terminal.
func (s *Server) handleTerminalInput(w http.ResponseWriter, r *http.Request) {
	if !terminalAllowed(w, r) {
		return
	}
	var req struct {
		Data string `json:"data"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req) != nil {
		writeJSONStatus(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	t := s.shellTerm()
	if t == nil {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "terminal not open"})
		return
	}
	if _, err := t.p.Write([]byte(req.Data)); err != nil {
		writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleTerminalStream: SSE amb la pantalla. «frame» com al Kanban (el
// primer porta totes les files, els següents només les canviades),
// «history» amb les línies que han sortit per dalt i «exited» quan el shell
// acaba.
func (s *Server) handleTerminalStream(w http.ResponseWriter, r *http.Request) {
	if !terminalAllowed(w, r) {
		return
	}
	t := s.shellTerm()
	if t == nil {
		writeJSONStatus(w, http.StatusConflict, map[string]string{"error": "terminal not open"})
		return
	}
	emit := sse(w)
	var (
		prev     []string
		lastSeq  uint64
		hist     uint64
		lastPing = time.Now()
	)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		if rows, next := t.vt.History(hist); next != hist {
			lines := make([]string, len(rows))
			for i, row := range rows {
				lines[i] = vterm.LineHTML(row, -1)
			}
			emit("history", map[string]any{"lines": lines})
			hist = next
		}
		if seq := t.vt.Seq(); seq != lastSeq || prev == nil {
			lastSeq = seq
			var f map[string]any
			f, prev = kanbanFrame(t.vt, prev)
			emit("frame", f)
			lastPing = time.Now()
		}
		if time.Since(lastPing) > 15*time.Second {
			emit("ping", map[string]any{})
			lastPing = time.Now()
		}
		select {
		case <-r.Context().Done():
			return
		case <-t.done:
			emit("exited", map[string]any{"exit": t.exit})
			return
		case <-tick.C:
		}
	}
}
