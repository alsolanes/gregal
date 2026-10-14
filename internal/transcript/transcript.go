package transcript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gregal/internal/agent"
	"gregal/internal/llm"
)

// Kind és el tipus d'un bloc del document.
type Kind int

const (
	// KindUser és el que va escriure la persona.
	KindUser Kind = iota
	// KindAnswer és una resposta final de l'assistent.
	KindAnswer
	// KindWork és un grup de feina: notes intermèdies i crides d'eina.
	KindWork
)

// Block és una peça del document, en l'ordre de la conversa.
type Block struct {
	Kind Kind
	Text string  // KindUser, KindAnswer
	Work []*Step // KindWork
}

// Step és una passa d'un grup de feina: o una nota de l'assistent o una
// crida d'eina amb el seu resultat.
type Step struct {
	Note string
	Tool *ToolCall
}

// ToolCall és una crida d'eina tal com es va fer.
type ToolCall struct {
	Name      string
	Args      string
	Result    string
	HasResult bool
}

// ToolCount compta les crides d'eina d'un grup.
func (b Block) ToolCount() int {
	n := 0
	for _, s := range b.Work {
		if s.Tool != nil {
			n++
		}
	}
	return n
}

// Meta és la capçalera del document.
type Meta struct {
	Title     string
	Workspace string
	SavedAt   time.Time
}

// Options controla el render.
type Options struct {
	// Redact activa la sanitització amb Redaction.
	Redact    bool
	Redaction RedactOptions
	// MaxResult i MaxArgs retallen resultats i arguments d'eina (runes).
	// 0 = valors per defecte.
	MaxResult int
	MaxArgs   int
	// RenderMarkdown converteix el text en HTML segur (només per a HTML).
	// nil = text escapat amb salts de línia conservats.
	RenderMarkdown func(string) string
	// Now és la data de l'exportació (tests); zero = ara.
	Now time.Time
}

const (
	defMaxResult = 2000
	defMaxArgs   = 4000
)

// eines que la UI no pinta: la checklist té panell propi.
var einesAmagades = map[string]bool{"todowrite": true, "todoread": true}

// prefixosInterns són missatges d'usuari que no va escriure la persona: avisos
// que el bucle injecta al model, resums de compactació i checkpoints.
var prefixosInterns = []string{
	agent.GuiaEinaText,
	agent.GuiaTodosPendents,
	agent.GuiaVerificacioFallida,
	agent.GuiaFracasRepetit,
	agent.DoomGuide,
	agent.PrefixErrorServidor,
	agent.FinalPrompt,
	"A stop hook asked you to keep working",
	"[Summary of the work done so far in this turn",
	"[Task in progress",
	"[Summary of the earlier conversation]",
	"Summary of the context of earlier tasks:",
	"AUTONOMOUS CHECKPOINT:",
}

// marquesContext són els blocs que s'enganxen al final del missatge de la
// tasca i són per al model: el mapa del projecte i el context dels hooks.
var marquesContext = []string{agent.MarcaContext, agent.MarcaContextCA, "[hook context]"}

// CleanUser torna el que va escriure la persona: sense mapa del projecte ni
// context injectat. "" vol dir que el missatge sencer era intern.
func CleanUser(s string) string {
	s = agent.SenseMapa(s)
	for _, m := range marquesContext {
		if strings.HasPrefix(strings.TrimSpace(s), m) {
			return ""
		}
		if i := strings.Index(s, "\n\n"+m); i >= 0 {
			s = s[:i]
		}
		if i := strings.Index(s, "\n"+m); i >= 0 {
			s = s[:i]
		}
	}
	t := strings.TrimSpace(s)
	for _, p := range prefixosInterns {
		if p != "" && strings.HasPrefix(t, strings.TrimSpace(p)) {
			return ""
		}
	}
	return t
}

// Build agrupa la conversa com la pinta la UI en reprendre-la: missatge de
// la persona, grup de feina plegat (notes i eines) i resposta final. Els
// missatges de sistema i el context intern no hi surten.
func Build(convo []llm.Message) []Block {
	var out []Block
	var cur *Block
	calls := map[string]*ToolCall{}
	tanca := func() {
		if cur != nil && len(cur.Work) > 0 {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, m := range convo {
		switch m.Role {
		case "user":
			t := CleanUser(m.Content)
			if t == "" && len(m.Images) == 0 {
				// Buit o intern: ni es pinta ni talla el grup en curs.
				continue
			}
			if len(m.Images) > 0 {
				t = strings.TrimSpace(t + "\n\n" + strings.Repeat("[image] ", len(m.Images)))
			}
			tanca()
			out = append(out, Block{Kind: KindUser, Text: t})
		case "assistant":
			text := strings.TrimSpace(m.Content)
			if len(m.ToolCalls) == 0 {
				tanca()
				if text != "" {
					out = append(out, Block{Kind: KindAnswer, Text: text})
				}
				continue
			}
			if cur == nil {
				cur = &Block{Kind: KindWork}
			}
			if text != "" {
				cur.Work = append(cur.Work, &Step{Note: text})
			}
			for _, c := range m.ToolCalls {
				if einesAmagades[c.Function.Name] {
					continue
				}
				tc := &ToolCall{Name: c.Function.Name, Args: c.Function.Arguments}
				cur.Work = append(cur.Work, &Step{Tool: tc})
				if c.ID != "" {
					calls[c.ID] = tc
				}
			}
		case "tool":
			if tc, ok := calls[m.ToolCallID]; ok {
				tc.Result, tc.HasResult = m.Content, true
				delete(calls, m.ToolCallID)
			}
		}
	}
	tanca()
	return out
}

// prepare aplica la sanitització i els retalls a una còpia dels blocs.
func prepare(blocks []Block, meta Meta, o Options) ([]Block, Meta) {
	if o.MaxResult <= 0 {
		o.MaxResult = defMaxResult
	}
	if o.MaxArgs <= 0 {
		o.MaxArgs = defMaxArgs
	}
	red := func(s string) string { return s }
	if o.Redact {
		r := NewRedactor(o.Redaction)
		red = r.Redact
	}
	meta.Title, meta.Workspace = red(meta.Title), red(meta.Workspace)
	out := make([]Block, len(blocks))
	for i, b := range blocks {
		nb := Block{Kind: b.Kind, Text: red(b.Text)}
		for _, s := range b.Work {
			ns := &Step{Note: red(s.Note)}
			if s.Tool != nil {
				// Primer es sanitza i després es retalla: un tall a mig secret
				// el faria irreconeixible per als patrons.
				ns.Tool = &ToolCall{
					Name:      s.Tool.Name,
					Args:      truncate(prettyJSON(red(s.Tool.Args)), o.MaxArgs),
					Result:    truncate(red(s.Tool.Result), o.MaxResult),
					HasResult: s.Tool.HasResult,
				}
			}
			nb.Work = append(nb.Work, ns)
		}
		out[i] = nb
	}
	return out, meta
}

func prettyJSON(s string) string {
	t := strings.TrimSpace(s)
	if t == "" || (t[0] != '{' && t[0] != '[') {
		return s
	}
	var b bytes.Buffer
	if err := json.Indent(&b, []byte(t), "", "  "); err != nil {
		return s
	}
	return b.String()
}

func truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + fmt.Sprintf("\n… [truncated, %d more characters]", len(r)-max)
}

func titleOf(m Meta) string {
	if t := strings.TrimSpace(m.Title); t != "" {
		return t
	}
	return "Gregal conversation"
}

func stamp(o Options) string {
	n := o.Now
	if n.IsZero() {
		n = time.Now()
	}
	return n.Format("2006-01-02 15:04")
}

func workLabel(b Block) string {
	n := b.ToolCount()
	if n == 1 {
		return "Work · 1 tool call"
	}
	return fmt.Sprintf("Work · %d tool calls", n)
}

// reBackticks troba les tirades de cometes inverses per triar una tanca
// de codi més llarga que qualsevol de les del contingut.
var reBackticks = regexp.MustCompile("`+")

func fence(s string) string {
	n := 3
	for _, m := range reBackticks.FindAllString(s, -1) {
		if len(m) >= n {
			n = len(m) + 1
		}
	}
	return strings.Repeat("`", n)
}

func codeBlock(lang, s string) string {
	f := fence(s)
	return f + lang + "\n" + strings.TrimRight(s, "\n") + "\n" + f + "\n"
}

// Markdown renderitza la conversa en Markdown (amb <details> per als
// grups de feina, que GitHub i la majoria de visors entenen).
func Markdown(meta Meta, convo []llm.Message, o Options) string {
	blocks, meta := prepare(Build(convo), meta, o)
	var b strings.Builder
	b.WriteString("# " + oneLine(titleOf(meta)) + "\n\n")
	b.WriteString("_Exported from Gregal · " + stamp(o))
	if meta.Workspace != "" {
		b.WriteString(" · " + oneLine(meta.Workspace))
	}
	if o.Redact {
		b.WriteString(" · sanitized")
	}
	b.WriteString("_\n\n")
	for _, bl := range blocks {
		switch bl.Kind {
		case KindUser:
			b.WriteString("## User\n\n" + bl.Text + "\n\n")
		case KindAnswer:
			b.WriteString("## Assistant\n\n" + bl.Text + "\n\n")
		case KindWork:
			b.WriteString("<details>\n<summary>" + workLabel(bl) + "</summary>\n\n")
			for _, s := range bl.Work {
				if s.Tool == nil {
					b.WriteString(s.Note + "\n\n")
					continue
				}
				b.WriteString("**Tool:** `" + strings.ReplaceAll(s.Tool.Name, "`", "") + "`\n\n")
				if strings.TrimSpace(s.Tool.Args) != "" {
					b.WriteString(codeBlock("json", s.Tool.Args) + "\n")
				}
				if s.Tool.HasResult {
					b.WriteString("Result:\n\n" + codeBlock("text", s.Tool.Result) + "\n")
				} else {
					b.WriteString("_(no result: interrupted)_\n\n")
				}
			}
			b.WriteString("</details>\n\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// htmlCSS és tot l'estil del document: inline, sense fonts ni recursos de
// fora, perquè el fitxer s'obri igual sense xarxa.
const htmlCSS = `:root{color-scheme:light dark;--bg:#fbfbf9;--ink:#1d2321;--muted:#5d6864;--line:#dde3e0;--user:#eef5f1;--code:#f2f4f3;--accent:#1f7a5c}
@media (prefers-color-scheme:dark){:root{--bg:#141917;--ink:#e6ece9;--muted:#9aa7a2;--line:#2b3431;--user:#1c2622;--code:#1b2120;--accent:#6fd0a6}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--ink);font:15px/1.55 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
main{max-width:860px;margin:0 auto;padding:32px 20px 64px}h1{font-size:1.5em;margin:0 0 4px}.meta{color:var(--muted);font-size:.85em;margin-bottom:28px}
.msg{margin:18px 0}.who{font-size:.75em;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:var(--muted);margin-bottom:4px}
.user .body{background:var(--user);border:1px solid var(--line);border-radius:12px;padding:10px 14px}
.text{white-space:pre-wrap;word-wrap:break-word;margin:0;font:inherit}
details.work{border:1px solid var(--line);border-radius:10px;margin:12px 0;padding:6px 12px}details.work>summary{cursor:pointer;color:var(--muted);font-size:.9em}
.note{margin:10px 0;color:var(--muted)}.tool{margin:10px 0;border-left:3px solid var(--accent);padding-left:10px}.tname{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-weight:600;font-size:.9em}
pre{background:var(--code);border:1px solid var(--line);border-radius:8px;padding:8px 10px;overflow:auto;font:12.5px/1.45 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;white-space:pre-wrap;word-wrap:break-word}
code{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace}table{border-collapse:collapse}td,th{border:1px solid var(--line);padding:4px 8px}a{color:var(--accent)}
.empty{color:var(--muted);font-style:italic}`

// HTMLCSP és la política del document: cap script ni recurs extern.
const HTMLCSP = "default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// HTML renderitza un document HTML autocontingut (CSS inline, cap script ni
// recurs de fora). Tot el text passa per html.EscapeString o pel
// RenderMarkdown injectat, que ha de tornar HTML segur.
func HTML(meta Meta, convo []llm.Message, o Options) string {
	blocks, meta := prepare(Build(convo), meta, o)
	esc := html.EscapeString
	body := func(s string) string {
		if o.RenderMarkdown != nil {
			return "<div class=\"md\">" + o.RenderMarkdown(s) + "</div>"
		}
		return "<pre class=\"text\">" + esc(s) + "</pre>"
	}
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<meta http-equiv=\"Content-Security-Policy\" content=\"" + HTMLCSP + "\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<meta name=\"robots\" content=\"noindex\">\n")
	b.WriteString("<title>" + esc(oneLine(titleOf(meta))) + "</title>\n<style>" + htmlCSS + "</style>\n</head>\n<body>\n<main>\n")
	b.WriteString("<h1>" + esc(oneLine(titleOf(meta))) + "</h1>\n<div class=\"meta\">Exported from Gregal · " + esc(stamp(o)))
	if meta.Workspace != "" {
		b.WriteString(" · " + esc(oneLine(meta.Workspace)))
	}
	if o.Redact {
		b.WriteString(" · sanitized")
	}
	b.WriteString("</div>\n")
	if len(blocks) == 0 {
		b.WriteString("<p class=\"empty\">Empty conversation.</p>\n")
	}
	for _, bl := range blocks {
		switch bl.Kind {
		case KindUser:
			b.WriteString("<section class=\"msg user\"><div class=\"who\">User</div><div class=\"body\">" + body(bl.Text) + "</div></section>\n")
		case KindAnswer:
			b.WriteString("<section class=\"msg assistant\"><div class=\"who\">Assistant</div><div class=\"body\">" + body(bl.Text) + "</div></section>\n")
		case KindWork:
			b.WriteString("<details class=\"work\"><summary>" + esc(workLabel(bl)) + "</summary>\n")
			for _, s := range bl.Work {
				if s.Tool == nil {
					b.WriteString("<div class=\"note\">" + body(s.Note) + "</div>\n")
					continue
				}
				b.WriteString("<div class=\"tool\"><div class=\"tname\">" + esc(s.Tool.Name) + "</div>")
				if strings.TrimSpace(s.Tool.Args) != "" {
					b.WriteString("<pre class=\"args\">" + esc(s.Tool.Args) + "</pre>")
				}
				if s.Tool.HasResult {
					b.WriteString("<details><summary>Result</summary><pre class=\"result\">" + esc(s.Tool.Result) + "</pre></details>")
				} else {
					b.WriteString("<div class=\"empty\">no result: interrupted</div>")
				}
				b.WriteString("</div>\n")
			}
			b.WriteString("</details>\n")
		}
	}
	b.WriteString("</main>\n</body>\n</html>\n")
	return b.String()
}

// Filename proposa un nom de fitxer a partir del títol: minúscules,
// guions i l'extensió del format.
func Filename(title, ext string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
		if b.Len() >= 60 {
			break
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		s = "conversation"
	}
	return "gregal-" + s + "." + strings.TrimPrefix(ext, ".")
}
