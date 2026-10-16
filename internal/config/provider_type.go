package config

import (
	"fmt"
	"sort"
	"strings"

	"gregal/internal/llm"
)

// Protocol de cada provider (providers.<nom>.type).
//
// Buit (o «openai») és el protocol per defecte, chat completions
// OpenAI-compatible. «anthropic» parla l'API Messages, «openai-responses»
// l'API Responses, «ollama» l'API nativa d'Ollama i «llamacpp» chat
// completions amb els extres de llama-server (vegeu internal/llm/provider_type.go). El client tria el
// cable pel base_url, així que dos providers amb el mateix base_url i tipus
// diferents s'han de desempatar: mana el que fan servir els rols; si els
// rols en fan servir tots dos, és un error de configuració.

// tipusPerURL calcula el protocol de cada base_url i l'error de conflicte.
func (c *Config) tipusPerURL() (map[string]string, error) {
	if c == nil {
		return nil, nil
	}
	usats := map[string]bool{}
	for _, r := range c.Roles {
		usats[r.Provider] = true
		if r.FallbackProvider != "" {
			usats[r.FallbackProvider] = true
		}
	}
	type cand struct {
		nom, tipus string
		usat       bool
	}
	perURL := map[string][]cand{}
	noms := make([]string, 0, len(c.Providers))
	for nom := range c.Providers {
		noms = append(noms, nom)
	}
	sort.Strings(noms)
	for _, nom := range noms {
		p := c.Providers[nom]
		base := strings.TrimSuffix(strings.TrimSpace(p.BaseURL), "/")
		if base == "" {
			continue
		}
		t, ok := llm.NormalizeProviderType(p.Type)
		if !ok {
			return nil, fmt.Errorf("provider %q: type %q desconegut (openai, anthropic, openai-responses, ollama, llamacpp)", nom, p.Type)
		}
		perURL[base] = append(perURL[base], cand{nom: nom, tipus: t, usat: usats[nom]})
	}
	out := map[string]string{}
	for base, cs := range perURL {
		triat, triatNom, hiHaUsat := "", "", false
		for _, cn := range cs {
			if cn.usat {
				if hiHaUsat && cn.tipus != triat {
					return nil, fmt.Errorf("providers %q i %q comparteixen base_url %s amb protocols diferents; els rols no poden fer servir tots dos", triatNom, cn.nom, base)
				}
				triat, triatNom, hiHaUsat = cn.tipus, cn.nom, true
			}
		}
		if !hiHaUsat {
			for _, cn := range cs {
				if cn.tipus != "" {
					triat = cn.tipus
					break
				}
			}
		}
		out[base] = triat
	}
	return out, nil
}

// ValidaTipusProviders comprova els type dels providers.
func (c *Config) ValidaTipusProviders() error {
	_, err := c.tipusPerURL()
	return err
}

// RegisterProviderTypes registra el protocol de cada base_url al client LLM
// (registre global de internal/llm). Es crida en carregar, en desar i en
// configurar el client, de manera que un provider afegit en calent des de la
// web ja parla el seu protocol a la crida següent.
func (c *Config) RegisterProviderTypes() {
	m, err := c.tipusPerURL()
	if err != nil {
		return
	}
	for base, t := range m {
		llm.RegisterProviderType(base, t)
	}
	for _, p := range c.Providers {
		if strings.TrimSpace(p.BaseURL) == "" {
			continue
		}
		llm.RegisterLocalOptions(p.BaseURL, llm.LocalOptions{NumCtx: p.NumCtx, KeepAlive: p.KeepAlive, Slot: p.Slot, SideSlot: p.SideSlot})
	}
}
