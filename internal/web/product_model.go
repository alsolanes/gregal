package web

import (
	"errors"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// El model d'un producte amb perfil propi (Gregal Jocs): saber si ja en té
// un que respongui i, la primera vegada, adoptar el de Gregal (una còpia;
// jocs/server/model_import.go en llegeix l'origen).

// ModelReady diu si el model actiu pot respondre: el proveïdor existeix i
// té clau, o és un servidor local. És el mateix que mira el diàleg del
// primer inici (app/onboarding.js).
func (s *Server) ModelReady() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		return false
	}
	p, r := s.roleRef()
	if _, ok := s.cfg.Providers[r.Provider]; !ok {
		return false
	}
	return p.HasAPIKey() || llm.IsLocalBase(p.BaseURL)
}

// AdoptModel copia un model al config d'aquest servidor: afegeix (o
// substitueix) el proveïdor i hi posa tots els rols, com quan tries un
// proveïdor al diàleg del primer inici. La clau literal va al magatzem de
// claus d'aquest perfil (auth.yaml), mai al config.yaml; una ${VAR} es
// copia com a referència.
func (s *Server) AdoptModel(m config.ImportedModel) error {
	if !validProvName(m.Provider) || m.Model == "" {
		return errors.New("invalid model")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cfg == nil {
		return errors.New("no configuration")
	}
	oldProviders, oldRoles := s.cfg.Providers, s.cfg.Roles
	providers := make(map[string]config.Provider, len(oldProviders)+1)
	for n, p := range oldProviders {
		providers[n] = p
	}
	p := m.Settings
	p.APIKey = m.EffectiveKey()
	providers[m.Provider] = p
	roles := make(map[string]config.Role, len(oldRoles))
	for n, r := range oldRoles {
		r.Provider, r.Model = m.Provider, m.Model
		r.ContextWindow, r.FallbackProvider, r.FallbackModel, r.Think = m.Role.ContextWindow, "", "", ""
		roles[n] = r
	}
	s.cfg.Providers, s.cfg.Roles = providers, roles
	s.cfg.SetRawKey(m.Provider, m.RawKey)
	revert := func() { s.cfg.Providers, s.cfg.Roles = oldProviders, oldRoles }
	if m.Key != "" {
		if err := config.SaveKey(m.Provider, m.Key); err != nil {
			revert()
			return err
		}
	} else if err := forgetStoredKey(m.Provider); err != nil {
		revert()
		return err
	}
	if err := s.cfg.Save(s.cfgPath); err != nil {
		revert()
		return err
	}
	return nil
}
