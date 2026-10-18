package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"gregal/internal/tools"
)

// Regles de permís amb glob (estil opencode). Una eina pot dur, a més de la
// decisió general de permissions.tools, una llista ordenada de patrons:
//
//	permissions:
//	  tools:
//	    bash:                 # un mapa en lloc d'allow|ask|deny
//	      "git *": ask
//	      "git status": allow
//	  rules:                  # el mateix, escrit a part
//	    edit:
//	      "docs/**": allow
//	      "*.env": deny
//	  subagents:              # el que canvia per a un subagent concret
//	    explorer:
//	      tools: {bash: deny}
//
// Dins d'una eina mana l'última regla que coincideix; l'ordre és el del
// fitxer. Vegeu agent/permrules.go per a què es compara i com es combina.

// PermRule és un patró i la seva decisió (allow|ask|deny).
type PermRule struct {
	Pattern  string
	Decision string
}

// PermRules són les regles per eina, en ordre.
type PermRules map[string][]PermRule

// PermOverride és el que un subagent canvia respecte dels permisos generals.
type PermOverride struct {
	Tools map[string]string `yaml:"tools,omitempty"`
	Rules PermRules         `yaml:"rules,omitempty"`
}

// UnmarshalYAML llegeix `eina: {patró: decisió, ...}` conservant l'ordre.
func (r *PermRules) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("permissions rules: expected a map of tool -> {pattern: decision}")
	}
	out := PermRules{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		tool := n.Content[i].Value
		list, err := ruleList(tool, n.Content[i+1])
		if err != nil {
			return err
		}
		out[tool] = append(out[tool], list...)
	}
	*r = out
	return nil
}

func ruleList(tool string, v *yaml.Node) ([]PermRule, error) {
	if v.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("permissions rules for %s: expected {pattern: decision}", tool)
	}
	var list []PermRule
	for i := 0; i+1 < len(v.Content); i += 2 {
		list = append(list, PermRule{Pattern: v.Content[i].Value, Decision: strings.ToLower(strings.TrimSpace(v.Content[i+1].Value))})
	}
	return list, nil
}

// MarshalYAML escriu les regles en ordre (un mapa de Go no el conserva).
func (r PermRules) MarshalYAML() (any, error) {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for _, tool := range sortedKeys(r) {
		inner := &yaml.Node{Kind: yaml.MappingNode}
		for _, rule := range r[tool] {
			inner.Content = append(inner.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: rule.Pattern},
				&yaml.Node{Kind: yaml.ScalarNode, Value: rule.Decision})
		}
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: tool}, inner)
	}
	return n, nil
}

func sortedKeys(r PermRules) []string {
	keys := make([]string, 0, len(r))
	for k := range r {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// moveToolMaps treu de `tools:` les entrades que són un mapa de patrons
// (`bash: {"git *": ask}`) i les passa a `rules:`, perquè Tools continuï
// sent eina → decisió per a tot el codi que ja el fa servir.
func moveToolMaps(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		return
	}
	var toolsNode, rulesNode *yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		switch n.Content[i].Value {
		case "tools":
			toolsNode = n.Content[i+1]
		case "rules":
			rulesNode = n.Content[i+1]
		}
	}
	if toolsNode == nil || toolsNode.Kind != yaml.MappingNode {
		return
	}
	var keep, moved []*yaml.Node
	for i := 0; i+1 < len(toolsNode.Content); i += 2 {
		if toolsNode.Content[i+1].Kind == yaml.MappingNode {
			moved = append(moved, toolsNode.Content[i], toolsNode.Content[i+1])
		} else {
			keep = append(keep, toolsNode.Content[i], toolsNode.Content[i+1])
		}
	}
	if len(moved) == 0 {
		return
	}
	toolsNode.Content = keep
	if rulesNode == nil {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "rules"}, &yaml.Node{Kind: yaml.MappingNode, Content: moved})
		return
	}
	if rulesNode.Kind == yaml.MappingNode {
		rulesNode.Content = append(rulesNode.Content, moved...)
	}
}

// UnmarshalYAML accepta `tools.<eina>` com a decisió o com a mapa de patrons.
func (p *PermissionsCfg) UnmarshalYAML(n *yaml.Node) error {
	moveToolMaps(n)
	type plain PermissionsCfg
	var v plain
	if err := n.Decode(&v); err != nil {
		return err
	}
	*p = PermissionsCfg(v)
	return nil
}

// UnmarshalYAML és el mateix per a un subagent.
func (o *PermOverride) UnmarshalYAML(n *yaml.Node) error {
	moveToolMaps(n)
	type plain PermOverride
	var v plain
	if err := n.Decode(&v); err != nil {
		return err
	}
	*o = PermOverride(v)
	return nil
}

func validDecision(d string) bool { return d == "allow" || d == "ask" || d == "deny" }

func (r PermRules) validate(where string) error {
	for tool, list := range r {
		if !tools.EsMCP(tool) && !tools.EsNativa(tool) {
			return fmt.Errorf("%s.%s: unknown tool", where, tool)
		}
		for _, rule := range list {
			if strings.TrimSpace(rule.Pattern) == "" {
				return fmt.Errorf("%s.%s: empty pattern", where, tool)
			}
			if !validDecision(rule.Decision) {
				return fmt.Errorf("%s.%s[%q]=%q invalid (allow|ask|deny)", where, tool, rule.Pattern, rule.Decision)
			}
		}
	}
	return nil
}

func (p PermissionsCfg) validateRules() error {
	if err := p.Rules.validate("permissions.rules"); err != nil {
		return err
	}
	for name, o := range p.Subagents {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("permissions.subagents: empty subagent name")
		}
		where := "permissions.subagents." + name
		for tool, d := range o.Tools {
			if !tools.EsMCP(tool) && !tools.EsNativa(tool) {
				return fmt.Errorf("%s.tools.%s: unknown tool", where, tool)
			}
			if !validDecision(d) {
				return fmt.Errorf("%s.tools.%s=%q invalid (allow|ask|deny)", where, tool, d)
			}
		}
		if err := o.Rules.validate(where + ".rules"); err != nil {
			return err
		}
	}
	return nil
}
