package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gregal/internal/pdfdoc"
)

// L'extractor de text de PDF viu a internal/pdfdoc: el comparteix amb les
// eines pdf_* de l'agent.

// ErrPDFEncrypted: el PDF està xifrat amb contrasenya d'usuari.
var ErrPDFEncrypted = pdfdoc.ErrEncrypted

// ExtractPDF torna una secció per pàgina amb text.
func ExtractPDF(data []byte) ([]Section, error) {
	pages, err := pdfdoc.ReadPagesBytes(context.Background(), data)
	if err != nil {
		return nil, err
	}
	var out []Section
	for _, pg := range pages {
		if strings.TrimSpace(pg.Text) == "" {
			continue
		}
		out = append(out, Section{Loc: fmt.Sprintf("page %d", pg.N), Text: pg.Text})
	}
	if len(out) == 0 {
		return nil, errors.New("PDF has no extractable text (scanned images?)")
	}
	return out, nil
}
