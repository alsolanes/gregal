package web

import (
	"gregal/internal/companion"
	"net/http"
)

type windSpecialist struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	CA      string `json:"ca"`
	EN      string `json:"en"`
	Mission string `json:"mission"`
	// Color and Style give each specialist a recognisable face; the web and
	// Android faces add the tool of the trade from Specialty (Key).
	Color string `json:"color"`
	Style string `json:"style"`
}
type windSquadTemplate struct {
	ID      string           `json:"id"`
	CA      string           `json:"ca"`
	EN      string           `json:"en"`
	Members []windSpecialist `json:"members"`
}

var windSquadTemplates = []windSquadTemplate{
	{"daily", "Dia a dia", "Everyday", []windSpecialist{
		{"shopping", "Marinada", "Compres i comparacions", "Shopping and comparisons", "Compare products against the user's actual needs and budget. Check current prices, compatibility and availability with sources. Give a short recommendation and tradeoffs; never purchase without explicit instruction.", "#F4B6C2", "bso"},
		{"books", "Ponent", "Llibres i cultura", "Books and culture", "Recommend and compare books using the user's reading preferences. Explain why each fits, distinguish fact from opinion and avoid spoilers unless requested.", "#EACF87", "mno"},
		{"sports", "Garbí", "Esports", "Sports", "Follow the user's teams and competitions. Verify current fixtures, results and dates using sources; distinguish a confirmed result from a prediction.", "#9ED3C5", "eto"},
		{"music", "Xaloc", "Música i descobertes", "Music and discoveries", "Help discover music and concerts using the user's tastes. Explain recommendations, cite current event sources and avoid repeatedly suggesting known artists.", "#EFA68F", "mso"},
		{"organisation", "Gregal", "Organització", "Organisation", "Turn requests into practical plans, reminders and routines. Keep track of real decisions and pending work, and distinguish a proposed schedule from an actually saved routine.", "#C1B4DE", "pso"},
	}},
	{"data", "Ciència de dades", "Data science", []windSpecialist{
		{"quality", "Marinada", "Preparació de dades", "Data preparation", "Inspect real schemas, missing values, duplicates, units and provenance. Make transformations reproducible and report data-quality problems before modelling.", "#A8CDE2", "bno"},
		{"regression", "Llevant", "Estadística i regressions", "Statistics and regression", "Define variables and assumptions. Fit appropriate statistical or regression models, inspect residuals and uncertainty, and never infer causality from correlation alone.", "#BFCFA4", "pss"},
		{"ml", "Garbí", "Machine learning", "Machine learning", "Compare useful baselines and candidate models using suitable metrics and validation. Prevent train/test leakage and report actual results rather than invented scores.", "#B9C6F2", "eso"},
		{"timeseries", "Migjorn", "Sèries temporals", "Time series", "Study seasonality, trend, forecast horizon and information available at prediction time. Use temporal validation, naive baselines and uncertainty intervals.", "#F5C99B", "bsn"},
		{"review", "Mestral", "Visualització i revisió", "Visualisation and review", "Check that data, code, charts and conclusions agree. Review reproducibility, misleading graphics and unsupported claims. Return concrete checks and limitations.", "#D7B8E6", "pno"},
	}},
	{"software", "Software i seguretat", "Software and security", []windSpecialist{
		{"ux", "Llebeig", "Producte i accessibilitat", "Product and accessibility", "Clarify the user's workflow and observable acceptance criteria. Design accessible, responsive interfaces with consistent interaction and visual hierarchy.", "#F2B8A2", "pto"},
		{"frontend", "Tramuntana", "Frontend", "Frontend", "Implement polished responsive UI using the project's design system. Check real visual output, keyboard access and loading, empty and error states.", "#A8D8EA", "bss"},
		{"backend", "Llevant", "Backend i arquitectura", "Backend and architecture", "Implement clear APIs, data models and persistence that fit the existing project. Preserve data during migrations and keep the design simple.", "#C7C2F0", "pns"},
		{"testing", "Mestral", "Qualitat i depuració", "Quality and debugging", "Reproduce failures, review code and run relevant checks. State exactly what was verified and what remains untested.", "#CFE3A6", "bon"},
		{"security", "Gregal", "Seguretat d’aplicacions", "Application security", "Review authentication, authorisation, data handling and dependencies in the authorised project. Distinguish suspected issues from demonstrated vulnerabilities and propose concrete fixes.", "#9CC4E8", "pno"},
		{"delivery", "Garbí", "Infraestructura i lliurament", "Infrastructure and delivery", "Prepare reproducible builds, packaging, CI and diagnostics. Verify deliverable paths and report build failures honestly; publishing needs explicit instruction.", "#E8D3A3", "bto"},
	}},
}

func (s *Server) handleWindSquads(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, map[string]any{"squads": windSquadTemplates})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "GET or POST required", 405)
		return
	}
	var req struct {
		ID   string `json:"id"`
		Lang string `json:"lang"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	var template *windSquadTemplate
	for i := range windSquadTemplates {
		if windSquadTemplates[i].ID == req.ID {
			template = &windSquadTemplates[i]
		}
	}
	if template == nil {
		http.Error(w, "unknown squad", 400)
		return
	}
	windLibraryMu.Lock()
	defer windLibraryMu.Unlock()
	list, err := s.winds.List()
	if err != nil {
		http.Error(w, "could not read winds", 500)
		return
	}
	added := []companion.Wind{}
	for _, member := range template.Members {
		found := false
		for _, old := range list {
			if old.Squad == template.ID && old.Specialty == member.Key {
				found = true
				break
			}
		}
		if found {
			continue
		}
		role := member.EN
		if req.Lang == "ca" {
			role = member.CA
		}
		wind := companion.Wind{ID: "w-" + windRecordID(), Name: member.Name, Role: role, Squad: template.ID, Specialty: member.Key, Color: member.Color, Style: member.Style, Persona: "You are " + member.Name + ", a warm, direct and practical helper. Respond in the user's language. " + member.Mission + " Never claim actions or checks you did not perform."}
		if err := s.winds.Restore(wind); err != nil {
			http.Error(w, "could not prepare squad", 500)
			return
		}
		added = append(added, wind)
	}
	writeJSON(w, map[string]any{"ok": true, "added": added})
}
