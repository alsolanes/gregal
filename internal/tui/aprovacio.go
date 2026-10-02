package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// El diàleg d'aprovació: què s'aprova i amb quins botons.
//
// Abans hi havia dos botons (Permet i Denega) i el que es veia del canvi
// era el JSON dels arguments retallat a 300 caràcters. El diff arribava
// després d'aprovar, que és tard. «Recorda per la sessió» existia però
// només com a lletra dins d'una pista, i a la pràctica ningú la trobava:
// l'agent semblava demanar permís per la mateixa cosa a cada pas.

// botoConfirm és un botó del diàleg.
type botoConfirm struct {
	text     string
	resposta string // el que rep answerPending
	to       string // "ok" | "warn" | "bad"
}

func (b botoConfirm) estil() lipgloss.Style {
	switch b.to {
	case "bad":
		return badStyle
	case "warn":
		return warnStyle
	default:
		return okStyle
	}
}

// botonsConfirm són els botons d'aquesta aprovació. «Sempre» només surt
// quan l'aprovació ve de l'agent: per a una ordre escrita a mà (/bash,
// /write) no hi ha cap signatura d'eina que recordar.
func (m Model) botonsConfirm() []botoConfirm {
	out := []botoConfirm{{text: "✓ " + T("aprova.permet"), resposta: "s", to: "ok"}}
	if m.agentActive {
		out = append(out, botoConfirm{text: "↻ " + T("aprova.sempre"), resposta: "r", to: "warn"})
	}
	return append(out, botoConfirm{text: "✗ " + T("aprova.denega"), resposta: "n", to: "bad"})
}

// retallaLinies deixa el bloc en n línies com a màxim.
func retallaLinies(s string, n int) string {
	ls := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(ls) <= n {
		return strings.Join(ls, "\n")
	}
	return strings.Join(append(ls[:n], dimStyle.Render("… i "+strconv.Itoa(len(ls)-n)+" línies més")), "\n")
}

// previewCanvi pinta el canvi que s'està aprovant, sense tocar res del
// disc: per a un edit, el diff del bloc que se substitueix; per a un
// write, el diff contra el fitxer que hi ha ara (o el cos, si és nou).
// Torna "" quan no hi ha res útil a ensenyar (la descripció ja ho diu).
func previewCanvi(dir, name, argsJSON string) string {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Old     string `json:"old_string"`
		New     string `json:"new_string"`
		Edits   []struct {
			Old string `json:"old"`
			New string `json:"new"`
		} `json:"edits"`
	}
	if json.Unmarshal([]byte(argsJSON), &a) != nil {
		return ""
	}
	ruta := a.Path
	if dir != "" && ruta != "" && !filepath.IsAbs(ruta) {
		ruta = filepath.Join(dir, ruta)
	}
	switch name {
	case "write":
		vell, err := os.ReadFile(ruta)
		if err != nil {
			// Fitxer nou: el cos, que és exactament el que es crearà.
			return diffBlock(a.Path, "", a.Content, 40)
		}
		if string(vell) == a.Content {
			return dimStyle.Render("(el contingut no canvia)")
		}
		return diffBlock(a.Path, string(vell), a.Content, 40)
	case "edit":
		if a.Old == "" && a.New == "" {
			return ""
		}
		return diffBlock(a.Path, a.Old, a.New, 40)
	case "patch":
		var b strings.Builder
		for i, e := range a.Edits {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(diffBlock(a.Path, e.Old, e.New, 20))
		}
		return b.String()
	}
	return ""
}

// aprovacioBox és la confirmació d'una eina delicada com a diàleg
// superposat: què vol fer, el canvi (si és una escriptura) i els tres
// botons. El que s'aprova no es retalla: és una ordre o un fitxer, i
// s'embolcalla a l'amplada del diàleg.
func (m Model) aprovacioBox(ample int) string {
	w := ampleDialeg(ample) - 4
	var caselles []string
	for i, b := range m.botonsConfirm() {
		if i == m.confirmIdx {
			caselles = append(caselles, b.estil().Render("▸ "+b.text))
		} else {
			caselles = append(caselles, dimStyle.Render("  "+b.text+"  "))
		}
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Width(w).Render(m.pending.desc) + "\n")
	if p := m.pending.preview; p != "" {
		b.WriteString(retallaLinies(p, 14) + "\n")
	}
	b.WriteString(strings.Join(caselles, "   "))
	return dialeg(T("app.confirmaEspera"), b.String(), T("app.aprovaPistes"), ample)
}
