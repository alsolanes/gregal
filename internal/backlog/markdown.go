package backlog

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// El format d'una tasca al repositori: capçalera YAML amb els camps curts i
// seccions Markdown per al text, perquè es pugui llegir i editar a mà i
// els diffs siguin clars. És el mateix que ensenya la skill work-items.

type capcalera struct {
	ID        int       `yaml:"id"`
	Type      string    `yaml:"type"`
	Title     string    `yaml:"title"`
	Status    string    `yaml:"status"`
	Priority  string    `yaml:"priority,omitempty"`
	Size      string    `yaml:"size,omitempty"`
	Parent    int       `yaml:"parent,omitempty"`
	DependsOn []int     `yaml:"depends_on,omitempty,flow"`
	Reason    string    `yaml:"reason,omitempty"`
	Workdir   string    `yaml:"workdir,omitempty"`
	Base      string    `yaml:"base,omitempty"`
	Review    *Review   `yaml:"review,omitempty"`
	Created   time.Time `yaml:"created"`
	Updated   time.Time `yaml:"updated"`
}

const formatLog = "2006-01-02 15:04"

// Markdown escriu una tasca en el format del repositori.
func Markdown(it Item) string {
	var b strings.Builder
	cap, _ := yaml.Marshal(capcalera{
		ID: it.ID, Type: it.Type, Title: it.Title, Status: it.Status,
		Priority: it.Priority, Size: it.Size, Parent: it.Parent, DependsOn: it.DependsOn,
		Reason: it.Reason, Workdir: it.Workdir, Base: it.Base, Review: it.Review,
		Created: it.Created.UTC(), Updated: it.Updated.UTC(),
	})
	b.WriteString("---\n")
	b.Write(cap)
	b.WriteString("---\n")
	seccio := func(nom, text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		fmt.Fprintf(&b, "\n## %s\n%s\n", nom, strings.TrimSpace(text))
	}
	seccio("Why", it.Why)
	if len(it.Criteria) > 0 {
		b.WriteString("\n## Acceptance criteria\n")
		for _, c := range it.Criteria {
			marca := " "
			if c.Done {
				marca = "x"
			}
			fmt.Fprintf(&b, "- [%s] %s\n", marca, unaLinia(c.Text))
			if c.Evidence != "" {
				fmt.Fprintf(&b, "  - evidence: %s\n", unaLinia(c.Evidence))
			}
		}
	}
	seccio("Out of scope", it.OutOfScope)
	if len(it.Log) > 0 {
		b.WriteString("\n## Log\n")
		for _, l := range it.Log {
			fmt.Fprintf(&b, "- %s", l.At.UTC().Format(formatLog))
			if l.Who != "" {
				fmt.Fprintf(&b, " · %s:", unaLinia(l.Who))
			}
			fmt.Fprintf(&b, " %s\n", unaLinia(l.Text))
		}
	}
	return b.String()
}

func unaLinia(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// ParseMarkdown llegeix una tasca escrita amb Markdown (a mà o per Gregal).
func ParseMarkdown(raw string) (Item, error) {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return Item{}, errors.New("missing the --- header")
	}
	fi := strings.Index(text[4:], "\n---")
	if fi < 0 {
		return Item{}, errors.New("the header is not closed with ---")
	}
	var c capcalera
	if err := yaml.Unmarshal([]byte(text[4:4+fi+1]), &c); err != nil {
		return Item{}, err
	}
	it := Item{
		ID: c.ID, Type: c.Type, Title: c.Title, Status: c.Status, Priority: c.Priority,
		Size: c.Size, Parent: c.Parent, DependsOn: c.DependsOn, Reason: c.Reason,
		Workdir: c.Workdir, Base: c.Base, Review: c.Review,
		Created: c.Created, Updated: c.Updated,
	}
	if it.ID <= 0 {
		return Item{}, errors.New("the header has no id")
	}
	seccions := seccionsDe(text[4+fi+len("\n---"):])
	junt := func(nom string) string { return strings.TrimSpace(strings.Join(seccions[nom], "\n")) }
	it.Why = junt("why")
	it.OutOfScope = junt("out of scope")
	it.Criteria = criterisDe(seccions["acceptance criteria"])
	for _, linia := range seccions["log"] {
		if e, ok := liniaDeLog(linia); ok {
			it.Log = append(it.Log, e)
		}
	}
	if err := normalitza(&it); err != nil {
		return Item{}, err
	}
	return it, nil
}

// seccionsDe parteix el cos per les capçaleres «## Nom» (en minúscules).
func seccionsDe(cos string) map[string][]string {
	seccions := map[string][]string{}
	actual := ""
	for _, linia := range strings.Split(cos, "\n") {
		if strings.HasPrefix(linia, "## ") {
			actual = strings.ToLower(strings.TrimSpace(linia[3:]))
			continue
		}
		if actual != "" {
			seccions[actual] = append(seccions[actual], linia)
		}
	}
	return seccions
}

// criterisDe llegeix «- [ ] text» / «- [x] text», i la prova a la línia de
// sota («  - evidence: …»).
func criterisDe(linies []string) []Criterion {
	var out []Criterion
	for _, linia := range linies {
		t := strings.TrimSpace(linia)
		switch {
		case strings.HasPrefix(t, "- [ ]"):
			out = append(out, Criterion{Text: strings.TrimSpace(t[5:])})
		case strings.HasPrefix(t, "- [x]"), strings.HasPrefix(t, "- [X]"):
			out = append(out, Criterion{Text: strings.TrimSpace(t[5:]), Done: true})
		case strings.HasPrefix(t, "- evidence:") && len(out) > 0:
			out[len(out)-1].Evidence = strings.TrimSpace(t[len("- evidence:"):])
		}
	}
	return out
}

// liniaDeLog llegeix «- 2026-11-18 10:04 · qui: text» (la data i qui són
// opcionals: una línia escrita a mà també val).
func liniaDeLog(linia string) (LogEntry, bool) {
	t := strings.TrimSpace(linia)
	if !strings.HasPrefix(t, "- ") {
		return LogEntry{}, false
	}
	t = t[2:]
	e := LogEntry{}
	if len(t) >= len(formatLog) {
		if at, err := time.Parse(formatLog, t[:len(formatLog)]); err == nil {
			e.At = at
			t = strings.TrimSpace(t[len(formatLog):])
		}
	}
	if strings.HasPrefix(t, "· ") {
		if who, rest, ok := strings.Cut(t[len("· "):], ":"); ok {
			e.Who, t = strings.TrimSpace(who), strings.TrimSpace(rest)
		}
	}
	e.Text = t
	return e, e.Text != ""
}
