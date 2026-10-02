package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// menuWin és l'alçada màxima de la finestra del menú (amb scroll si cal).
// La llista no canvia mentre el menú és actiu, així que l'alçada és
// constant i el text de dalt no balla en navegar.
const menuWin = 8

// menuItem és una fila seleccionable: text visible + valor.
type menuItem struct {
	text string
	val  string
}

// menuState és un menú modal navegable amb fletxes (estil opencode).
// run s'executa en triar: torna missatge a mostrar + submenú (o nil).
type menuState struct {
	title string
	items []menuItem
	idx   int
	top   int // primera fila visible (scroll)
	back  *menuState
	// filtre és el que s'ha escrit per acotar la llista. Amb un
	// proveïdor que llista seixanta models, triar-ne un volia dir baixar
	// cinquanta cops amb la fletxa: ara escrius tres lletres i ja hi ets.
	filtre string
	run    func(m *Model, val string) (msg string, next *menuState)
}

// visibles són les files que encaixen amb el filtre (totes si és buit).
// La comparació ignora majúscules i accents no: és un filtre de teclat,
// no un cercador.
func (mn *menuState) visibles() []menuItem {
	if mn == nil {
		return nil
	}
	f := strings.ToLower(strings.TrimSpace(mn.filtre))
	if f == "" {
		return mn.items
	}
	out := make([]menuItem, 0, len(mn.items))
	for _, it := range mn.items {
		if strings.Contains(strings.ToLower(it.text), f) {
			out = append(out, it)
		}
	}
	return out
}

// tria retorna la fila seleccionada (ok=false si el filtre no en deixa cap).
func (mn *menuState) tria() (menuItem, bool) {
	vis := mn.visibles()
	if len(vis) == 0 {
		return menuItem{}, false
	}
	if mn.idx < 0 || mn.idx >= len(vis) {
		mn.idx = 0
	}
	return vis[mn.idx], true
}

// escriu afegeix text al filtre i torna al principi de la llista.
func (mn *menuState) escriu(s string) {
	mn.filtre += s
	mn.idx, mn.top = 0, 0
}

// esborra treu l'últim caràcter del filtre.
func (mn *menuState) esborra() {
	if r := []rune(mn.filtre); len(r) > 0 {
		mn.filtre = string(r[:len(r)-1])
	}
	mn.idx, mn.top = 0, 0
}

// menuLine pinta el menú com a diàleg superposat (finestra fixa amb
// scroll). L'alt no canvia en navegar: la llista és constant mentre el
// menú és obert, i el text de sota no balla.
func menuLine(mn *menuState, ample int) string {
	if mn == nil || len(mn.items) == 0 {
		return ""
	}
	vis := mn.visibles()
	if mn.idx >= len(vis) {
		mn.idx = max(0, len(vis)-1)
	}
	if mn.idx < mn.top {
		mn.top = mn.idx
	}
	if mn.idx >= mn.top+menuWin {
		mn.top = mn.idx - menuWin + 1
	}
	if mn.top < 0 {
		mn.top = 0
	}
	inner := ampleDialeg(ample) - 4
	count := faintStyle.Render(fmt.Sprintf("%d/%d", min(mn.idx+1, len(vis)), len(vis)))
	titol := "◆ " + mn.title
	if mn.filtre != "" {
		titol += "  /" + mn.filtre
	}
	gap := inner - lipgloss.Width(titol) - lipgloss.Width(count)
	if gap < 1 {
		gap = 1
	}
	var b strings.Builder
	if len(vis) == 0 {
		b.WriteString(dimStyle.Render("  " + T("menu.capCoincidencia")))
	}
	shown := 0
	for i := mn.top; i < len(vis) && shown < menuWin; i++ {
		text := ansi.Truncate(vis[i].text, inner-2, "…")
		if i == mn.idx {
			b.WriteString(menuHL.Width(inner).Render("▸ "+text) + "\n")
		} else {
			b.WriteString(dimStyle.Render("  "+text) + "\n")
		}
		shown++
	}
	return dialeg(titol+strings.Repeat(" ", gap)+count, b.String(), T("menu.pistes"), ample)
}

// menuMove mou el cursor del menú (amb wrap sobre les files visibles).
func menuMove(mn *menuState, d int) {
	if mn == nil {
		return
	}
	n := len(mn.visibles())
	if n == 0 {
		return
	}
	mn.idx = (mn.idx + d + n) % n
}
