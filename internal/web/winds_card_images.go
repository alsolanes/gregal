package web

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Fotos per a les respostes del xat dels vents. Una foto és el millor d'una
// resposta sobre un lloc, un plat o una persona, però els models barats gairebé
// mai criden web_images (és un pas més). Per això el vent només ha de dir QUÈ
// vol ensenyar i el servidor busca la foto abans de desar la resposta:
//
//   - a les targetes (```gregal-plan, ```gregal-recipe, ```gregal-result):
//     image_query, o el títol si no n'hi ha;
//   - al text: una línia ![Sagrada Família](image: Sagrada Familia Barcelona).
//
// Si no en troba cap, la targeta queda sense foto i la línia d'imatge se'n va.

// cardImageSearch és web_images; els tests el substitueixen.
var cardImageSearch = tools.WebImages

var (
	cardBlockRe  = regexp.MustCompile("(?s)```(gregal-(?:plan|recipe|result))([^\\n]*)\\n?(.*?)```")
	cardImageURL = regexp.MustCompile(`(?m)^\s*image:\s*(https?://\S+)`)
	// textImageRe: ![alt](image: consulta) — la foto que el vent demana al text.
	textImageRe = regexp.MustCompile(`!\[([^\]\n]*)\]\(\s*image:\s*([^)\n]{2,80})\)`)
	blankRunRe  = regexp.MustCompile(`\n[ \t]*\n(?:[ \t]*\n)+`)
)

// windCardImages torna el text amb les fotos posades. Com a molt quatre
// cerques per resposta i 8 s en total.
func (s *Server) windCardImages(reply string) string {
	hasCard, hasText := strings.Contains(reply, "```gregal-"), strings.Contains(reply, "](image:") || strings.Contains(reply, "]( image:")
	if !hasCard && !hasText {
		return reply
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	searches := 0
	find := func(q string) string {
		if q = strings.TrimSpace(q); q == "" || searches >= 4 || ctx.Err() != nil {
			return ""
		}
		searches++
		out, err := cardImageSearch(ctx, q, 3)
		if err != nil {
			return ""
		}
		if u := cardImageURL.FindStringSubmatch(out); u != nil {
			return u[1]
		}
		return ""
	}
	if hasCard {
		reply = cardBlockRe.ReplaceAllStringFunc(reply, func(block string) string {
			m := cardBlockRe.FindStringSubmatch(block)
			kind, raw := m[1], strings.TrimSpace(strings.TrimSpace(m[2])+"\n"+m[3])
			card, ok := cardJSON(raw)
			if !ok {
				return block
			}
			query, _ := card["image_query"].(string)
			delete(card, "image_query")
			image, _ := card["image"].(string)
			image = strings.TrimSpace(image)
			// Una adreça que no ve de cap eina (el model se la inventa) no val.
			if image != "" && !strings.HasPrefix(image, "https://") && !strings.HasPrefix(image, "http://") && !windImageID.MatchString(strings.TrimPrefix(image, "/api/winds/image?id=")) {
				delete(card, "image")
				image = ""
			}
			// A les opcions, la foto només si el vent l'ha demanada (image_query).
			if image == "" && (kind != "gregal-result" || query != "") {
				for _, q := range []string{query, cardString(card, "title")} {
					if u := find(q); u != "" {
						card["image"] = u
						break
					}
				}
			}
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(false)
			if enc.Encode(card) != nil {
				return block
			}
			return "```" + kind + "\n" + strings.TrimSpace(buf.String()) + "\n```"
		})
	}
	if hasText {
		reply = textImageRe.ReplaceAllStringFunc(reply, func(tok string) string {
			m := textImageRe.FindStringSubmatch(tok)
			alt := strings.TrimSpace(m[1])
			u := find(m[2])
			if u == "" {
				return ""
			}
			return "![" + alt + "](" + u + ")"
		})
		// Una línia que només portava la foto no trobada queda buida: fora.
		reply = strings.TrimSpace(blankRunRe.ReplaceAllString(reply, "\n\n"))
	}
	return reply
}

// cardJSON llegeix l'objecte d'una targeta. Els models barats, de tant en
// tant, tornen a començar un tros a mig escriure («…Pont Eiffel, {"time":
// "15:30",…» repetint l'element sencer): si el JSON no és vàlid, prova de
// treure el tros començat i repetit, i així la targeta (i la foto) surten.
func cardJSON(raw string) (map[string]any, bool) {
	var card map[string]any
	if json.Unmarshal([]byte(raw), &card) == nil {
		return card, true
	}
	tries := 0
	for i := 1; i < len(raw) && tries < 60; i++ {
		if raw[i] != '{' || i+20 > len(raw) {
			continue
		}
		probe := raw[i : i+20]
		j := strings.Index(raw[:i], probe)
		if j < 0 {
			continue
		}
		tries++
		card = nil
		if json.Unmarshal([]byte(raw[:j]+raw[i:]), &card) == nil {
			return card, true
		}
	}
	return nil, false
}

func cardString(card map[string]any, key string) string {
	v, _ := card[key].(string)
	return v
}

// subjectPicker diu de què va una resposta, per buscar-ne la foto: 2-4
// paraules en anglès o "" si no va d'una cosa que es pugui fotografiar. Els
// tests el substitueixen.
var subjectPicker = func(s *Server, ctx context.Context, question, reply string) string {
	p, role, ok := s.sideRoleRef()
	if !ok {
		return ""
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	if len(reply) > 1500 {
		reply = reply[:1500]
	}
	out, err := agent.SideCallLateral(ctx, client, cfg, p, role, []llm.Message{
		{Role: "system", Content: subjectPickerPrompt},
		{Role: "user", Content: "Question: " + oneLine(question, 300) + "\n\nAnswer:\n" + reply},
	})
	if err != nil {
		return ""
	}
	return cleanSubject(out)
}

const subjectPickerPrompt = "You choose the photo for a chat answer. Reply with ONLY 2-4 English words naming the one specific place, building, dish, artwork, animal, plant, product or well-known person the answer is mainly about, as you would search for it on Wikimedia Commons (for example: Sagrada Familia Barcelona, pa amb tomaquet, Merce Rodoreda). If the answer is not mainly about one such thing (advice, code, feelings, small talk, plans, general prices, abstract topics), reply NONE."

// cleanSubject: la primera línia, sense cometes ni punt; NONE o massa llarg, res.
func cleanSubject(out string) string {
	out = strings.TrimSpace(strings.SplitN(strings.TrimSpace(out), "\n", 2)[0])
	out = strings.Trim(out, " \"'`.*«»")
	if out == "" || strings.HasPrefix(strings.ToUpper(out), "NONE") || len(strings.Fields(out)) > 6 || len(out) > 60 {
		return ""
	}
	return out
}

// windSubjectPhoto: si la resposta va d'una cosa concreta (un lloc, un plat,
// una persona…) i el vent no hi ha posat cap foto, el servidor en tria una.
// Els models barats gairebé mai criden web_images ni escriuen la línia de
// foto, i una foto és el que més millora aquestes respostes. Va després de
// la primera frase o paràgraf. Com a molt 10 s; si res, la resposta queda igual.
func (s *Server) windSubjectPhoto(question, reply string) string {
	if len(reply) < 160 || strings.Contains(reply, "![") || strings.Contains(reply, "```gregal-") || strings.Contains(reply, "```chart") {
		return reply
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	subject := subjectPicker(s, ctx, question, reply)
	if subject == "" || ctx.Err() != nil {
		return reply
	}
	out, err := cardImageSearch(ctx, subject, 3)
	if err != nil {
		return reply
	}
	u := cardImageURL.FindStringSubmatch(out)
	if u == nil {
		return reply
	}
	line := "![" + subject + "](" + u[1] + ")"
	// Després del primer paràgraf (o al final si només n'hi ha un), mai dins
	// d'una llista, una taula o un bloc de codi.
	if i := strings.Index(reply, "\n\n"); i > 0 {
		first := reply[:i]
		if !strings.HasPrefix(strings.TrimSpace(first), "|") && !strings.HasPrefix(strings.TrimSpace(first), "```") && !strings.HasPrefix(strings.TrimSpace(first), "- ") {
			return first + "\n\n" + line + reply[i:]
		}
		return line + "\n\n" + reply
	}
	return reply + "\n\n" + line
}
