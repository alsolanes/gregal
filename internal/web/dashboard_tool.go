package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// dashboard_save: l'agent desa gràfics directament a un dashboard de l'app
// (la secció «Plots & dashboard»), en comptes de fer una web a part. Vist el
// 2026-10-13: davant de «fes-me un dashboard» el model escrivia un HTML
// sencer dins una crida d'eina (minuts) perquè no tenia cap altra via.

type dashboardSaveArgs struct {
	Dashboard string     `json:"dashboard"`
	Plots     []plotSpec `json:"plots"`
	Replace   bool       `json:"replace"`
}

func (s *Server) dashboardSave(argsJSON string) (string, error) {
	var a dashboardSaveArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	name := strings.TrimSpace(a.Dashboard)
	if name == "" {
		name = "Dashboard"
	}
	if len([]rune(name)) > 200 {
		return "", fmt.Errorf("dashboard name too long")
	}
	if len(a.Plots) == 0 {
		return "", fmt.Errorf("plots is empty: pass one or more charts with the gregal-plot JSON shape")
	}
	if len(a.Plots) > 24 {
		return "", fmt.Errorf("too many plots in one call (max 24)")
	}
	for i, p := range a.Plots {
		if err := validatePlot(p); err != nil {
			return "", fmt.Errorf("plot %d (%q): %w", i+1, p.Title, err)
		}
	}
	path := s.plotStorePath()
	s.mu.Lock()
	origin := s.convoFile
	if origin == "" {
		origin = s.id
	}
	s.mu.Unlock()

	plotStoreMu.Lock()
	defer plotStoreMu.Unlock()
	lib := plotLibrary{Plots: []savedPlot{}}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &lib); err != nil {
			return "", fmt.Errorf("cannot read plot library")
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot read plot library")
	}
	if len(lib.Boards) == 0 {
		lib.Boards = []plotBoard{{ID: "default", Name: "Dashboard"}}
	}
	board := ""
	for _, b := range lib.Boards {
		if strings.EqualFold(strings.TrimSpace(b.Name), name) {
			board = b.ID
			name = b.Name
			break
		}
	}
	created := false
	if board == "" {
		if len(lib.Boards) >= 30 {
			return "", fmt.Errorf("too many dashboards (max 30)")
		}
		board = fmt.Sprintf("b%d", time.Now().UnixNano())
		lib.Boards = append(lib.Boards, plotBoard{ID: board, Name: name})
		created = true
	}
	if a.Replace {
		kept := lib.Plots[:0]
		for _, p := range lib.Plots {
			if !(p.Dashboard && (p.Board == board || (p.Board == "" && board == "default"))) {
				kept = append(kept, p)
			}
		}
		lib.Plots = kept
	}
	now := time.Now()
	for i, p := range a.Plots {
		lib.Plots = append(lib.Plots, savedPlot{
			ID: fmt.Sprintf("%d", now.UnixNano()+int64(i)), Spec: p, Session: origin,
			Created: now.UTC().Format(time.RFC3339), Dashboard: true, Board: board, Width: 1,
		})
	}
	lib.Revision++
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("cannot save plot library")
	}
	raw, _ := json.MarshalIndent(lib, "", "  ")
	tmp, err := os.CreateTemp(filepath.Dir(path), ".plots-*")
	if err != nil {
		return "", fmt.Errorf("cannot save plot library")
	}
	_, err = tmp.Write(raw)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("cannot save plot library")
	}
	verb := "Added"
	if created {
		verb = "Created dashboard and added"
	}
	return fmt.Sprintf("%s %d chart(s) to the in-app dashboard %q (Plots & dashboard). The user can open it from the sidebar.", verb, len(a.Plots), name), nil
}
