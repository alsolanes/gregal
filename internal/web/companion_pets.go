package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/companion"
	"gregal/internal/config"
	"gregal/internal/llm"
)

// Mascotes del company: catàleg + check-in amb la veu de cada mascota.
//
//	GET  /api/companion/pets          → {pets:[{id,name,role}]} (sense persones)
//	POST /api/companion/checkin{pet}  → check-in amb la persona de la mascota

// handleCompanionPets torna el catàleg de mascotes. La persona NO viatja al
// client: és prompt de sistema del backend.
func (s *Server) handleCompanionPets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	pets := companion.Pets()
	out := make([]map[string]any, 0, len(pets))
	for _, p := range pets {
		out = append(out, companion.PetDigest(p))
	}
	writeJSON(w, map[string]any{"pets": out})
}

// petCheckiner genera un check-in amb el system de la mascota; testeja'l.
var petCheckiner = defaultPetCheckin

func defaultPetCheckin(ctx context.Context, client *llm.Client, cfg *config.Config, p config.Provider, role config.Role, system, user string) (string, error) {
	msgs := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	out, err := agent.SideCallLateral(ctx, client, cfg, p, role, msgs)
	if err != nil {
		return "", err
	}
	return companion.TruncateCheckin(out), nil
}

// handleCompanionCheckinPet és el check-in amb camp pet opcional: amb
// mascota, la persona viatja com a system i la resposta marca qui ha parlat.
// Sense pet, delega al check-in base d'avui (cap canvi de comportament).
func (s *Server) handleCompanionCheckinPet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Pet string `json:"pet"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	petID := strings.TrimSpace(req.Pet)
	if petID == "" {
		s.handleCompanionCheckin(w, r)
		return
	}
	pet, ok := companion.PetByID(petID)
	if !ok {
		http.Error(w, "unknown pet", http.StatusBadRequest)
		return
	}
	p, role, ok2 := s.sideRoleRef()
	if !ok2 {
		http.Error(w, "no side role available", http.StatusServiceUnavailable)
		return
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	memory := s.personalMemory()
	digest := s.lastSessionDigest()
	system := companion.PetCheckinSystem(pet)
	user := companion.PetCheckinPrompt(pet, memory, digest)
	out, err := petCheckiner(ctx, client, cfg, p, role, system, user)
	if err != nil {
		http.Error(w, redactError(err), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]any{"checkin": out, "pet": pet.ID, "at": time.Now().UTC().Format(time.RFC3339)})
}
