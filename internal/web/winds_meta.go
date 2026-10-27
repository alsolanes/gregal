package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"gregal/internal/companion"
)

// Les preferències d'organització de la llista de converses: favorits
// (surten a dalt de tot) i seccions temàtiques creades per l'usuari.
// Es desen al winds.json del servidor i valen igual per al mòbil i la web.

// handleWindsMeta GET torna les preferències; POST les desa senceres
// (la UI hi envia sempre l'estat complet, com fa amb els vents).
func (s *Server) handleWindsMeta(w http.ResponseWriter, r *http.Request) {
	if s.winds == nil {
		http.Error(w, "vents no disponibles", 500)
		return
	}
	if r.Method == http.MethodGet {
		meta, err := s.winds.Meta()
		if err != nil {
			http.Error(w, "no s'han pogut llegir les preferències", 500)
			return
		}
		writeJSON(w, map[string]any{"meta": meta})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "només GET o POST", 405)
		return
	}
	var meta companion.WindMeta
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&meta); err != nil {
		http.Error(w, "cos no vàlid", 400)
		return
	}
	// Claus de fila vàlides: "wind:<id>" o "rot:<id>"; una mateixa fila
	// no pot repetir-se ni a favorits ni dins una secció.
	clean := func(keys []string) []string {
		seen := map[string]bool{}
		out := make([]string, 0, len(keys))
		for _, k := range keys {
			if !strings.HasPrefix(k, "wind:") && !strings.HasPrefix(k, "rot:") {
				continue
			}
			k = strings.TrimSpace(k)
			if k == "" || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, k)
		}
		return out
	}
	meta.Favorites = clean(meta.Favorites)
	seenIDs := map[string]bool{}
	for i := range meta.Sections {
		meta.Sections[i].Name = strings.TrimSpace(meta.Sections[i].Name)
		meta.Sections[i].Items = clean(meta.Sections[i].Items)
		if meta.Sections[i].Name == "" {
			http.Error(w, "la secció necessita un nom", 400)
			return
		}
		if meta.Sections[i].ID == "" {
			meta.Sections[i].ID = slugSection(meta.Sections[i].Name)
		}
		if seenIDs[meta.Sections[i].ID] {
			http.Error(w, "id de secció repetit", 400)
			return
		}
		seenIDs[meta.Sections[i].ID] = true
	}
	if err := s.winds.SaveMeta(meta); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	writeJSON(w, map[string]any{"meta": meta})
}

// slugSection fa un id curt i estable a partir del nom de la secció.
func slugSection(name string) string {
	s := strings.ToLower(strings.Join(strings.Fields(name), "-"))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = out[:40]
	}
	if out == "" {
		out = "seccio"
	}
	return out
}
