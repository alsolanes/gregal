package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/session"
)

// Plot specs contain data only; executable Chart.js options never cross the API.
type plotSeries struct {
	Label  string    `json:"label"`
	Values []float64 `json:"values"`
}

func (series *plotSeries) UnmarshalJSON(raw []byte) error {
	var input struct {
		Label  string     `json:"label"`
		Values []*float64 `json:"values"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		return err
	}
	series.Label = input.Label
	series.Values = make([]float64, len(input.Values))
	for i, value := range input.Values {
		if value == nil {
			return fmt.Errorf("values must be numbers, not null")
		}
		series.Values[i] = *value
	}
	return nil
}

type plotSpec struct {
	Title  string       `json:"title"`
	Type   string       `json:"type"`
	Labels []string     `json:"labels"`
	Series []plotSeries `json:"series"`
}
type savedPlot struct {
	ID        string   `json:"id"`
	Spec      plotSpec `json:"spec"`
	Session   string   `json:"session"`
	Created   string   `json:"created"`
	Dashboard bool     `json:"dashboard"`
	Board     string   `json:"board"`
	Width     int      `json:"width"`
	Note      string   `json:"note"`
}
type plotLibrary struct {
	Revision int         `json:"revision"`
	Plots    []savedPlot `json:"plots"`
	Boards   []plotBoard `json:"boards"`
}

type plotBoard struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var plotStoreMu sync.Mutex

const plotPrompt = "\n\nThe web chat supports inline plots. When a user requests a chart, use a fenced gregal-plot block containing JSON: {\"title\":\"Revenue\",\"type\":\"line\",\"labels\":[\"Jan\",\"Feb\"],\"series\":[{\"label\":\"EUR\",\"values\":[10,20]}]}. Types: line, bar, scatter. For scatter, labels are numeric x coordinates written as strings. Each series must have one finite numeric value per label. Maximum 2000 labels and 20 series. Use actual inspected or user-provided data; clearly identify illustrative data. The user can save the plot and add it to the project dashboard. Do not claim it has been saved automatically. No HTML or JavaScript is needed."

func validatePlot(p plotSpec) error {
	if strings.TrimSpace(p.Title) == "" || len(p.Title) > 200 {
		return fmt.Errorf("title must contain 1–200 bytes")
	}
	if p.Type != "line" && p.Type != "bar" && p.Type != "scatter" {
		return fmt.Errorf("type must be line, bar or scatter")
	}
	if len(p.Labels) == 0 || len(p.Labels) > 2000 || len(p.Series) == 0 || len(p.Series) > 20 {
		return fmt.Errorf("expected 1–2000 labels and 1–20 series")
	}
	for _, label := range p.Labels {
		if len(label) > 200 {
			return fmt.Errorf("label too long")
		}
	}
	for _, series := range p.Series {
		if len(series.Label) > 200 || len(series.Values) != len(p.Labels) {
			return fmt.Errorf("series values must match labels")
		}
		for _, value := range series.Values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("values must be finite")
			}
		}
	}
	if p.Type == "scatter" {
		for _, label := range p.Labels {
			var x *float64
			if err := json.Unmarshal([]byte(label), &x); err != nil || x == nil || math.IsNaN(*x) || math.IsInf(*x, 0) {
				return fmt.Errorf("scatter labels must be numeric x coordinates")
			}
		}
	}
	return nil
}

func (s *Server) plotStorePath() string {
	s.mu.Lock()
	cwd, user := s.cwd, s.user
	s.mu.Unlock()
	name := ""
	if user != nil {
		name = user.Name
	}
	root, _ := filepath.Abs(cwd)
	if filepath.Separator == '\\' {
		root = strings.ToLower(root)
	}
	sum := sha256.Sum256([]byte(name + "\x00" + filepath.Clean(root)))
	return filepath.Join(session.DirFor(name), "plots", fmt.Sprintf("%x.json", sum))
}

// PATCH uses optimistic concurrency so two sessions cannot silently overwrite
// each other's dashboard edits. The store mutex also covers atomic disk writes.
func (s *Server) handlePlots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "GET, POST, PATCH, DELETE")
		http.Error(w, "method not allowed", 405)
		return
	}
	path := s.plotStorePath()
	plotStoreMu.Lock()
	defer plotStoreMu.Unlock()
	lib := plotLibrary{Plots: []savedPlot{}}
	raw, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(raw, &lib)
	}
	if err != nil && !os.IsNotExist(err) {
		http.Error(w, "cannot read plot library", 500)
		return
	}
	if len(lib.Boards) == 0 {
		lib.Boards = []plotBoard{{ID: "default", Name: "Dashboard"}}
	}
	for i := range lib.Plots {
		if lib.Plots[i].Board == "" {
			lib.Plots[i].Board = "default"
		}
	}
	if r.Method != http.MethodGet {
		var req struct {
			Revision      int      `json:"revision"`
			ID            string   `json:"id"`
			Spec          plotSpec `json:"spec"`
			Dashboard     bool     `json:"dashboard"`
			Width         int      `json:"width"`
			Note          string   `json:"note"`
			Direction     int      `json:"direction"`
			DashboardOnly bool     `json:"dashboard_only"`
			Operation     string   `json:"operation"`
			Name          string   `json:"name"`
			Board         string   `json:"board"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			http.Error(w, "invalid plot request", 400)
			return
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			http.Error(w, "expected one JSON object", 400)
			return
		}
		if req.Revision != lib.Revision {
			http.Error(w, "plot library changed; reload and retry", 409)
			return
		}
		if req.Board == "" {
			req.Board = "default"
		}
		boardIndex := -1
		for i, board := range lib.Boards {
			if board.ID == req.Board {
				boardIndex = i
				break
			}
		}
		if req.Operation != "" {
			if r.Method != http.MethodPost {
				http.Error(w, "dashboard operations require POST", 400)
				return
			}
			switch req.Operation {
			case "create_board", "rename_board":
				name := strings.TrimSpace(req.Name)
				if name == "" || len(name) > 200 {
					http.Error(w, "expected dashboard name (1–200 bytes)", 400)
					return
				}
				if req.Operation == "create_board" {
					if len(lib.Boards) >= 30 {
						http.Error(w, "dashboard limit reached (30)", 400)
						return
					}
					lib.Boards = append(lib.Boards, plotBoard{ID: fmt.Sprintf("b%d", time.Now().UnixNano()), Name: name})
				} else {
					if boardIndex < 0 {
						http.Error(w, "dashboard not found", 404)
						return
					}
					lib.Boards[boardIndex].Name = name
				}
			case "delete_board":
				if boardIndex < 0 {
					http.Error(w, "dashboard not found", 404)
					return
				}
				if req.Board == "default" {
					http.Error(w, "default dashboard cannot be deleted", 400)
					return
				}
				lib.Boards = append(lib.Boards[:boardIndex], lib.Boards[boardIndex+1:]...)
				for i := range lib.Plots {
					if lib.Plots[i].Board == req.Board {
						lib.Plots[i].Board = "default"
						lib.Plots[i].Dashboard = false
					}
				}
			default:
				http.Error(w, "unknown dashboard operation", 400)
				return
			}
		} else if r.Method == http.MethodPost {
			if boardIndex < 0 {
				http.Error(w, "dashboard not found", 404)
				return
			}
			if err := validatePlot(req.Spec); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			if len(lib.Plots) >= 200 {
				http.Error(w, "plot library is full (200 plots)", 400)
				return
			}
			s.mu.Lock()
			origin := s.id
			s.mu.Unlock()
			lib.Plots = append(lib.Plots, savedPlot{ID: fmt.Sprintf("%d", time.Now().UnixNano()), Spec: req.Spec, Session: origin, Created: time.Now().UTC().Format(time.RFC3339), Dashboard: req.Dashboard, Board: req.Board, Width: 1})
		} else {
			idx := -1
			for i, p := range lib.Plots {
				if p.ID == req.ID {
					idx = i
					break
				}
			}
			if idx < 0 {
				http.Error(w, "plot not found", 404)
				return
			}
			if r.Method == http.MethodDelete {
				lib.Plots = append(lib.Plots[:idx], lib.Plots[idx+1:]...)
			} else {
				if boardIndex < 0 {
					http.Error(w, "dashboard not found", 404)
					return
				}
				if req.Width != 1 && req.Width != 2 || len(req.Note) > 4000 || req.Direction < -1 || req.Direction > 1 {
					http.Error(w, "invalid dashboard layout", 400)
					return
				}
				if req.Spec.Title != "" {
					if err := validatePlot(req.Spec); err != nil {
						http.Error(w, err.Error(), 400)
						return
					}
					lib.Plots[idx].Spec = req.Spec
				}
				lib.Plots[idx].Dashboard = req.Dashboard
				lib.Plots[idx].Board = req.Board
				lib.Plots[idx].Width = req.Width
				lib.Plots[idx].Note = req.Note
				next := idx + req.Direction
				if req.Direction != 0 && req.DashboardOnly {
					for next >= 0 && next < len(lib.Plots) && (!lib.Plots[next].Dashboard || lib.Plots[next].Board != req.Board) {
						next += req.Direction
					}
				}
				if next >= 0 && next < len(lib.Plots) {
					lib.Plots[idx], lib.Plots[next] = lib.Plots[next], lib.Plots[idx]
				}
			}
		}
		lib.Revision++
		raw, err = json.Marshal(lib)
		if err == nil {
			err = os.MkdirAll(filepath.Dir(path), 0700)
		}
		var tmp *os.File
		if err == nil {
			tmp, err = os.CreateTemp(filepath.Dir(path), ".plots-*")
		}
		if err == nil {
			defer os.Remove(tmp.Name())
			if err = tmp.Chmod(0600); err == nil {
				_, err = tmp.Write(raw)
			}
			if err == nil {
				err = tmp.Sync()
			}
			closeErr := tmp.Close()
			if err == nil {
				err = closeErr
			}
			if err == nil {
				err = os.Rename(tmp.Name(), path)
			}
		}
		if err != nil {
			http.Error(w, "cannot save plot library", 500)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(lib)
}
