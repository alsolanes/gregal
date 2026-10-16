package agent

// Best-of-N: la mateixa tasca, N maneres. Cada intent treballa en un
// worktree propi creat des de HEAD (tools.WorktreeCreate), opcionalment amb
// un model diferent; en acabar es passa la verificació configurada a cada
// worktree, es mesura el diff i el revisor el puntua amb una rúbrica. La
// persona tria quin intent porta al repo principal (tools.WorktreeApply) i
// la resta es llença (tools.WorktreeDiscard). El repo principal no es toca
// fins aleshores.

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
	"gregal/internal/verify"
)

// Topalls del best-of.
const (
	BestOfMin = 2
	BestOfMax = 4
)

// Estats d'un intent.
const (
	BestOfQueued   = "queued"
	BestOfRunning  = "running"
	BestOfChecking = "checking"
	BestOfDone     = "done"
	BestOfFailed   = "failed"
)

// BestOfAttempt és un intent i la seva fitxa de comparació.
type BestOfAttempt struct {
	Index    int    `json:"index"`
	Worktree string `json:"worktree"`
	Path     string `json:"path"`
	Branch   string `json:"branch"`
	Role     string `json:"role"`
	Model    string `json:"model"`
	State    string `json:"state"`
	Answer   string `json:"answer,omitempty"`
	Error    string `json:"error,omitempty"`
	// Diff: fitxers tocats i línies afegides/esborrades.
	Files     []string `json:"files"`
	Added     int      `json:"added"`
	Deleted   int      `json:"deleted"`
	DiffBytes int      `json:"diff_bytes"`
	// Checks és la verificació configurada; ChecksPassed nil = cap check.
	Checks       []AutonomousCheck `json:"checks"`
	ChecksPassed *bool             `json:"checks_passed"`
	// Score és la nota del revisor (0-10); nil = sense revisió.
	Score     *int     `json:"score"`
	ScoreNote string   `json:"score_note,omitempty"`
	// Votes: intents (aquest inclòs) que han arribat al mateix pegat.
	Votes int `json:"votes"`
	// CrossPassed/CrossTotal: tests escrits pels altres intents que aquest
	// codi passa (bestof_select.go).
	CrossPassed int `json:"cross_passed"`
	CrossTotal  int `json:"cross_total"`
	CostUSD   *float64 `json:"cost_usd"`
	Steps     int      `json:"steps"`
	Seconds   float64  `json:"seconds"`
}

// BestOfResult és el conjunt d'intents.
type BestOfResult struct {
	ID       string          `json:"id"`
	Task     string          `json:"task"`
	Main     string          `json:"main"`
	Attempts []BestOfAttempt `json:"attempts"`
	// Recommended és l'índex de l'intent que surt millor (-1 = cap).
	Recommended int `json:"recommended"`
}

// BestOfRunFunc executa una tasca en un worktree (per defecte,
// RunNonInteractiveOpts). La web hi posa la cua d'execucions.
type BestOfRunFunc func(ctx context.Context, cfg *config.Config, o RunOpts) (RunResult, error)

// BestOfScoreFunc puntua un diff (0-10).
type BestOfScoreFunc func(ctx context.Context, task, diff, checks string) (int, string, error)

// BestOfOpts són les opcions del best-of.
type BestOfOpts struct {
	ID   string
	Task string
	N    int
	// Models: per intent, un nom de rol o provider/model. Buit (o més curt
	// que N) = el rol code.
	Models      []string
	Dir         string
	Mode        string
	MaxSteps    int
	AutoApprove bool
	// Verify són les ordres de verificació (buit = agent.autonomous.verify).
	Verify []string
	// Parallel topa els intents alhora (0 = N).
	Parallel int
	Run      BestOfRunFunc
	Score    BestOfScoreFunc
	// OnUpdate rep l'estat de cada intent quan canvia.
	OnUpdate func(BestOfAttempt)
}

// BestOfName és el nom del worktree de l'intent i (1..N).
func BestOfName(id string, i int) string { return fmt.Sprintf("bestof-%s-%d", id, i) }

// NewBestOfID dona un id curt per als noms dels worktrees.
func NewBestOfID() string { return time.Now().UTC().Format("0102-150405") }

// bestOfRole resol el model d'un intent: un rol del config o un
// provider/model (sobre la base del rol code). Torna el config a fer servir
// i el nom del rol.
func bestOfRole(cfg *config.Config, spec string, i int) (*config.Config, string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = "code"
	}
	if _, ok := cfg.Roles[spec]; ok {
		return cfg, spec, nil
	}
	prov, model, ok := strings.Cut(spec, "/")
	if !ok || prov == "" || model == "" {
		return nil, "", fmt.Errorf("model %q: ni rol del config ni provider/model", spec)
	}
	if _, ok := cfg.Providers[prov]; !ok {
		return nil, "", fmt.Errorf("model %q: provider %q desconegut", spec, prov)
	}
	c := *cfg
	c.Roles = maps.Clone(cfg.Roles)
	base := cfg.Roles["code"]
	base.Provider, base.Model = prov, model
	base.FallbackProvider, base.FallbackModel = "", ""
	name := fmt.Sprintf("bestof-%d", i)
	c.Roles[name] = base
	return &c, name, nil
}

// DiffStats compta fitxers i línies d'un pegat git.
func DiffStats(patch string) (files []string, added, deleted int) {
	files = []string{}
	for _, l := range strings.Split(strings.ReplaceAll(patch, "\r", ""), "\n") {
		switch {
		case strings.HasPrefix(l, "diff --git "):
			f := strings.TrimPrefix(l, "diff --git ")
			if i := strings.Index(f, " b/"); i >= 0 {
				f = f[i+3:]
			}
			files = append(files, strings.TrimPrefix(f, "a/"))
		case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"):
		case strings.HasPrefix(l, "+"):
			added++
		case strings.HasPrefix(l, "-"):
			deleted++
		}
	}
	return files, added, deleted
}

// ResumChecks és el text de les comprovacions per al revisor.
func ResumChecks(checks []AutonomousCheck) string {
	var b strings.Builder
	for _, c := range checks {
		st := "PASS"
		if c.Code != 0 {
			st = fmt.Sprintf("FAIL (exit %d)", c.Code)
		}
		fmt.Fprintf(&b, "$ %s → %s\n", c.Command, st)
	}
	return b.String()
}

// BestOfScorer és la puntuació amb el rol reviewer (nil si no n'hi ha).
func BestOfScorer(client *llm.Client, cfg *config.Config) BestOfScoreFunc {
	p, r, ok := CriticRole(cfg)
	if !ok || client == nil {
		return nil
	}
	return func(ctx context.Context, task, diff, checks string) (int, string, error) {
		cctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		return verify.Score(cctx, client, p, r, task, diff, checks, cfg.Lang())
	}
}

// RunBestOf crea els worktrees, hi corre la tasca i en fa la comparació.
// Els worktrees queden vius: qui crida decideix quin s'aplica
// (BestOfApply) i llença la resta (BestOfDiscard).
func RunBestOf(ctx context.Context, client *llm.Client, cfg *config.Config, o BestOfOpts) (BestOfResult, error) {
	if cfg == nil {
		return BestOfResult{}, fmt.Errorf("sense config")
	}
	if strings.TrimSpace(o.Task) == "" {
		return BestOfResult{}, fmt.Errorf("tasca buida")
	}
	if o.N < BestOfMin || o.N > BestOfMax {
		return BestOfResult{}, fmt.Errorf("N ha d'anar de %d a %d (rebut %d)", BestOfMin, BestOfMax, o.N)
	}
	if o.ID == "" {
		o.ID = NewBestOfID()
	}
	// Els intents són feina de codi sense ningú davant: code (o debug, que
	// té la mateixa política); l'autònom necessitaria aprovacions.
	if o.Mode != ModeDebug {
		o.Mode = ModeCode
	}
	if o.MaxSteps <= 0 {
		o.MaxSteps = cfg.Agent.MaxSteps
	}
	if o.MaxSteps <= 0 || o.MaxSteps > 200 {
		o.MaxSteps = 40
	}
	verifyCmds := o.Verify
	if len(verifyCmds) == 0 {
		verifyCmds = cfg.AutonomousConfig().Verify
	}
	run := o.Run
	if run == nil {
		run = func(ctx context.Context, c *config.Config, ro RunOpts) (RunResult, error) {
			return RunNonInteractiveOpts(ctx, client, c, ro)
		}
	}
	main, err := tools.MainRepoRoot(o.Dir)
	if err != nil {
		return BestOfResult{}, err
	}
	res := BestOfResult{ID: o.ID, Task: o.Task, Main: main, Recommended: -1}
	// Models i worktrees abans d'arrencar res: un model mal escrit no ha de
	// deixar mitja feina feta. git worktree add no va bé en paral·lel.
	cfgs := make([]*config.Config, o.N)
	for i := 0; i < o.N; i++ {
		spec := ""
		if i < len(o.Models) {
			spec = o.Models[i]
		}
		c, role, err := bestOfRole(cfg, spec, i+1)
		if err != nil {
			return res, err
		}
		cfgs[i] = c
		r := c.Roles[role]
		res.Attempts = append(res.Attempts, BestOfAttempt{Index: i + 1, Role: role, Model: r.Provider + "/" + r.Model,
			State: BestOfQueued, Files: []string{}, Checks: []AutonomousCheck{}})
	}
	for i := range res.Attempts {
		wt, err := tools.WorktreeCreate(main, BestOfName(o.ID, i+1))
		if err != nil {
			for j := 0; j < i; j++ {
				_ = tools.WorktreeDiscard(main, res.Attempts[j].Worktree)
			}
			return res, fmt.Errorf("worktree de l'intent %d: %w", i+1, err)
		}
		a := &res.Attempts[i]
		a.Worktree, a.Path, a.Branch = wt.Name, wt.Path, wt.Branch
	}
	var mu sync.Mutex
	update := func(i int, f func(a *BestOfAttempt)) {
		mu.Lock()
		f(&res.Attempts[i])
		snap := res.Attempts[i]
		mu.Unlock()
		if o.OnUpdate != nil {
			o.OnUpdate(snap)
		}
	}
	for i := range res.Attempts {
		if o.OnUpdate != nil {
			o.OnUpdate(res.Attempts[i])
		}
	}
	par := o.Parallel
	if par <= 0 || par > o.N {
		par = o.N
	}
	sem := make(chan struct{}, par)
	var wg sync.WaitGroup
	for i := range res.Attempts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				update(i, func(a *BestOfAttempt) { a.State, a.Error = BestOfFailed, ctx.Err().Error() })
				return
			}
			defer func() { <-sem }()
			bestOfAttempt(ctx, cfgs[i], o, verifyCmds, run, i, &mu, &res, update)
		}(i)
	}
	wg.Wait()
	// Selecció: vot entre pegats equivalents i proves creuades.
	patches := map[int]string{}
	for i, a := range res.Attempts {
		if a.State == BestOfDone && a.DiffBytes > 0 {
			if p, _, err := tools.WorktreeDiff(a.Path, a.Worktree); err == nil {
				patches[i] = p
			}
		}
	}
	votes := bestOfVotes(patches)
	pass, total := bestOfCrossTests(ctx, res.Attempts, verifyCmds)
	for i := range res.Attempts {
		res.Attempts[i].Votes = votes[i]
		idx := res.Attempts[i].Index
		res.Attempts[i].CrossPassed, res.Attempts[i].CrossTotal = pass[idx], total[idx]
	}
	res.Recommended = BestOfRecommend(res.Attempts)
	return res, ctx.Err()
}

func bestOfAttempt(ctx context.Context, cfg *config.Config, o BestOfOpts, verifyCmds []string, run BestOfRunFunc, i int, mu *sync.Mutex, res *BestOfResult, update func(int, func(*BestOfAttempt))) {
	mu.Lock()
	a := res.Attempts[i]
	mu.Unlock()
	start := time.Now()
	update(i, func(a *BestOfAttempt) { a.State = BestOfRunning })
	rr, err := run(ctx, cfg, RunOpts{Task: o.Task, Mode: o.Mode, MaxSteps: o.MaxSteps, AutoApprove: o.AutoApprove, Dir: a.Path, Role: a.Role})
	update(i, func(a *BestOfAttempt) {
		a.Answer, a.Steps, a.CostUSD = rr.Answer, rr.Steps, rr.CostUSD
		if err != nil {
			a.Error = err.Error()
		}
		a.State = BestOfChecking
	})
	patch, _, derr := tools.WorktreeDiff(a.Path, a.Worktree)
	files, add, del := DiffStats(patch)
	var checks []AutonomousCheck
	if len(verifyCmds) > 0 && ctx.Err() == nil {
		cctx, cancel := context.WithTimeout(ctx, 2*verifyTimeout)
		checks = RunAutonomousChecks(cctx, a.Path, verifyCmds)
		cancel()
	}
	var score *int
	note := ""
	if o.Score != nil && ctx.Err() == nil {
		if n, why, serr := o.Score(ctx, o.Task, patch, ResumChecks(checks)); serr == nil {
			score, note = &n, why
		} else {
			note = serr.Error()
		}
	}
	update(i, func(a *BestOfAttempt) {
		a.Files, a.Added, a.Deleted, a.DiffBytes = files, add, del, len(patch)
		if derr != nil && a.Error == "" {
			a.Error = "diff: " + derr.Error()
		}
		if checks == nil {
			checks = []AutonomousCheck{}
		}
		a.Checks = checks
		if len(checks) > 0 {
			ok := true
			for _, c := range checks {
				if c.Code != 0 {
					ok = false
				}
			}
			a.ChecksPassed = &ok
		}
		a.Score, a.ScoreNote = score, note
		a.Seconds = time.Since(start).Round(time.Second).Seconds()
		if err != nil {
			a.State = BestOfFailed
		} else {
			a.State = BestOfDone
		}
	})
}

// BestOfRecommend tria l'intent que surt millor: sense error i amb canvis,
// checks verds abans que vermells, més nota, diff més petit, més barat.
func BestOfRecommend(list []BestOfAttempt) int {
	idx := []int{}
	for i, a := range list {
		if a.State == BestOfDone && a.DiffBytes > 0 {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return -1
	}
	rank := func(a BestOfAttempt) (int, int) {
		c := 1
		if a.ChecksPassed != nil {
			if *a.ChecksPassed {
				c = 2
			} else {
				c = 0
			}
		}
		s := -1
		if a.Score != nil {
			s = *a.Score
		}
		return c, s
	}
	cross := func(a BestOfAttempt) float64 {
		if a.CrossTotal == 0 {
			return -1
		}
		return float64(a.CrossPassed) / float64(a.CrossTotal)
	}
	sort.SliceStable(idx, func(x, y int) bool {
		a, b := list[idx[x]], list[idx[y]]
		ca, sa := rank(a)
		cb, sb := rank(b)
		if ca != cb {
			return ca > cb
		}
		if xa, xb := cross(a), cross(b); xa != xb {
			return xa > xb
		}
		if a.Votes != b.Votes {
			return a.Votes > b.Votes
		}
		if sa != sb {
			return sa > sb
		}
		if a.Added+a.Deleted != b.Added+b.Deleted {
			return a.Added+a.Deleted < b.Added+b.Deleted
		}
		if a.CostUSD != nil && b.CostUSD != nil {
			return *a.CostUSD < *b.CostUSD
		}
		return false
	})
	return list[idx[0]].Index
}

// BestOfApply porta l'intent triat al repo principal com a pegat (sense
// commit) i llença els worktrees dels altres. Amb keep=false també llença
// el triat (la feina ja és al repo principal).
func BestOfApply(dir string, attempts []BestOfAttempt, index int, keep bool) (string, []string, error) {
	var chosen *BestOfAttempt
	for i := range attempts {
		if attempts[i].Index == index {
			chosen = &attempts[i]
		}
	}
	if chosen == nil {
		return "", nil, fmt.Errorf("intent %d inexistent", index)
	}
	msg, err := tools.WorktreeApply(dir, chosen.Worktree, "patch")
	if err != nil {
		return "", nil, err
	}
	var removed []string
	for _, a := range attempts {
		if a.Index == index && keep {
			continue
		}
		if err := tools.WorktreeDiscard(dir, a.Worktree); err == nil {
			removed = append(removed, a.Worktree)
		}
	}
	return msg, removed, nil
}

// BestOfDiscard llença tots els worktrees dels intents.
func BestOfDiscard(dir string, attempts []BestOfAttempt) []string {
	var removed []string
	for _, a := range attempts {
		if a.Worktree == "" {
			continue
		}
		if err := tools.WorktreeDiscard(dir, a.Worktree); err == nil {
			removed = append(removed, a.Worktree)
		}
	}
	return removed
}

// RenderBestOf és la taula de comparació per al terminal.
func RenderBestOf(r BestOfResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Best-of-%d · %s\n", len(r.Attempts), r.ID)
	for _, a := range r.Attempts {
		checks := "-"
		if a.ChecksPassed != nil {
			checks = "fail"
			if *a.ChecksPassed {
				checks = "pass"
			}
		}
		score := "-"
		if a.Score != nil {
			score = fmt.Sprintf("%d/10", *a.Score)
		}
		cost := "-"
		if a.CostUSD != nil {
			cost = FmtCost(*a.CostUSD)
		}
		mark := " "
		if a.Index == r.Recommended {
			mark = "*"
		}
		cross := "-"
		if a.CrossTotal > 0 {
			cross = fmt.Sprintf("%d/%d", a.CrossPassed, a.CrossTotal)
		}
		fmt.Fprintf(&b, "%s #%d %-28s %-7s files %-2d +%d -%d  checks %-4s  cross %-5s  votes %d  score %-5s  cost %s  %s\n",
			mark, a.Index, a.Model, a.State, len(a.Files), a.Added, a.Deleted, checks, cross, a.Votes, score, cost, a.Worktree)
		if a.Error != "" {
			fmt.Fprintf(&b, "     error: %s\n", retallaText(a.Error, 160))
		}
	}
	return b.String()
}
