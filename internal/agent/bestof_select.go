package agent

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Selecció del millor intent sense haver de preguntar (Agentless,
// CodeMonkeys): a més de les comprovacions de cada intent,
//   - proves creuades: els tests que ha escrit cada intent s'executen
//     contra el codi dels altres; un arreglat que passa els tests dels
//     altres és més probable que sigui correcte;
//   - vot: intents que arriben al mateix pegat (normalitzat) es reforcen.

var testPathRe = regexp.MustCompile(`(?i)(^|/)(test_[^/]+\.py|[^/]+_test\.(go|py)|[^/]+\.(test|spec)\.[jt]sx?|tests?/[^/]+)$`)

func esFitxerTest(p string) bool { return testPathRe.MatchString(filepath.ToSlash(p)) }

// normalPatch redueix un diff a les línies canviades sense espais ni
// comentaris, per comparar intents.
func normalPatch(patch string) string {
	var b strings.Builder
	for _, l := range strings.Split(patch, "\n") {
		if strings.HasPrefix(l, "+++") || strings.HasPrefix(l, "---") {
			continue
		}
		if !strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "-") {
			continue
		}
		t := strings.Join(strings.Fields(l[1:]), " ")
		if t == "" || strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") {
			continue
		}
		b.WriteString(l[:1] + t + "\n")
	}
	return b.String()
}

// bestOfVotes compta, per a cada intent, quants intents (ell inclòs) han
// arribat al mateix pegat normalitzat. Els tests no compten (cada intent
// els escriu a la seva manera).
func bestOfVotes(patches map[int]string) map[int]int {
	key := map[int]string{}
	count := map[string]int{}
	for i, p := range patches {
		var codi []string
		for _, bloc := range strings.Split(p, "diff --git ") {
			cap := strings.SplitN(bloc, "\n", 2)[0]
			if esFitxerTest(strings.TrimPrefix(strings.Fields(cap+" x")[0], "a/")) {
				continue
			}
			codi = append(codi, bloc)
		}
		k := normalPatch(strings.Join(codi, "diff --git "))
		if k == "" {
			continue
		}
		key[i] = k
		count[k]++
	}
	out := map[int]int{}
	for i, k := range key {
		out[i] = count[k]
	}
	return out
}

// bestOfCrossTests porta els fitxers de test de cada intent als altres i hi
// torna a passar les comprovacions. Torna, per intent, quants conjunts de
// tests aliens ha passat i quants n'ha provat.
func bestOfCrossTests(ctx context.Context, attempts []BestOfAttempt, verify []string) (map[int]int, map[int]int) {
	pass, total := map[int]int{}, map[int]int{}
	if len(verify) == 0 {
		return pass, total
	}
	// Tests nous o canviats de cada intent (contingut des del seu worktree).
	tests := map[int]map[string][]byte{}
	for i, a := range attempts {
		if a.State != BestOfDone || a.DiffBytes == 0 {
			continue
		}
		for _, f := range a.Files {
			if !esFitxerTest(f) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(a.Path, filepath.FromSlash(f)))
			if err != nil {
				continue
			}
			if tests[i] == nil {
				tests[i] = map[string][]byte{}
			}
			tests[i][f] = data
		}
	}
	for k, a := range attempts {
		if a.State != BestOfDone || a.DiffBytes == 0 || ctx.Err() != nil {
			continue
		}
		for i, set := range tests {
			if i == k {
				continue
			}
			// Copia els tests de l'intent i (guardant els originals de k).
			orig := map[string][]byte{}
			missing := map[string]bool{}
			for f, data := range set {
				dst := filepath.Join(a.Path, filepath.FromSlash(f))
				if old, err := os.ReadFile(dst); err == nil {
					orig[f] = old
				} else {
					missing[f] = true
					os.MkdirAll(filepath.Dir(dst), 0o755)
				}
				os.WriteFile(dst, data, 0o644)
			}
			cctx, cancel := context.WithTimeout(ctx, 2*verifyTimeout)
			checks := RunAutonomousChecks(cctx, a.Path, verify)
			cancel()
			ok := len(checks) > 0
			for _, c := range checks {
				if c.Code != 0 {
					ok = false
				}
			}
			total[a.Index]++
			if ok {
				pass[a.Index]++
			}
			for f := range set {
				dst := filepath.Join(a.Path, filepath.FromSlash(f))
				if missing[f] {
					os.Remove(dst)
				} else {
					os.WriteFile(dst, orig[f], 0o644)
				}
			}
		}
	}
	return pass, total
}
