package web

import (
	"bytes"
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"gregal/internal/tools"
)

// Fotos per a les targetes riques del xat (```gregal-plan, ```gregal-recipe).
// Una foto és el millor d'una resposta sobre un lloc o un plat, però els models
// barats sovint no criden web_images. Si la targeta arriba sense image, el
// servidor en busca una (image_query que hagi escrit el vent, o el títol) i la
// hi posa. Si no en troba cap, la targeta queda com estava.

// cardImageSearch és web_images; els tests el substitueixen.
var cardImageSearch = tools.WebImages

var (
	cardBlockRe  = regexp.MustCompile("(?s)```(gregal-(?:plan|recipe|result))([^\\n]*)\\n?(.*?)```")
	cardImageURL = regexp.MustCompile(`(?m)^\s*image:\s*(https?://\S+)`)
)

// windCardImages torna el text amb la foto posada a les targetes que no en
// tenien. Com a molt tres cerques per resposta i 8 s en total.
func (s *Server) windCardImages(reply string) string {
	if !strings.Contains(reply, "```gregal-") {
		return reply
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	searches := 0
	return cardBlockRe.ReplaceAllStringFunc(reply, func(block string) string {
		m := cardBlockRe.FindStringSubmatch(block)
		kind, raw := m[1], strings.TrimSpace(strings.TrimSpace(m[2])+"\n"+m[3])
		var card map[string]any
		if json.Unmarshal([]byte(raw), &card) != nil {
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
		if image == "" && (kind != "gregal-result" || query != "") && searches < 3 && ctx.Err() == nil {
			for _, q := range []string{query, cardString(card, "title")} {
				if q = strings.TrimSpace(q); q == "" {
					continue
				}
				searches++
				if out, err := cardImageSearch(ctx, q, 3); err == nil {
					if u := cardImageURL.FindStringSubmatch(out); u != nil {
						card["image"] = u[1]
						break
					}
				}
				if searches >= 3 || ctx.Err() != nil {
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

func cardString(card map[string]any, key string) string {
	v, _ := card[key].(string)
	return v
}
