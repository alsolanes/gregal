package tui

import (
	"fmt"
	"sort"

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
		if next != nil && next.back == nil {
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
	}
	// Qualsevol lletra filtra la llista.
	if len(k) == 1 && k != " " {
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
	return &menuState{title: "rol actiu", items: items, run: func(m *Model, val string) (string, *menuState) {
		if val == "noop" {
			return systemLine(T("pick.capRol")), nil
		}
		r := m.cfg.Roles[val]
		m.role = val
		m.status = "llest"
		return systemLine(fmt.Sprintf("rol actiu: %s (%s/%s)", val, r.Provider, r.Model)), nil
	}}
}

// modeMenu tria els modes habituals amb fletxes (i desa). El mode autònom és
// explícit amb /mode autonomous per evitar activar una execució llarga per
// error des del selector ràpid.
func modeMenu() *menuState {
	// Cada fila diu què fa el mode: és aquí on s'expliquen, no a la
	// benvinguda.
	return &menuState{title: "mode", items: []menuItem{
		{text: T("mode.code.badge") + "  " + T("mode.code.desc"), val: "code"},
		{text: T("mode.inspect.badge") + "  " + T("mode.inspect.desc"), val: "inspect"},
		{text: T("mode.chat.badge") + "  " + T("mode.chat.desc"), val: "chat"},
		{text: T("mode.goal.badge") + "  " + T("mode.goal.desc"), val: "goal"},
	}, run: func(m *Model, val string) (string, *menuState) {
		m.setMode(val)
		return "", nil
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
				return systemLine(prov+" no llista models (clau buida o url caiguda)") + "\n" + kp, nil
			}
			return systemLine(prov + " no llista models (escriu: /model " + prov + "/nom)"), nil
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

// resumeMenu tria sessió desada amb fletxes.
func resumeMenu() *menuState {
	infos, err := session.List(session.DefaultDir())
	if err != nil || len(infos) == 0 {
		return nil
	}
	var items []menuItem
	for _, in := range infos {
		items = append(items, menuItem{
			text: sessioText(in),
			val:  in.Name,
		})
	}
	return &menuState{title: T("menu.sessions.titol"), items: items, run: func(m *Model, val string) (string, *menuState) {
		m.resumeSession(val)
		return "", nil
	}}
}
