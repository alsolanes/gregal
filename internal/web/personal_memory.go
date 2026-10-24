package web

import (
	"path/filepath"

	"gregal/internal/companion"
	"gregal/internal/config"
)

// windByRef resol un vent per id o nom (els models confonen l'id llarg
// amb el nom i així els recordatoris no fallen).
func windByRef(s *Server, ref string) (companion.Wind, bool) {
	if s == nil || s.winds == nil {
		return companion.Wind{}, false
	}
	return s.winds.ByRef(ref)
}

// personalMemoryPath follows the authenticated user, never the browser's session
// ID or process-global environment. Legacy Tothom/local keeps the global file.
func (s *Server) personalMemoryPath() string {
	if s.user != nil && !s.user.Tothom {
		return filepath.Join(userStateDir(s.cfgPath, s.user.Name), "memory.md")
	}
	return config.UserMemoryPath()
}

func (s *Server) personalMemory() string  { return config.LoadUserMemoryAt(s.personalMemoryPath()) }
func (s *Server) personalNotes() []string { return config.RecallAt(s.personalMemoryPath(), "") }
func (s *Server) rememberPersonal(text string) error {
	return config.RememberAt(s.personalMemoryPath(), text)
}
func (s *Server) forgetPersonal(term string) (int, error) {
	return config.ForgetAt(s.personalMemoryPath(), term)
}
