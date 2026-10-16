package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"gregal/internal/brand"
)

// brandView és un paquet tal com el veu la pantalla de Brandings: el que
// porta (colors escrits, tipus de lletra, logo) i el nom de tema que fa
// servir el web (data-theme).
type brandView struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Version  string         `json:"version"`
	Author   string         `json:"author,omitempty"`
	Mode     string         `json:"mode"`
	Theme    string         `json:"theme"`
	Colors   brand.Colors   `json:"colors"`
	Fonts    brand.Fonts    `json:"fonts"`
	Logo     string         `json:"logo,omitempty"`
	Swatches []brand.Swatch `json:"swatches"`
	CSS      string         `json:"css"`
}

func brandViewOf(p *brand.Pack) brandView {
	return brandView{ID: p.ID, Name: p.Name, Version: p.Version, Author: p.Author, Mode: p.Mode,
		Theme: brand.ThemeName(p.ID), Colors: p.Colors, Fonts: p.Fonts, Logo: p.Logo,
		Swatches: p.Swatches(), CSS: p.CSS()}
}

func (s *Server) brandsState() map[string]any {
	list := []brandView{}
	for _, p := range brand.List() {
		list = append(list, brandViewOf(p))
	}
	s.mu.Lock()
	active := s.cfg.Brand
	s.mu.Unlock()
	if active != "" {
		if _, err := brand.Get(active); err != nil {
			active = ""
		}
	}
	return map[string]any{"active": active, "brands": list}
}

// readPack llegeix un paquet del cos de la petició (el fitxer tal qual).
func readPack(r *http.Request) (*brand.Pack, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, brand.MaxFile+1))
	if err != nil {
		return nil, err
	}
	return brand.Parse(raw)
}

// handleBrands: GET llista els paquets instal·lats i l'actiu; POST
// instal·la (o actualitza) el paquet del cos.
func (s *Server) handleBrands(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.brandsState())
	case http.MethodPost:
		if !brandAdmin(w, r) {
			return
		}
		p, err := readPack(r)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := brand.Install(p); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		st := s.brandsState()
		st["installed"] = p.ID
		writeJSON(w, st)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

// handleBrandPreview valida un fitxer sense instal·lar-lo i diu què
// passaria: paquet nou, actualització, la mateixa versió o una de més
// antiga que la instal·lada.
func (s *Server) handleBrandPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	p, err := readPack(r)
	if err != nil {
		writeJSONStatus(w, 400, map[string]any{"error": err.Error()})
		return
	}
	out := map[string]any{"brand": brandViewOf(p), "status": "new"}
	if old, err := brand.Get(p.ID); err == nil {
		out["installed_version"] = old.Version
		switch c := brand.CompareVersions(p.Version, old.Version); {
		case c > 0:
			out["status"] = "update"
		case c == 0:
			out["status"] = "same"
		default:
			out["status"] = "older"
		}
	}
	writeJSON(w, out)
}

// handleBrandActive fixa el paquet actiu (id "" = cap) i el desa al config.
func (s *Server) handleBrandActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	if !brandAdmin(w, r) {
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "unreadable request", 400)
		return
	}
	id := brand.Slug(req.ID)
	if id != "" {
		if _, err := brand.Get(id); err != nil {
			http.Error(w, "branding not installed", 404)
			return
		}
	}
	if err := s.setActiveBrand(id); err != nil {
		http.Error(w, "not saved: "+err.Error(), 500)
		return
	}
	writeJSON(w, s.brandsState())
}

func (s *Server) setActiveBrand(id string) error {
	s.mu.Lock()
	old := s.cfg.Brand
	s.cfg.Brand = id
	err := s.cfg.Save(s.cfgPath)
	if err != nil {
		s.cfg.Brand = old
	}
	s.mu.Unlock()
	if err == nil {
		brand.Activate(id)
	}
	return err
}

// handleBrandItem: GET /api/brands/{id}/file descarrega el paquet (per
// compartir-lo); DELETE /api/brands/{id} el desinstal·la.
func (s *Server) handleBrandItem(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/brands/"), "/")
	id, sub, _ := strings.Cut(rest, "/")
	id = brand.Slug(id)
	switch {
	case r.Method == http.MethodGet && sub == "file":
		p, err := brand.Get(id)
		if err != nil {
			http.Error(w, "branding not installed", 404)
			return
		}
		raw, err := p.Marshal()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="`+p.ID+`.gregal-brand.json"`)
		w.Write(raw)
	case r.Method == http.MethodDelete && sub == "":
		if !brandAdmin(w, r) {
			return
		}
		if err := brand.Remove(id); errors.Is(err, brand.ErrNotFound) {
			http.Error(w, "branding not installed", 404)
			return
		} else if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		s.mu.Lock()
		wasActive := s.cfg.Brand == id
		s.mu.Unlock()
		if wasActive {
			if err := s.setActiveBrand(""); err != nil {
				http.Error(w, "not saved: "+err.Error(), 500)
				return
			}
		}
		writeJSON(w, s.brandsState())
	default:
		http.Error(w, "not found", 404)
	}
}

// brandAdmin: el branding és de tot el servidor, així que amb diversos
// usuaris només l'administrador l'instal·la, el treu o l'activa.
func brandAdmin(w http.ResponseWriter, r *http.Request) bool {
	if u := userFrom(r); u != nil && !u.Admin {
		http.Error(w, "only the administrator can change the branding", 403)
		return false
	}
	return true
}
