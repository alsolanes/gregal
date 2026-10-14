package web

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"gregal/internal/llm"
	"gregal/internal/session"
	"gregal/internal/tema"
	"gregal/internal/transcript"
)

// Exportació de converses (Markdown o HTML autocontingut).
//
// GET /api/export                 → la conversa viva de la pestanya
// GET /api/sessions/{name}/export → una conversa desada
//
// Paràmetres: format=md|html (md per defecte), redact=1 (sanitza claus,
// secrets del config i camins personals) i emails=1 (amaga també els
// correus; només amb redact). La resposta és una descàrrega
// (Content-Disposition: attachment).

// redactOptions recull els secrets que aquest servidor coneix: els del
// config (providers, Telegram, MCP), les variables d'entorn amb nom de
// secret i el token de l'API.
func (s *Server) redactOptions(emails bool) transcript.RedactOptions {
	secrets := transcript.SecretsFromConfig(s.cfg)
	if t := strings.TrimSpace(s.Token); t != "" {
		secrets = append(secrets, t)
	}
	return transcript.RedactOptions{Secrets: secrets, Emails: emails, HomeDirs: transcript.DefaultHomeDirs()}
}

// exportMarkdownHTML és el renderitzador de markdown del web (goldmark +
// bluemonday) amb el tema clar: el document exportat es llegeix fora.
func exportMarkdownHTML(text string) string { return Markdown(text, tema.Per("clar")) }

func boolParam(r *http.Request, k string) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(k))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// renderExport genera el document i en torna el cos, el tipus i l'extensió.
func (s *Server) renderExport(format string, meta transcript.Meta, convo []llm.Message, redact, emails bool) (body, ctype, ext string, ok bool) {
	opt := transcript.Options{Redact: redact}
	if redact {
		opt.Redaction = s.redactOptions(emails)
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "md", "markdown":
		return transcript.Markdown(meta, convo, opt), "text/markdown; charset=utf-8", "md", true
	case "html":
		opt.RenderMarkdown = exportMarkdownHTML
		return transcript.HTML(meta, convo, opt), "text/html; charset=utf-8", "html", true
	}
	return "", "", "", false
}

func (s *Server) writeExport(w http.ResponseWriter, r *http.Request, meta transcript.Meta, convo []llm.Message) {
	body, ctype, ext, ok := s.renderExport(r.URL.Query().Get("format"), meta, convo, boolParam(r, "redact"), boolParam(r, "emails"))
	if !ok {
		http.Error(w, "format must be md or html", http.StatusBadRequest)
		return
	}
	name := transcript.Filename(meta.Title, ext)
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"; filename*=UTF-8''`+url.PathEscape(name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(body))
}

// liveExport torna la conversa viva de la pestanya amb la seva capçalera.
func (s *Server) liveExport() (transcript.Meta, []llm.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	convo := append([]llm.Message(nil), s.convo...)
	title := session.TitleFrom(convo)
	if title == "" {
		title = s.title
	}
	return transcript.Meta{Title: title, Workspace: s.cwd, SavedAt: time.Now()}, convo
}

// savedExport carrega una conversa desada de l'usuari.
func (s *Server) savedExport(name string) (transcript.Meta, []llm.Message, error) {
	sess, err := session.Load(s.sessDir(), name)
	if err != nil {
		return transcript.Meta{}, nil, err
	}
	title := sess.Title
	if title == "" {
		title = session.TitleFrom(sess.Convo)
	}
	return transcript.Meta{Title: title, Workspace: sess.Workspace, SavedAt: sess.SavedAt}, sess.Convo, nil
}

func (s *Server) handleExportCurrent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	meta, convo := s.liveExport()
	s.writeExport(w, r, meta, convo)
}

func (s *Server) handleExportSaved(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	meta, convo, err := s.savedExport(r.PathValue("name"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	s.writeExport(w, r, meta, convo)
}
