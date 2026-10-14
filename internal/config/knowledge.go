package config

import (
	"fmt"
	"strings"

	"gregal/internal/knowledge"
)

// KnowledgeCfg és la secció `knowledge:` del config: límits de la
// indexació i, opcionalment, un model d'embeddings per a la cerca
// semàntica (sense, la cerca és BM25 per paraules, sense xarxa).
//
//	knowledge:
//	  embeddings: {provider: openai, model: text-embedding-3-small}
//	  max_files: 2000
//	  max_mb: 200
//	  max_file_mb: 25
type KnowledgeCfg struct {
	Embeddings *KnowledgeEmbedCfg `yaml:"embeddings,omitempty"`
	MaxFiles   int                `yaml:"max_files,omitempty"`
	MaxMB      int                `yaml:"max_mb,omitempty"`
	MaxFileMB  int                `yaml:"max_file_mb,omitempty"`
}

// KnowledgeEmbedCfg tria el provider (dels de `providers:`) i el model
// d'embeddings. El provider ha de ser OpenAI-compatible (/embeddings).
type KnowledgeEmbedCfg struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

func (c *Config) validateKnowledge() error {
	k := c.Knowledge
	if k.MaxFiles < 0 || k.MaxMB < 0 || k.MaxFileMB < 0 {
		return fmt.Errorf("knowledge: limits cannot be negative")
	}
	if e := k.Embeddings; e != nil && (strings.TrimSpace(e.Provider) != "" || strings.TrimSpace(e.Model) != "") {
		p, ok := c.Providers[e.Provider]
		if !ok {
			return fmt.Errorf("knowledge.embeddings: unknown provider %q", e.Provider)
		}
		if strings.TrimSpace(e.Model) == "" {
			return fmt.Errorf("knowledge.embeddings: model is required")
		}
		switch strings.ToLower(strings.TrimSpace(p.Type)) {
		case "", "openai", "openai-responses":
		default:
			return fmt.Errorf("knowledge.embeddings: provider %q (%s) has no OpenAI-compatible /embeddings endpoint", e.Provider, p.Type)
		}
	}
	return nil
}

// applyKnowledge passa els límits i l'embedding al paquet knowledge.
func (c *Config) applyKnowledge() {
	s := knowledge.Settings{MaxFiles: c.Knowledge.MaxFiles, MaxTotalMB: c.Knowledge.MaxMB, MaxFileMB: c.Knowledge.MaxFileMB}
	if e := c.Knowledge.Embeddings; e != nil && strings.TrimSpace(e.Model) != "" {
		if p, ok := c.Providers[e.Provider]; ok {
			s.Embed = knowledge.EmbedCfg{Provider: e.Provider, BaseURL: p.BaseURL, APIKey: p.APIKey, Model: strings.TrimSpace(e.Model)}
		}
	}
	knowledge.Configure(s)
}
