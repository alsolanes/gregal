package web

import (
	"time"

	"gregal/internal/config"
	"gregal/internal/session"
)

const modelAvailabilityCacheTTL = time.Minute

// windsModelScope és la clau del model per defecte dels vents: no és un rol
// del config (no defineix temperatura ni límits), només una preferència de
// model que windRoleRef aplica damunt del rol base del servidor.
const windsModelScope = "vents"

// windRoleRef resol amb quin provider/model parlen els vents: el rol base
// del servidor, però amb el model per defecte propi si l'usuari n'ha triat
// un (POST /api/model amb scope "vents"). Un override que ja no existeix al
// config cau al rol base: el vent mai es queda mut per un model esborrat.
// Cal cridar-lo amb s.mu agafat, com roleRef().
func (s *Server) windRoleRef() (config.Provider, config.Role) {
	p, r := s.roleRef()
	ov, ok := s.modelOverride[windsModelScope]
	if !ok || s.unavailableModels[windsModelScope] == ov {
		return p, r
	}
	if pp, m, ok := splitModel(ov); ok {
		if prov, ok := s.cfg.Providers[pp]; ok {
			r.Provider, r.Model = pp, m
			return prov, r
		}
		return p, r
	}
	if prov, ok := s.cfg.Providers[r.Provider]; ok {
		r.Model = ov
		return prov, r
	}
	return p, r
}

func (s *Server) defaultModelOverrides() map[string]string {
	user := ""
	if s != nil && s.user != nil {
		user = s.user.Name
	}
	defaults, err := session.LoadModelDefaults(user)
	if err != nil || defaults == nil {
		return map[string]string{}
	}
	return cloneModelOverrides(defaults)
}

func cloneModelOverrides(overrides map[string]string) map[string]string {
	copy := make(map[string]string, len(overrides))
	for role, selection := range overrides {
		copy[role] = selection
	}
	return copy
}

// refreshModelAvailability checks a persisted override outside s.mu. A stale
// or removed model falls back to the role's configured model, while transient
// provider errors leave the user's selection untouched.
//
// Only the first check for a role waits for the provider. After that a
// stale result is used at once and refreshed in the background: the
// synchronous GET /models ran before every turn that started more than a
// minute after the previous one (most human turns) and cost 0.4-4 s on the
// critical path.
func (s *Server) refreshModelAvailability(role string) (string, bool) {
	s.mu.Lock()
	selection := s.modelOverride[role]
	if selection == "" {
		s.mu.Unlock()
		return "", false
	}
	checked := s.modelCheckedAt[role]
	if !checked.IsZero() {
		unavailable := s.unavailableModels[role] == selection
		if time.Since(checked) >= modelAvailabilityCacheTTL {
			// Marked now so concurrent turns don't start several refreshes.
			s.modelCheckedAt[role] = time.Now()
			go s.checkModelAvailability(role)
		}
		s.mu.Unlock()
		return selection, unavailable
	}
	s.mu.Unlock()
	return s.checkModelAvailability(role)
}

// checkModelAvailability asks the provider whether the override still exists.
func (s *Server) checkModelAvailability(role string) (string, bool) {
	s.mu.Lock()
	selection := s.modelOverride[role]
	if selection == "" {
		s.mu.Unlock()
		return "", false
	}
	providerName, modelName, ok := splitModel(selection)
	if !ok {
		providerName = s.cfg.Roles[role].Provider
		modelName = selection
	}
	provider, providerExists := s.cfg.Providers[providerName]
	s.mu.Unlock()
	if !providerExists {
		s.mu.Lock()
		if s.modelOverride[role] == selection {
			if s.unavailableModels == nil {
				s.unavailableModels = map[string]string{}
			}
			if s.modelCheckedAt == nil {
				s.modelCheckedAt = map[string]time.Time{}
			}
			s.unavailableModels[role] = selection
			s.modelCheckedAt[role] = time.Now()
		}
		s.mu.Unlock()
		return selection, true
	}

	ids, err := fetchModelIDs(provider.BaseURL, provider.APIKey)
	if err != nil {
		s.mu.Lock()
		unavailable := s.modelOverride[role] == selection && s.unavailableModels[role] == selection
		s.mu.Unlock()
		return selection, unavailable
	}
	available := false
	for _, id := range ids {
		if id == modelName {
			available = true
			break
		}
	}
	s.mu.Lock()
	if s.modelOverride[role] == selection {
		if s.modelCheckedAt == nil {
			s.modelCheckedAt = map[string]time.Time{}
		}
		if s.unavailableModels == nil {
			s.unavailableModels = map[string]string{}
		}
		s.modelCheckedAt[role] = time.Now()
		if available {
			delete(s.unavailableModels, role)
		} else {
			s.unavailableModels[role] = selection
		}
	}
	s.mu.Unlock()
	return selection, !available
}
