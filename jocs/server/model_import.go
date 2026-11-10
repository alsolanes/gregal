package server

// «Fes servir el mateix model que Gregal»: Gregal Jocs té el seu perfil
// (internal/profile) i arrenca sense model. Si el de Gregal en té un que
// respon, la pàgina dels jocs ho ofereix una vegada i, si dius que sí, se'n
// copia el proveïdor, el model i la clau al perfil de Gregal Jocs.
//
//   - Els fitxers de Gregal (~/.config/gregal/config.yaml i auth.yaml, i els
//     models triats a la UI a ~/.local/share/gregal/model-preferences) només
//     es llegeixen; no s'hi escriu mai.
//   - És una còpia única: si després canvies el model de Gregal, el de Gregal
//     Jocs no canvia (i a l'inrevés).
//   - L'oferta surt una sola vegada: en importar o en dir «ara no» queda
//     apuntat a <config de Gregal Jocs>/gregal-model-import.json.
//   - Només l'administrador (els proveïdors són de tot el servidor).

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/profile"
	"gregal/internal/session"
	"gregal/internal/web"
)

// modelImportFile és on queda apuntat que l'oferta ja s'ha fet servir.
const modelImportFile = "gregal-model-import.json"

type modelImportMark struct {
	State    string `json:"state"` // imported | dismissed
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	At       int64  `json:"at"`
}

func (s *Server) modelImportPath() string { return filepath.Join(s.host.StateDir(), modelImportFile) }

func (s *Server) modelImportMark() modelImportMark {
	var m modelImportMark
	if raw, err := os.ReadFile(s.modelImportPath()); err == nil {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

// gregalModel llegeix (només lectura) el model que fa servir Gregal: el
// del seu config, o el que s'hi ha triat a la UI (mode local, sense comptes).
func gregalModel() (config.ImportedModel, error) {
	if profile.Product() == "" {
		// Aquest procés ja és Gregal: no hi ha res a importar.
		return config.ImportedModel{}, config.ErrNoProfileModel
	}
	sel, _ := session.LoadModelDefaultsIn(profile.DefaultDataDirFor(""), "")
	return config.ReadProfileModel(profile.ConfigDirFor(""), sel)
}

// usable diu si el model importat respondria: clau o servidor local.
func usable(m config.ImportedModel) bool {
	return m.EffectiveKey() != "" || llm.IsLocalBase(m.Settings.BaseURL)
}

func isAdmin(r *http.Request) bool {
	u := web.UserFrom(r)
	return u == nil || u.Admin
}

// handleModelImport: GET → {offer, provider, model, label, state};
// POST {action: import|dismiss}.
func (s *Server) handleModelImport(w http.ResponseWriter, r *http.Request) {
	mark := s.modelImportMark()
	if r.Method == http.MethodGet {
		out := map[string]any{"offer": false, "state": mark.State}
		if mark.State == "" && isAdmin(r) && !s.host.ModelReady() {
			if m, err := gregalModel(); err == nil && usable(m) {
				label := m.Provider
				if p, ok := config.ProveidorPerNom(m.Provider); ok && p.Etiqueta != "" {
					label = p.Etiqueta
				}
				out["offer"], out["provider"], out["model"], out["label"] = true, m.Provider, m.Model, label
			}
		}
		writeJSON(w, out)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !isAdmin(r) {
		writeJSONStatus(w, http.StatusForbidden, map[string]string{"error": s.tr("Només l'administrador pot triar el model.", "Only the administrator can choose the model.")})
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	switch req.Action {
	case "dismiss":
		if mark.State == "" {
			if err := writeJSONFile(s.modelImportPath(), modelImportMark{State: "dismissed", At: time.Now().Unix()}); err != nil {
				writeJSONStatus(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		writeJSON(w, map[string]bool{"ok": true})
	case "import":
		m, err := s.importGregalModel(mark)
		if err != nil {
			writeJSONStatus(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"ok": true, "provider": m.Provider, "model": m.Model})
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
	}
}

// importGregalModel fa la còpia (una sola vegada, i només si Gregal Jocs
// encara no té cap model que respongui).
func (s *Server) importGregalModel(mark modelImportMark) (config.ImportedModel, error) {
	if mark.State == "imported" {
		return config.ImportedModel{}, errConst(s.tr("El model de Gregal ja s'ha importat.", "Gregal's model was already imported."))
	}
	if s.host.ModelReady() {
		return config.ImportedModel{}, errConst(s.tr("Gregal Jocs ja té un model.", "Gregal Jocs already has a model."))
	}
	m, err := gregalModel()
	if err != nil || !usable(m) {
		if err != nil && !errors.Is(err, config.ErrNoProfileModel) && !errors.Is(err, os.ErrNotExist) {
			return m, err
		}
		return m, errConst(s.tr("Gregal no té cap model a punt per copiar.", "Gregal has no working model to copy."))
	}
	if err := s.host.AdoptModel(m); err != nil {
		return m, err
	}
	if err := writeJSONFile(s.modelImportPath(), modelImportMark{State: "imported", Provider: m.Provider, Model: m.Model, At: time.Now().Unix()}); err != nil {
		return m, err
	}
	return m, nil
}
