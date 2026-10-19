package config

import "gregal/internal/skills"

// SkillsCfg controla les habilitats especialistes opcionals.
type SkillsCfg struct {
	// Nil conserva els valors per defecte; una llista buida vol dir totes apagades.
	Enabled []string `yaml:"enabled"`
}

// Actives aplica els valors per defecte a configuracions antigues sense secció skills.
func (c SkillsCfg) Actives() []string {
	if c.Enabled == nil {
		return skills.PerDefecte()
	}
	return append([]string(nil), c.Enabled...)
}
