package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/agent"
	"gregal/internal/brand"
	"gregal/internal/config"
	"gregal/internal/evals"
	"gregal/internal/llm"
	"gregal/internal/lsp"
	"gregal/internal/mcp"
	"gregal/internal/sandbox"
	"gregal/internal/selfupdate"
	"gregal/internal/service"
	"gregal/internal/session"
	"gregal/internal/telegram"
	"gregal/internal/tools"
	"gregal/internal/tui"
	"gregal/internal/web"
)

// version es fixa en compilar: go build -ldflags "-X main.version=vX.Y.Z".
var version = "v1.31.1"

// repoDir l'omple la compilacio (-ldflags "-X main.repoDir=..."): aixi el
// binari recorda d'on va sortir i `gregal update` el sap trobar encara que
// es cridi des d'un altre directori.
var repoDir string

func main() {
	// Procés auxiliar del sandbox Landlock (internal/sandbox): aplica les
	// regles i fa exec de l'ordre. Va primer de tot i no carrega res més.
	if len(os.Args) > 1 && os.Args[1] == sandbox.HelperArg {
		os.Exit(sandbox.RunHelper(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "update" {
		runUpdate(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runInit(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "completion" {
		runCompletion(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "config" {
		runConfig(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "memory" {
		runMemory(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "advise" {
		runAdvise(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCP(os.Args[2:])
		return
	}

	verFlag := flag.Bool("version", false, "show version and exit")
	checkFlag := flag.Bool("check-config", false, "validate configuration and exit")
	doctorFlag := flag.Bool("doctor", false, "check local configuration, providers and workspace health")
	cfgPath := flag.String("config", "", "configuration path (default ~/.config/gregal/config.yaml)")
	promptFlag := flag.String("p", "", "noninteractive mode: execute a task and exit")
	outputFlag := flag.String("output", "text", "output format for -p: text|json|stream-json (one JSON line per step, tool call and result, then the result)")
	approveFlag := flag.Bool("auto-approve", false, "with -p: approve ask-policy tools without prompting (not with --mode autonomous)")
	stepsFlag := flag.Int("max-steps", 0, "with -p: hard step limit, 1-200 (0 = configuration limit, which may be extended)")
	modeFlag := flag.String("mode", "", "with -p: code|inspect|chat|goal|autonomous (empty = configuration)")
	verifyFlag := flag.String("verify", "", "with -p: verification command; retry the agent on failure up to --verify-rounds (exit code 3 if it still fails)")
	verifyRoundsFlag := flag.Int("verify-rounds", 2, "with -p and --verify: maximum repair rounds (1-3)")
	goalFlag := flag.String("goal", "", "run a goal until an independent verifier confirms it with evidence: the id of a saved goal, \"last\", or the goal text (mode autonomous unless --mode; stops on --goal-rounds/--goal-minutes/--goal-cost/--max-steps; exit 3 if not met)")
	goalRounds := flag.Int("goal-rounds", 0, "with --goal: maximum rounds (default 4, at most 10)")
	goalMinutes := flag.Int("goal-minutes", 0, "with --goal: time limit in minutes (default 60)")
	goalCost := flag.Float64("goal-cost", 0, "with --goal: stop when the estimated cost reaches this many USD (0 = no limit)")
	serveFlag := flag.Bool("serve", false, "start the local web interface (chat and agent)")
	portFlag := flag.String("port", "8097", "with --serve: listen port")
	addrFlag := flag.String("addr", "", "with --serve: listen address (default 127.0.0.1; authentication required outside loopback)")
	tokenFlag := flag.String("token", "", "with --serve: API token (default $GREGAL_API_TOKEN)")
	openFlag := flag.Bool("open", false, "with --serve: open the browser")
	logFlag := flag.String("log-file", "", "with --serve: append the server log to this file too (or $GREGAL_LOG_FILE)")
	lifelineFlag := flag.Bool("lifeline", false, "with --serve: exit cleanly when stdin closes (for apps that run the server as a child process)")
	passwdFlag := flag.String("passwd", "", "change a configured user's password, read from stdin")
	tgFlag := flag.Bool("telegram", false, "start the Telegram bot")
	dirFlag := flag.String("dir", "", "agent workspace (default current directory)")
	evalFlag := flag.String("eval", "", "run evaluations from a directory; exit with code 1 if a task fails")
	evalTasks := flag.String("tasks", "", "with --eval: comma-separated task names (default all)")
	evalBaseline := flag.Bool("eval-baseline", false, "with --eval: save verdicts as baseline.json")
	evalRepeat := flag.Int("repeat", 1, "with --eval: run each task N times (success rate, median/IQR, pass@k)")
	bestOfFlag := flag.Int("best-of", 0, "with -p: try the task N ways (2-4) in separate git worktrees, verify and score each, and print a comparison; apply one with gregal worktree apply")
	bestOfApply := flag.Bool("best-of-apply", false, "with --best-of: apply the recommended attempt to the working tree (no commit) and discard the other worktrees")
	bestOfModels := flag.String("best-of-models", "", "with --best-of: comma-separated role names or provider/model per attempt (default: the code role)")
	continueFlag := flag.Bool("c", false, "with -p: continue the most recent saved session of this folder")
	flag.BoolVar(continueFlag, "continue", false, "with -p: same as -c")
	resumeFlag := flag.String("resume", "", "with -p: continue the saved session with this name")
	modelFlag := flag.String("model", "", "with -p: model for this run, as provider/model or a model of the code role's provider")
	noSessionFlag := flag.Bool("no-session", false, "with -p: do not save this run as a session")
	worktreeFlag := flag.String("worktree", "", "with -p or the TUI: work in an isolated git worktree .gregal/worktrees/<name> on branch gregal/<name> (created from HEAD if needed)")
	flag.Parse()

	// Una paraula que no és cap ordre coneguda no obre el TUI: un binari
	// vell que rebia `gregal mcp demo` (ordre posterior) ignorava els
	// arguments, obria el TUI, tornava a engegar el connector demo del config
	// compartit i així indefinidament (milers de processos).
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "gregal: unknown command %q (see gregal -h)\n", flag.Arg(0))
		os.Exit(2)
	}

	if *verFlag {
		fmt.Println("gregal", version)
		return
	}

	// Un --config explícit que no existeix és un error, no un config nou:
	// Load hi escrivia la plantilla per defecte i el torn anava a parar a
	// localhost:5800 sense dir res. Va passar amb una ruta relativa al banc
	// A/B (relativa al directori des d'on es llançava, no al de la tasca).
	// Sense --config, el primer arrencament sí que crea el de per defecte.
	if p := strings.TrimSpace(*cfgPath); p != "" {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "gregal: config: %s\n", cliText(configLanguage(p), "config.missing", p))
			os.Exit(1)
		}
	}
	cfg, path, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gregal: config: %s\n", cliText(configLanguage(path), "config.load_failed", err))
		os.Exit(1)
	}

	// Checkpoints git (refs amagats): valen per a tots els fronts.
	tools.ConfigureGitCheckpoints(cfg.Git.CheckpointsOn(), cfg.Git.CheckpointKeep)
	// Paquets de branding: temes de l'app i dels documents, a tots els fronts.
	brand.Setup(cfg.Brand)

	if *checkFlag {
		fmt.Printf("OK  %s\n", cliText(cliLanguage(cfg), "config.valid", path, cfg.RoleNames()))
		return
	}

	if *doctorFlag {
		os.Exit(doctor(cfg, path))
	}

	if g := strings.TrimSpace(*goalFlag); g != "" {
		if d := strings.TrimSpace(*dirFlag); d != "" {
			if err := os.Chdir(d); err != nil {
				fmt.Fprintln(os.Stderr, "gregal:", cliText(cliLanguage(cfg), "headless.dir_failed", err))
				os.Exit(2)
			}
		}
		if err := aplicaModel(cfg, *modelFlag); err != nil {
			fmt.Fprintln(os.Stderr, "gregal:", err)
			os.Exit(2)
		}
		os.Exit(runGoalHeadless(cfg, path, goalFlags{Spec: g, Output: *outputFlag, Mode: *modeFlag, MaxRounds: *goalRounds,
			MaxSteps: *stepsFlag, MaxMinutes: *goalMinutes, MaxCostUSD: *goalCost, VerifyCmd: *verifyFlag, AutoApprove: *approveFlag}))
	}

	if *promptFlag != "" {
		lang := cliLanguage(cfg)
		// Tot el que és d'ús es valida abans de gastar cap crida al model.
		if err := validaHeadless(cfg, *outputFlag, *modeFlag, *approveFlag, *stepsFlag, lang); err != nil {
			fmt.Fprintln(os.Stderr, "gregal:", err)
			os.Exit(2)
		}
		// --dir també val amb -p (abans només amb --serve i --telegram: el
		// torn treballava al directori des d'on s'havia llançat).
		if d := strings.TrimSpace(*dirFlag); d != "" {
			if err := os.Chdir(d); err != nil {
				fmt.Fprintln(os.Stderr, "gregal:", cliText(lang, "headless.dir_failed", err))
				os.Exit(2)
			}
		}
		if err := entraWorktree(*worktreeFlag); err != nil {
			fmt.Fprintln(os.Stderr, "gregal: --worktree:", err)
			os.Exit(2)
		}
		task, err := tascaAmbStdin(*promptFlag, os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gregal:", err)
			os.Exit(2)
		}
		if err := aplicaModel(cfg, *modelFlag); err != nil {
			fmt.Fprintln(os.Stderr, "gregal:", err)
			os.Exit(2)
		}
		if *bestOfFlag != 0 {
			os.Exit(runBestOfHeadless(cfg, task, *outputFlag, *bestOfFlag, *bestOfModels, *approveFlag, *stepsFlag, *modeFlag, *verifyFlag, *bestOfApply))
		}
		ex := headlessExtra{Desa: !*noSessionFlag, Continua: *continueFlag, Resume: *resumeFlag}
		os.Exit(runHeadless(cfg, task, *outputFlag, *approveFlag, *stepsFlag, *modeFlag, *verifyFlag, *verifyRoundsFlag, ex))
	}

	if *evalFlag != "" {
		os.Exit(runEvals(cfg, *evalFlag, *evalTasks, *stepsFlag, *evalRepeat, *evalBaseline))
	}

	if u := strings.TrimSpace(*passwdFlag); u != "" {
		us := web.NewUsers(cfg, path)
		if !us.Actiu() {
			fmt.Fprintln(os.Stderr, "gregal passwd: el config no té cap secció users:")
			os.Exit(1)
		}
		noms := us.Noms()
		trobat := false
		for _, n := range noms {
			if n == u {
				trobat = true
			}
		}
		if !trobat {
			fmt.Fprintf(os.Stderr, "gregal passwd: %q no és al config (hi ha: %s)\n", u, strings.Join(noms, ", "))
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "contrasenya nova per a %s: ", u)
		rd := bufio.NewReader(os.Stdin)
		pw, _ := rd.ReadString('\n')
		pw = strings.TrimRight(pw, "\r\n")
		if len(pw) < 8 {
			fmt.Fprintln(os.Stderr, "\nmassa curta: calen 8 caràcters o més")
			os.Exit(1)
		}
		if err := us.SetPassword(u, pw); err != nil {
			fmt.Fprintln(os.Stderr, "gregal passwd:", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "\ncontrasenya de %s desada (PBKDF2, fitxer a 0600)\n", u)
		return
	}

	if *serveFlag {
		if strings.TrimSpace(*dirFlag) != "" {
			if err := os.Chdir(*dirFlag); err != nil {
				fmt.Fprintln(os.Stderr, "gregal serve: dir:", err)
				os.Exit(1)
			}
		}
		lease, err := service.Acquire()
		if err != nil {
			fmt.Fprintln(os.Stderr, "gregal serve:", err)
			os.Exit(1)
		}
		defer lease.Release()
		web.SetupMCP(cfg)
		logPath := strings.TrimSpace(*logFlag)
		if logPath == "" {
			logPath = strings.TrimSpace(os.Getenv("GREGAL_LOG_FILE"))
		}
		cleanupLog, err := web.SetupServerLog(logPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "gregal serve: log:", err)
			os.Exit(1)
		}
		defer cleanupLog()
		addr := adrecaEscolta(*addrFlag, *portFlag)
		host, _, _ := net.SplitHostPort(addr)
		tok := strings.TrimSpace(*tokenFlag)
		if tok == "" {
			tok = strings.TrimSpace(os.Getenv("GREGAL_API_TOKEN"))
		}
		url := "http://" + addr + "/"
		fmt.Printf("gregal web a %s\n", url)
		// Obert a la xarxa sense token ni usuaris: en lloc de negar-s'hi, es
		// fa servir una clau pròpia (desada, la mateixa a cada arrencada) i
		// el missatge diu què escriure al mòbil.
		if !nomesLocal(host) && tok == "" && len(cfg.Users) == 0 {
			k, err := claveServidor()
			if err != nil {
				fmt.Fprintln(os.Stderr, "gregal serve: key:", err)
				os.Exit(1)
			}
			tok = k
		}
		_, port, _ := net.SplitHostPort(addr)
		fmt.Print(infoConnexio(cliLanguage(cfg), host, port, tok, strings.TrimSpace(os.Getenv("GREGAL_PUBLIC_URL")), len(cfg.Users) > 0, adrecesXarxa()))
		if *openFlag {
			web.Open(url)
		}
		srv := web.New(cfg, path)
		srv.SetToken(tok)
		// Tenim el lock: cap altre procés pot estar executant els torns
		// que un servidor anterior va deixar a mitges.
		if n := srv.CloseOrphanRuns(); n > 0 {
			fmt.Fprintf(os.Stderr, "gregal serve: %d interrupted run(s) from a previous process marked as failed\n", n)
		}
		cleanupDescriptor, err := service.Publish(service.Descriptor{
			Protocol: web.APIProtocol, Instance: srv.InstanceID(), Address: url,
			Token: tok, PID: os.Getpid(), StartedAt: time.Now().UTC(),
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "gregal serve: descriptor:", err)
			os.Exit(1)
		}
		defer cleanupDescriptor()
		shutdown := newServeShutdown(
			srv.Stop,
			func() { time.Sleep(cancelGrace) },
			cleanupDescriptor,
			func() { _ = lease.Release() },
			cleanupLog,
		)
		shutdown.watchSignals()
		if *lifelineFlag {
			shutdown.watchLifeline(os.Stdin)
		}
		if err := srv.Run(addr); err != nil {
			fmt.Fprintln(os.Stderr, "gregal serve:", err)
			os.Exit(1)
		}
		return
	}

	if *tgFlag {
		runTelegram(cfg, path, *dirFlag)
		return
	}

	if err := entraWorktree(*worktreeFlag); err != nil {
		fmt.Fprintln(os.Stderr, "gregal: --worktree:", err)
		os.Exit(2)
	}
	// Un servidor MCP fill mai obre el TUI: si l'ordre no es reconeix i hi
	// arribem, tornaríem a engegar els connectors del config i, amb ells, a
	// nosaltres mateixos.
	if os.Getenv(mcp.ChildEnv) != "" {
		fmt.Fprintln(os.Stderr, "gregal: started as an MCP server child; refusing to open the TUI")
		os.Exit(2)
	}
	client := llm.New()
	cfg.ConfigureClient(client)
	m := tui.New(cfg, path, client, version)
	// Roda del ratolí activa d'entrada: és com s'espera llegir una conversa
	// (opencode, Claude Code). Amb /mouse es desactiva si es vol la selecció
	// nativa del terminal per copiar i enganxar.
	// Amb mouse: off al config no s'activa gens: a Windows, activar-lo en
	// arrencar i desactivar-lo després (DisableMouse) passa per reiniciar
	// el lector de consola, que no sempre allibera la selecció del
	// terminal. Si no s'ha demanat, no es toca.
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.MouseOn() {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(m, opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "gregal:", err)
		os.Exit(1)
	}
}

// runTelegram engega el bot de Telegram (long polling).
func runTelegram(cfg *config.Config, cfgPath, dir string) {
	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintln(os.Stderr, "gregal telegram: dir:", err)
			os.Exit(1)
		}
	}
	if strings.TrimSpace(cfg.Telegram.Token) == "" {
		fmt.Fprintln(os.Stderr, "gregal telegram: falta telegram.token al config (o GREGAL_TELEGRAM_TOKEN)")
		os.Exit(2)
	}
	cwd, _ := os.Getwd()
	mcp.Setup(toMCPServers(cfg))
	logw := telegramLog()
	defer logw.Close()
	bot := telegram.New(telegram.Options{
		Cfg:          cfg,
		CfgPath:      cfgPath,
		Cwd:          cwd,
		Token:        cfg.Telegram.Token,
		AllowedUsers: cfg.Telegram.AllowedUsers,
		AllowGroups:  cfg.Telegram.AllowGroups,
		SessionsDir:  session.DefaultDir(),
		GoalsDir:     filepath.Dir(cfgPath),
		Log: func(format string, args ...any) {
			fmt.Fprintf(logw, "%s %s\n", time.Now().Format("2006-01-02 15:04:05"), fmt.Sprintf(format, args...))
		},
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Printf("gregal telegram: en marxa · projecte %s · log %s\n", cwd, telegramLogPath())
	if err := bot.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "gregal telegram:", err)
		os.Exit(1)
	}
}

// telegramLogPath retorna ~/.local/share/gregal/telegram.log.
func telegramLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "telegram.log"
	}
	return filepath.Join(home, ".local", "share", "gregal", "telegram.log")
}

// telegramLog obre el registre del bot (0600); si no pot, escriu a stderr.
func telegramLog() io.WriteCloser {
	path := telegramLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return os.Stderr
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return os.Stderr
	}
	return f
}

// runHeadless implementa `gregal -p "tasca"` (estil codex exec / vibe -p).
// runEvals corre el banc d'evals amb el config carregat i pinta la taula.
func runEvals(cfg *config.Config, dir, tasks string, maxSteps, repeat int, desaBaseline bool) int {
	llista, err := evals.Load(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gregal:", err)
		return 2
	}
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
	agent.SetLifecycleHooks(cfg.Hooks)
	client := llm.New()
	cfg.ConfigureClient(client)
	agent.SetupDelegate(cfg, client)
	var noms []string
	if strings.TrimSpace(tasks) != "" {
		noms = strings.Split(tasks, ",")
	}
	if maxSteps <= 0 {
		maxSteps = 25
	}
	res, err := evals.Run(context.Background(), cfg, client, dir, llista, evals.Options{
		MaxSteps: maxSteps, Names: noms, Repeat: repeat,
		OnEvent: func(s string) { fmt.Fprintln(os.Stderr, s) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "gregal:", err)
		return 2
	}
	fmt.Println()
	passed, total, regs := evals.Report(os.Stdout, res, evals.LoadBaseline(dir))
	if desaBaseline {
		if err := evals.SaveBaseline(dir, res); err != nil {
			fmt.Fprintln(os.Stderr, "gregal: baseline:", err)
		} else {
			fmt.Println("baseline desada a", dir+"/baseline.json")
		}
	}
	if passed < total || len(regs) > 0 {
		return 1
	}
	return 0
}

// Codis de sortida del mode -p.
const (
	exitOK           = 0
	exitError        = 1 // el torn ha fallat (proveïdor, config, cancel·lació)
	exitUsage        = 2 // flags invàlids
	exitVerifyFailed = 3 // --verify no ha passat després de totes les rondes
)

// validaHeadless comprova els flags de -p abans de fer cap crida (abans un
// --output mal escrit es descobria després del torn sencer, ja pagat, i un
// --mode mal escrit es convertia en code sense dir res).
func validaHeadless(cfg *config.Config, output, mode string, autoApprove bool, maxSteps int, lang string) error {
	if output != "text" && output != "json" && output != "stream-json" {
		return fmt.Errorf("%s", cliText(lang, "headless.bad_output", output))
	}
	if mode != "" && !agent.ValidMode(mode) {
		return fmt.Errorf("%s", cliText(lang, "headless.bad_mode", mode))
	}
	if maxSteps < 0 || maxSteps > 200 {
		return fmt.Errorf("%s", cliText(lang, "headless.bad_max_steps", maxSteps))
	}
	efectiu := mode
	if efectiu == "" && cfg != nil {
		efectiu = cfg.Mode
	}
	// El mode autònom no salta mai les aprovacions (és el seu contracte de
	// seguretat) i sense terminal ningú no les pot donar: amb --auto-approve
	// el torn es passava els passos denegat. Millor dir-ho ara.
	if efectiu == agent.ModeAutonomous && autoApprove {
		return fmt.Errorf("%s", cliText(lang, "headless.autonomous_auto_approve"))
	}
	return nil
}

// headlessJSON és la sortida de --output json: el resultat i, si el torn
// ha fallat, l'error (abans, en fallar, stdout quedava buit).
type headlessJSON struct {
	agent.RunResult
	// Session és el nom amb què s'ha desat la tirada (per a --resume).
	Session string `json:"session,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Amb verifyCmd no buit, el torn no es dona per bo fins que la comanda
// passa (gate mecànic: fins a verifyRounds rondes de reparació). Torna el
// codi de sortida.
func runHeadless(cfg *config.Config, task, output string, autoApprove bool, maxSteps int, mode, verifyCmd string, verifyRounds int, ex headlessExtra) int {
	lang := cliLanguage(cfg)
	cwd, _ := os.Getwd()
	sessDir := session.DefaultDir()
	nomSessio, prior, err := sessioPrevia(sessDir, cwd, ex)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gregal:", err)
		return exitUsage
	}
	if mode == "" {
		mode = cfg.Mode
	}
	// --max-steps explícit és un topall dur; el del config conserva les
	// ampliacions del motor.
	hard := maxSteps > 0
	if maxSteps == 0 {
		maxSteps = cfg.Agent.MaxSteps
	}
	mcp.Setup(toMCPServers(cfg))
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
	agent.SetLifecycleHooks(cfg.Hooks)
	agent.SetupPrices(cfg)
	// Els servidors de llenguatge que hagi engegat l'eina lsp s'aturen amb
	// la tirada (shutdown/exit), no queden orfes.
	defer lsp.Default.Close()
	cli := llm.New()
	cfg.ConfigureClient(cli)
	agent.SetupDelegate(cfg, cli)
	var stream *emissorJSONL
	var onEvent func(agent.Event)
	if output == "stream-json" {
		stream = &emissorJSONL{w: os.Stdout}
		onEvent = stream.event
	}
	res, err := agent.RunAmbVerificacio(context.Background(), cli, cfg, agent.RunOpts{
		Task: task, Mode: mode, MaxSteps: maxSteps, HardStepLimit: hard, AutoApprove: autoApprove, Dir: cwd, Prior: prior,
		OnEvent: onEvent,
		OnStep: func(step int, tools []string) {
			if stream != nil {
				stream.pas(step, tools)
			}
			if len(tools) == 0 {
				fmt.Fprintln(os.Stderr, cliText(lang, "headless.step_answering", step))
			} else {
				fmt.Fprintln(os.Stderr, cliText(lang, "headless.step_tools", step, strings.Join(tools, ", ")))
			}
		},
	}, verifyCmd, verifyRounds)
	if res.Tools == nil {
		res.Tools = []string{}
	}
	if res.RouteWhy != "" {
		fmt.Fprintf(os.Stderr, "🔀 %s (%s → %s)\n", res.RouteWhy, res.RoutedFrom, res.RoutedTo)
	}
	// La sessió es desa també si el torn ha fallat a mitges: la feina feta
	// hi és, i -c la pot reprendre.
	if ex.Desa {
		rol := "code"
		if res.RoutedTo != "" {
			rol = res.RoutedTo
		}
		if nom, errDesa := desaSessioHeadless(sessDir, nomSessio, cwd, rol, res.Convo); errDesa != nil {
			fmt.Fprintln(os.Stderr, "gregal:", cliText(lang, "headless.session_failed", errDesa))
		} else if nom != "" {
			nomSessio = nom
			if output == "text" {
				fmt.Fprintln(os.Stderr, cliText(lang, "headless.session_saved", nom))
			}
		}
	} else {
		nomSessio = ""
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gregal:", err)
		switch output {
		case "json":
			raw, _ := json.MarshalIndent(headlessJSON{RunResult: res, Session: nomSessio, Error: err.Error()}, "", "  ")
			fmt.Println(string(raw))
		case "stream-json":
			stream.resultat(headlessJSON{RunResult: res, Session: nomSessio, Error: err.Error()})
		}
		return exitError
	}
	code := exitOK
	if res.Verified != nil && !*res.Verified {
		code = exitVerifyFailed
		fmt.Fprintln(os.Stderr, "gregal:", cliText(lang, "headless.verify_failed", res.VerifyRounds, verifyCmd))
	}
	if res.Exhausted {
		fmt.Fprintln(os.Stderr, "gregal:", cliText(lang, "headless.exhausted"))
	}
	// Torn tallat per un error amb feina feta: la resposta parcial surt
	// igualment (no es perd), però el codi de sortida diu que ha fallat.
	if res.Interrupted != "" {
		fmt.Fprintln(os.Stderr, "gregal:", cliText(lang, "headless.interrupted", res.Interrupted))
		code = exitError
	}
	switch output {
	case "json":
		raw, _ := json.MarshalIndent(headlessJSON{RunResult: res, Session: nomSessio}, "", "  ")
		fmt.Println(string(raw))
		return code
	case "stream-json":
		stream.resultat(headlessJSON{RunResult: res, Session: nomSessio})
		return code
	}
	fmt.Println(res.Answer)
	if res.FallbackModel != "" {
		fmt.Fprintln(os.Stderr, cliText(lang, "headless.fallback", res.FallbackModel))
	}
	if res.CostUSD != nil {
		fmt.Fprintln(os.Stderr, cliText(lang, "headless.cost", agent.FmtCost(*res.CostUSD), res.UpTokens, res.DownTokens))
	}
	return code
}
func runCompletion(args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "ús: gregal completion bash|zsh")
		os.Exit(2)
	}
	switch args[0] {
	case "bash":
		fmt.Println(bashCompletion)
	case "zsh":
		fmt.Println(zshCompletion)
	default:
		fmt.Fprintln(os.Stderr, "gregal completion: shell desconeguda:", args[0])
		os.Exit(2)
	}
}

// Completions estàtiques (flags + subcomandaments; les ordres / van dins el TUI).
const bashCompletion = `# gregal — bash completion. Desa a ~/.local/share/bash-completion/completions/gregal
_gregal_complete() {
	local cur="${COMP_WORDS[COMP_CWORD]}"
	if [[ $COMP_CWORD -eq 1 ]]; then
		COMPREPLY=($(compgen -W "init completion --version --check-config --config" -- "$cur"))
	elif [[ "${COMP_WORDS[1]}" == "completion" && $COMP_CWORD -eq 2 ]]; then
		COMPREPLY=($(compgen -W "bash zsh" -- "$cur"))
	fi
}
complete -F _gregal_complete gregal`

const zshCompletion = `#compdef gregal
# gregal — zsh completion. Desa com a _gregal en un dir del teu $fpath.
local context state line
typeset -A opt_args
_arguments -C \
	'1: :->cmd' \
	'*:: :->args'
case $state in
	cmd)
		_values 'gregal' \
			'init[genera config + AGENTS.md]' \
			'completion[imprimeix el completion]' \
			'--version[mostra la versió]' \
			'--check-config[valida la configuració]' \
			'--config:[ruta del config]:_files'
		;;
	args)
		case $line[1] in
			completion) _values 'shell' 'bash' 'zsh' ;;
		esac
		;;
esac`

// toMCPServers adapta el config al paquet mcp (compartit TUI/headless).
func toMCPServers(cfg *config.Config) map[string]mcp.Server {
	return mcp.FromConfig(cfg.MCP)
}

// runInit implementa `gregal init [--force] [--config=RUTA]`.
func runInit(args []string) {
	force := false
	cfgPath := ""
	for _, a := range args {
		switch {
		case a == "--force":
			force = true
		case strings.HasPrefix(a, "--config="):
			cfgPath = strings.TrimPrefix(a, "--config=")
		default:
			fmt.Fprintf(os.Stderr, "gregal init: %s\n", cliText(configLanguage(cfgPath), "init.unknown_argument", a))
			os.Exit(2)
		}
	}
	path, err := config.InitConfig(cfgPath, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gregal init: %s\n", cliText(configLanguage(path), "init.create_failed", err))
		os.Exit(1)
	}
	cfg, path, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gregal init: %s\n", cliText(configLanguage(path), "init.invalid_generated", err))
		os.Exit(1)
	}
	lang := cliLanguage(cfg)
	fmt.Println(cliText(lang, "init.config_path", path))
	if _, err := os.Stat("AGENTS.md"); os.IsNotExist(err) {
		if err := os.WriteFile("AGENTS.md", []byte(config.AgentsTemplate), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "gregal init: %s\n", cliText(lang, "init.agents_failed", err))
			os.Exit(1)
		}
		fmt.Println(cliText(lang, "init.agents_created"))
	} else {
		fmt.Println(cliText(lang, "init.agents_exists"))
	}
}

// adrecaEscolta ajunta --addr i --port. El flag es diu "adreça" i el
// README escriu «--addr 0.0.0.0», així que qui hi posa «0.0.0.0:9000» no
// fa res estrany: abans ho enganxàvem igualment amb el port i sortia un
// "too many colons in address" que no diu quin dels dos valors sobrava.
// Si l'adreça ja porta port, mana ella.
func adrecaEscolta(addr, port string) string {
	host := strings.TrimSpace(addr)
	if h, p, err := net.SplitHostPort(host); err == nil && p != "" {
		host, port = h, p
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if strings.TrimSpace(port) == "" {
		port = "8097"
	}
	return net.JoinHostPort(host, port)
}

// nomesLocal diu si escoltar en aquesta adreça deixa l'agent només a
// aquesta màquina. Comparàvem la cadena amb "127.0.0.1", així que
// "127.0.0.2" o "::1" —loopback tots dos— es cridaven com si fossin la
// LAN, i l'avís de seguretat perd valor si salta quan no toca.
func nomesLocal(host string) bool {
	h := strings.TrimSpace(host)
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// runUpdate actualitza Gregal des del repositori: baixa, compila, comprova el
// binari nou i NOMES LLAVORS el substitueix; despres reinicia els serveis i
// comprova que hagin quedat actius. Si res falla, l'antic segueix instal·lat.
func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	repo := fs.String("repo", repoDir, "camí del repositori de Gregal (per defecte, el de la compilació o el directori actual)")
	check := fs.Bool("check", false, "només mira si hi ha res de nou; no toca res")
	forca := fs.Bool("forca", false, "recompila i reinstal·la encara que estiguis al dia")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	cwd, _ := os.Getwd()
	res, err := selfupdate.Executa(context.Background(), selfupdate.Opcions{
		Repo:          *repo,
		Cwd:           cwd,
		NomesComprova: *check,
		Forca:         *forca,
		Sortida:       func(s string) { fmt.Println(s) },
		Error:         func(s string) { fmt.Fprintln(os.Stderr, s) },
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ "+err.Error())
		os.Exit(1)
	}
	if len(res.Incidencies) > 0 {
		fmt.Println("⚠️  " + strings.Join(res.Incidencies, "; "))
		fmt.Println("   el binari anterior és al costat de l actual, amb el sufix .abans: torna enrere amb un mv")
		os.Exit(1)
	}
	if res.VersioDespres != "" {
		fmt.Printf("🎉 %s → %s\n", res.VersioAbans, res.VersioDespres)
	}
}
