package tools

import (
	"context"
	"errors"
)

// PublicarFitxer el posa el servidor web: copia el fitxer a un lloc públic i en
// torna l'enllaç, el nom i la mida. És una variable perquè les eines no poden
// importar el paquet web (el web ja importa les eines).
var PublicarFitxer func(ctx context.Context, path string) (enllac string, nom string, mida int64, err error)

// ErrFitxersNo surt quan l'eina existeix però aquest servidor no sap publicar.
var ErrFitxersNo = errors.New("enviar fitxers no està disponible en aquest servidor")

// PublicaFitxer envia un fitxer al xat i en torna l'enllaç perquè el model el
// posi a la resposta.
func PublicaFitxer(ctx context.Context, path string) (string, string, int64, error) {
	if PublicarFitxer == nil {
		return "", "", 0, ErrFitxersNo
	}
	return PublicarFitxer(ctx, path)
}
