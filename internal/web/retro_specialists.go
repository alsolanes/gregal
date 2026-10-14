package web

import (
	"regexp"
	"strings"
)

// Especialistes del #retro. En un prompt normal només hi ha el coordinador
// (i l'investigador si ha corregut el scout), i el retro quedava amb una sola
// veu. Aquests tres no han fet la feina: la revisen des del seu angle
// (funcionalitat, codi, seguretat) amb la mateixa evidència i les mateixes
// regles de rebuts. Demanat el 2026-10-14: «que els agents siguin
// especialistes en diferents temes i puguin parlar segons expertise».

const retroSpecialistRole = "specialist"

var retroSpecialistOrder = []string{"product", "code", "security"}

func retroSpecialists(lang string) []retroAgent {
	ca := lang == "ca"
	name := func(en, cat string) string {
		if ca {
			return cat
		}
		return en
	}
	return []retroAgent{
		{ID: "product", Name: name("Product", "Funcionalitat"), Avatar: "product",
			Role: retroSpecialistRole + ": functionality and user experience. Did not do the work; checks it does what was asked, edge cases and how it feels to use"},
		{ID: "code", Name: name("Code", "Codi"), Avatar: "code",
			Role: retroSpecialistRole + ": code quality. Did not do the work; checks design, readability, tests and maintainability"},
		{ID: "security", Name: name("Security", "Seguretat"), Avatar: "security",
			Role: retroSpecialistRole + ": security. Did not do the work; checks secrets, permissions, injection, unsafe commands and data exposure"},
	}
}

// retroWithSpecialists afegeix els especialistes que encara no hi siguin
// (també als canals creats abans que existissin).
func retroWithSpecialists(agents []retroAgent, lang string) []retroAgent {
	out := append([]retroAgent(nil), agents...)
	for _, sp := range retroSpecialists(lang) {
		found := false
		for _, a := range out {
			if a.ID == sp.ID {
				found = true
				break
			}
		}
		if !found {
			out = append(out, sp)
		}
	}
	return out
}

func retroIsSpecialist(a *retroAgent) bool {
	return a != nil && strings.HasPrefix(a.Role, retroSpecialistRole)
}

// Temes de cada especialista (català i anglès). Una pregunta sense @ va al
// coordinador i, a més, als especialistes del tema (com a molt dos).
var retroTopics = map[string]*regexp.Regexp{
	"security": regexp.MustCompile(`(?i)(segur|secur|vulnerab|secret|token|password|contrasenya|clau d|api key|injec|xss|csrf|sql|permís|permis|permission|auth|xifr|encrypt|privacitat|privacy|filtra|leak|perill|danger)`),
	"code":     regexp.MustCompile(`(?i)(\bcodi\b|\bcode\b|refactor|\btests?\b|\bbugs?\b|qualitat|quality|rendiment|performance|disseny|design|arquitect|architect|llegib|readab|mantenib|maintain|\blint|duplica|complex)`),
	"product":  regexp.MustCompile(`(?i)(funcional|feature|usuari|\buser|\bux\b|\bui\b|requisit|requirement|cas(os)? límit|edge case|el que (he|vaig|t'he) demanat|what i asked|comportament|behaviou?r|experiència|experience|fa el que|does what|útil|useful)`),
}

var retroAllRe = regexp.MustCompile(`(?i)(?:^|[^\w@])@(all|tots|totes|team|equip|everyone)\b`)

// retroRespondersFor tria qui contesta un missatge: les mencions manen;
// @tots fa parlar tothom (fins a 5); si no, el coordinador més els
// especialistes del tema.
func retroRespondersFor(text string, c *retroChannel) []string {
	if retroAllRe.MatchString(text) {
		var out []string
		for _, a := range c.Agents {
			out = append(out, a.ID)
			if len(out) == 5 {
				break
			}
		}
		return out
	}
	if m := parseRetroMentions(text, c.Agents); len(m) > 0 {
		return m
	}
	var out []string
	if c.agent("coordinator") != nil {
		out = append(out, "coordinator")
	} else if len(c.Agents) > 0 {
		out = append(out, c.Agents[0].ID)
	}
	n := 0
	for _, id := range retroSpecialistOrder {
		if c.agent(id) != nil && retroTopics[id].MatchString(text) {
			out = append(out, id)
			if n++; n == 2 {
				break
			}
		}
	}
	return out
}

// Instruccions per als especialistes, comunes al stand-up i a les respostes.
const retroSpecialistRules = `SPECIALISTS: agents whose role starts with "specialist" did NOT do the work. They review it only from their angle (functionality, code quality or security) using the same EVIDENCE and receipts. They never invent problems the evidence does not show; when the evidence is not enough to judge, they say exactly what would need checking.`
