package tui

import (
	"regexp"
	"strings"
)

var conversaANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// findNext cerca dins del transcript renderitzat i mou el viewport a la
// coincidència. Una segona crida amb la mateixa consulta continua des d'on era
// i, en arribar al final, torna al principi.
func (m *Model) findNext(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	flat := strings.Split(strings.Join(m.lines, "\n"), "\n")
	if len(flat) == 0 {
		return false
	}
	start := 0
	if strings.EqualFold(query, m.lastFind) {
		start = (m.findLine + 1) % len(flat)
	}
	q := strings.ToLower(query)
	for step := 0; step < len(flat); step++ {
		i := (start + step) % len(flat)
		plain := conversaANSI.ReplaceAllString(flat[i], "")
		if strings.Contains(strings.ToLower(plain), q) {
			m.lastFind, m.findLine = query, i
			m.vp.YOffset = max(0, i-2)
			return true
		}
	}
	m.lastFind = query
	return false
}
