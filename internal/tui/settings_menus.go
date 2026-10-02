package tui

import (
	"fmt"
	"sort"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/tema"
	"gregal/internal/tools"
)

// settingsMenu és el centre de configuració del TUI. Manté les opcions
// agrupades en una sola finestra, perquè no calgui recordar una ordre
// diferent per a cada ajust (estil selector de terminal d'OpenCode).
func settingsMenu(m *Model) *menuState {
	r := m.roleRef()
	reviewer := m.cfg.Roles["reviewer"]
	maskedKey := ""
	if p, ok := m.cfg.Providers[r.Provider]; ok {
		maskedKey = p.MaskedKey()
	}
	if maskedKey == "" {
		maskedKey = "sense clau"
	}
	items := []menuItem{
		{text: fmt.Sprintf("mode        %s", m.mode), val: "mode"},
		{text: fmt.Sprintf("rol         %s  (%s/%s)", m.role, r.Provider, r.Model), val: "role"},
		{text: fmt.Sprintf("model       %s/%s", r.Provider, r.Model), val: "model"},
		{text: fmt.Sprintf("clau d'API  %s (%s)", r.Provider, maskedKey), val: "apikey"},
		{text: fmt.Sprintf("tema        %s", m.cfg.Tema()), val: "theme"},
		{text: fmt.Sprintf("revisor     %s  (%s/%s)", m.cfg.Verify.Mode, reviewer.Provider, reviewer.Model), val: "reviewer"},
		{text: T("menu.set.perms"), val: "permissions"},
		{text: fmt.Sprintf("providers   %d endpoints configurats", len(m.cfg.Providers)), val: "providers"},
		{text: T("menu.set.connectors"), val: "connectors"},
		{text: T("menu.set.parallel"), val: "parallel"},
		{text: T("menu.set.sessions"), val: "sessions"},
		{text: T("menu.set.goals"), val: "goals"},
		{text: T("menu.set.help"), val: "help"},
	}
	return &menuState{title: T("menu.set.titol"), items: items, run: func(m *Model, val string) (string, *menuState) {
		switch val {
		case "mode":
			return "", modeMenu()
		case "role":
			return "", roleMenu(m)
		case "model":
			return "", modelMenu(m)
		case "apikey":
			return "", providerKeyMenu(m)
		case "theme":
			return "", themeMenu(m)
		case "reviewer":
			return "", reviewerMenu(m)
		case "permissions":
			return "", permissionsMenu(m)
		case "providers":
			return "", providerSettingsMenu(m)
		case "connectors":
			return "", mcpMenu(m)
		case "parallel":
			return systemLine(T("set.parallelNota")), nil
		case "sessions":
			if next := resumeMenu(); next != nil {
				return "", next
			}
			return systemLine(T("menu.set.capsessio")), nil
		case "goals":
			return "", goalMenu(m)
		case "help":
			return helpBlock(), nil
		default:
			return "", nil
		}
	}}
}

// providerKeyMenu permet escollir quin provider vols configurar amb clau d'API.
func providerKeyMenu(m *Model) *menuState {
	names := make([]string, 0, len(m.cfg.Providers))
	for name := range m.cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return &menuState{
			title: "claus d'API",
			items: []menuItem{{text: "cap provider configurat (/provider add <nom> <url>)", val: "noop"}},
			run:   func(*Model, string) (string, *menuState) { return "", nil },
		}
	}
	items := make([]menuItem, 0, len(names))
	curProv := m.roleRef().Provider
	for _, name := range names {
		p := m.cfg.Providers[name]
		masked := p.MaskedKey()
		if masked == "" {
			masked = "sense clau"
		}
		text := fmt.Sprintf("%-12s · clau %s", name, masked)
		if name == curProv {
			text += " ← actual"
		}
		items = append(items, menuItem{text: text, val: name})
	}
	return &menuState{
		title: "clau d'API · tria provider",
		items: items,
		run: func(m *Model, name string) (string, *menuState) {
			if name == "noop" {
				return "", nil
			}
			m.pendingKeyFor = name
			return systemLine("escriu la clau de " + name + T("prov.senseHistorial")), nil
		},
	}
}

// themeMenu permet canviar de tema amb reflex instantani. Els temes
// surten del registre (primer els de casa, després els inspirats).
func themeMenu(m *Model) *menuState {
	cur := m.cfg.Tema()
	items := make([]menuItem, 0, len(config.TemesSuportats))
	for _, nom := range config.TemesSuportats {
		text := tema.Descripcio(nom)
		if nom == cur {
			text += " ← actual"
		}
		items = append(items, menuItem{text: text, val: nom})
	}
	return &menuState{
		title: "tema de color",
		items: items,
		run: func(m *Model, val string) (string, *menuState) {
			m.cfg.Theme = val
			SetTema(val)
			msg := okStyle.Render("✓ tema → "+val) + dimStyle.Render(" (desat)")
			if err := m.cfg.Save(m.cfgPath); err != nil {
				msg += dimStyle.Render(" (error en desar: " + err.Error() + ")")
			}
			m.refresh()
			return msg, nil
		},
	}
}

// mcpMenu mostra connectors externs configurats o l'estat d'inactivitat.
func mcpMenu(m *Model) *menuState {
	if m.mcp == nil || m.mcp.Summary() == "inactiu" {
		return &menuState{
			title: "connectors (mcp)",
			items: []menuItem{
				{text: T("app.mcpInactiu"), val: "noop"},
			},
			run: func(*Model, string) (string, *menuState) { return "", nil },
		}
	}
	var items []menuItem
	for _, t := range m.mcp.Tools {
		desc := t.Spec.Description
		if len(desc) > 50 {
			desc = desc[:47] + "…"
		}
		items = append(items, menuItem{
			text: fmt.Sprintf("%-16s — %s", t.Spec.Name, desc),
			val:  t.Spec.Name,
		})
	}
	if len(items) == 0 {
		items = append(items, menuItem{text: "connectat però sense eines registrades", val: "noop"})
	}
	return &menuState{
		title: "connectors mcp · " + m.mcp.Summary(),
		items: items,
		run: func(m *Model, name string) (string, *menuState) {
			if name == "noop" {
				return "", nil
			}
			for _, t := range m.mcp.Tools {
				if t.Spec.Name == name {
					return systemLine(fmt.Sprintf("eina mcp: %s\n%s", t.Spec.Name, t.Spec.Description)), nil
				}
			}
			return "", nil
		},
	}
}

// providerSettingsMenu posa els endpoints en una finestra navegable. Les
// operacions destructives o amb secrets continuen sent ordres explícites
// (/provider add|key|rm), però consultar, provar i assignar un model no
// obliga a recordar sintaxi.
func providerSettingsMenu(m *Model) *menuState {
	names := make([]string, 0, len(m.cfg.Providers))
	for name := range m.cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]menuItem, 0, len(names)+2)
	items = append(items, menuItem{text: T("menu.provAfegeix"), val: "+"})
	for _, name := range names {
		p := m.cfg.Providers[name]
		items = append(items, menuItem{text: fmt.Sprintf("%s — %s · clau %s", name, p.BaseURL, p.MaskedKey()), val: name})
	}
	items = append(items, menuItem{text: T("menu.ajudaProv"), val: "help"})
	return &menuState{title: "providers", items: items, run: func(m *Model, val string) (string, *menuState) {
		if val == "+" {
			return "", providerAddMenu(m)
		}
		if val == "help" {
			return systemLine(T("menu.altaProv")), nil
		}
		return "", providerDetailMenu(m, val)
	}}
}

func providerDetailMenu(m *Model, name string) *menuState {
	p, ok := m.cfg.Providers[name]
	if !ok {
		return &menuState{title: "provider", items: []menuItem{{text: "provider no trobat", val: "noop"}}, run: func(*Model, string) (string, *menuState) { return "", nil }}
	}
	items := []menuItem{
		{text: T("menu.prov.test"), val: "test"},
		{text: T("menu.prov.assign"), val: "model"},
		{text: T("prov.canviarClau"), val: "key"},
		{text: "veure detalls", val: "details"},
		{text: T("prov.esborrar"), val: "rm"},
	}
	return &menuState{title: T("menu.providerPrefix") + name, items: items, run: func(m *Model, action string) (string, *menuState) {
		switch action {
		case "test":
			return testProvider(m.cfg, name), nil
		case "details":
			return systemLine(fmt.Sprintf("%s · %s · clau %s", name, p.BaseURL, p.MaskedKey())), nil
		case "key":
			m.pendingKeyFor = name
			return systemLine("escriu la clau de " + name + T("prov.senseHistorial")), nil
		case "rm":
			for rn, r := range m.cfg.Roles {
				if r.Provider == name {
					return systemLine(T("prov.enUs") + rn + T("prov.canviaPrimer")), nil
				}
			}
			delete(m.cfg.Providers, name)
			if err := m.cfg.Save(m.cfgPath); err != nil {
				return systemLine(T("prov.noDesat") + err.Error()), nil
			}
			return okStyle.Render("✓ provider " + name + " esborrat"), nil
		case "model":
			ids, err := fetchModelIDs(p.BaseURL, p.APIKey)
			if err != nil || len(ids) == 0 {
				return systemLine(name + " no llista models: " + errText(err)), nil
			}
			return "", modelsDelProveidor(m, name, ids)
		default:
			return "", nil
		}
	}}
}

// nativeTools surt del registre canònic: el menú governa exactament
// les eines que la política entén (abans hi faltaven office i browser).
func nativeTools() []string { return tools.Configurables() }

// permissionsMenu permet configurar allow/ask/deny sense YAML.
func permissionsMenu(m *Model) *menuState {
	permLabel := "mode permissiu: desactivat"
	if m.permissive {
		permLabel = "mode permissiu: ACTIU (omet preguntes)"
	}
	items := make([]menuItem, 0, len(nativeTools())+1)
	items = append(items, menuItem{text: permLabel, val: "permissive_toggle"})
	for _, name := range nativeTools() {
		decision, _ := m.policy.For(name, "{}")
		items = append(items, menuItem{text: fmt.Sprintf("%-5s — %s", name, decision), val: name})
	}
	var pm *menuState
	// refresca torna el mateix menú amb els valors nous, el cursor on era i
	// el mateix pare: canviar un permís no tanca la finestra, que és el
	// que fa falta quan se'n canvien tres seguits.
	refresca := func(m *Model) *menuState {
		nou := permissionsMenu(m)
		nou.idx, nou.top, nou.back, nou.filtre = pm.idx, pm.top, pm.back, pm.filtre
		return nou
	}
	pm = &menuState{title: "permisos d'eines", items: items, run: func(m *Model, tool string) (string, *menuState) {
		if tool == "permissive_toggle" {
			m.permissive = !m.permissive
			if m.permissive {
				return warnStyle.Render(T("app.permissiuActiu")) + dimStyle.Render(T("app.denyBloqueja")), refresca(m)
			}
			return systemLine(T("app.permissiuOff")), refresca(m)
		}
		var choices []menuItem
		for _, d := range []string{"allow", "ask", "deny"} {
			text := d
			if cur, ok := m.cfg.Permissions.Tools[tool]; ok && cur == d {
				text += T("set.explicit")
			}
			choices = append(choices, menuItem{text: text, val: d})
		}
		return "", &menuState{title: "permís: " + tool, items: choices, run: func(m *Model, decision string) (string, *menuState) {
			if m.cfg.Permissions.Tools == nil {
				m.cfg.Permissions.Tools = map[string]string{}
			}
			old, had := m.cfg.Permissions.Tools[tool]
			m.cfg.Permissions.Tools[tool] = decision
			if err := m.cfg.Save(m.cfgPath); err != nil {
				if had {
					m.cfg.Permissions.Tools[tool] = old
				} else {
					delete(m.cfg.Permissions.Tools, tool)
				}
				return systemLine(T("set.permisNoDesat") + err.Error()), refresca(m)
			}
			m.policy = &agent.Policy{Tools: m.cfg.Permissions.Tools, BashAllow: m.cfg.Permissions.BashAllow, BashDeny: m.cfg.Permissions.BashDeny}
			agent.SetPostEditHook(m.cfg.Hooks.PostEdit)
			agent.SetDiagMode(m.cfg.Hooks.Diag)
			return okStyle.Render("✓ "+tool+" → "+decision) + dimStyle.Render(" (desat)"), refresca(m)
		}}
	}}
	return pm
}

// reviewerMenu configura tant quan revisa com quin model fa de revisor.
func reviewerMenu(m *Model) *menuState {
	r := m.cfg.Roles["reviewer"]
	items := []menuItem{{text: fmt.Sprintf("model — %s/%s", r.Provider, r.Model), val: "model"}}
	for _, mode := range []string{"off", "manual", "auto", "both", "strict"} {
		text := "mode — " + mode
		if m.cfg.Verify.Mode == mode {
			text += " ← actual"
		}
		items = append(items, menuItem{text: text, val: mode})
	}
	return &menuState{title: "revisor", items: items, run: func(m *Model, val string) (string, *menuState) {
		if val == "model" {
			return "", reviewerProviderMenu(m)
		}
		old := m.cfg.Verify.Mode
		m.cfg.Verify.Mode = val
		if err := m.cfg.Save(m.cfgPath); err != nil {
			m.cfg.Verify.Mode = old
			return systemLine("revisor no desat: " + err.Error()), nil
		}
		return okStyle.Render("✓ revisor: "+val) + dimStyle.Render(" (desat)"), nil
	}}
}

func reviewerProviderMenu(m *Model) *menuState {
	names := make([]string, 0, len(m.cfg.Providers))
	for name := range m.cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]menuItem, 0, len(names))
	for _, name := range names {
		p := m.cfg.Providers[name]
		items = append(items, menuItem{text: name + " — " + p.BaseURL, val: name})
	}
	return &menuState{title: T("menu.provRevisor"), items: items, run: func(m *Model, provider string) (string, *menuState) {
		p := m.cfg.Providers[provider]
		ids, err := fetchModelIDs(p.BaseURL, p.APIKey)
		if err != nil || len(ids) == 0 {
			return systemLine(provider + " no llista models: " + errText(err)), nil
		}
		modelItems := make([]menuItem, 0, len(ids))
		cur := m.cfg.Roles["reviewer"]
		for _, id := range ids {
			text := id
			if cur.Provider == provider && cur.Model == id {
				text += " ← actual"
			}
			modelItems = append(modelItems, menuItem{text: text, val: id})
		}
		return "", &menuState{title: "model del revisor", items: modelItems, run: func(m *Model, model string) (string, *menuState) {
			old := m.cfg.Roles["reviewer"]
			next := old
			next.Provider, next.Model = provider, model
			m.cfg.Roles["reviewer"] = next
			if err := m.cfg.Save(m.cfgPath); err != nil {
				m.cfg.Roles["reviewer"] = old
				return systemLine("revisor no desat: " + err.Error()), nil
			}
			return okStyle.Render("✓ revisor → "+provider+"/"+model) + dimStyle.Render(" (desat)"), nil
		}}
	}}
}

func errText(err error) string {
	if err == nil {
		return T("menu.senseModels")
	}
	return err.Error()
}
