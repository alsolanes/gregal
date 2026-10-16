package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Revisió en paral·lel del #retro. Quan es passa una feina a un altre agent,
// gairebé sempre hi troba algun error: aquí cada especialista és aquest
// segon parell d'ulls. En obrir el canal, mentre els agents que han fet la
// feina expliquen què han fet, cada especialista fa la seva crida al model
// (en paral·lel) i publica com a molt tres troballes amb gravetat, rebuts i
// un arranjament que es pot engegar amb un clic. Demanat el 2026-10-15:
// «un equip al darrere que ens ajudi», també quan el torn no tenia agents
// específics.

// retroReviewBudget és el temps màxim de les revisions; una que no arriba
// a temps simplement no publica res.
var retroReviewBudget = 90 * time.Second

// Què mira cada especialista quan revisa.
var retroReviewFocus = map[string]string{
	"design":   "visual quality, layout and hierarchy, consistency with the existing style, empty/loading/error states, responsiveness and accessibility (contrast, focus, labels). Only applies if the work touched something a person sees (UI, styles, screens, text output)",
	"code":     "correctness, structure, naming, duplication, error handling, concurrency, performance and consistency with the rest of the codebase. Only applies if code was written or changed",
	"qa":       "bugs and regressions: edge cases and inputs that would break it, missing or weak tests, and whether the checks that ran actually prove it works. Only applies if something was built or changed",
	"product":  "whether it does what was asked, what is missing or half done, the flow for the person using it and the messages they see",
	"security": "secrets, permissions, injection, unsafe commands, data exposure and risky dependencies. Only applies if the work touched something with a security angle",
}

const (
	retroKindFinding = "finding"
	retroKindVerdict = "verdict"
)

var retroSeverities = map[string]bool{"high": true, "medium": true, "low": true}

type retroReviewDraft struct {
	Applies  *bool  `json:"applies"`
	Verdict  string `json:"verdict"`
	Findings []struct {
		Severity string   `json:"severity"`
		Title    string   `json:"title"`
		Text     string   `json:"text"`
		Receipts []string `json:"receipts"`
		Fix      string   `json:"fix"`
	} `json:"findings"`
}

// retroReviewers són els especialistes que revisen aquest canal, en ordre.
// Qui no hi té res a dir ni es crida: vist el 2026-10-15, amb només el
// prompt, Disseny revisava canvis de backend i Codi una resposta sense
// fitxers.
func retroReviewers(c *retroChannel) []retroAgent {
	var out []retroAgent
	for _, a := range c.Agents {
		if retroIsSpecialist(&a) && retroReviewFocus[a.ID] != "" && retroReviewApplies(c, a.ID) {
			out = append(out, a)
		}
	}
	return out
}

// Fitxers que una persona veu (interfície, estils, pantalles).
// .js/.ts només dins de carpetes de front (internal/web/*.go és backend).
var retroUIFile = regexp.MustCompile(`(?i)(\.(css|scss|sass|less|html?|jsx|tsx|vue|svelte|xaml|storyboard|xib|kt|swift|dart)\b|(^|/)(app|ui|web|frontend|client|components?|pages?)/\S*\.(js|mjs|ts)\b|(^|/)(tui|views?|screens?|layouts?|styles?|res/layout)/)`)

// retroReviewApplies diu si l'angle d'un especialista té on mirar.
func retroReviewApplies(c *retroChannel, id string) bool {
	files, ran := false, false
	ui := retroTopics["design"].MatchString(c.Title)
	for _, e := range c.Evidence {
		switch e.Kind {
		case "file":
			files = true
			if retroUIFile.MatchString(e.Title) {
				ui = true
			}
		case "command", "test", "verify", "checkpoint":
			ran = true
		case "delegate": // «task: …» d'un subagent
			ui = ui || retroTopics["design"].MatchString(e.Title)
		case "deliverable":
			files = true // una feina d'equip: el lliurament és la feina
		}
	}
	switch id {
	case "design":
		return ui
	case "code":
		return files
	case "qa":
		return files || ran
	}
	return true
}

// retroWorkers són els agents que han fet la feina (sense especialistes).
func retroWorkers(c *retroChannel) []retroAgent {
	var out []retroAgent
	for _, a := range c.Agents {
		if !retroIsSpecialist(&a) {
			out = append(out, a)
		}
	}
	return out
}

func retroReviewPrompt(c *retroChannel, a retroAgent) (string, string) {
	sys := "You are " + a.Name + " (id " + a.ID + "), a reviewer on the team behind an AI coding agent. You did NOT do this work: it was handed to you as a fresh pair of eyes. " +
		"Your angle: " + retroReviewFocus[a.ID] + ".\n" +
		"A second reviewer almost always finds something: look hard for bugs, weak spots and concrete improvements from your angle. " +
		"Every finding must be grounded in the EVIDENCE: cite evidence ids in receipts and name the file, line or behaviour. Never invent code you cannot see; when the evidence is not enough to be sure, say exactly what to check and use severity low.\n" +
		retroRules + "\n" + retroPersonaDirective(c.Persona) +
		"Review the WORK itself (the code, files, commands and the answer to the task), never the evidence format, the logs or the retro. Only findings worth acting on: no finding is better than a filler one. " +
		"If nothing in this work touches your angle (for example a plain answer with no files changed), set \"applies\": false and leave findings empty.\n" +
		"LANES: stay in yours and do not repeat what belongs to another reviewer. design = what people see; code = how the code is written; qa = whether it was tested and what would break it (test runs and coverage belong here); product = whether it does what was asked, and scope; security = secrets and risks.\n" +
		"READING DIFFS: lines starting with + were added and - removed; other lines are unchanged context. The diff is against the last commit, so it can include earlier uncommitted work: do not say something was added in this run unless the + lines and the agents' actions show it.\n" +
		"Severity: high = a bug, data loss, a security hole or it does not do what was asked; medium = a likely problem or a clear gap (missing test, unhandled edge case, inconsistency); low = polish or an improvement.\n" +
		"At most 3 findings, most important first. Each one: \"title\" (at most 8 words), \"text\" (1-3 sentences: what, where, why it matters), \"receipts\", \"fix\" (one imperative instruction another agent could carry out, or empty).\n" +
		"\"verdict\": one sentence with your overall view from your angle (what is solid, where it is weak).\n" +
		`OUTPUT: only one JSON object, no code fences: {"applies":true,"verdict":"...","findings":[{"severity":"medium","title":"...","text":"...","receipts":["e3"],"fix":"..."}]}` + "\n" +
		config.LanguageDirective(c.Lang)
	user := "TASK: " + c.Title + "\n\n" + renderRetroEvidenceN(retroPack{Agents: retroWorkers(c), Evidence: c.Evidence}, 1400)
	return sys, user
}

// retroValidateReview converteix la revisió en missatges: el veredicte i
// les troballes. Cap missatge (i cap error) si l'angle no hi aplica.
func retroValidateReview(js string, c *retroChannel, author string) ([]retroMessage, error) {
	var d retroReviewDraft
	if js == "" {
		return nil, errors.New("no JSON object found")
	}
	if err := json.Unmarshal([]byte(js), &d); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}
	if d.Applies != nil && !*d.Applies {
		return nil, nil
	}
	known := c.evidenceIDs()
	var out []retroMessage
	if v := strings.TrimSpace(d.Verdict); v != "" {
		out = append(out, retroMessage{Author: author, Kind: retroKindVerdict, Text: retroClip(v, 300)})
	}
	n := 0
	for _, f := range d.Findings {
		text := strings.TrimSpace(f.Text)
		title := strings.TrimSpace(f.Title)
		if text == "" && title == "" {
			continue
		}
		sev := strings.ToLower(strings.TrimSpace(f.Severity))
		if !retroSeverities[sev] {
			sev = "low"
		}
		out = append(out, retroMessage{
			Author: author, Kind: retroKindFinding, Severity: sev,
			Title: retroClip(title, 80), Text: retroClip(text, 600),
			Receipts:      retroCleanReceipts(f.Receipts, known),
			FixSuggestion: retroClip(strings.TrimSpace(f.Fix), 400),
		})
		if n++; n == 3 {
			break
		}
	}
	return out, nil
}

// retroRunReviews fa les revisions en paral·lel. Cadascuna publica en
// acabar, però mai abans que el resum d'obertura (before): així el canal
// sempre comença per què s'ha fet. Torna quan han acabat totes.
func (s *Server) retroRunReviews(ctx context.Context, c *retroChannel, before <-chan struct{}, publish func([]retroMessage)) {
	var wg sync.WaitGroup
	for _, a := range retroReviewers(c) {
		wg.Add(1)
		go func(a retroAgent) {
			defer wg.Done()
			defer s.retroTyping(c.Key, []string{a.ID}, false)
			sys, user := retroReviewPrompt(c, a)
			var msgs []retroMessage
			_, _, err := s.retroAsk(ctx, sys, user, func(js string) error {
				var verr error
				msgs, verr = retroValidateReview(js, c, a.ID)
				return verr
			})
			if err != nil {
				log.Printf("retro review %s skipped: %s", a.ID, redactError(err))
				return
			}
			select {
			case <-before:
			case <-ctx.Done():
				return
			}
			if len(msgs) > 0 {
				publish(msgs)
			}
		}(a)
	}
	wg.Wait()
}

// retroReviewContext és el context de les revisions: sense raonament (com
// la resta del retro) i amb el seu temps màxim.
func retroReviewContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), retroReviewBudget)
	return llm.WithThink(ctx, "no"), cancel
}

// retroFindingCounts compta les troballes per gravetat (per al webhook i
// els tests).
func retroFindingCounts(c *retroChannel) map[string]int {
	out := map[string]int{}
	for _, m := range c.Messages {
		if m.Kind == retroKindFinding {
			out[m.Severity]++
		}
	}
	return out
}
