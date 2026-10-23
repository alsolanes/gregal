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
// Cada Old passa per la mateixa cascada que Edit (fuzzy.go).
func Patch(path string, ops []PatchOp) error {
	_, err := pedaça(path, ops, nil)
	return err
}

// pedaça és el cos de Patch; seg com a edita.
func pedaça(path string, ops []PatchOp, seg *Seguiment) (string, error) {
	if len(ops) == 0 {
		return "", fmt.Errorf("patch: cap edició")
	}
	// Com a Edit: per damunt del sostre, fallar net val més que OOM.
	if fi, err := os.Stat(path); err == nil && fi.Size() > MaxCaptureBytes {
		return "", fmt.Errorf("%s massa gran per al patch (%d bytes; sostre %d): fes servir bash per a fitxers així", path, fi.Size(), MaxCaptureBytes)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	// Mateixa història que a Edit: un fitxer amb \r\n i un bloc escrit amb
	// \n no casen mai. Aquí feia més mal encara, perquè patch aplica
	// diverses edicions seguides i n'hi ha prou que en falli una perquè no
	// se'n faci cap. Es treballa en pla i es torna a desar com era.
	orig := senseCR(string(raw))
	crlf := strings.Contains(string(raw), "\r\n")
	alDia, perque := true, ""
	if seg != nil {
		alDia, perque = seg.alDia(path, raw)
	}
	cur := orig
	var notes []string
	for i, op := range ops {
		op.Old, op.New = senseCR(op.Old), senseCR(op.New)
		if op.AfterLine > 0 {
			// Inserir per número de línia en un fitxer que el model no té
			// al dia és inserir a cegues.
			if !alDia {
				return "", fmt.Errorf("patch: edició %d: %s %s; after_line compta línies d'una versió que no és l'actual. Llegeix-lo amb read i torna-ho a provar", i+1, path, perque)
			}
			lines := strings.Split(cur, "\n")
			if op.AfterLine > len(lines) {
				return "", fmt.Errorf("patch: edició %d: línia %d fora de rang (%d línies)", i+1, op.AfterLine, len(lines))
			}
			ins := strings.Split(op.New, "\n")
			lines = append(lines[:op.AfterLine], append(ins, lines[op.AfterLine:]...)...)
			cur = strings.Join(lines, "\n")
			continue
		}
		if strings.TrimSpace(op.Old) == "" {
			return "", fmt.Errorf("patch: edició %d: old buit (fes servir after_line per inserir)", i+1)
		}
		nou, nivell, err := substitueix(cur, op.Old, op.New)
		if err != nil {
			return "", fmt.Errorf("patch: edició %d: %w", i+1, errEdicio(path, cur, op.Old, err))
		}
		if nivell != nivExacte {
			if !alDia {
				return "", fmt.Errorf("patch: edició %d: %w", i+1, errNoAlDia(path, perque, nivell))
			}
			notes = append(notes, fmt.Sprintf("edició %d: «%s»", i+1, nivell))
		}
		cur = nou
	}
	desat := cur
	if crlf {
		desat = strings.ReplaceAll(cur, "\n", "\r\n")
	}
	Active.SnapOp(path, "patch")
	if err := os.WriteFile(path, []byte(desat), 0o644); err != nil {
		return "", err
	}
	if seg != nil {
		seg.conegut(path, []byte(desat))
	}
	var nota string
	if len(notes) > 0 {
		nota = "\n(s'ha aplicat amb coincidència aproximada — " + strings.Join(notes, ", ") + "; revisa el diff)"
	}
	return nota + resumCanvi(orig, cur), nil
}
