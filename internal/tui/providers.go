package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// providersBlock llista providers (URL + clau emmascarada + rols que l'usen).
func providersBlock(cfg *config.Config) string {
	var b strings.Builder
	b.WriteString(headStyle.Render("◆ providers") + faintStyle.Render(T("prov.ajuda1")) + "\n")
	names := make([]string, 0, len(cfg.Providers))
	for n := range cfg.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	usedBy := map[string][]string{}
	for rn, r := range cfg.Roles {
		usedBy[r.Provider] = append(usedBy[r.Provider], rn+"/"+r.Model)
	}
	for _, n := range names {
		p := cfg.Providers[n]
		line := "  " + cmdStyle.Render(n) + dimStyle.Render("  "+p.BaseURL+"  clau: "+p.MaskedKey())
		if ub, ok := usedBy[n]; ok {
			sort.Strings(ub)
			line += faintStyle.Render("  ← " + strings.Join(ub, ", "))
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// providerCmd executa /provider <sub>. Torna el missatge (ja amb estil).
func (m *Model) providerCmd(arg string) string {
	f := strings.Fields(arg)
	if len(f) == 0 {
		return providersBlock(m.cfg)
	}
	save := func() string {
		if err := m.cfg.Save(m.cfgPath); err != nil {
			return systemLine(T("prov.noDesat") + err.Error())
		}
		return ""
	}
	switch f[0] {
	case "add":
		if len(f) < 3 {
			return systemLine(T("prov.usAdd"))
		}
		nom, url := f[1], strings.TrimSuffix(f[2], "/")
		if !validProvName(nom) {
			return systemLine(T("prov.nomInvalid"))
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return systemLine(T("prov.urlInvalid"))
		}
		if _, ok := m.cfg.Providers[nom]; ok {
			return systemLine(T("prov.jaExisteix") + nom)
		}
		p := config.Provider{BaseURL: url}
		raw := ""
		if len(f) >= 4 {
			raw = f[3]
			p.APIKey = expandKey(raw)
		}
		if m.cfg.Providers == nil {
			m.cfg.Providers = map[string]config.Provider{}
		}
		m.cfg.Providers[nom] = p
		m.cfg.SetRawKey(nom, raw)
		if e := save(); e != "" {
			return e
		}
		msg := okStyle.Render("✓ provider "+nom+" desat") + dimStyle.Render(" ("+url+")")
		if ids, err := fetchModelIDs(url, p.APIKey); err == nil && len(ids) > 0 {
			msg += dimStyle.Render(" — models: "+shortIDs(ids)) + "\n" +
				systemLine(T("prov.assigna")+nom+" <model>")
		} else {
			msg += dimStyle.Render(T("prov.prova") + nom)
		}
		return msg
	case "url":
		if len(f) < 3 {
			return systemLine(T("prov.usUrl"))
		}
		p, ok := m.cfg.Providers[f[1]]
		if !ok {
			return systemLine("no existeix: " + f[1])
		}
		url := strings.TrimSuffix(strings.TrimSpace(f[2]), "/")
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return systemLine(T("prov.urlInvalid"))
		}
		p.BaseURL = url
		m.cfg.Providers[f[1]] = p
		if e := save(); e != "" {
			return e
		}
		return okStyle.Render("✓ "+f[1]+" → "+url) + dimStyle.Render(" — comprova amb /provider test "+f[1])
	case "rm":
		if len(f) < 2 {
			return systemLine(T("prov.usRm"))
		}
		nom := f[1]
		if _, ok := m.cfg.Providers[nom]; !ok {
			return systemLine("no existeix: " + nom)
		}
		for rn, r := range m.cfg.Roles {
			if r.Provider == nom {
				return systemLine(T("prov.enUs") + rn + T("prov.canviaPrimer"))
			}
		}
		delete(m.cfg.Providers, nom)
		if e := save(); e != "" {
			return e
		}
		return okStyle.Render("✓ provider " + nom + " esborrat")
	case "rol":
		if len(f) < 3 {
			return systemLine(T("prov.usRol"))
		}
		rol, prov := f[1], f[2]
		r, ok := m.cfg.Roles[rol]
		if !ok {
			return systemLine("rol desconegut: " + rol)
		}
		if _, ok := m.cfg.Providers[prov]; !ok {
			return systemLine("provider desconegut: " + prov + T("prov.creaAmb"))
		}
		r.Provider = prov
		if len(f) >= 4 {
			r.Model = f[3]
		} else if ids, err := fetchModelIDs(m.cfg.Providers[prov].BaseURL, m.cfg.Providers[prov].APIKey); err == nil && len(ids) > 0 {
			// Sense model explícit: corregeix majúscules o demana tria.
			if real, ok := matchModelFold(ids, r.Model); ok {
				r.Model = real
			} else {
				return warnStyle.Render("⚠ "+prov+" no té el model "+r.Model) + dimStyle.Render(" — disponibles: "+shortIDs(ids)) + "\n" +
					systemLine("tria'n un: /provider rol "+rol+" "+prov+" <model>")
			}
		}
		m.cfg.Roles[rol] = r
		if e := save(); e != "" {
			return e
		}
		return okStyle.Render(fmt.Sprintf("✓ rol %s → %s/%s", rol, prov, r.Model))
	case "key":
		if len(f) < 2 {
			return systemLine(T("prov.usKey"))
		}
		nom := f[1]
		if _, ok := m.cfg.Providers[nom]; !ok {
			return systemLine("no existeix: " + nom)
		}
		if len(f) < 3 {
			// Sense clau a la línia: la següent entrada es menja
			// com a secret (sense eco ni historial).
			m.pendingKeyFor = nom
			return systemLine("escriu la clau de " + nom + T("prov.senseHistorial"))
		}
		return m.applyProviderKey(nom, f[2])
	case "test":
		if len(f) < 2 {
			return systemLine(T("prov.usTest"))
		}
		return testProvider(m.cfg, f[1])
	default:
		return systemLine(T("prov.subordres"))
	}
}

// applyProviderKey desa la clau d'un provider (mateixes regles que
// /provider key: ${VAR} o buida persisteix; literal = només memòria).
func (m *Model) applyProviderKey(nom, raw string) string {
	p, ok := m.cfg.Providers[nom]
	if !ok {
		return systemLine("no existeix: " + nom)
	}
	if raw == "-" {
		raw = ""
	}
	p.APIKey = expandKey(raw)
	m.cfg.Providers[nom] = p
	save := func() string {
		if err := m.cfg.Save(m.cfgPath); err != nil {
			return systemLine(T("prov.noDesat") + err.Error())
		}
		return ""
	}
	if strings.HasPrefix(strings.TrimSpace(raw), "${") || raw == "" {
		m.cfg.SetRawKey(nom, raw)
		if e := save(); e != "" {
			return e
		}
		return okStyle.Render("✓ clau de " + nom + " desada")
	}
	// Secret literal: va al magatzem de claus (~/.config/gregal/auth.yaml,
	// 0600), mai al config.yaml. Abans només vivia en memòria i, sense
	// cap ${VAR} definida, cada arrencada te la tornava a demanar.
	m.cfg.SetRawKey(nom, "")
	if err := config.SaveKey(nom, p.APIKey); err != nil {
		return systemLine("clau no desada: " + err.Error())
	}
	if e := save(); e != "" {
		return e
	}
	return okStyle.Render("✓ clau de "+nom+" desada") + dimStyle.Render(" (a "+config.AuthPath()+", només per a tu)")
}

// keyPromptIfMissing: en triar un provider sense clau, prepara la
// captura de la següent línia i torna el text d'avís (o "" si té clau).
func (m *Model) keyPromptIfMissing(prov string) string {
	if p, ok := m.cfg.Providers[prov]; ok && strings.TrimSpace(p.APIKey) == "" {
		m.pendingKeyFor = prov
		return warnStyle.Render("⚠ "+prov+" va sense clau api") + dimStyle.Render(" — escriu-la ara (no quedarà a l'historial) o /provider test "+prov)
	}
	return ""
}

// expandKey resol ${VAR} contra l'entorn; un literal es deixa tal qual.
func expandKey(raw string) string { return os.ExpandEnv(strings.TrimSpace(raw)) }

// fetchModelIDs delega al probe compartit (internal/llm).
func fetchModelIDs(base, key string) ([]string, error) {
	return llm.ProbeModels(base, key)
}

func matchModelFold(ids []string, want string) (string, bool) {
	for _, id := range ids {
		if id == want {
			return id, true
		}
	}
	var fold string
	n := 0
	for _, id := range ids {
		if strings.EqualFold(id, want) {
			fold, n = id, n+1
		}
	}
	if n == 1 {
		return fold, true
	}
	return "", false
}

// shortIDs resumeix la llista per mostrar-la (màx 8 + "… +N").
func shortIDs(ids []string) string {
	if len(ids) > 8 {
		ids = append(ids[:8], fmt.Sprintf("… +%d", len(ids)-8))
	}
	return strings.Join(ids, ", ")
}

// testProvider fa GET <base_url>/models i informa (sense exposar la clau).
func testProvider(cfg *config.Config, nom string) string {
	p, ok := cfg.Providers[nom]
	if !ok {
		return systemLine("no existeix: " + nom)
	}
	ids, err := fetchModelIDs(p.BaseURL, p.APIKey)
	if err != nil {
		return badStyle.Render("✗ "+nom+": no llista models") + dimStyle.Render(" ("+err.Error()+")")
	}
	if len(ids) == 0 {
		return okStyle.Render("✓ "+nom+" respon") + dimStyle.Render(T("prov.senseModels"))
	}
	return okStyle.Render(fmt.Sprintf("✓ %s respon (%d models)", nom, len(ids))) + dimStyle.Render(" — "+shortIDs(ids))
}

func validProvName(n string) bool {
	if n == "" {
		return false
	}
	for _, r := range n {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
