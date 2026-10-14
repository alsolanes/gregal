package web

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Worktrees, commit i PR des de la web.
//
//   GET    /api/worktrees            llista els worktrees de gregal del repo
//   POST   /api/worktrees            {name, open_session?:true} crea (i obre
//                                    una sessió nova dins seu)
//   DELETE /api/worktrees?name=…     treu-lo (mai --force); &delete_branch=1
//   POST   /api/worktrees/apply      {name, how: diff|patch|merge}
//   POST   /api/git/commit           {message, paths?, all?}
//   POST   /api/git/pr               {title, body, base?, draft?, branch?}
//   POST   /api/git/suggest          {kind: commit|pr} → proposta del rol chat
//
// Commit i PR des d'aquí són accions de l'usuari (botó i diàleg): no passen
// per la política de l'agent, però sí pel mateix codi que les eines.

func (s *Server) sessionCwd() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cwd
}

func (s *Server) handleWorktrees(w http.ResponseWriter, r *http.Request) {
	cwd := s.sessionCwd()
	switch r.Method {
	case http.MethodGet:
		list, err := tools.WorktreeList(cwd)
		if err != nil {
			writeJSON(w, map[string]any{"available": false, "error": err.Error(), "worktrees": []tools.Worktree{}})
			return
		}
		if list == nil {
			list = []tools.Worktree{}
		}
		main, _ := tools.MainRepoRoot(cwd)
		writeJSON(w, map[string]any{"available": true, "main": main, "current": tools.WorktreeNameOf(cwd), "worktrees": list})
	case http.MethodPost:
		var req struct {
			Name        string `json:"name"`
			OpenSession *bool  `json:"open_session"`
			Title       string `json:"title"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "json invàlid", 400)
			return
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = "s-" + time.Now().Format("0102-150405")
		}
		wt, err := tools.WorktreeCreate(cwd, name)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		out := map[string]any{"worktree": wt}
		if req.OpenSession == nil || *req.OpenSession {
			if s.hub == nil {
				http.Error(w, "sense hub de sessions", 500)
				return
			}
			// Ningú no obre un worktree fora de les seves carpetes. No va als
			// workspaces recents (setWorkspace): és temporal per naturalesa.
			if err := s.user.Allow(wt.Path); err != nil {
				http.Error(w, err.Error(), http.StatusForbidden)
				return
			}
			ns := s.hub.SessionFor(userFrom(r), newSessionID())
			ns.mu.Lock()
			ns.cwd = wt.Path
			if ns.policy != nil {
				ns.policy.ProjectDir = wt.Path
			}
			ns.title = firstNonEmpty(strings.TrimSpace(req.Title), "⎇ "+wt.Name)
			out["session"] = map[string]any{"id": ns.id, "title": ns.title, "cwd": ns.cwd, "project": projectName(ns.cwd), "worktree": wt.Name}
			ns.mu.Unlock()
		}
		writeJSON(w, out)
	case http.MethodDelete:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" || !tools.ValidWorktreeName(name) {
			http.Error(w, "cal ?name=", 400)
			return
		}
		// Cap sessió viva no pot quedar-se sense carpeta.
		if s.hub != nil {
			for _, item := range s.hub.listFor(userFrom(r)) {
				if c, _ := item["cwd"].(string); tools.WorktreeNameOf(c) == name {
					http.Error(w, "una sessió oberta treballa en aquest worktree: tanca-la primer", 409)
					return
				}
			}
		}
		del := r.URL.Query().Get("delete_branch") == "1"
		msg, err := tools.WorktreeRemove(cwd, name, del)
		if err != nil {
			http.Error(w, err.Error(), 409)
			return
		}
		writeJSON(w, map[string]string{"summary": msg})
	default:
		http.Error(w, "GET, POST o DELETE", 405)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *Server) handleWorktreeApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Name string `json:"name"`
		How  string `json:"how"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || !tools.ValidWorktreeName(req.Name) {
		http.Error(w, "cal {\"name\": …, \"how\": \"diff|patch|merge\"}", 400)
		return
	}
	cwd := s.sessionCwd()
	if req.How == "" || req.How == "diff" {
		patch, wt, err := tools.WorktreeDiff(cwd, req.Name)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		const max = 200000
		trunc := len(patch) > max
		if trunc {
			patch = patch[:max]
		}
		writeJSON(w, map[string]any{"worktree": wt, "diff": patch, "truncated": trunc})
		return
	}
	out, err := tools.WorktreeApply(cwd, req.Name, req.How)
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	writeJSON(w, map[string]string{"summary": out})
}

func (s *Server) handleGitCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req tools.GitCommitArgs
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "json invàlid", 400)
		return
	}
	// Des del botó, sense camins: tot el que hi ha al workspace (és el que
	// la pàgina Canvis ensenya), no només el que ha tocat l'agent.
	if len(req.Paths) == 0 {
		req.All = true
	}
	out, err := tools.GitCommit(s.id, s.sessionCwd(), req)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]string{"summary": out})
}

func (s *Server) handleGitPR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req tools.PRArgs
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "json invàlid", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	out, err := tools.GHPRCreate(ctx, s.sessionCwd(), req)
	if err != nil {
		msg := err.Error()
		if out != "" {
			msg = out + "\n" + msg
		}
		http.Error(w, msg, 400)
		return
	}
	writeJSON(w, map[string]string{"summary": out})
}

// handleGitSuggest demana al rol chat un missatge de commit (o títol i cos
// de PR) a partir del diff del workspace. És només una proposta: l'usuari
// l'edita al diàleg abans de fer res.
func (s *Server) handleGitSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "només POST", 405)
		return
	}
	var req struct {
		Kind string `json:"kind"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	cwd := s.sessionCwd()
	diff := strings.TrimSpace(tools.GitDiff(cwd))
	if req.Kind == "pr" {
		// Per a la PR compten els commits de la branca, no (només) el working tree.
		if log, err := gitLogForPR(cwd); err == nil && log != "" {
			diff = "Commits on this branch:\n" + log + "\n\n" + diff
		}
	}
	if diff == "" {
		http.Error(w, s.tr("no hi ha canvis per resumir", "there are no changes to summarise"), 400)
		return
	}
	if len(diff) > 12000 {
		diff = diff[:12000] + "\n…(truncated)"
	}
	instr := "Write a git commit message for these changes: one summary line in the imperative mood (max 72 characters), a blank line, then a few short bullet points if useful. Output only the message."
	if req.Kind == "pr" {
		instr = "Write a GitHub pull request for these changes. First line: the title (max 72 characters, no prefix). Then a blank line, then the body in Markdown: a short summary and a bullet list of the main changes. Output only the title and body."
	}
	if s.cfg.Language == "ca" {
		instr += " Write it in Catalan."
	}
	roleName := "chat"
	if _, ok := s.cfg.Roles[roleName]; !ok {
		roleName = s.role
	}
	s.mu.Lock()
	p, role := s.roleRefFor(roleName)
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	msgs := []llm.Message{
		{Role: "system", Content: "You write concise, accurate git commit messages and pull request descriptions. Never invent changes that are not in the diff."},
		{Role: "user", Content: instr + "\n\n" + diff},
	}
	out, err := s.client.Chat(ctx, p.BaseURL, p.APIKey, role.Model, msgs, 0.2, 800)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	out = strings.TrimSpace(strings.Trim(strings.TrimSpace(out), "`"))
	if req.Kind == "pr" {
		title, body, _ := strings.Cut(out, "\n")
		writeJSON(w, map[string]string{"title": strings.TrimSpace(strings.TrimLeft(title, "# ")), "body": strings.TrimSpace(body)})
		return
	}
	writeJSON(w, map[string]string{"message": out})
}

// gitLogForPR dona els commits de la branca actual que la base remota no té.
func gitLogForPR(dir string) (string, error) {
	base := "origin/HEAD"
	out, err := gitText(dir, "log", "--oneline", "--no-decorate", "-n", "30", base+"..HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func gitText(dir string, args ...string) (string, error) {
	return tools.GitText(filepath.Clean(dir), args...)
}
