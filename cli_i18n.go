package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"gregal/internal/config"
)

type cliMessage struct {
	en string
	ca string
}

var cliMessages = map[string]cliMessage{
	"config.missing":             {"does not exist (to create one: gregal init --config=%s)", "no existeix (per crear-ne un: gregal init --config=%s)"},
	"config.load_failed":         {"could not load configuration: %s", "no s'ha pogut carregar la configuració: %s"},
	"config.valid":               {"configuration valid: %s\nroles: %s", "configuració vàlida: %s\nrols: %s"},
	"init.unknown_argument":      {"unknown argument: %s", "argument desconegut: %s"},
	"init.create_failed":         {"could not create configuration: %s", "no s'ha pogut crear la configuració: %s"},
	"init.invalid_generated":     {"generated configuration is invalid: %s", "el config generat no valida: %s"},
	"init.agents_failed":         {"could not create AGENTS.md: %s", "no s'ha pogut crear AGENTS.md: %s"},
	"init.config_path":           {"configuration: %s", "config: %s"},
	"init.agents_created":        {"AGENTS.md: created", "AGENTS.md: creat"},
	"init.agents_exists":         {"AGENTS.md: already exists (left unchanged)", "AGENTS.md: ja existeix (no tocat)"},
	"config.usage":               {"usage: gregal config [list|set-key]", "ús: gregal config [list|set-key]"},
	"config.path":                {"configuration: %s", "config: %s"},
	"config.key_label":           {"key", "clau"},
	"config.no_terminal":         {"no terminal: use `gregal config set-key <provider> --stdin [--persist] < key.txt`", "sense terminal: usa `gregal config set-key <provider> --stdin [--persist] < clau.txt`"},
	"config.choose_provider":     {"choose provider (name or number, Enter = exit): ", "tria provider (nom o número, Enter = sortir): "},
	"config.provider_missing":    {"does not exist: %s", "no existeix: %s"},
	"config.key_exists":          {"%s already has a key (%s). Change it? [y/N]: ", "%s ja té clau (%s). La canviem? [s/N]: "},
	"config.paste_key":           {"paste the key for %s (hidden): ", "enganxa la clau de %s (no es mostra): "},
	"config.cancelled":           {"cancelled.", "cancel·lat."},
	"config.test_connection":     {"testing connection... ", "prova la connexió… "},
	"config.save_anyway":         {"save it anyway? [y/N]: ", "desa-la igualment? [s/N]: "},
	"config.response_models":     {"responds (%d models", "respon (%d models"},
	"config.save_failed":         {"could not save: %s", "no s'ha pogut desar: %s"},
	"config.key_saved":           {"key for %s saved to %s (permanent)", "clau de %s desada a %s (permanent)"},
	"config.set_key_usage":       {"usage: gregal config set-key <provider> <key|--stdin> [--persist]", "ús: gregal config set-key <provider> <clau|--stdin> [--persist]"},
	"config.empty_key":           {"empty key: cancelled.", "clau buida: cancel·lat."},
	"config.probe_warning":       {"warning: no response (%s) - saving anyway with --persist.", "avís: no respon (%s) — es desa igualment amb --persist."},
	"config.models_response":     {"responds (%d models)", "respon (%d models)"},
	"config.active_process":      {"active only in this process (add --persist to save it to disk).", "activa només en aquest procés (afegeix --persist per desar-la al disc)."},
	"doctor.status_ok":           {"OK", "OK"},
	"doctor.status_failed":       {"FAIL", "NO"},
	"doctor.config":              {"configuration", "configuració"},
	"doctor.config_valid":        {"validation", "validació"},
	"doctor.config_invalid":      {"failed: %s", "fallida: %s"},
	"doctor.roles_consistent":    {"passed: roles, router, verification, and budget are consistent", "correcte: rols, router, verify i budget coherents"},
	"doctor.directory":           {"directory", "directori"},
	"doctor.directory_missing":   {"does not exist (created on first use)", "no existeix (es crea al primer ús)"},
	"doctor.permissions":         {"permissions %s (should be rwx------)", "permisos %s (hauria de ser rwx------)"},
	"doctor.provider":            {"provider", "proveïdor"},
	"doctor.no_base_url":         {"no baseURL configured", "sense baseURL"},
	"doctor.unreachable":         {"unreachable (%s)", "inabastable (%s)"},
	"doctor.responds":            {"responds (%s)", "respon (%s)"},
	"doctor.window":              {"context window", "finestra"},
	"doctor.source_config":       {"config", "config"},
	"doctor.source_model":        {"model", "model"},
	"doctor.default_source":      {"default", "defecte"},
	"doctor.default_window_hint": {" - the provider does not declare it; set context_window on the role if known", " — el proveïdor no la declara; posa context_window al rol si la saps"},
	"doctor.browser":             {"browser", "navegador"},
	"doctor.browser_missing":     {"no Chrome/Edge found: the browser tool will not work (set GREGAL_BROWSER)", "cap Chrome/Edge trobat: l'eina browser no funcionarà (defineix GREGAL_BROWSER)"},
	"doctor.web":                 {"web search", "web"},
	"doctor.no_search_engine":    {"no search engine responds (%s)", "cap motor de cerca respon (%s)"},
	"doctor.search_via":          {"search via %s", "cerca via %s"},
	"doctor.no_response":         {" (no response: %s)", " (sense resposta: %s)"},
	"doctor.binary":              {"binary", "binari"},
	"doctor.failed_checks":       {"%d check(s) failed", "%d comprovació(ns) en vermell"},
	"doctor.all_green":           {"all checks passed", "tot verd"},
}

func cliText(lang, key string, args ...any) string {
	m, ok := cliMessages[key]
	if !ok {
		return key
	}
	text := m.en
	if lang == "ca" {
		text = m.ca
	}
	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}

func cliLanguage(cfg *config.Config) string {
	if cfg == nil {
		return "en"
	}
	return cfg.Lang()
}

// configLanguage reads only the language setting so user-facing load errors
// can still follow a valid `lang` value when the rest of the config is invalid.
func configLanguage(path string) string {
	if strings.TrimSpace(path) == "" {
		path = config.DefaultPath()
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "en"
	}
	var language struct {
		Language string `yaml:"lang"`
	}
	if err := yaml.Unmarshal(raw, &language); err != nil {
		return "en"
	}
	if language.Language == "ca" {
		return "ca"
	}
	return "en"
}
