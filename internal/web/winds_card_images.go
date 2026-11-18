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
	cardBlockRe = regexp.MustCompile("(?s)```(gregal-(?:plan|recipe|result|compare|facts))([^\\n]*)\\n?(.*?)```")
	// textImageRe: ![alt](image: consulta) — la foto que el vent demana al text.
	textImageRe = regexp.MustCompile(`!\[([^\]\n]*)\]\(\s*image:\s*([^)\n]{2,80})\)`)
	blankRunRe  = regexp.MustCompile(`\n[ \t]*\n(?:[ \t]*\n)+`)
)

// windCardImages torna el text amb les fotos posades. Com a molt quatre
// cerques per resposta i 8 s en total.
func (s *Server) windCardImages(reply string) string {
	reply = embedImageLinks(reply)
	hasCard, hasText := strings.Contains(reply, "```gregal-"), strings.Contains(reply, "](image:") || strings.Contains(reply, "]( image:")
	if !hasCard && !hasText {
		return reply
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	searches := 0
	find := func(q string) string {
		if q = strings.TrimSpace(q); q == "" || searches >= 6 || ctx.Err() != nil {
			return ""
		}
		searches++
		out, err := cardImageSearch(ctx, q, 3)
		if err != nil {
			return ""
		}
		return relevantImage(q, out)
	}
	if hasCard {
		reply = cardBlockRe.ReplaceAllStringFunc(reply, func(block string) string {
			m := cardBlockRe.FindStringSubmatch(block)
			kind, raw := m[1], strings.TrimSpace(strings.TrimSpace(m[2])+"\n"+m[3])
			card, ok := cardJSON(raw)
			if !ok {
				return block
			}
			// Xifres clau: no porten foto; només es deixa el JSON net (reparat).
			if kind == "gregal-facts" {
				var buf bytes.Buffer
				enc := json.NewEncoder(&buf)
				enc.SetEscapeHTML(false)
				if enc.Encode(card) != nil {
					return block
				}
				return "```" + kind + "\n" + strings.TrimSpace(buf.String()) + "\n```"
			}
			// Comparació: una foto per a cada cosa que es compara (no una a dalt).
			if kind == "gregal-compare" {
				items, _ := card["items"].([]any)
				// Sense image_query, el nom és en català («Terra», «Mart») i a
				// Commons no troba res: una sola crida curta en dona el títol
				// en anglès de cada un (Earth, Mars).
				var missing []string
				for _, it := range items {
					if item, ok := it.(map[string]any); ok && cardString(item, "image_query") == "" && cardString(item, "image") == "" {
						missing = append(missing, cardString(item, "name"))
					}
				}
				english := map[string]string{}
				if len(missing) > 0 && ctx.Err() == nil {
					english = compareEnglishNames(s, ctx, missing)
				}
				for _, it := range items {
					if item, ok := it.(map[string]any); ok && cardString(item, "image_query") == "" {
						if en := english[cardString(item, "name")]; en != "" {
							item["image_query"] = en
						}
					}
				}
				for _, it := range items {
					item, ok := it.(map[string]any)
					if !ok {
						continue
					}
					q, _ := item["image_query"].(string)
					delete(item, "image_query")
					img, _ := item["image"].(string)
					if img = strings.TrimSpace(img); img != "" && !strings.HasPrefix(img, "https://") && !strings.HasPrefix(img, "http://") {
						delete(item, "image")
						img = ""
					}
					if img == "" {
						for _, qq := range []string{q, cardString(item, "name")} {
							if u := find(qq); u != "" {
								item["image"] = u
								break
							}
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
	// Claudàtors que no lliguen (el model tanca la llista abans d'hora:
	// «…}],"pick":true},{…») o que falten al final: es reparen.
	if fixed := balanceJSON(raw); fixed != raw {
		card = nil
		if json.Unmarshal([]byte(fixed), &card) == nil {
			return card, true
		}
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
var subjectPicker = func(s *Server, ctx context.Context, question, reply string, asked bool) string {
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
	prompt := subjectPickerPrompt
	if asked {
		prompt = subjectPickerAskedPrompt
	}
	out, err := agent.SideCallLateral(ctx, client, cfg, p, role, []llm.Message{
		{Role: "system", Content: prompt},
		{Role: "user", Content: "Question: " + oneLine(question, 300) + "\n\nAnswer:\n" + reply},
	})
	if err != nil {
		return ""
	}
	return cleanSubject(out)
}

const subjectPickerPrompt = "You choose the photo for a chat answer. Reply with ONLY 2-4 English words naming the one specific place, building, dish, artwork, animal, plant, product, organ or body part, or well-known person the answer is mainly about, as you would search for it on Wikimedia Commons (for example: Sagrada Familia Barcelona, pa amb tomaquet, Merce Rodoreda). If the answer is not mainly about one such thing (advice, code, feelings, small talk, plans, general prices, abstract topics), reply NONE."

// subjectPickerAskedPrompt: la persona ha demanat imatges; val tot el que es
// pot veure en una foto real de la Viquipèdia o Commons (també anatomia o
// objectes), no només llocs i plats.
const subjectPickerAskedPrompt = "The user asked to SEE images. Reply with ONLY 2-4 English words naming the one thing to show in a real photo or illustration from Wikipedia / Wikimedia Commons: a place, building, dish, artwork, animal, plant, product, person, object, body part or anatomical structure, as the Wikipedia article would title it (for example: Abdominal external oblique muscle, Sagrada Familia, Golden retriever). Reply NONE only if nothing in the answer can be pictured."

// askedForImages: la pregunta demana veure imatges o fotos.
func askedForImages(question string) bool {
	q := " " + imageNorm(question) + " "
	for _, w := range []string{" imatge", " imatges", " foto ", " fotos", " fotografia", " image", " images", " photo", " photos", " picture", " pictures", " imagen", " imagenes"} {
		if strings.Contains(q, w) {
			return true
		}
	}
	return false
}

// photoCardRe: targetes que ja duen foto o que no la volen al costat.
var photoCardRe = regexp.MustCompile("```gregal-(?:recipe|plan|result|route|compare|web)")

// replyHasImage: la resposta ja porta una foto (al text o a una targeta).
func replyHasImage(reply string) bool {
	return strings.Contains(reply, "![") || strings.Contains(reply, `"image":"http`) || strings.Contains(reply, `"image":"/api/winds/image`)
}

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
	// Si t'ha demanat imatges i la resposta no en té cap (el model ha fet un
	// gràfic, una web o un dibuix), hi va la foto encara que hi hagi targetes.
	asked := askedForImages(question)
	// Les xifres clau (```gregal-facts) no porten foto: no la treuen.
	if len(reply) < 160 || strings.Contains(reply, "![") || (!asked && (photoCardRe.MatchString(reply) || strings.Contains(reply, "```chart"))) {
		return reply
	}
	if asked && replyHasImage(reply) {
		return reply
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ctx = backgroundModelCtx(ctx)
	subject := subjectPicker(s, ctx, question, reply, asked)
	if subject == "" || ctx.Err() != nil {
		return reply
	}
	out, err := cardImageSearch(ctx, subject, 3)
	if err != nil {
		return reply
	}
	img := relevantImage(subject, out)
	if img == "" {
		return reply
	}
	line := "![" + subject + "](" + img + ")"
	if asked {
		// Abans de la primera targeta o gràfic: el que has demanat, a dalt.
		if i := strings.Index(reply, "```"); i > 0 {
			if j := strings.LastIndex(reply[:i], "\n\n"); j > 0 {
				return reply[:j] + "\n\n" + line + reply[j:]
			}
		}
	}
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

// relevantImage tria, d'entre els resultats de web_images, la primera foto que
// es diu com el que s'ha buscat: el títol, la pàgina o el nom del fitxer han de
// compartir una paraula amb sentit de la consulta. Abans es posava la primera
// que sortia, i una recepta de sípia amb pèsols acabava amb una foto de
// farcellets de col. Millor cap foto que una que enganya.
func relevantImage(query, out string) string {
	words := imageQueryWords(query)
	if len(words) == 0 {
		return ""
	}
	blocks := strings.Split(out, "\n")
	title := ""
	for _, line := range blocks {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "image:") {
			img := strings.TrimSpace(strings.TrimPrefix(t, "image:"))
			if strings.HasPrefix(img, "http") && imageMatches(words, title+" "+img) {
				return img
			}
			continue
		}
		if strings.HasPrefix(t, "page:") {
			continue
		}
		// «1. Títol»: el títol del resultat següent.
		if i := strings.Index(t, ". "); i > 0 && i < 4 {
			t = t[i+2:]
		}
		title = t
	}
	return ""
}

// imageStop: paraules que no diuen què és (articles, preposicions, «recepta»…).
var imageStop = map[string]bool{
	"amb": true, "del": true, "dels": true, "les": true, "els": true, "per": true, "una": true, "uns": true, "the": true, "and": true,
	"with": true, "from": true, "con": true, "los": true, "las": true, "recepta": true, "recipe": true, "receta": true, "plat": true,
	"dish": true, "foto": true, "photo": true, "image": true, "casa": true, "home": true, "made": true, "style": true, "tradicional": true,
	"traditional": true, "catalan": true, "catalana": true, "catala": true,
}

func imageNorm(s string) string {
	s = strings.ToLower(s)
	r := strings.NewReplacer("à", "a", "á", "a", "è", "e", "é", "e", "í", "i", "ï", "i", "ò", "o", "ó", "o", "ú", "u", "ü", "u", "ç", "c", "ñ", "n", "·", "", "%20", " ", "_", " ", "-", " ")
	return r.Replace(s)
}

func imageQueryWords(q string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(imageNorm(q), func(r rune) bool { return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') }) {
		if len(w) >= 3 && !imageStop[w] {
			out = append(out, w)
		}
	}
	return out
}

// imageMatches: la foto és del que s'ha buscat si al títol o a l'adreça hi
// surten dues paraules de la consulta, o la més llarga (la que diu més: a
// «cuttlefish with peas», una foto de pèsols sols no val). Es compara el
// començament de cada paraula, per plurals («pesols»/«pesol»).
func imageMatches(words []string, hay string) bool {
	h := imageNorm(hay)
	longest, hits, longHit := 0, 0, false
	for i, w := range words {
		if len(w) > len(words[longest]) {
			longest = i
		}
	}
	for i, w := range words {
		stem := w
		if len(stem) > 5 {
			stem = stem[:5]
		}
		if strings.Contains(h, stem) {
			hits++
			if i == longest {
				longHit = true
			}
		}
	}
	return hits >= 2 || longHit
}

// Una línia que només és l'adreça d'una foto, o un enllaç a una foto, és una
// foto: el model de vegades l'escriu així en lloc de ![...](...), i llavors es
// veia l'adreça (sobretot a l'Android). També en llistes numerades.
// També «[foto.jpg](…/f/…/foto.jpg) (327 kB)»: el que torna send_file quan
// el vent baixa una foto i te l'envia (es veia com un enllaç a un fitxer).
// Un enllaç a una foto dins d'una frase es queda a la frase, i la foto va a
// sota: si no, només en veies l'enllaç.
var (
	bareImageLineRe = regexp.MustCompile(`(?mi)^([ \t]*(?:(?:[-*]|\d{1,2}[.)])[ \t]+)?)(https?://[^\s<>()]+\.(?:jpe?g|png|webp|gif)(?:\?[^\s<>()]*)?)[ \t]*$`)
	linkImageLineRe = regexp.MustCompile(`(?mi)^([ \t]*(?:(?:[-*]|\d{1,2}[.)])[ \t]+)?)\[([^\]\n]+)\]\((https?://[^\s<>()]+\.(?:jpe?g|png|webp|gif)(?:\?[^\s<>()]*)?)\)(?:[ \t]*\([\d.,]+[ \t]*[kKMG]?B\))?[ \t]*$`)
	inlineImageRe   = regexp.MustCompile(`(?i)(^|[^!])\[([^\]\n]+)\]\((https?://[^\s<>()]+\.(?:jpe?g|png|webp|gif)(?:\?[^\s<>()]*)?)\)`)
	embeddedImageRe = regexp.MustCompile(`!\[([^\]\n]*)\]\((https?://[^\s<>()]+)\)`)
	imageExtRe      = regexp.MustCompile(`(?i)\.(?:jpe?g|png|webp|gif)$`)
	// La pàgina d'un fitxer de Commons o de la Viquipèdia no és la foto: el
	// navegador no la pot pintar i l'Android l'amaga. Special:FilePath hi porta.
	filePageRe = regexp.MustCompile(`(?i)^https?://([a-z]+\.(?:m\.)?(?:wikimedia|wikipedia)\.org)/wiki/(?:File|Fitxer|Archivo|Fichier|Imatge|Image):([^?#]+)$`)
)

// imageAlt: el nom d'un fitxer com a peu, sense l'extensió ni guions.
func imageAlt(alt string) string {
	alt = strings.TrimSpace(alt)
	if imageExtRe.MatchString(alt) {
		alt = strings.NewReplacer("-", " ", "_", " ").Replace(imageExtRe.ReplaceAllString(alt, ""))
	}
	return alt
}

// directImageURL: l'adreça que de debò torna la foto.
func directImageURL(u string) string {
	if m := filePageRe.FindStringSubmatch(u); m != nil {
		return "https://" + m[1] + "/wiki/Special:FilePath/" + m[2] + "?width=960"
	}
	return u
}

func embedImageLinks(reply string) string {
	if !strings.Contains(reply, "http") {
		return reply
	}
	// Dins d'un bloc de codi no es toca res.
	parts := strings.Split(reply, "```")
	for i := 0; i < len(parts); i += 2 {
		p := linkImageLineRe.ReplaceAllStringFunc(parts[i], func(line string) string {
			m := linkImageLineRe.FindStringSubmatch(line)
			return m[1] + "![" + imageAlt(m[2]) + "](" + m[3] + ")"
		})
		p = bareImageLineRe.ReplaceAllString(p, "${1}![](${2})")
		// Les que ja hi són com a foto no es repeteixen.
		shown := map[string]bool{}
		for _, m := range embeddedImageRe.FindAllStringSubmatch(p, -1) {
			shown[m[2]] = true
		}
		lines := strings.Split(p, "\n")
		for k, line := range lines {
			var fotos []string
			for _, m := range inlineImageRe.FindAllStringSubmatch(line, -1) {
				if !shown[m[3]] {
					shown[m[3]] = true
					fotos = append(fotos, "!["+imageAlt(m[2])+"]("+m[3]+")")
				}
			}
			if len(fotos) > 0 {
				lines[k] = line + "\n\n" + strings.Join(fotos, "\n")
			}
		}
		p = strings.Join(lines, "\n")
		parts[i] = embeddedImageRe.ReplaceAllStringFunc(p, func(img string) string {
			m := embeddedImageRe.FindStringSubmatch(img)
			if d := directImageURL(m[2]); d != m[2] {
				return "![" + m[1] + "](" + d + ")"
			}
			return img
		})
	}
	return strings.Join(parts, "```")
}

// compareEnglishNames tradueix els noms d'una comparació al títol de
// l'article de la Viquipèdia en anglès (per trobar-ne la foto). Una crida
// curta al model lateral; els tests la substitueixen.
var compareEnglishNames = func(s *Server, ctx context.Context, names []string) map[string]string {
	out := map[string]string{}
	if s == nil || len(names) == 0 {
		return out
	}
	p, role, ok := s.sideRoleRef()
	if !ok {
		return out
	}
	s.mu.Lock()
	cfg, client := s.cfg, s.client
	s.mu.Unlock()
	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	ans, err := agent.SideCallLateral(cctx, client, cfg, p, role, []llm.Message{
		{Role: "system", Content: "For each line, reply with ONLY the English Wikipedia article title of that thing (2-4 words), one per line, same order. Example: «Terra» → Earth; «Guepard» → Cheetah; «Sagrada Família» → Sagrada Família."},
		{Role: "user", Content: strings.Join(names, "\n")},
	})
	if err != nil {
		return out
	}
	lines := strings.Split(strings.TrimSpace(ans), "\n")
	for i, n := range names {
		if i < len(lines) {
			if en := cleanSubject(strings.TrimLeft(lines[i], "-•*0123456789.) ")); en != "" {
				out[n] = en
			}
		}
	}
	return out
}

// balanceJSON arregla els errors de parèntesis més habituals dels models en
// el JSON d'una targeta: un tancament que no correspon al que hi ha obert es
// treu, una coma just abans d'un tancament també, i el que queda obert es
// tanca al final. Fora de les cadenes (respecta les cometes escapades).
func balanceJSON(raw string) string {
	var b strings.Builder
	var stack []byte
	inStr, esc := false, false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if inStr {
			b.WriteByte(c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			stack = append(stack, c)
		case '}', ']':
			want := byte('{')
			if c == ']' {
				want = '['
			}
			if len(stack) == 0 || stack[len(stack)-1] != want {
				continue // no lliga amb el que hi ha obert: fora
			}
			stack = stack[:len(stack)-1]
			// Coma penjada abans del tancament: fora.
			s := strings.TrimRight(b.String(), " \t\r\n")
			if strings.HasSuffix(s, ",") {
				b.Reset()
				b.WriteString(strings.TrimSuffix(s, ","))
			}
		}
		b.WriteByte(c)
	}
	if inStr {
		b.WriteByte('"')
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == '{' {
			b.WriteByte('}')
		} else {
			b.WriteByte(']')
		}
	}
	return b.String()
}
