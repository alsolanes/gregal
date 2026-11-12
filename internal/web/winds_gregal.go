package web

import (
	"context"
	"strings"
	"sync"

	"gregal/internal/companion"
	"gregal/internal/llm"
)

// Gregal, l'assistent principal (pla v2 §16, B14): un vent integrat que fa la
// feina ell mateix amb totes les eines (cercar, recordatoris, vigilàncies,
// fitxers, ordinador, connexions) i que, a més, pot consultar els altres vents
// amb l'eina `wind` quan el seu ofici o la seva memòria ajuden. Substitueix el
// Ventet com a porta d'entrada: aquell només responia amb una crida al model
// sense eines i, per a tota la resta, demanava «Endavant» per passar-ho a un
// altre xat.
const gregalWindID = "gregal"

const gregalPersona = "You are Gregal, the user's personal assistant. Reply in the user's language, warm and brief. " +
	"Do the work yourself with your tools: search the web, read pages, set and edit reminders, watch things for the user (watch), " +
	"work with files in your folder, use your own computer and the user's connected accounts. Do not hand the conversation over: " +
	"you own the result. When a specialist wind clearly knows more (its trade, its memory of this user), ask it with the wind tool " +
	"in the background, then give the user the answer and say who helped (\"Marinada helped with this\"). " +
	"Ask for permission only before acting outside: sending, buying, publishing, deleting or writing outside your folder. " +
	"When something needs a decision, say it plainly and offer the main option. " +
	"When you return options, a comparison or things to choose from (trains, flights, products, places, prices), browse the real pages first " +
	"(your browser can open shops and booking sites), then write one short sentence and a ```gregal-result block with JSON: " +
	"{\"title\",\"subtitle\",\"icon\":\"one emoji\",\"rows\":[{\"lead\",\"text\",\"value\",\"tag\",\"pick\":true,\"url\"}],\"sources\":[{\"name\",\"url\"}],\"more\":{\"label\",\"url\"}}. " +
	"At most 5 rows; mark the best one with pick true and a short tag; only prices and times you actually read, never invented. " +
	"Keep the rest of the reply short: the card carries the details."

var gregalMu sync.Mutex

// gregalWindTool és l'eina wind del torn de Gregal. Va per una variable perquè
// el bucle dels vents i la consulta a un vent (que torna a fer servir el bucle)
// no formin un cicle d'inicialització.
var gregalWindTool func(*Server, context.Context, string) (string, error)

func init() { gregalWindTool = (*Server).runWindTool }

// ensureGregalWind crea Gregal si el compte encara no el té: amb el nom, el
// color i la cara que la persona havia donat al Ventet (o «Gregal»), i amb
// l'historial del Ventet com a inici del seu fil.
func (s *Server) ensureGregalWind() (companion.Wind, bool) {
	gregalMu.Lock()
	defer gregalMu.Unlock()
	if s.winds == nil {
		return companion.Wind{}, false
	}
	if w, ok := s.winds.ByID(gregalWindID); ok {
		// Un Gregal creat amb una versió anterior de la descripció per defecte
		// (sense targetes de resultat) l'actualitza; una de personalitzada, no.
		if strings.HasPrefix(w.Persona, "You are Gregal, the user's personal assistant.") && w.Persona != gregalPersona {
			w.Persona = gregalPersona
			if saved, err := s.winds.Save(w); err == nil {
				w = saved
			}
		}
		return w, true
	}
	cap := s.loadCap()
	name := strings.TrimSpace(cap.Name)
	if name == "" || name == capName {
		name = "Gregal"
	}
	color := strings.TrimSpace(cap.Color)
	if color == "" {
		color = "#3D7BFF"
	}
	w, err := s.winds.SaveNew(companion.Wind{
		Name: name, Color: color, Style: cap.Style, Persona: gregalPersona,
		Role: s.tr("El teu assistent", "Your assistant"),
	}, gregalWindID)
	if err != nil {
		return companion.Wind{}, false
	}
	// L'historial del Ventet passa a ser l'inici del fil de Gregal (només
	// el text de la conversa; les propostes «Endavant» ja no tenen sentit).
	if len(cap.History) > 0 {
		hist := windHistory(s, w.ID)
		hist.mu.Lock()
		empty := len(hist.messages) == 0
		hist.mu.Unlock()
		if empty {
			var msgs []llm.Message
			for _, m := range cap.History {
				text := strings.TrimSpace(m.Text)
				if text == "" {
					continue
				}
				role := "assistant"
				if m.From == "user" {
					role = "user"
				}
				msgs = append(msgs, llm.Message{Role: role, Content: text, Ts: m.Ts, ID: m.ID})
			}
			if len(msgs) > 0 {
				_ = s.appendWindThread(w, hist, msgs...)
			}
		}
	}
	return w, true
}

// windIsMainCtx diu si el torn és el de Gregal (l'assistent principal).
func windIsMainCtx(ctx context.Context) bool {
	k, ok := windSteerKeyOf(ctx)
	return ok && k.kind == "wind" && k.id == gregalWindID
}

// withoutGregal treu Gregal de les llistes d'especialistes: no es consulta a
// si mateix ni entra a les colles.
func withoutGregal(ws []companion.Wind) []companion.Wind {
	out := ws[:0:0]
	for _, w := range ws {
		if w.ID != gregalWindID {
			out = append(out, w)
		}
	}
	return out
}
