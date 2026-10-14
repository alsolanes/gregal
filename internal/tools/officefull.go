package tools

import (
	"fmt"
	"strings"
)

// OfficeReadFull és OfficeRead sense els topalls pensats per al context del
// model: tots els fulls, files i paràgrafs (fins a sostres de seguretat
// alts). La fa servir la indexació de coneixement (internal/knowledge), que
// trosseja el text després. El format és el mateix: «[FULL nom]» per full
// d'Excel, «— Diapo N —» per diapositiva i «[Estil] text» per paràgraf de
// Word amb estil.
func OfficeReadFull(path string) (string, error) {
	kind := OfficeKind(path)
	if kind == "legacy" {
		return "", fmt.Errorf("legacy office format (.doc/.xls/.ppt) is not supported: %s", path)
	}
	if kind == "" {
		return "", fmt.Errorf("not an office document: %s", path)
	}
	z, err := officeOpen(path)
	if err != nil {
		return "", err
	}
	defer z.Close()
	var out string
	switch kind {
	case "xlsx":
		out = officeReadXlsxLim(z, 200, 200000, 60)
	case "docx":
		out = officeReadDocxLim(z, 1000000)
	case "pptx":
		out = officeReadPptx(z)
	}
	return strings.TrimSpace(out), nil
}
