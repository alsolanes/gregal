package tui

import (
	"fmt"
	"os"
	"strings"

	"gregal/internal/config"
)

// Alta de proveïdors des del catàleg.
//
// Fins ara donar d'alta un endpoint volia dir `/provider add <nom> <url>`
// amb la URL apresa de memòria. El catàleg (internal/config/cataleg.go)
// porta les URL bones i el nom de la variable d'entorn habitual: tries
// «OpenRouter» i només queda enganxar la clau, que a més ja s'agafa sola
// si la variable existeix.

// providerAddMenu tria un proveïdor conegut i el dona d'alta.
func providerAddMenu(m *Model) *menuState {
	items := make([]menuItem, 0, len(config.Cataleg())+1)
	for _, pc := range config.Cataleg() {
		text := fmt.Sprintf("%-14s %s", pc.Etiqueta, pc.BaseURL)
		switch {
		case teProveidorAmbURL(m, pc.BaseURL):
			text += "  ← ja el tens"
		case pc.Local:
			text += "  (local, sense clau)"
		case os.Getenv(pc.EnvVar) != "":
			text += "  ($" + pc.EnvVar + " definida)"
		}
		items = append(items, menuItem{text: text, val: pc.Nom})
	}
	items = append(items, menuItem{text: T("menu.provAMa"), val: ""})
	return &menuState{title: T("menu.provAlta"), items: items, run: func(m *Model, nom string) (string, *menuState) {
		if nom == "" {
			return systemLine(T("menu.altaProv")), nil
		}
		pc, ok := config.ProveidorPerNom(nom)
		if !ok {
			return "", nil
		}
		return m.altaProveidor(pc)
	}}
}

// teProveidorAmbURL diu si ja hi ha un endpoint configurat amb aquesta URL
// (el nom pot ser un altre: el config d'exemple en diu «cloud»).
func teProveidorAmbURL(m *Model, url string) bool {
	for _, p := range m.cfg.Providers {
		if strings.TrimSuffix(p.BaseURL, "/") == strings.TrimSuffix(url, "/") {
			return true
		}
	}
	return false
}

// altaProveidor crea l'entrada al config i deixa el següent pas a punt:
// demanar la clau si en cal, o triar model si ja respon.
func (m *Model) altaProveidor(pc config.ProveidorConegut) (string, *menuState) {
	nom := m.cfg.NomLliure(pc.Nom)
	p := config.Provider{BaseURL: pc.BaseURL}
	raw := ""
	// Si la variable d'entorn habitual ja hi és, s'hi apunta en comptes
	// de copiar-ne el valor: el config queda net i la clau segueix sent
	// de l'entorn.
	if pc.EnvVar != "" && strings.TrimSpace(os.Getenv(pc.EnvVar)) != "" {
		raw = "${" + pc.EnvVar + "}"
		p.APIKey = os.Getenv(pc.EnvVar)
	}
	if m.cfg.Providers == nil {
		m.cfg.Providers = map[string]config.Provider{}
	}
	m.cfg.Providers[nom] = p
	m.cfg.SetRawKey(nom, raw)
	if err := m.cfg.Save(m.cfgPath); err != nil {
		delete(m.cfg.Providers, nom)
		return systemLine(T("prov.noDesat") + err.Error()), nil
	}
	capsalera := okStyle.Render("✓ "+pc.Etiqueta+" donat d'alta com a "+nom) + dimStyle.Render("  "+pc.BaseURL)
	if pc.Nota != "" {
		capsalera += "\n" + faintStyle.Render("  "+pc.Nota)
	}
	// Sense clau i no és local: el següent que cal és la clau, i es
	// demana aquí mateix en comptes de deixar-ho per a un altre dia.
	if !pc.Local && strings.TrimSpace(p.APIKey) == "" {
		m.pendingKeyFor = nom
		return capsalera + "\n" + systemLine("escriu la clau de "+nom+T("prov.senseHistorial")), nil
	}
	ids, err := fetchModelIDs(p.BaseURL, p.APIKey)
	if err != nil || len(ids) == 0 {
		return capsalera + "\n" + systemLine(nom+" encara no llista models: "+errText(err)), nil
	}
	m.push(capsalera)
	return "", modelsDelProveidor(m, nom, ids)
}

// modelsDelProveidor és el selector de model d'un provider, que assigna
// al rol actiu i DESA. Abans el menú de models canviava el rol només per
// a la sessió i `/provider rol` sí que ho desava: dos camins que feien
// coses diferents sense dir-ho.
func modelsDelProveidor(m *Model, nom string, ids []string) *menuState {
	items := make([]menuItem, 0, len(ids))
	cur := m.roleRef()
	for _, id := range ids {
		text := id
		if cur.Provider == nom && cur.Model == id {
			text += " ← actual"
		}
		items = append(items, menuItem{text: text, val: id})
	}
	return &menuState{title: T("menu.modelDe") + nom, items: items, run: func(m *Model, model string) (string, *menuState) {
		r := m.roleRef()
		old := r
		r.Provider, r.Model = nom, model
		m.cfg.Roles[m.role] = r
		m.status = T("barra.llest")
		msg := okStyle.Render(fmt.Sprintf("✓ "+T("rol.assignat"), m.role, nom, model))
		if err := m.cfg.Save(m.cfgPath); err != nil {
			m.cfg.Roles[m.role] = old
			return systemLine(T("prov.noDesat") + err.Error()), nil
		}
		return msg + dimStyle.Render(" (desat)"), nil
	}}
}
