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

// RunAmbVerificacio és RunNonInteractiveOpts + gate mecànic: en acabar el
// torn, executa verifyCmd al workspace (o.Dir) i, si falla, rellança
// l'agent amb la sortida del verify com a guia (fins a rounds vegades). No
// confia que el model s'autoverifiqui: mesurat en local, el model tanca el
// torn net sense haver executat la comprovació (T5: verify.py ignorat i
// "tot fet" igualment). Amb verifyCmd buit es comporta exactament com
// RunNonInteractiveOpts (una ronda, sense verificar).
//
// El resultat suma totes les rondes (passos, eines, trace, tokens, cost):
// abans només en quedava l'última i la resposta podia atribuir a «ja hi
// era» el que l'agent havia arreglat a la ronda anterior. Amb verifyCmd,
// Verified surt sempre (true o false): qui crida decideix el codi de
// sortida (el CLI torna 3 si la verificació no passa).
func RunAmbVerificacio(ctx context.Context, client *llm.Client, cfg *config.Config, o RunOpts, verifyCmd string, rounds int) (RunResult, error) {
	task := o.Task
	tasca := task
	verificar := strings.TrimSpace(verifyCmd) != ""
	onStep := o.OnStep
	var acum RunResult
	acum.Tools = []string{}
	if rounds < 1 {
		rounds = 1
	}
	if rounds > MaxVerifyRounds {
		rounds = MaxVerifyRounds
	}
	// El cost se suma ronda a ronda; si alguna no en té (sense preu), no se'n
	// mostra cap abans que un de parcial.
	totCost, senseCost := 0.0, false
	for r := 1; ; r++ {
		ro := o
		ro.Task = tasca
		// El prefix de ronda només té sentit amb --verify.
		if onStep != nil && verificar {
			ronda := r
			ro.OnStep = func(step int, tools []string) {
				marcats := make([]string, len(tools))
				for i, t := range tools {
					marcats[i] = fmt.Sprintf("[v%d] %s", ronda, t)
				}
				onStep(step, marcats)
			}
		}
		res, err := RunNonInteractiveOpts(ctx, client, cfg, ro)
		acum = sumaRonda(acum, res, r, verificar)
		if res.CostUSD == nil {
			senseCost = true
		}
		if senseCost {
			acum.CostUSD = nil
		} else {
			totCost += *res.CostUSD
			v := totCost
			acum.CostUSD = &v
		}
		if err != nil {
			return acum, err
		}
		if !verificar {
			return acum, nil
		}
		acum.VerifyRounds = r
		codi, sortida := execVerificacio(ctx, o.Dir, verifyCmd)
		acum.VerifSortida = sortida
		ok := codi == 0
		acum.Verified = &ok
		if ok {
			return acum, nil
		}
		if onStep != nil {
			onStep(0, []string{fmt.Sprintf("[v%d] verification failed (exit %d)", r, codi)})
		}
		if r >= rounds {
			return acum, nil
		}
		tasca = buildTascaReparacio(task, verifyCmd, codi, sortida, res.Answer)
	}
}

// sumaRonda afegeix una ronda al resultat acumulat. La resposta, el model
// de fallback i l'encaminament són els de l'última ronda; la resta se suma.
func sumaRonda(acum, res RunResult, ronda int, marcaRonda bool) RunResult {
	if res.Answer != "" || ronda == 1 {
		acum.Answer = res.Answer
	}
	acum.Steps += res.Steps
	acum.Convo = res.Convo
	acum.Tools = append(acum.Tools, res.Tools...)
	for _, tc := range res.Trace {
		if marcaRonda {
			tc.Round = ronda
		}
		acum.Trace = append(acum.Trace, tc)
	}
	acum.UpTokens += res.UpTokens
	acum.DownTokens += res.DownTokens
	acum.CachedTokens += res.CachedTokens
	acum.LLMCalls += res.LLMCalls
	switch {
	case acum.TokensSource == "" || acum.TokensSource == res.TokensSource:
		acum.TokensSource = res.TokensSource
	default:
		acum.TokensSource = "mixed"
	}
	if res.FallbackModel != "" {
		acum.FallbackModel = res.FallbackModel
	}
	acum.RoutedFrom, acum.RoutedTo, acum.RouteWhy = res.RoutedFrom, res.RoutedTo, res.RouteWhy
	acum.Exhausted = res.Exhausted
	return acum
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
	fmt.Fprintf(&b, "Original task: %s\n\n", original)
	fmt.Fprintf(&b, "The external verification `%s` FAILED (exit code %d):\n%s\n\n", cmd, codi, sortida)
	if strings.TrimSpace(respostaPrevia) != "" {
		fmt.Fprintf(&b, "Your previous answer said:\n%s\n\n", respostaPrevia)
	}
	b.WriteString("The changes from the previous round are already on disk. Do not claim it works: fix it with tools and run the verification command yourself until it passes. If you believe the verification itself is wrong (not your code), explain exactly why with evidence from the files; do not change it to make it pass. In your final answer, list every change you made in all rounds.")
	return b.String()
}
