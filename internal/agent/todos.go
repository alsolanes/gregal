package agent

// Pla viu: el checklist (todowrite) ha de reflectir per on va la feina
// sense que calgui confiar que el model se'n recordi a cada pas.
//
// Dues peces:
//   - TodosDelPla converteix el pla aprovat (/plan → executa) en checklist:
//     els passos numerats surten al cockpit des del primer segon, en
//     comptes d'esperar que el model els torni a escriure amb todowrite.
//   - RecordatoriTodos és la nota que s'afegeix a l'últim resultat d'eina
//     quan s'han executat prou eines sense cap todowrite: al model li
//     costa poc marcar el pas fet, i a qui mira la pantalla li canvia molt.

import (
	"fmt"
	"regexp"
	"strings"

	"gregal/internal/tools"
)

// TodoNudgeEvery és el nombre d'eines executades sense todowrite a partir
// del qual es recorda al model que actualitzi el checklist.
const TodoNudgeEvery = 5

var rePasPla = regexp.MustCompile(`^\s*(?:\*\*)?(\d{1,2})[.)]\s*(?:\*\*)?\s*(.+?)\s*$`)

// TodosDelPla extreu els passos numerats d'un pla ("1. …", "2) …") com a
// checklist, el primer en marxa. Ignora la línia VERIFICACIÓ i les
// sub-línies; sense cap pas numerat torna nil.
func TodosDelPla(pla string) []tools.TodoItem {
	var items []tools.TodoItem
	seen := map[string]bool{}
	darrer := 0
	for _, ln := range strings.Split(pla, "\n") {
		// Línia sagnada (dos espais o tabulador): sub-llista d'un pas.
		if strings.HasPrefix(ln, "  ") || strings.HasPrefix(ln, "\t") {
			continue
		}
		m := rePasPla.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		// Els passos van seguits (1, 2, 3…): un "3." després d'un "7." és
		// una llista dins d'un pas, no un pas nou.
		n := 0
		fmt.Sscanf(m[1], "%d", &n)
		if n != darrer+1 {
			continue
		}
		darrer = n
		titol := strings.TrimSpace(strings.Trim(strings.ReplaceAll(m[2], "**", ""), "*_ "))
		if strings.HasPrefix(strings.ToUpper(titol), "VERIFICACIÓ") || titol == "" || seen[titol] {
			continue
		}
		seen[titol] = true
		if r := []rune(titol); len(r) > 120 {
			titol = string(r[:117]) + "…"
		}
		items = append(items, tools.TodoItem{Title: titol, Status: "pending"})
		if len(items) >= 20 {
			break
		}
	}
	if len(items) == 0 {
		return nil
	}
	items[0].Status = "working"
	return items
}

// RecordatoriTodos torna la nota per al model quan hi ha checklist amb
// passos pendents i s'han executat n eines des de l'últim todowrite; buit
// si no toca. El text va enganxat al final del resultat de l'última eina
// del pas, no com a missatge d'usuari: així no trenca el torn ni compta
// com a instrucció nova.
func RecordatoriTodos(items []tools.TodoItem, n int) string {
	if n < TodoNudgeEvery {
		return ""
	}
	done, total := tools.StatsTodos(items)
	if total == 0 || done >= total {
		return ""
	}
	actual := ""
	for _, it := range items {
		if it.Status == "working" {
			actual = it.Title
			break
		}
	}
	msg := fmt.Sprintf("\n\n[checklist %d/%d sense actualitzar des de fa %d eines", done, total, n)
	if actual != "" {
		msg += fmt.Sprintf("; pas en marxa: «%s»", truncaRunes(actual, 60))
	}
	return msg + ". Si l'has acabat, crida todowrite marcant-lo done i el següent working; si no, continua.]"
}

func truncaRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
