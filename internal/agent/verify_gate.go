package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/config"
	"gregal/internal/llm"
)

// Límit dur de rondes de verificació (el flag pot demanar menys, mai més).
const MaxVerifyRounds = 3

// Timeout d'una comanda de verificació: prou per a go test ./... o pytest.
const verifyTimeout = 120 * time.Second

// La sortida del verify es retalla perquè càpiga al context del reintent.
const verifSortidaMax = 4000

// RunAmbVerificacio és RunNonInteractiveEx + gate mecànic: en acabar el
// torn, executa verifyCmd al workdir i, si falla, rellança l'agent amb la
// sortida del verify com a guia (fins a rounds vegades). No confia que el
// model s'autoverifiqui: mesurat en local, el model tanca el torn net sense
// haver executat la comprovació (T5: verify.py ignorat i "tot fet"
// igualment). Amb verifyCmd buit es comporta exactament com
// RunNonInteractiveEx (una ronda, sense verificar).
func RunAmbVerificacio(ctx context.Context, client *llm.Client, cfg *config.Config, task, mode string, maxSteps int, autoApprove bool, onStep func(step int, tools []string), workdir, verifyCmd string, rounds int) (RunResult, error) {
	tasca := task
	var res RunResult
	var err error
	if rounds < 1 {
		rounds = 1
	}
	if rounds > MaxVerifyRounds {
		rounds = MaxVerifyRounds
	}
	totUp, totDown, totCached, totCalls := 0, 0, 0, 0
	// El cost se suma ronda a ronda; si alguna no en té (sense preu), no se'n
	// mostra cap abans que un de parcial.
	totCost, senseCost := 0.0, false
	for r := 1; ; r++ {
		var envolta func(step int, tools []string)
		if onStep != nil {
			ronda := r
			envolta = func(step int, tools []string) {
				marcats := make([]string, len(tools))
				for i, t := range tools {
					marcats[i] = fmt.Sprintf("[v%d] %s", ronda, t)
				}
				onStep(step, marcats)
			}
		}
		res, err = RunNonInteractiveEx(ctx, client, cfg, tasca, mode, maxSteps, autoApprove, envolta)
		if err != nil {
			return res, err
		}
		totUp += res.UpTokens
		totDown += res.DownTokens
		totCached += res.CachedTokens
		totCalls += res.LLMCalls
		res.UpTokens, res.DownTokens = totUp, totDown
		res.CachedTokens, res.LLMCalls = totCached, totCalls
		if res.CostUSD == nil {
			senseCost = true
		}
		if senseCost {
			res.CostUSD = nil
		} else {
			totCost += *res.CostUSD
			v := totCost
			res.CostUSD = &v
		}
		if strings.TrimSpace(verifyCmd) == "" {
			return res, nil
		}
		codi, sortida := execVerificacio(ctx, workdir, verifyCmd)
		res.VerifSortida = sortida
		if codi == 0 {
			res.Verified = true
			return res, nil
		}
		if onStep != nil {
			onStep(0, []string{fmt.Sprintf("[v%d] verificacio-KO (codi %d)", r, codi)})
		}
		if r >= rounds {
			return res, nil
		}
		tasca = buildTascaReparacio(task, verifyCmd, codi, sortida, res.Answer)
	}
}

// execVerificacio executa cmd amb el shell del sistema al workdir (buit = directori actual)
// amb timeout. Torna el codi de sortida i la sortida combinada retallada.
// codi 127 = no s'ha pogut ni executar. Mai fa panic.
func execVerificacio(ctx context.Context, workdir, cmd string) (int, string) {
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()
	shell, shellArgs := verifyShell(cmd)
	c := exec.CommandContext(ctx, shell, shellArgs...)
	if strings.TrimSpace(workdir) != "" {
		c.Dir = workdir
	}
	c.Env = verifyEnv()
	var buf bytes.Buffer
	c.Stdout = &buf
	c.Stderr = &buf
	err := c.Run()
	sortida := strings.TrimSpace(buf.String())
	if r := []rune(sortida); len(r) > verifSortidaMax {
		sortida = string(r[:verifSortidaMax]) + "\n…(sortida retallada)"
	}
	if err == nil {
		return 0, sortida
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), sortida
	}
	return 127, "no s'ha pogut executar la verificació: " + err.Error() + "\n" + sortida
}

// verifyShell keeps the verification gate usable on Windows as well as on
// Unix. The command remains a shell command, but Windows installations do
// not normally ship sh.exe.
func verifyShell(cmd string) (string, []string) {
	if os.PathSeparator == '\\' {
		return "cmd.exe", []string{"/d", "/s", "/c", cmd}
	}
	return "sh", []string{"-c", cmd}
}

// verifyEnv hereta l'entorn però garanteix al PATH els toolchains coneguts
// de la màquina (go a ~/sdk/go/bin, etc.). El gate corre amb el shell del sistema
// i, sense això, `go test ./...` torna 127 encara que l'agent sí que troba
// el go (candidats d'entorn.go). Mesurat a T6: 127 amb la feina correcta.
func verifyEnv() []string {
	home, _ := os.UserHomeDir()
	candidats := []string{
		filepath.Join(home, "sdk", "go", "bin"),
		filepath.Join(home, "go", "bin"),
		"/usr/local/go/bin",
		filepath.Join(home, ".local", "bin"),
		// node no és al PATH del sistema: viu al runtime d'Hermes
		// (mesurat: sense això, el check post-edit de .js no corre mai
		// i els errors de sintaxi JS només surten al verify final).
		filepath.Join(home, ".hermes", "node", "bin"),
	}
	// Separador i nom de la variable segons el sistema: a Windows és ";"
	// i la clau sol ser "Path".
	sep := string(os.PathListSeparator)
	actual := os.Getenv("PATH")
	afegits := []string{}
	for _, d := range candidats {
		if st, err := os.Stat(d); err == nil && st.IsDir() && !strings.Contains(sep+actual+sep, sep+d+sep) {
			afegits = append(afegits, d)
		}
	}
	env := os.Environ()
	if len(afegits) == 0 {
		return env
	}
	nouPath := strings.Join(afegits, sep) + sep + actual
	for i, kv := range env {
		if strings.HasPrefix(strings.ToUpper(kv), "PATH=") {
			env[i] = "PATH=" + nouPath
			return env
		}
	}
	return append(env, "PATH="+nouPath)
}

// buildTascaReparacio munta la tasca de la ronda següent: la tasca original
// (no la resposta prèvia com a instrucció, per no heretar-ne els errors),
// l'evidència del verify i la prohibició d'afirmar èxit sense verd.
func buildTascaReparacio(original, cmd string, codi int, sortida, respostaPrevia string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Tasca original: %s\n\n", original)
	fmt.Fprintf(&b, "La verificació externa «%s» ha FALLAT (codi %d):\n%s\n\n", cmd, codi, sortida)
	if strings.TrimSpace(respostaPrevia) != "" {
		fmt.Fprintf(&b, "La teva resposta anterior deia:\n%s\n\n", respostaPrevia)
	}
	b.WriteString("No afirmis que funciona: arregla-ho amb eines i executa tu mateix la comanda de verificació fins que passi. Si creus que és la verificació la que és incorrecta (i no el teu codi), explica exactament per què amb evidència del fitxer; no la toquis per fer-la passar.")
	return b.String()
}
