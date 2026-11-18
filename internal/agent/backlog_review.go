package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gregal/internal/backlog"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/verify"
)

// La revisió d'una tasca del backlog. Quan una tasca passa a in_review, un
// model que no ha escrit el codi (el rol revisor) en mira el diff, les
// comprovacions del projecte i els criteris, i diu criteri per criteri si
// es compleix i amb quina prova. És el mateix verificador del mode
// objectiu (verify/goal.go): sense un veredicte explícit no hi ha aprovació,
// i el que l'agent diu de si mateix és una afirmació, no una prova.

const backlogReviewTimeout = 15 * time.Minute

// SetupBacklogReview fa que, en aquest procés, cada tasca que passa a
// in_review es revisi sola. Només el criden els front-ends que viuen prou
// (web, TUI, Telegram): un `gregal -p` sortiria a mitja revisió.
func SetupBacklogReview(cfg *config.Config, client *llm.Client) {
	if cfg == nil || client == nil {
		return
	}
	backlog.OnInReview = func(root string, it backlog.Item) {
		_, _ = StartBacklogReview(client, cfg, root, it.ID)
	}
}

// backlogVerifier és el model revisor; els tests el substitueixen.
var backlogVerifier = func(ctx context.Context, client *llm.Client, cfg *config.Config, dir, prompt string) (string, error) {
	role := cfg.AutonomousConfig().ReviewerRole
	if r, ok := cfg.Roles[role]; !ok || strings.TrimSpace(r.Model) == "" {
		role = "" // sense rol revisor: el per defecte del mode inspect
	}
	rr, err := RunNonInteractiveOpts(ctx, client, cfg, RunOpts{
		Task: prompt, Mode: ModeInspect, MaxSteps: goalVerifierSteps, HardStepLimit: true,
		Dir: dir, Role: role, Tools: goalVerifierTools,
	})
	return rr.Answer, err
}

// ReviewBacklogItem revisa una tasca que és a in_review i en desa el
// veredicte (backlog.Store.SetReview n'aplica el resultat).
func ReviewBacklogItem(ctx context.Context, client *llm.Client, cfg *config.Config, root string, id int) (backlog.Item, error) {
	st, it, err := beginBacklogReview(root, id)
	if err != nil {
		return it, err
	}
	return finishBacklogReview(ctx, client, cfg, st, it)
}

// StartBacklogReview deixa la revisió en marxa i torna de seguida: la
// resposta ja diu «running» i el veredicte arriba al log quan acaba.
func StartBacklogReview(client *llm.Client, cfg *config.Config, root string, id int) (backlog.Item, error) {
	st, it, err := beginBacklogReview(root, id)
	if err != nil {
		return it, err
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				_, _ = st.SetReview(id, backlog.Review{Status: backlog.ReviewError, Summary: fmt.Sprint("the reviewer crashed: ", r)}, "reviewer")
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), backlogReviewTimeout)
		defer cancel()
		_, _ = finishBacklogReview(ctx, client, cfg, st, it)
	}()
	return it, nil
}

func beginBacklogReview(root string, id int) (*backlog.Store, backlog.Item, error) {
	st, err := backlog.Open(root)
	if err != nil {
		return nil, backlog.Item{}, err
	}
	it, err := st.Get(id)
	if err != nil {
		return nil, backlog.Item{}, err
	}
	if it.Status != backlog.StatusInReview {
		return nil, it, fmt.Errorf("only an item in review is reviewed (#%d is %s)", id, it.Status)
	}
	if it.Review.Running() {
		return nil, it, errors.New("a review of this item is already running")
	}
	it, err = st.SetReview(id, backlog.Review{Status: backlog.ReviewRunning}, "reviewer")
	return st, it, err
}

func finishBacklogReview(ctx context.Context, client *llm.Client, cfg *config.Config, st *backlog.Store, it backlog.Item) (backlog.Item, error) {
	id := it.ID
	dir := it.Workdir
	if fi, err := os.Stat(dir); dir == "" || err != nil || !fi.IsDir() {
		dir = st.Root
	}

	checks := BacklogChecks(cfg, dir)
	var results []AutonomousCheck
	if len(checks) > 0 {
		cctx, cancel := context.WithTimeout(ctx, verifyTimeout*time.Duration(len(checks)))
		results = RunAutonomousChecks(cctx, dir, checks)
		cancel()
	}
	review := backlog.Review{}
	for _, c := range results {
		review.Checks = append(review.Checks, backlog.ReviewCheck{Command: c.Command, Passed: c.Code == 0})
	}
	for _, c := range results {
		if c.Code != 0 {
			// Una comprovació que falla ja diu prou: no cal gastar el model.
			review.Status = backlog.ReviewNotMet
			review.Summary = fmt.Sprintf("check `%s` failed (exit %d): %s", c.Command, c.Code, oneLine(trimTail(c.Output, 300)))
			return st.SetReview(id, review, "reviewer")
		}
	}

	lang := cfg.Lang()
	prompt := verify.BuildGoalPrompt(backlogReviewBody(it), backlogEvidence(dir, it, results), lang)
	answer, err := backlogVerifier(ctx, client, cfg, dir, prompt)
	if err != nil {
		review.Status, review.Summary = backlog.ReviewError, err.Error()
		return st.SetReview(id, review, "reviewer")
	}
	v := verify.ParseGoalVerdict(answer)
	for _, c := range v.Criteria {
		review.Criteria = append(review.Criteria, backlog.ReviewCriterion{Text: c.Text, Status: c.Status, Evidence: c.Evidence})
	}
	if v.Met {
		review.Status = backlog.ReviewMet
	} else {
		review.Status = backlog.ReviewNotMet
		review.Summary = firstOf(strings.TrimSpace(v.Missing), v.Reason)
		if strings.EqualFold(review.Summary, "none") {
			review.Summary = v.Reason
		}
	}
	return st.SetReview(id, review, "reviewer")
}

// backlogReviewBody és la tasca tal com la veu el revisor: els criteris
// numerats, perquè els jutgi en el mateix ordre.
func backlogReviewBody(it backlog.Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Backlog item #%d (%s): %s\n", it.ID, it.Type, it.Title)
	if it.Why != "" {
		fmt.Fprintf(&b, "\nWhy:\n%s\n", it.Why)
	}
	b.WriteString("\nAcceptance criteria (judge each one, in this order, one CRITERIA line per criterion):\n")
	if len(it.Criteria) == 0 {
		b.WriteString("(none written: derive 2-5 from the title and the why)\n")
	}
	for n, c := range it.Criteria {
		fmt.Fprintf(&b, "%d. %s\n", n+1, c.Text)
	}
	if it.OutOfScope != "" {
		fmt.Fprintf(&b, "\nOut of scope (do not require it):\n%s\n", it.OutOfScope)
	}
	return b.String()
}

// backlogEvidence és el que Gregal posa davant del revisor: comprovacions,
// els canvis des que es va començar la tasca, i el que l'agent diu haver
// demostrat (marcat com a afirmació).
func backlogEvidence(dir string, it backlog.Item, checks []AutonomousCheck) string {
	var b strings.Builder
	if base := strings.TrimSpace(it.Base); base != "" && gitOK(dir, "cat-file", "-e", base+"^{commit}") {
		b.WriteString("CHECKS (run by Gregal just now):\n")
		if len(checks) == 0 {
			b.WriteString("- none: the project has no check command; read the code and run read-only commands yourself\n")
		}
		for _, c := range checks {
			fmt.Fprintf(&b, "- `%s`: PASS\n", c.Command) // les que fallen no arriben al revisor
			if out := strings.TrimSpace(c.Output); out != "" {
				b.WriteString(indent(trimTail(out, 800)) + "\n")
			}
		}
		b.WriteString("\nCHANGES SINCE THE WORK STARTED (commits, stat and diff against " + base[:min(len(base), 12)] + "):\n")
		if log := gitText(dir, "log", "--oneline", base+"..HEAD"); log != "" {
			b.WriteString(log + "\n")
		}
		b.WriteString(gitText(dir, "diff", "--stat", base) + "\n")
		if nous := gitText(dir, "ls-files", "--others", "--exclude-standard"); nous != "" {
			b.WriteString("Untracked new files:\n" + nous + "\n")
		}
		b.WriteString(trimTail(gitText(dir, "diff", "--no-color", base), 12000) + "\n")
	} else {
		b.WriteString(GoalEvidence(dir, checks, nil, ""))
	}
	var claims []string
	for n, c := range it.Criteria {
		if c.Done && c.Evidence != "" {
			claims = append(claims, fmt.Sprintf("- %d. %s", n+1, c.Evidence))
		}
	}
	if len(claims) > 0 {
		b.WriteString("\nWHAT THE WORKER SAYS PROVES EACH CRITERION (claims to check, not evidence):\n" + strings.Join(claims, "\n") + "\n")
	}
	return b.String()
}

func gitText(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitOK(dir string, args ...string) bool {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run() == nil
}

// GitHead és el commit on és dir ("" si no és un repo amb commits).
func GitHead(dir string) string { return backlog.Head(dir) }

var checkTarget = regexp.MustCompile(`(?m)^check\s*:`)

// BacklogChecks són les comprovacions que el revisor executa: les del
// config (agent.autonomous.verify) o, si no n'hi ha, la comanda de
// comprovació del projecte si en té una de les habituals (make check, just
// check, scripts/check, npm run check). Sense cap, el revisor mira el codi.
func BacklogChecks(cfg *config.Config, dir string) []string {
	if cfg != nil {
		if v := cfg.AutonomousConfig().Verify; len(v) > 0 {
			return v
		}
	}
	llegeix := func(noms ...string) string {
		for _, n := range noms {
			if raw, err := os.ReadFile(filepath.Join(dir, n)); err == nil {
				return string(raw)
			}
		}
		return ""
	}
	if checkTarget.MatchString(llegeix("Makefile", "makefile", "GNUmakefile")) {
		return []string{"make check"}
	}
	if checkTarget.MatchString(llegeix("justfile", "Justfile", ".justfile")) {
		return []string{"just check"}
	}
	for _, s := range []string{"scripts/check", "scripts/check.sh"} {
		if fi, err := os.Stat(filepath.Join(dir, s)); err == nil && !fi.IsDir() {
			return []string{"sh " + s}
		}
	}
	if raw := llegeix("package.json"); raw != "" {
		var p struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal([]byte(raw), &p) == nil && p.Scripts["check"] != "" {
			return []string{"npm run check"}
		}
	}
	return nil
}
