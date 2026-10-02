package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/tools"
)

// doctor comprova la salut local: config, proveïdors, memòria i binari.
// Cada línia diu OK o NO; el codi de sortida és 1 si alguna falla.
// No envia cap secret: només fa GET anònims curts als baseURL.
func doctor(cfg *config.Config, path string) int {
	lang := cliLanguage(cfg)
	text := func(key string, args ...any) string { return cliText(lang, key, args...) }
	bad := 0
	ok := func(name, detail string) { fmt.Printf("%-4s%-12s %s\n", text("doctor.status_ok"), name, detail) }
	no := func(name, detail string) {
		bad++
		fmt.Printf("%-4s%-12s %s\n", text("doctor.status_failed"), name, detail)
	}

	ok(text("doctor.config"), path+" ("+cfg.RoleNames()+")")
	if err := cfg.Validate(); err != nil {
		no(text("doctor.config_valid"), text("doctor.config_invalid", err))
	} else {
		ok(text("doctor.config_valid"), text("doctor.roles_consistent"))
	}

	home, _ := os.UserHomeDir()
	mem := filepath.Join(home, ".config", "gregal")
	if fi, err := os.Stat(mem); err != nil || !fi.IsDir() {
		no(text("doctor.directory"), mem+": "+text("doctor.directory_missing"))
	} else if runtime.GOOS != "windows" && fi.Mode().Perm()&0o077 != 0 {
		no(text("doctor.directory"), mem+": "+text("doctor.permissions", fi.Mode().String()))
	} else {
		ok(text("doctor.directory"), mem)
	}

	seen := map[string]bool{}
	client := &http.Client{Timeout: 8 * time.Second}
	for _, r := range cfg.Roles {
		if seen[r.Provider] {
			continue
		}
		seen[r.Provider] = true
		p, okp := cfg.Providers[r.Provider]
		if !okp || p.BaseURL == "" {
			no(text("doctor.provider"), r.Provider+": "+text("doctor.no_base_url"))
			continue
		}
		resp, err := client.Get(p.BaseURL)
		if err != nil {
			no(text("doctor.provider"), r.Provider+": "+text("doctor.unreachable", shortErr(err)))
			continue
		}
		resp.Body.Close()
		ok(text("doctor.provider"), r.Provider+": "+text("doctor.responds", p.BaseURL))
	}

	// Finestra de context per rol: del config, del model o per defecte.
	dctx, dcancel := context.WithTimeout(context.Background(), 6*time.Second)
	agent.DetectWindows(dctx, cfg)
	dcancel()
	for _, name := range strings.Split(cfg.RoleNames(), ", ") {
		r, okr := cfg.Roles[strings.TrimSpace(name)]
		if !okr {
			continue
		}
		n, src := agent.WindowSource(cfg, r)
		source := src
		switch src {
		case "config":
			source = text("doctor.source_config")
		case "model":
			source = text("doctor.source_model")
		case "defecte":
			source = text("doctor.default_source")
		}
		detall := fmt.Sprintf("%s: %s tokens (%s)", strings.TrimSpace(name), llm.FmtCount(n), source)
		if src == "defecte" {
			detall += text("doctor.default_window_hint")
		}
		ok(text("doctor.window"), detall)
	}

	// Navegador per a l'eina browser (Chrome/Edge de la màquina).
	if exe := tools.BrowserExecutable(); exe != "" {
		ok(text("doctor.browser"), exe)
	} else {
		no(text("doctor.browser"), text("doctor.browser_missing"))
	}

	// Cerca web: quins motors responen (sense cap servei local).
	wctx, wcancel := context.WithTimeout(context.Background(), 15*time.Second)
	okE, koE := tools.WebEnginesDisponibles(wctx)
	wcancel()
	if len(okE) == 0 {
		no(text("doctor.web"), text("doctor.no_search_engine", strings.Join(koE, ", ")))
	} else {
		detall := text("doctor.search_via", strings.Join(okE, "+"))
		if len(koE) > 0 {
			detall += text("doctor.no_response", strings.Join(koE, ", "))
		}
		ok(text("doctor.web"), detall)
	}

	ok(text("doctor.binary"), "gregal "+version)
	if bad > 0 {
		fmt.Println(text("doctor.failed_checks", bad))
		return 1
	}
	fmt.Println(text("doctor.all_green"))
	return 0
}

func shortErr(err error) string {
	s := err.Error()
	if len(s) > 80 {
		s = s[:80] + "…"
	}
	return s
}
