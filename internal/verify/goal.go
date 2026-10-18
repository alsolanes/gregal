package verify

import (
	"strings"

	"gregal/internal/config"
)

// El veredicte d'un objectiu (mode goal). Diferent del d'una revisió de
// codi (BuildPrompt): aquí la pregunta no és «està bé el que s'ha fet?»
// sinó «es compleixen els criteris, i què ho demostra?». Un criteri només
// compta com a complert amb una prova concreta (un fitxer, la sortida d'una
// comprovació); allò que només l'agent afirma queda «UNVERIFIED».

// Estats d'un criteri.
const (
	CriterionMet        = "MET"
	CriterionNotMet     = "NOT MET"
	CriterionUnverified = "UNVERIFIED"
)

// GoalCriterion és un criteri amb el seu estat i la prova que el verificador hi dóna.
type GoalCriterion struct {
	Text     string `json:"text"`
	Status   string `json:"status"`
	Evidence string `json:"evidence,omitempty"`
}

// GoalVerdict és el resultat de verificar un objectiu.
type GoalVerdict struct {
	// Met només és cert si el verificador diu MET, no hi ha cap criteri
	// pendent i cada criteri complert porta una prova.
	Met      bool            `json:"met"`
	Criteria []GoalCriterion `json:"criteria,omitempty"`
	// Missing és el que falta, perquè la ronda següent ho tingui clar.
	Missing string `json:"missing,omitempty"`
	// Reason explica per què no és Met quan el verificador deia MET però no ho demostrava.
	Reason string `json:"reason,omitempty"`
}

// BuildGoalPrompt és l'encàrrec del verificador. evidence és el paquet de
// proves que ha reunit Gregal (comprovacions, diff, fitxers). El prompt és
// en anglès; els motius surten en la llengua de la sessió (lang).
func BuildGoalPrompt(goalBody, evidence, lang string) string {
	var b strings.Builder
	b.WriteString(`You are the independent verifier of a goal. Another agent claims to have done the work; you decide whether the goal is MET, from evidence only.

RULES
- Judge every criterion of the goal separately (if it lists none, derive 2-5 from the task).
- A criterion is MET only with concrete evidence: a check that passed (command and its output), a file you read and what it contains, or a line of the diff. The agent's own statements are claims, not evidence.
- You may use your tools (read, grep, glob, lsp, read-only shell commands) to look at the files; never modify anything. Prefer looking at the real files over trusting a summary.
- If you could not check a criterion, it is UNVERIFIED, and that means the goal is not met.
- Be strict: partial work, failing or missing checks, or a criterion nobody demonstrated means NOT MET.

OUTPUT FORMAT (exactly; the first line decides):
VERDICT: MET
or
VERDICT: NOT MET
CRITERIA:
- [MET] <criterion> :: <evidence: file path, check command and result>
- [NOT MET] <criterion> :: <what is wrong>
- [UNVERIFIED] <criterion> :: <what you could not check>
MISSING: <what still has to be done, in 1-3 sentences; "none" when MET>

`)
	b.WriteString(config.LanguageDirective(lang) + " Keep VERDICT, CRITERIA, MISSING and the [..] tags exactly as written above.\n\nGOAL:\n")
	b.WriteString(goalBody)
	b.WriteString("\n\nEVIDENCE COLLECTED BY GREGAL AFTER THE LAST ROUND:\n")
	b.WriteString(evidence)
	return b.String()
}

// ParseGoalVerdict llegeix la resposta del verificador. És estricte: sense
// una línia VERDICT clara, sense criteris, amb algun criteri que no sigui
// MET, o amb un MET sense prova, l'objectiu no es dóna per complert.
func ParseGoalVerdict(text string) GoalVerdict {
	var v GoalVerdict
	said := ""
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "*#> "))
		up := strings.ToUpper(t)
		switch {
		case strings.HasPrefix(up, "VERDICT:"):
			if said == "" {
				said = strings.TrimSpace(up[len("VERDICT:"):])
			}
		case strings.HasPrefix(up, "MISSING:"):
			v.Missing = strings.TrimSpace(t[len("MISSING:"):])
		case strings.HasPrefix(t, "- ["), strings.HasPrefix(t, "* ["):
			if c, ok := parseCriterion(t[2:]); ok {
				v.Criteria = append(v.Criteria, c)
			}
		}
	}
	said = strings.Trim(said, "*_ .")
	switch {
	case said != "MET":
		if said == "" {
			v.Reason = "the verifier gave no explicit verdict"
		}
		return v
	case len(v.Criteria) == 0:
		v.Reason = "the verifier said MET but listed no criteria with evidence"
		return v
	}
	for _, c := range v.Criteria {
		if c.Status != CriterionMet {
			v.Reason = "the verifier said MET but a criterion is " + c.Status + ": " + c.Text
			return v
		}
		if strings.TrimSpace(c.Evidence) == "" {
			v.Reason = "the verifier said MET but gave no evidence for: " + c.Text
			return v
		}
	}
	v.Met = true
	return v
}

func parseCriterion(s string) (GoalCriterion, bool) {
	end := strings.Index(s, "]")
	if !strings.HasPrefix(s, "[") || end < 0 {
		return GoalCriterion{}, false
	}
	status := strings.ToUpper(strings.TrimSpace(s[1:end]))
	switch status {
	case CriterionMet, CriterionNotMet, CriterionUnverified:
	default:
		return GoalCriterion{}, false
	}
	rest := strings.TrimSpace(s[end+1:])
	text, ev, _ := strings.Cut(rest, "::")
	return GoalCriterion{Text: strings.TrimSpace(text), Status: status, Evidence: strings.TrimSpace(ev)}, true
}
