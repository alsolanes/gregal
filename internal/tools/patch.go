package tools

import (
	"fmt"
	"os"
	"strings"
)

// PatchOp és una edició: substitució de bloc (Old→New) o inserció
// després d'una línia (AfterLine>0, New). Una op fa servir un sol mode.
type PatchOp struct {
	Old       string
	New       string
	AfterLine int
}

// Patch aplica totes les edicions de cop o cap: primer valida (cada Old
// únic, cada AfterLine dins de rang), després escriu una sola vegada amb
// un sol snapshot al journal. L'ordre importa: s'apliquen en seqüència.
func Patch(path string, ops []PatchOp) error {
	if len(ops) == 0 {
		return fmt.Errorf("patch: cap edició")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cur := string(raw)
	// Mateixa història que a Edit: un fitxer amb \r\n i un bloc escrit amb
	// \n no casen mai. Aquí feia més mal encara, perquè patch aplica
	// diverses edicions seguides i n'hi ha prou que en falli una perquè no
	// se'n faci cap. Es treballa en pla i es torna a desar com era.
	crlf := strings.Contains(cur, "\r\n")
	if crlf {
		cur = senseCR(cur)
	}
	for i, op := range ops {
		op.Old, op.New = senseCR(op.Old), senseCR(op.New)
		if op.AfterLine > 0 {
			lines := strings.Split(cur, "\n")
			if op.AfterLine > len(lines) {
				return fmt.Errorf("patch: edició %d: línia %d fora de rang (%d línies)", i+1, op.AfterLine, len(lines))
			}
			ins := strings.Split(op.New, "\n")
			lines = append(lines[:op.AfterLine], append(ins, lines[op.AfterLine:]...)...)
			cur = strings.Join(lines, "\n")
			continue
		}
		if strings.TrimSpace(op.Old) == "" {
			return fmt.Errorf("patch: edició %d: old buit (fes servir after_line per inserir)", i+1)
		}
		if n := strings.Count(cur, op.Old); n != 1 {
			if n == 0 {
				return fmt.Errorf("patch: edició %d: bloc no trobat a %s: %s", i+1, path, perQueNoHiEs(cur, op.Old))
			}
			return fmt.Errorf("patch: edició %d: bloc ambigu a %s (%d ocurrències): posa-hi més context perquè n'hi hagi una de sola", i+1, path, n)
		}
		cur = strings.Replace(cur, op.Old, op.New, 1)
	}
	if crlf {
		cur = strings.ReplaceAll(cur, "\n", "\r\n")
	}
	Active.SnapOp(path, "patch")
	return os.WriteFile(path, []byte(cur), 0o644)
}
