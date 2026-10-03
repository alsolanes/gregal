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
	"gregal/internal/config"
	"gregal/internal/evals"
	"gregal/internal/llm"
	"gregal/internal/mcp"
	"gregal/internal/selfupdate"
	"gregal/internal/service"
	"gregal/internal/session"
	"gregal/internal/telegram"
	"gregal/internal/tui"
	"gregal/internal/web"
)

// version es fixa en compilar: go build -ldflags "-X main.version=vX.Y.Z".
var version = "v1.7.2"

// repoDir l'omple la compilacio (-ldflags "-X main.repoDir=..."): aixi el
// binari recorda d'on va sortir i `gregal update` el sap trobar encara que
// es cridi des d'un altre directori.
var repoDir string

func main() {
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
	if len(os.Args) > 1 && os.Args[1] == "advise" {
		runAdvise(os.Args[2:])
		return
	}

	verFlag := flag.Bool("version", false, "show version and exit")
	checkFlag := flag.Bool("check-config", false, "validate configuration and exit")
	doctorFlag := flag.Bool("doctor", false, "check local configuration, providers and workspace health")
	cfgPath := flag.String("config", "", "configuration path (default ~/.config/gregal/config.yaml)")
	promptFlag := flag.String("p", "", "noninteractive mode: execute a task and exit")
	outputFlag := flag.String("output", "text", "output format for -p: text|json")
	approveFlag := flag.Bool("auto-approve", false, "with -p: approve ask-policy tools without prompting")
	stepsFlag := flag.Int("max-steps", 0, "with -p: step limit (0 = configuration)")
	modeFlag := flag.String("mode", "", "with -p: code|inspect|chat|goal|autonomous (empty = configuration)")
	verifyFlag := flag.String("verify", "", "with -p: verification command; retry the agent on failure up to --verify-rounds")
	verifyRoundsFlag := flag.Int("verify-rounds", 2, "with -p and --verify: maximum repair rounds (1-3)")
	serveFlag := flag.Bool("serve", false, "start the local web interface (chat and agent)")
	portFlag := flag.String("port", "8097", "with --serve: listen port")
	addrFlag := flag.String("addr", "", "with --serve: listen address (default 127.0.0.1; authentication required outside loopback)")
	tokenFlag := flag.String("token", "", "with --serve: API token (default $GREGAL_API_TOKEN)")
	openFlag := flag.Bool("open", false, "with --serve: open the browser")
	passwdFlag := flag.String("passwd", "", "change a configured user's password, read from stdin")
	tgFlag := flag.Bool("telegram", false, "start the Telegram bot")
	dirFlag := flag.String("dir", "", "agent workspace (default current directory)")
	evalFlag := flag.String("eval", "", "run evaluations from a directory; exit with code 1 if a task fails")
	evalTasks := flag.String("tasks", "", "with --eval: comma-separated task names (default all)")
	evalBaseline := flag.Bool("eval-baseline", false, "with --eval: save verdicts as baseline.json")
	flag.Parse()

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

	if *checkFlag {
		fmt.Printf("OK  %s\n", cliText(cliLanguage(cfg), "config.valid", path, cfg.RoleNames()))
		return
	}

	if *doctorFlag {
		os.Exit(doctor(cfg, path))
	}

	if *promptFlag != "" {
		runHeadless(cfg, *promptFlag, *outputFlag, *approveFlag, *stepsFlag, *modeFlag, *verifyFlag, *verifyRoundsFlag)
		return
	}

	if *evalFlag != "" {
		os.Exit(runEvals(cfg, *evalFlag, *evalTasks, *stepsFlag, *evalBaseline))
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
		addr := adrecaEscolta(*addrFlag, *portFlag)
		host, _, _ := net.SplitHostPort(addr)
		tok := strings.TrimSpace(*tokenFlag)
		if tok == "" {
			tok = strings.TrimSpace(os.Getenv("GREGAL_API_TOKEN"))
		}
		url := "http://" + addr + "/"
		fmt.Printf("gregal web a %s\n", url)
		if !nomesLocal(host) && tok == "" && len(cfg.Users) == 0 {
			fmt.Fprintln(os.Stderr, "gregal serve: remote access requires --token, GREGAL_API_TOKEN, or configured users")
			os.Exit(1)
		}
		if *openFlag {
			web.Open(url)
		}
		srv := web.New(cfg, path)
		srv.SetToken(tok)
		cleanupDescriptor, err := service.Publish(service.Descriptor{
			Protocol: web.APIProtocol, Instance: srv.InstanceID(), Address: url,
			Token: tok, PID: os.Getpid(), StartedAt: time.Now().UTC(),
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "gregal serve: descriptor:", err)
			os.Exit(1)
		}
		defer cleanupDescriptor()
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
func runEvals(cfg *config.Config, dir, tasks string, maxSteps int, desaBaseline bool) int {
	llista, err := evals.Load(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gregal:", err)
		return 2
	}
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
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
		MaxSteps: maxSteps, Names: noms,
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

// Amb verifyCmd no buit, el torn no es dona per bo fins que la comanda
// passa (gate mecànic: fins a verifyRounds rondes de reparació).
func runHeadless(cfg *config.Config, task, output string, autoApprove bool, maxSteps int, mode, verifyCmd string, verifyRounds int) {
	if mode == "" {
		mode = cfg.Mode
	}
	if maxSteps == 0 {
		maxSteps = cfg.Agent.MaxSteps
	}
	mcp.Setup(toMCPServers(cfg))
	agent.SetPostEditHook(cfg.Hooks.PostEdit)
	agent.SetDiagMode(cfg.Hooks.Diag)
	agent.SetupPrices(cfg)
	cli := llm.New()
	cfg.ConfigureClient(cli)
	agent.SetupDelegate(cfg, cli)
	cwd, _ := os.Getwd()
	res, err := agent.RunAmbVerificacio(context.Background(), cli, cfg, task, mode, maxSteps, autoApprove,
		func(step int, tools []string) {
			if len(tools) == 0 {
				fmt.Fprintf(os.Stderr, "pas %d: responent…\n", step)
			} else {
				fmt.Fprintf(os.Stderr, "pas %d: %s\n", step, strings.Join(tools, ", "))
			}
		}, cwd, verifyCmd, verifyRounds)
	if res.RouteWhy != "" {
		fmt.Fprintf(os.Stderr, "🔀 %s (%s → %s)\n", res.RouteWhy, res.RoutedFrom, res.RoutedTo)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gregal:", err)
		os.Exit(1)
	}
	if output == "json" {
		raw, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(raw))
		return
	}
	if output != "text" {
		fmt.Fprintln(os.Stderr, "gregal: output ha de ser text|json")
		os.Exit(2)
	}
	fmt.Println(res.Answer)
	if res.FallbackModel != "" {
		fmt.Fprintf(os.Stderr, "↪ primari caigut: ha respost el fallback %s\n", res.FallbackModel)
	}
	if res.CostUSD != nil {
		fmt.Fprintf(os.Stderr, "cost estimat: ~%s (%d↑ %d↓)\n", agent.FmtCost(*res.CostUSD), res.UpTokens, res.DownTokens)
	}
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
	out := map[string]mcp.Server{}
	for name, s := range cfg.MCP {
		out[name] = mcp.Server{Command: s.Command, Args: s.Args, Env: s.Env}
	}
	return out
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
