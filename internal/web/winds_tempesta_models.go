package web

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"gregal/internal/session"
)

// «A fons» (winds_tempesta.go) amb els models que tria l'usuari: un per a
// cada angle i un per a la síntesi. Es desen amb les altres preferències de
// model de l'usuari (com el model dels vents, scope «vents»), amb la clau
// «~tempesta.<slot>». Sense tria, o si el model ja no existeix, mana la tria
// automàtica de tempestaModels i tempestaSynthRole.

// tempestaSlots: els angles de tempestaAngles i la síntesi.
var tempestaSlots = []string{"direct", "sources", "skeptic", "synth"}

// tempestaAuto es desa quan l'usuari torna a «Automàtic»: el fitxer de
// preferències no admet valors buits.
const tempestaAuto = "auto"

func tempestaModelKey(slot string) string { return "~tempesta." + slot }

// tempestaDescs: què fa cada agent, per a l'app (la consigna del model és
// el brief de tempestaAngles).
var tempestaDescs = map[string][2]string{
	"direct":  {"Respon la pregunta pas a pas i comprova els fets clau amb eines.", "Answers the question step by step and checks the key facts with tools."},
	"sources": {"Busca fonts oficials i actuals i hi basa cada afirmació.", "Looks for official, current sources and bases every claim on them."},
	"skeptic": {"Busca el parany, el cas límit o l'alternativa que faria fallar la resposta òbvia.", "Looks for the trap, edge case or better option that would make the obvious answer wrong."},
}

// tempestaChosen és el model que l'usuari ha triat per a slot, si encara
// es pot fer servir. Conserva la resta del rol dels vents (temperatura,
// tokens). Cal s.mu agafat.
func (s *Server) tempestaChosen(slot string) (routedRole, bool) {
	key := tempestaModelKey(slot)
	ov := s.modelOverride[key]
	pName, m, ok := splitModel(ov)
	if !ok || s.unavailableModels[key] == ov {
		return routedRole{}, false
	}
	prov, ok := s.cfg.Providers[pName]
	if !ok {
		return routedRole{}, false
	}
	_, r := s.windRoleRef()
	r.Provider, r.Model = pName, m
	return routedRole{p: prov, role: r}, true
}

// refreshTempestaModels comprova (amb la memòria cau de
// refreshModelAvailability) que els models triats encara existeixen.
func (s *Server) refreshTempestaModels() {
	for _, slot := range tempestaSlots {
		s.mu.Lock()
		_, _, chosen := splitModel(s.modelOverride[tempestaModelKey(slot)])
		s.mu.Unlock()
		if chosen {
			s.refreshModelAvailability(tempestaModelKey(slot))
		}
	}
}

// tempestaChoices: slot → "provider/model" triat, o "" (automàtic). Cal s.mu agafat.
func (s *Server) tempestaChoices() map[string]string {
	out := map[string]string{}
	for _, slot := range tempestaSlots {
		v := s.modelOverride[tempestaModelKey(slot)]
		if _, _, ok := splitModel(v); !ok {
			v = ""
		}
		out[slot] = v
	}
	return out
}

// GET/POST /api/winds/tempesta/models: els models d'«A fons». POST
// {slot, model} desa un slot; model buit = automàtic.
func (s *Server) handleTempestaModels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
	case http.MethodPost:
		var req struct {
			Slot  string `json:"slot"`
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "petició il·legible", http.StatusBadRequest)
			return
		}
		if !slices.Contains(tempestaSlots, req.Slot) {
			http.Error(w, "agent desconegut", http.StatusBadRequest)
			return
		}
		value := tempestaAuto
		if raw := strings.TrimSpace(req.Model); raw != "" {
			pName, m, ok := splitModel(raw)
			s.mu.Lock()
			prov, found := s.cfg.Providers[pName]
			s.mu.Unlock()
			if !ok || !found || strings.TrimSpace(m) == "" {
				http.Error(w, "cal provider/model d'un provider configurat", http.StatusBadRequest)
				return
			}
			ids, err := fetchModelIDs(prov.BaseURL, prov.APIKey)
			if err != nil {
				http.Error(w, "no puc validar els models del provider: "+err.Error(), http.StatusBadGateway)
				return
			}
			if !slices.Contains(ids, m) {
				http.Error(w, "model no disponible al provider", http.StatusBadRequest)
				return
			}
			value = pName + "/" + m
		}
		key := tempestaModelKey(req.Slot)
		s.mu.Lock()
		if err := session.SaveModelDefault(s.usuari(), key, value); err != nil {
			s.mu.Unlock()
			http.Error(w, "no puc desar la preferència de model: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.modelOverride[key] = value
		delete(s.unavailableModels, key)
		delete(s.modelCheckedAt, key)
		s.autosaveLocked()
		s.mu.Unlock()
	default:
		http.Error(w, "mètode no permès", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	models := s.tempestaChoices()
	s.mu.Unlock()
	writeJSON(w, map[string]any{"models": models})
}
