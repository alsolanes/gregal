package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
	"gregal/internal/transcript"
)

// exportArgs és el que es pot dir a /export: format (md per defecte, html),
// un camí opcional i --redact / --emails.
type exportArgs struct {
	format string
	path   string
	redact bool
	emails bool
}

func parseExportArgs(args []string) (exportArgs, error) {
	a := exportArgs{format: "md"}
	for _, x := range args {
		switch strings.ToLower(x) {
		case "md", "markdown":
			a.format = "md"
		case "html":
			a.format = "html"
		case "--redact", "-r":
			a.redact = true
		case "--emails":
			a.redact, a.emails = true, true
		default:
			if strings.HasPrefix(x, "-") {
				return a, fmt.Errorf("unknown option %s", x)
			}
			if a.path != "" {
				return a, errors.New("only one path")
			}
			a.path = x
		}
	}
	return a, nil
}

// exportConversa escriu la conversa a un fitxer i en torna el camí. Sense
// camí, va al directori de treball amb un nom tret del títol i l'hora (no
// trepitja mai una exportació anterior). Un camí relatiu és relatiu a cwd;
// si és una carpeta, el fitxer hi va a dins.
func exportConversa(convo []llm.Message, cwd string, cfg *config.Config, a exportArgs, now time.Time) (string, error) {
	if len(convo) == 0 {
		return "", errors.New("empty conversation")
	}
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	title := session.TitleFrom(convo)
	opt := transcript.Options{Redact: a.redact, Now: now}
	if a.redact {
		opt.Redaction = transcript.RedactOptions{Secrets: transcript.SecretsFromConfig(cfg), Emails: a.emails, HomeDirs: transcript.DefaultHomeDirs()}
	}
	meta := transcript.Meta{Title: title, Workspace: cwd, SavedAt: now}
	var body string
	if a.format == "html" {
		opt.Brand = transcript.ActiveBrand()
		body = transcript.HTML(meta, convo, opt)
	} else {
		body = transcript.Markdown(meta, convo, opt)
	}
	nom := transcript.Filename(title, a.format)
	nom = strings.TrimSuffix(nom, "."+a.format) + "-" + now.Format("20060102-150405") + "." + a.format
	path := a.path
	switch {
	case path == "":
		path = filepath.Join(cwd, nom)
	case !filepath.IsAbs(path):
		path = filepath.Join(cwd, path)
	}
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		path = filepath.Join(path, nom)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// exportCmd és l'ordre /export.
func (m Model) exportCmd(args []string) (tea.Model, tea.Cmd) {
	a, err := parseExportArgs(args)
	if err != nil {
		m.push(systemLine(T("app.usExport")))
		return m, nil
	}
	if len(m.convo) == 0 {
		m.push(systemLine(T("app.exportBuida")))
		return m, nil
	}
	path, err := exportConversa(m.convo, m.cwd, m.cfg, a, time.Now())
	if err != nil {
		m.push(systemLine("⚠ " + err.Error()))
		return m, nil
	}
	m.push(systemLine(fmt.Sprintf(T("app.exportat"), path)))
	return m, nil
}
