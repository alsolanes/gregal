package main

// gregal config — el camí oficial i fàcil per posar claus api.
//
//   gregal config            → assistent: tria provider, enganxa la clau
//                              (sense eco), la prova i la desa al disc.
//   gregal config list       → providers amb estat de clau.
//   gregal config set-key <provider> <clau|--stdin> [--persist]
//                            → no interactiu (scripts).
//
// Persistència: per defecte la clau queda activa en memòria del procés
// que la rep (TUI/app/serve). Aquest config la DESA al fitxer
// (~/.config/gregal/config.yaml, 0600) perquè "es quedi posada".
// ${VAR} continua sent l'opció sense secrets al disc.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"

	"gregal/internal/config"
	"gregal/internal/llm"
)

func runConfig(args []string) {
	cfgPath := ""
	rest := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			cfgPath = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	lang := configLanguage(cfgPath)
	if len(rest) >= 1 && rest[0] == "list" {
		cfg, path, err := config.Load(cfgPath)
		if err != nil {
			fatalCfg(err, configLanguage(path))
		}
		lang = cliLanguage(cfg)
		fmt.Println(cliText(lang, "config.path", path))
		fmt.Print(configList(cfg))
		return
	}
	if len(rest) >= 1 && rest[0] == "set-key" {
		runConfigSetKey(cfgPath, rest[1:])
		return
	}
	if len(rest) >= 1 {
		fmt.Fprintln(os.Stderr, cliText(lang, "config.usage"))
		os.Exit(2)
	}
	runConfigWizard(cfgPath)
}

func fatalCfg(err error, lang string) {
	fmt.Fprintf(os.Stderr, "config: %s\n", cliText(lang, "config.load_failed", err))
	os.Exit(1)
}

func configList(cfg *config.Config) string {
	names := make([]string, 0, len(cfg.Providers))
	for n := range cfg.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	usedBy := map[string][]string{}
	for rn, r := range cfg.Roles {
		usedBy[r.Provider] = append(usedBy[r.Provider], rn+"/"+r.Model)
	}
	for i, n := range names {
		p := cfg.Providers[n]
		line := fmt.Sprintf("  %d. %-12s %-32s %s: %s", i+1, n, p.BaseURL, cliText(cliLanguage(cfg), "config.key_label"), p.MaskedKey())
		if ub, ok := usedBy[n]; ok {
			sort.Strings(ub)
			line += "  ← " + strings.Join(ub, ", ")
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func runConfigWizard(cfgPath string) {
	lang := configLanguage(cfgPath)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, cliText(lang, "config.no_terminal"))
		os.Exit(2)
	}
	cfg, path, err := config.Load(cfgPath)
	if err != nil {
		fatalCfg(err, configLanguage(path))
	}
	lang = cliLanguage(cfg)
	fmt.Println(cliText(lang, "config.path", path))
	for {
		fmt.Print(configList(cfg))
		fmt.Print(cliText(lang, "config.choose_provider"))
		sel := readLine()
		if sel == "" {
			return
		}
		nom := resolveProvider(cfg, sel)
		if nom == "" {
			fmt.Println(cliText(lang, "config.provider_missing", sel))
			continue
		}
		p := cfg.Providers[nom]
		if strings.TrimSpace(p.APIKey) != "" {
			fmt.Print(cliText(lang, "config.key_exists", nom, p.MaskedKey()))
			if !readYes() {
				continue
			}
		}
		fmt.Print(cliText(lang, "config.paste_key", nom))
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
			fmt.Println(cliText(lang, "config.cancelled"))
			continue
		}
		key := strings.TrimSpace(string(raw))
		p.APIKey = key
		cfg.Providers[nom] = p
		fmt.Print(cliText(lang, "config.test_connection"))
		if ids, err := llm.ProbeModels(p.BaseURL, key); err != nil {
			fmt.Println("✗ " + err.Error())
			fmt.Print(cliText(lang, "config.save_anyway"))
			if !readYes() {
				p.APIKey = ""
				cfg.Providers[nom] = p
				continue
			}
		} else {
			fmt.Print("✓ ", cliText(lang, "config.response_models", len(ids)))
			if len(ids) > 0 {
				fmt.Printf(": %s", llm.ShortIDs(ids))
			}
			fmt.Println(")")
		}
		cfg.SetRawKey(nom, key)
		if err := cfg.Save(path); err != nil {
			fmt.Println(cliText(lang, "config.save_failed", err))
			continue
		}
		fmt.Printf("✓ %s\n", cliText(lang, "config.key_saved", nom, path))
	}
}

func runConfigSetKey(cfgPath string, args []string) {
	lang := configLanguage(cfgPath)
	persist := false
	useStdin := false
	pos := []string{}
	for _, a := range args {
		switch a {
		case "--persist":
			persist = true
		case "--stdin":
			useStdin = true
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) < 1 || (len(pos) < 2 && !useStdin) {
		fmt.Fprintln(os.Stderr, cliText(lang, "config.set_key_usage"))
		os.Exit(2)
	}
	nom := pos[0]
	key := ""
	if useStdin {
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 4096))
		if err != nil {
			fatalCfg(err, lang)
		}
		key = strings.TrimSpace(string(raw))
	} else {
		key = pos[1]
	}
	if key == "" {
		fmt.Fprintln(os.Stderr, cliText(lang, "config.empty_key"))
		os.Exit(2)
	}
	cfg, path, err := config.Load(cfgPath)
	if err != nil {
		fatalCfg(err, configLanguage(path))
	}
	lang = cliLanguage(cfg)
	p, ok := cfg.Providers[nom]
	if !ok {
		fmt.Fprintln(os.Stderr, cliText(lang, "config.provider_missing", nom))
		os.Exit(2)
	}
	p.APIKey = key
	cfg.Providers[nom] = p
	if ids, err := llm.ProbeModels(p.BaseURL, key); err != nil {
		fmt.Println(cliText(lang, "config.probe_warning", err))
	} else {
		fmt.Printf("%s\n", cliText(lang, "config.models_response", len(ids)))
	}
	if persist {
		cfg.SetRawKey(nom, key)
		if err := cfg.Save(path); err != nil {
			fatalCfg(err, lang)
		}
		fmt.Printf("✓ %s\n", cliText(lang, "config.key_saved", nom, path))
		return
	}
	fmt.Println(cliText(lang, "config.active_process"))
}

func resolveProvider(cfg *config.Config, sel string) string {
	sel = strings.TrimSpace(sel)
	if _, ok := cfg.Providers[sel]; ok {
		return sel
	}
	names := make([]string, 0, len(cfg.Providers))
	for n := range cfg.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	var idx int
	if _, err := fmt.Sscanf(sel, "%d", &idx); err == nil && idx >= 1 && idx <= len(names) {
		return names[idx-1]
	}
	return ""
}

func readLine() string {
	b, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimSpace(b)
}

func readYes() bool {
	s := strings.ToLower(readLine())
	return s == "s" || s == "sí" || s == "si" || s == "y" || s == "yes"
}
