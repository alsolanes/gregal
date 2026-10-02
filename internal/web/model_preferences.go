package web

import (
	"time"

	"gregal/internal/session"
)

const modelAvailabilityCacheTTL = time.Minute

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
func (s *Server) refreshModelAvailability(role string) (string, bool) {
	s.mu.Lock()
	selection := s.modelOverride[role]
	if selection == "" {
		s.mu.Unlock()
		return "", false
	}
	if time.Since(s.modelCheckedAt[role]) < modelAvailabilityCacheTTL {
		unavailable := s.unavailableModels[role] == selection
		s.mu.Unlock()
		return selection, unavailable
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
