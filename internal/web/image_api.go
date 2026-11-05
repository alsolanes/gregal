package web

import (
	"encoding/json"
	"net/http"

	"gregal/internal/tools"
)

// POST /api/image/normalize {image: data URL} → {image: data URL}. El
// navegador (fora de Safari) no sap pintar una foto HEIC: la web la passa per
// aquí en adjuntar-la i rep un JPEG que ja pot previsualitzar i enviar.
func (s *Server) handleImageNormalize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Image string `json:"image"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "petició il·legible", http.StatusBadRequest)
		return
	}
	out, err := tools.NormalizeDataURL(req.Image, "imatge")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]string{"image": out})
}
