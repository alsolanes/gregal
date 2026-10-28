package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"gregal/internal/agent"
	"gregal/internal/session"
)

// menuKeys gestiona tecles quan un menú és actiu. Torna true si consumida.
func (m *Model) menuKeys(k string) bool {
	switch k {
	case "up", "ctrl+p":
		menuMove(m.menu, -1)
		return true
	case "down", "ctrl+n":
		menuMove(m.menu, 1)
		return true
	case "enter", "tab":
		it, ok := m.menu.tria()
		if !ok {
			return true
		}
		run := m.menu.run
		msg, next := run(m, it.val)
		// El menú nou torna al d'ara amb Esc, tret que ja digui a on torna
		// (un menú que es refresca a si mateix vol tornar al seu pare, no a
		// la seva versió vella).
		if next != nil && next.back == nil && next != m.menu {
			next.back = m.menu
		}
		m.menu = next
		if msg != "" {
			m.push(msg)
		}
		return true
	case "esc":
		// Amb filtre escrit, Esc el neteja abans de tancar el menu: si no,
		// una lletra de mes et treia del selector.
		if m.menu.filtre != "" {
			m.menu.filtre = ""
			m.menu.idx, m.menu.top = 0, 0
			return true
		}
		m.menu = m.menu.back
		return true
	case "backspace":
		m.menu.esborra()
		return true
	case "pgdown":
		menuJump(m.menu, menuWin)
		return true
	case "pgup":
		menuJump(m.menu, -menuWin)
		return true
	case "home":
		menuJump(m.menu, -1<<20)
		return true
	case "end":
		menuJump(m.menu, 1<<20)
		return true
	}
	// Qualsevol lletra filtra la llista, també les accentuades («ç», «à»):
	// abans només comptava un byte i «barç» filtrava per «bar».
	if utf8.RuneCountInString(k) == 1 && k != " " {
		m.menu.escriu(k)
		return true
	}
	if k == "space" {
		m.menu.escriu(" ")
		return true
	}
	return false
}

// roleMenu tria rol actiu amb fletxes.
func roleMenu(m *Model) *menuState {
	var items []menuItem
	names := make([]string, 0, len(m.cfg.Roles))
	for n, r := range m.cfg.Roles {
		if n != "reviewer" && r.Model != "" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		r := m.cfg.Roles[n]
		items = append(items, menuItem{text: fmt.Sprintf("%s — %s/%s", n, r.Provider, r.Model), val: n})
	}
	if len(items) == 0 {
		items = append(items, menuItem{text: T("pick.capRol"), val: "noop"})
	}
	return &menuState{title: T("pick.rolActiuTitol"), items: items, run: func(m *Model, val string) (string, *menuState) {
		if val == "noop" {
			return systemLine(T("pick.capRol")), nil
		}
		r := m.cfg.Roles[val]
		m.role = val
		m.status = T("barra.llest")
		return systemLine(fmt.Sprintf(T("pick.rolActiu"), val, r.Provider, r.Model)), nil
	}}
}

// modeMenu tria el mode amb fletxes (i desa): els tres de totes les
// interfícies (agent/access.go). Objectiu i Depuració són avançats: /goal i
// /mode debug. Triar-lo només canvia el mode.
func modeMenu() *menuState {
	// Cada fila diu què fa el mode: és aquí on s'expliquen, no a la
	// benvinguda.
	return &menuState{title: "mode", items: []menuItem{
		{text: T("mode.chat.badge") + "  " + T("mode.chat.desc"), val: agent.UIModeAsk},
		{text: T("mode.plan.badge") + "  " + T("mode.plan.desc"), val: agent.UIModePlan},
		{text: T("mode.code.badge") + "  " + T("mode.code.desc"), val: agent.UIModeAct},
	}, run: func(m *Model, val string) (string, *menuState) {
		if val == agent.UIModeAct {
			m.setMode(m.currentActMode())
		} else {
			m.setMode(agent.ModeFor(val, ""))
		}
		return "", nil
	}}
}

// accessMenu tria el permís d'Actua: els tres de totes les interfícies.
// Els permisos per eina (allow/ask/deny) són avançats: /permissions tools
// o el fitxer de config.
func accessMenu(m *Model) *menuState {
	ara := m.access()
	items := make([]menuItem, 0, len(agent.Accesses)+1)
	for _, a := range agent.Accesses {
		marca := "  "
		if a == ara {
			marca = "● "
		}
		items = append(items, menuItem{text: marca + accessLabel(a) + "  " + accessHelp(a), val: a})
	}
	items = append(items, menuItem{text: "  " + T("perm.tools"), val: "tools"})
	return &menuState{title: T("perm.titol"), items: items, run: func(m *Model, val string) (string, *menuState) {
		if val == "tools" {
			return "", permissionsMenu(m)
		}
		m.setAccess(val)
		return okStyle.Render("◆ " + accessLabel(val)), nil
	}}
}

// modelMenu tria provider amb fletxes i després model (llista viva).
func modelMenu(m *Model) *menuState {
	names := make([]string, 0, len(m.cfg.Providers))
	for n := range m.cfg.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	var items []menuItem
	for _, n := range names {
		p := m.cfg.Providers[n]
		items = append(items, menuItem{text: fmt.Sprintf("%s — %s", n, p.BaseURL), val: n})
	}
	return &menuState{title: T("menu.provider.titol"), items: items, run: func(m *Model, prov string) (string, *menuState) {
		p := m.cfg.Providers[prov]
		ids, err := fetchModelIDs(p.BaseURL, p.APIKey)
		if err != nil || len(ids) == 0 {
			if kp := m.keyPromptIfMissing(prov); kp != "" {
				return systemLine(prov+T("pick.noLlistaClau")) + "\n" + kp, nil
			}
			return systemLine(prov + fmt.Sprintf(T("pick.noLlistaEscriu"), prov)), nil
		}
		// Un sol cami: triar model aqui desa, com /provider rol. Abans
		// aquest menu nomes canviava el rol per a la sessio mentre que
		// l'ordre si que ho desava, i res ho deia.
		return "", modelsDelProveidor(m, prov, ids)
	}}
}

func indexSlash(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// sessioDeVents diu si una conversa és dels Vents (xat amb un vent o
// rotllana). Viuen al servidor web: si el TUI les reprèn i hi escriu, les
// torna a desar com a sessions de codi amb una altra carpeta.
func sessioDeVents(name string) bool {
	return strings.HasPrefix(name, "rotllana-") || strings.HasPrefix(name, "company-")
}

// carpetaNoHiEs diu si la carpeta d'una conversa ja no existeix (reprendre-la
// fallaria). Sense carpeta, la conversa és general i es pot obrir.
func carpetaNoHiEs(ws string) bool {
	if strings.TrimSpace(ws) == "" {
		return false
	}
	abs, err := filepath.Abs(ws)
	if err != nil {
		return true
	}
	info, err := os.Stat(abs)
	return err != nil || !info.IsDir()
}

// resumeMenu tria sessió desada amb fletxes. Les converses dels Vents no hi
// surten, i les d'una carpeta que ja no existeix van al final, marcades: triar-ne
// una ho explica sense tancar el selector.
func resumeMenu() *menuState {
	infos, err := session.List(session.DefaultDir())
	if err != nil {
		return nil
	}
	var items, perdudes []menuItem
	gone := map[string]bool{}
	for _, in := range infos {
		if sessioDeVents(in.Name) {
			continue
		}
		missing, seen := gone[in.Workspace]
		if !seen {
			missing = carpetaNoHiEs(in.Workspace)
			gone[in.Workspace] = missing
		}
		if missing {
			perdudes = append(perdudes, menuItem{text: "⚠ " + sessioText(in), val: "\x00" + in.Name})
			continue
		}
		items = append(items, menuItem{text: sessioText(in), val: in.Name})
	}
	items = append(items, perdudes...)
	if len(items) == 0 {
		return nil
	}
	var mn *menuState
	mn = &menuState{title: T("menu.sessions.titol"), items: items, run: func(m *Model, val string) (string, *menuState) {
		if name, ok := strings.CutPrefix(val, "\x00"); ok {
			ws := ""
			for _, in := range infos {
				if in.Name == name {
					ws = in.Workspace
				}
			}
			return systemLine(T("app.workspaceNoDisp") + ws), mn
		}
		m.resumeSession(val)
		return "", nil
	}}
	return mn
}
