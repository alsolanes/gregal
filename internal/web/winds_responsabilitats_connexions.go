package web

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Fonts «connection» de les responsabilitats (B6) sobre les connexions de B5:
// un calendari o una bústia connectats es vigilen amb el feed de canvis de
// connexioChanges, amb el cursor desat a respCursor.Feed. La primera mirada
// només fixa el punt de partida; després, només el que és nou o ha canviat
// entra a la preparació (com a dada, mai com a instrucció).
func init() { respConnectionSource = connexioRespSource }

type respConnexioSource struct {
	s  *Server
	id string
}

func connexioRespSource(s *Server, _ string, src respSource) (Source, error) {
	id := strings.TrimSpace(src.Ref)
	if !strings.HasPrefix(id, "con-") {
		return nil, fmt.Errorf("la font de connexió necessita l'id de la connexió (con-…)")
	}
	// connexioSources ja és només del compte d'aquest servidor: una connexió
	// d'un altre compte no hi surt i no es pot vigilar.
	for _, c := range s.connexioSources() {
		if c.ID != id {
			continue
		}
		if !c.Changes {
			return nil, fmt.Errorf("%s (%s) no té un registre de canvis que es pugui vigilar", c.AccountLabel, c.Service)
		}
		return respConnexioSource{s: s, id: id}, nil
	}
	return nil, fmt.Errorf("la connexió %s no està disponible en aquest compte (desconnectada o sense permís de lectura)", id)
}

func (c respConnexioSource) Check(ctx context.Context, prev respCursor) (respObservation, error) {
	items, next, err := c.s.connexioChanges(ctx, c.id, prev.Feed)
	if err != nil {
		return respObservation{Cursor: prev}, err
	}
	cur := prev
	cur.Feed = next
	cur.Hash = respHash("feed:" + next)
	if prev.Feed == "" || len(items) == 0 {
		// Primera mirada (l'estat actual no és notícia) o res de nou.
		return respObservation{Cursor: cur}, nil
	}
	var b strings.Builder
	for _, it := range items {
		fmt.Fprintf(&b, "- %s %s: %s", it.Kind, it.Change, strings.TrimSpace(it.Title))
		if !it.At.IsZero() {
			b.WriteString(" (" + it.At.UTC().Format(time.RFC3339) + ")")
		}
		if d := strings.TrimSpace(it.Detail); d != "" {
			b.WriteString(" — " + trimRunes(d, 400))
		}
		b.WriteByte('\n')
	}
	return respObservation{Cursor: cur, Changed: true, Excerpt: trimRunes(b.String(), respSnapshot)}, nil
}
