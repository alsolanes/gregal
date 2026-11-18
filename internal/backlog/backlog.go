// Package backlog és la llista de feina pendent d'un projecte: tasques amb
// tipus, criteris d'acceptació, estat i un log que només creix (la skill
// work-items explica com s'escriuen). No depèn de cap forja: un projecte
// pot ser un repo de git, un repo sense remot o una carpeta qualsevol.
//
// Les tasques viuen a les dades de l'app (un JSON per projecte a
// <dades>/backlog/) tret que el projecte tingui la carpeta .gregal/backlog/:
// llavors cada tasca és un fitxer Markdown allà, i es versionen amb el
// codi. MouAlRepo passa d'un lloc a l'altre.
//
// Una sessió que treballa en un worktree veu el mateix backlog que el repo
// principal: l'arrel es resol amb git abans de triar on llegir.
package backlog

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Tipus d'una tasca. Només aquests: els tipus no es multipliquen.
const (
	TypeFeature = "feature"
	TypeBug     = "bug"
	TypeTask    = "task"
	TypeSpike   = "spike"
	TypeEpic    = "epic"
)

// Estats.
const (
	StatusProposed   = "proposed"
	StatusReady      = "ready"
	StatusInProgress = "in_progress"
	StatusInReview   = "in_review"
	StatusDone       = "done"
	StatusBlocked    = "blocked"
	StatusDropped    = "dropped"
)

// Types i Statuses en l'ordre en què es mostren.
var (
	Types      = []string{TypeFeature, TypeBug, TypeTask, TypeSpike, TypeEpic}
	Statuses   = []string{StatusProposed, StatusReady, StatusInProgress, StatusInReview, StatusDone, StatusBlocked, StatusDropped}
	Priorities = []string{"now", "next", "later"}
	Sizes      = []string{"S", "M", "L"}
)

// Criterion és un criteri d'acceptació. Només compta com a complert amb
// una prova (la sortida d'un test, una comanda, una captura).
type Criterion struct {
	Text     string `json:"text"`
	Done     bool   `json:"done,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

// LogEntry és una línia del log. El log no es reescriu mai.
type LogEntry struct {
	At   time.Time `json:"at"`
	Who  string    `json:"who,omitempty"`
	Text string    `json:"text"`
}

// Item és una tasca del backlog.
type Item struct {
	ID         int         `json:"id"`
	Type       string      `json:"type"`
	Title      string      `json:"title"`
	Status     string      `json:"status"`
	Priority   string      `json:"priority,omitempty"`
	Size       string      `json:"size,omitempty"`
	Parent     int         `json:"parent,omitempty"`
	DependsOn  []int       `json:"depends_on,omitempty"`
	Why        string      `json:"why,omitempty"`
	Criteria   []Criterion `json:"criteria,omitempty"`
	OutOfScope string      `json:"out_of_scope,omitempty"`
	// Reason és per què està blocked o dropped.
	Reason string `json:"reason,omitempty"`
	// Workdir és on s'hi ha treballat (el worktree, o l'arrel) i Base el
	// commit d'on va partir la feina: el revisor hi mira el diff.
	Workdir string `json:"workdir,omitempty"`
	Base    string `json:"base,omitempty"`
	// Review és l'última revisió independent (vegeu agent/backlog_review.go).
	Review  *Review    `json:"review,omitempty"`
	Log     []LogEntry `json:"log,omitempty"`
	Created time.Time  `json:"created"`
	Updated time.Time  `json:"updated"`
}

// Estats d'una revisió.
const (
	ReviewRunning = "running"
	ReviewMet     = "met"
	ReviewNotMet  = "not_met"
	ReviewError   = "error"
)

// reviewStale és quan una revisió «running» es dona per perduda (el procés
// que la feia s'ha tancat): ja no bloqueja res i es pot tornar a demanar.
const reviewStale = 30 * time.Minute

// Review és el veredicte del revisor: un model que no ha escrit el codi,
// amb el diff, les comprovacions del projecte i els criteris davant.
type Review struct {
	Status   string            `json:"status" yaml:"status"`
	At       time.Time         `json:"at" yaml:"at"`
	Summary  string            `json:"summary,omitempty" yaml:"summary,omitempty"`
	Criteria []ReviewCriterion `json:"criteria,omitempty" yaml:"criteria,omitempty"`
	Checks   []ReviewCheck     `json:"checks,omitempty" yaml:"checks,omitempty"`
}

// ReviewCriterion és el que el revisor diu d'un criteri (MET, NOT MET o
// UNVERIFIED) i amb quina prova.
type ReviewCriterion struct {
	Text     string `json:"text" yaml:"text"`
	Status   string `json:"status" yaml:"status"`
	Evidence string `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

// ReviewCheck és una comprovació del projecte que Gregal ha executat.
type ReviewCheck struct {
	Command string `json:"command" yaml:"command"`
	Passed  bool   `json:"passed" yaml:"passed"`
}

// Running diu si la revisió encara corre (i no s'ha quedat penjada).
func (r *Review) Running() bool {
	return r != nil && r.Status == ReviewRunning && time.Since(r.At) < reviewStale
}

// OnInReview, si hi és, s'avisa (en una goroutine) cada cop que una tasca
// passa a in_review: el front-end que té model hi engega el revisor.
var OnInReview func(root string, it Item)

// Storage diu on viu el backlog d'un projecte.
const (
	StorageApp  = "app"
	StorageRepo = "repo"
)

// Root és l'arrel del projecte: el repo principal si dir és dins d'un repo
// de git (també des d'un worktree), i si no, dir mateix.
func Root(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	cmd := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return filepath.Clean(dir)
	}
	common := filepath.FromSlash(strings.TrimSpace(string(out)))
	if filepath.Base(common) == ".git" {
		return filepath.Dir(common)
	}
	return filepath.Clean(dir)
}

// Head és el commit on és dir ("" si no és un repo amb commits). És el
// punt de partida de la feina: el revisor en mira el diff.
func Head(dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Errors que la UI i l'eina expliquen tal qual.
var (
	ErrNotFound = errors.New("there is no item with that number")
	ErrNoDir    = errors.New("a backlog needs a project folder")
)

func valid(v string, allowed []string) bool {
	for _, a := range allowed {
		if a == v {
			return true
		}
	}
	return false
}

// normalitza deixa els camps lliures com es desen i comprova els tancats.
func normalitza(it *Item) error {
	it.Title = strings.TrimSpace(it.Title)
	it.Type = strings.ToLower(strings.TrimSpace(it.Type))
	it.Priority = strings.ToLower(strings.TrimSpace(it.Priority))
	it.Size = strings.ToUpper(strings.TrimSpace(it.Size))
	it.Status = normalStatus(it.Status)
	if it.Title == "" {
		return errors.New("the title is missing")
	}
	if !valid(it.Type, Types) {
		return fmt.Errorf("unknown type %q: %s", it.Type, strings.Join(Types, ", "))
	}
	if it.Status == "" {
		it.Status = StatusProposed
	}
	if !valid(it.Status, Statuses) {
		return fmt.Errorf("unknown status %q: %s", it.Status, strings.Join(Statuses, ", "))
	}
	if it.Priority == "" {
		it.Priority = "next"
	}
	if !valid(it.Priority, Priorities) {
		return fmt.Errorf("unknown priority %q: now, next or later", it.Priority)
	}
	if it.Size != "" && !valid(it.Size, Sizes) {
		return fmt.Errorf("unknown size %q: S, M or L", it.Size)
	}
	net := it.Criteria[:0]
	for _, c := range it.Criteria {
		c.Text = strings.TrimSpace(c.Text)
		c.Evidence = strings.TrimSpace(c.Evidence)
		if c.Text != "" {
			net = append(net, c)
		}
	}
	it.Criteria = net
	return nil
}

// normalStatus accepta «in progress», «in-progress» i similars.
func normalStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer(" ", "_", "-", "_").Replace(s)
	return s
}

// CheckMove diu si una tasca pot passar a l'estat donat, amb el motiu en
// cas que no. Són les regles de la skill work-items, aplicades aquí perquè
// valguin igual des de la UI, l'eina i el mòbil.
func CheckMove(it Item, to string, reason string, all []Item) error {
	to = normalStatus(to)
	if !valid(to, Statuses) {
		return fmt.Errorf("unknown status %q: %s", to, strings.Join(Statuses, ", "))
	}
	switch to {
	case StatusReady:
		return checkReady(it)
	case StatusInProgress:
		return checkStart(it, all)
	case StatusDone:
		return checkDone(it, all)
	case StatusBlocked, StatusDropped:
		if strings.TrimSpace(reason) == "" {
			return fmt.Errorf("moving to %s needs a reason", to)
		}
	}
	return nil
}

// checkReady: amb el perquè, criteris (una investigació en pot no tenir-ne)
// i mida S o M. Una èpica no es treballa directament.
func checkReady(it Item) error {
	switch {
	case it.Type == TypeEpic:
		return errors.New("an epic is not worked on directly: split it into items and make those ready")
	case strings.TrimSpace(it.Why) == "":
		return errors.New("to be ready it needs the why")
	case len(it.Criteria) == 0 && it.Type != TypeSpike:
		return errors.New("to be ready it needs acceptance criteria")
	case it.Size == "L":
		return errors.New("a size L item must be split before it is ready")
	}
	return nil
}

// checkStart: només una tasca a punt (o que ja s'havia començat) i sense
// dependències obertes.
func checkStart(it Item, all []Item) error {
	if !valid(it.Status, []string{StatusReady, StatusBlocked, StatusInReview, StatusInProgress}) {
		return fmt.Errorf("only a ready item can be started (this one is %s)", it.Status)
	}
	if oberta := dependenciesObertes(it, all); len(oberta) > 0 {
		return fmt.Errorf("it depends on open items: %s", oberta)
	}
	return nil
}

// checkDone: una tasca es tanca des de revisió, sense revisió en marxa i
// amb prova a cada criteri; una èpica, quan ho són les seves tasques.
func checkDone(it Item, all []Item) error {
	if it.Type == TypeEpic {
		return epicOpen(it, all)
	}
	if it.Status != StatusInReview {
		return fmt.Errorf("only an item in review can be done (this one is %s)", it.Status)
	}
	if it.Review.Running() {
		return errors.New("the review is still running: wait for its verdict")
	}
	for i, c := range it.Criteria {
		if !c.Done || c.Evidence == "" {
			return fmt.Errorf("criterion %d has no evidence: %s", i+1, c.Text)
		}
	}
	return nil
}

func epicOpen(it Item, all []Item) error {
	for _, f := range all {
		if f.Parent == it.ID && f.Status != StatusDone && f.Status != StatusDropped {
			return fmt.Errorf("item #%d of the epic is still open", f.ID)
		}
	}
	return nil
}

func dependenciesObertes(it Item, all []Item) string {
	var obertes []string
	for _, d := range it.DependsOn {
		for _, f := range all {
			if f.ID == d && f.Status != StatusDone && f.Status != StatusDropped {
				obertes = append(obertes, fmt.Sprintf("#%d", d))
			}
		}
	}
	return strings.Join(obertes, ", ")
}

// Task és l'encàrrec que rep l'agent quan es posa a treballar en una tasca.
func Task(it Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Work on backlog item #%d (%s): %s\n\n", it.ID, it.Type, it.Title)
	if it.Why != "" {
		fmt.Fprintf(&b, "Why:\n%s\n\n", it.Why)
	}
	if len(it.Criteria) > 0 {
		b.WriteString("Acceptance criteria:\n")
		for i, c := range it.Criteria {
			marca := " "
			if c.Done {
				marca = "x"
			}
			fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, marca, c.Text)
		}
		b.WriteString("\n")
	}
	if it.OutOfScope != "" {
		fmt.Fprintf(&b, "Out of scope:\n%s\n\n", it.OutOfScope)
	}
	if n := len(it.Log); n > 0 {
		b.WriteString("Recent log:\n")
		ini := n - 5
		if ini < 0 {
			ini = 0
		}
		for _, l := range it.Log[ini:] {
			fmt.Fprintf(&b, "- %s %s\n", l.At.Local().Format("2006-01-02"), l.Text)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, `How to work on it:
- Work only on these criteria. Anything else you find becomes a new item (backlog create), not extra scope.
- If a criterion is ambiguous or wrong, stop: backlog move %[1]d to blocked with the question as reason.
- As each criterion is met, backlog check it with its evidence (test name, command output, file).
- At the end run the project's checks, log what changed (backlog log), and move the item to in_review. Do not mark it done: the reviewer does.
`, it.ID)
	return b.String()
}

// Resum és una línia per tasca per a llistes curtes (eina, notificacions).
func Resum(it Item) string {
	fets := 0
	for _, c := range it.Criteria {
		if c.Done {
			fets++
		}
	}
	s := fmt.Sprintf("#%d [%s] %s · %s · %s", it.ID, it.Type, it.Title, it.Status, it.Priority)
	if it.Size != "" {
		s += " · " + it.Size
	}
	if len(it.Criteria) > 0 {
		s += fmt.Sprintf(" · %d/%d criteria", fets, len(it.Criteria))
	}
	if it.Reason != "" && (it.Status == StatusBlocked || it.Status == StatusDropped) {
		s += " · " + it.Reason
	}
	return s
}
