// Package agent implementa l'agent loop: el model demana eines (format
// OpenAI, verificat contra llama.cpp local) i el loop les executa fins a
// resposta final o límit de passos.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// Specs retorna les eines natives exposades al model.
func Specs() []llm.ToolSpec {
	str := map[string]any{"type": "string"}
	return []llm.ToolSpec{
		{Name: "read", Description: "Llegeix un fitxer del disc. Torna línies numerades. Si necessites diversos fitxers, demana'ls tots en el mateix pas: les lectures van en paral·lel.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path":   str,
				"offset": map[string]any{"type": "integer"},
				"limit":  map[string]any{"type": "integer"},
			}, "required": []string{"path"}}},
		{Name: "bash", Description: "Executa una comanda shell amb timeout de 2 minuts. Torna la sortida. Compilar i passar els tests d'un paquet hi caben. El que és llarg de mena —engegar un servidor, la suite sencera— NO ho facis aquí: es talla a mitges. Fes servir bash_background.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"command": str,
			}, "required": []string{"command"}}},
		{Name: "write", Description: "Escriu un fitxer sencer amb el contingut donat: per a fitxers NOUS. Per canviar un fitxer que ja existeix, edit o patch: reescriure'l sencer és lent (el model torna a escriure cada línia) i hi pots perdre codi que no has llegit.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str, "content": str,
			}, "required": []string{"path", "content"}}},
		{Name: "edit", Description: "Substitueix UN bloc exacte (ha de ser únic) dins d'un fitxer. Si has de fer diversos canvis al mateix fitxer, fes servir patch: amb edits solts, si un falla el fitxer queda a mig canviar.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str, "old_string": str, "new_string": str,
			}, "required": []string{"path", "old_string", "new_string"}}},
		{Name: "grep", Description: "Cerca un text literal en fitxers sota un directori. Omet .git i binaris.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"pattern": str, "dir": str, "include": str,
			}, "required": []string{"pattern"}}},
		{Name: "glob", Description: "Llista fitxers que casen amb un pattern (estil *.go, dir/*.txt).",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"pattern": str, "dir": str,
			}, "required": []string{"pattern"}}},
		{Name: "web_search", Description: "Cerca a la web (diversos motors alhora, sense cap servei local). Torna títol+URL+resum per resultat; després obre els útils amb web_fetch. Diverses cerques o pàgines en un mateix pas van en paral·lel.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"query": str, "count": map[string]any{"type": "integer"},
			}, "required": []string{"query"}}},
		{Name: "web_fetch", Description: "Llegeix una pàgina http(s) i en torna el contingut principal en markdown (sense menús, anuncis ni imatges), amb títol, data i URL. Opcions: find=text per veure només els paràgrafs que el contenen (pàgines llargues), max_chars per ampliar el retall (fins a 30000), raw=true per la pàgina sencera en text pla. Hosts locals bloquejats.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"url":       str,
				"find":      str,
				"max_chars": map[string]any{"type": "integer"},
				"raw":       map[string]any{"type": "boolean"},
			}, "required": []string{"url"}}},
		{Name: "patch", Description: "Aplica diverses edicions a un fitxer de cop (atòmic: tot o res). Cada edició és old→new (bloc únic) o after_line+new (inserció).",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str,
				"edits": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
					"old":        str,
					"new":        str,
					"after_line": map[string]any{"type": "integer"},
				}}},
			}, "required": []string{"path", "edits"}}},
		{Name: "read_image", Description: "Llegeix una imatge del disc (png/jpg/webp/gif, màxim 8MB) i l'adjunta al missatge perquè el model la vegi.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str,
			}, "required": []string{"path"}}},
		{Name: "gh_issue", Description: "Consulta issues de GitHub amb la CLI gh (només lectura): action=list|view, number opcional, repo opcional (owner/name), query opcional.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"list", "view"}},
				"number": map[string]any{"type": "integer"},
				"query":  str, "repo": str,
			}, "required": []string{"action"}}},
		{Name: "gh_pr", Description: "Consulta pull requests de GitHub amb la CLI gh (només lectura): action=list|view|diff|checks, number obligatori excepte list, repo opcional.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"action": map[string]any{"type": "string", "enum": []string{"list", "view", "diff", "checks"}},
				"number": map[string]any{"type": "integer"},
				"query":  str, "repo": str,
			}, "required": []string{"action"}}},
		{Name: "office_read", Description: "Llegeix Word/Excel/PowerPoint (.docx/.xlsx/.pptx) i en torna el text o les dades (retallat). Formats antics .doc/.xls/.ppt NO suportats.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str,
			}, "required": []string{"path"}}},
		{Name: "office_open", Description: "Obre un document amb el programa associat del sistema (Word/Excel/PowerPoint). No bloqueja: torna de seguida. Mentre el document sigui obert no s'hi pot escriure amb office_edit (fitxer bloquejat): avisa l'usuari que el tanqui per continuar editant.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str,
			}, "required": []string{"path"}}},
		{Name: "bash_background", Description: "Engega una comanda llarga en segon pla (servidor, build, suite de tests) i torna un id. No espera: llegeix-ne la sortida amb bash_output i atura-la amb bash_kill. Fes-la servir quan la comanda pugui passar dels 2 minuts del bash normal.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"command": str,
			}, "required": []string{"command"}}},
		{Name: "bash_output", Description: "Llegeix la sortida nova d'un procés de segon pla (id de bash_background). Torna també si ja ha acabat i amb quin codi.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"id": str, "lines": map[string]any{"type": "integer"},
			}, "required": []string{"id"}}},
		{Name: "bash_kill", Description: "Atura un procés de segon pla pel seu id. Sense id, llista els processos vius.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"id": str,
			}}},
		{Name: "office_edit", Description: "Edita office (journal: /rewind ho desfà). op=replace substitueix text en docx/pptx (find, replace; text PLA, conserva el format del primer fragment). op=append afegeix paràgrafs al final d'un docx (o a l'última diapositiva d'un pptx) amb content: s'entén # títol, ## subtítol, - llista, **negreta**, *cursiva*. op=set_cell posa value a una cel·la xlsx (sheet, cell com B2). op=set_range omple un bloc xlsx des de cell (cantonada superior esquerra) amb content: una línia per fila, cel·les separades per |.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path":  str,
				"op":    map[string]any{"type": "string", "enum": []string{"set_cell", "set_range", "replace", "append"}},
				"sheet": str, "cell": str, "value": str, "find": str, "replace": str, "content": str,
			}, "required": []string{"path", "op"}}},
		{Name: "office_create", Description: "Crea un document office nou (.docx/.xlsx/.pptx) amb un títol i contingut inicial (text amb salts de línia; en xlsx cada línia és una fila i les cel·les se separen amb |). Escriu TEXT PLA: res de markdown. S'entén # títol, ## subtítol, - llista, **negreta**, *cursiva* i taules | a | b | (es converteixen a format de debò); la resta de marques es netegen. Si el fitxer ja existeix, falla.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"path": str, "title": str, "content": str,
			}, "required": []string{"path"}}},
		{Name: "question", Description: "Pregunta a l'usuari quan necessites una decisió per continuar (màxim 4 opcions curtes + text lliure permès). Fes-la servir en comptes d'endevinar requisits ambigus.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"query": str,
				"options": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
					"label": str, "description": str,
				}, "required": []string{"label"}}},
			}, "required": []string{"query", "options"}}},
		{Name: "todowrite", Description: "Publica la llista de passos de la tasca (màxim 20) perquè la UI mostri el progrés amb checks. Marca un sol pas com a working i la resta pending/done. Estats EXACTES: pending, working, done (res més).",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"items": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{
					"title": str, "status": map[string]any{"type": "string", "enum": []string{"pending", "working", "done"}},
				}, "required": []string{"title", "status"}}},
			}, "required": []string{"items"}}},
		{Name: "browser", Description: "Navegador real (Chrome/Edge de la màquina, perfil propi del gregal, finestra visible) per a pàgines amb JavaScript, apps locals (localhost) i llocs amb sessió iniciada. action=open (url) carrega i llegeix; read torna la pàgina en markdown + elements interactius numerats [n]; click (target: [n], selector CSS o text visible); type (target, text, enter=true per enviar); press (key: Enter|Tab|Escape|ArrowDown…); screenshot adjunta una captura; back; eval (js) executa JavaScript; close. Per a pàgines estàtiques web_fetch és més ràpid.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"action":    map[string]any{"type": "string", "enum": []string{"open", "read", "click", "type", "press", "screenshot", "back", "eval", "close"}},
				"url":       str,
				"target":    str,
				"text":      str,
				"key":       str,
				"js":        str,
				"find":      str,
				"enter":     map[string]any{"type": "boolean"},
				"raw":       map[string]any{"type": "boolean"},
				"max_chars": map[string]any{"type": "integer"},
			}, "required": []string{"action"}}},
		{Name: "todoread", Description: "Llegeix la llista de passos publicada (checks de progrés).",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "skill", Description: "Obre una skill pel nom i en torna el contingut sencer. Les skills són coneixement del gregal mateix (sessions, configuració, modes) i del projecte. L'índex amb els noms i de què va cadascuna és al teu prompt: obre la que toqui ABANS de dir que no saps una cosa o de deduir-la.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"nom": str,
			}, "required": []string{"nom"}}},
	}
}

// PolicyFor retorna allow|ask|deny amb la política per defecte.
func PolicyFor(name, argsJSON string) (string, string) {
	return DefaultPolicy().For(name, argsJSON)
}

// Policy és una política de permisos (del config o per defecte).
type Policy struct {
	Tools     map[string]string
	BashAllow []string
	BashDeny  []string
	// ProjectDir habilita el pas automàtic d'escriptures (write/edit) en
	// mode code quan la ruta queda dins. Buit = comportament clàssic (ask).
	ProjectDir string
}

// DefaultPolicy retorna la política base (read allow, bash classify, write/edit ask).
func DefaultPolicy() *Policy { return &Policy{} }

// Les eines conegudes viuen a tools.Noms(): un sol registre per a
// l'agent, la validació del config i el menú de permisos del TUI.

// isMCP diu si és una eina d'un servidor MCP (prefix mcp_).
func isMCP(name string) bool { return tools.EsMCP(name) }

// extraSpecs i extraExec són eines externes (MCP) registrades a l'arrencada.
var extraSpecs []llm.ToolSpec
var extraExec = map[string]func(argsJSON string) (string, error){}

// RegisterExtra registra una eina externa (idempotent per nom).
func RegisterExtra(spec llm.ToolSpec, fn func(argsJSON string) (string, error)) {
	for _, s := range extraSpecs {
		if s.Name == spec.Name {
			return
		}
	}
	extraSpecs = append(extraSpecs, spec)
	extraExec[spec.Name] = fn
}

// SpecsAll retorna les natives + delegate + les externes registrades.
func SpecsAll() []llm.ToolSpec {
	all := append(append([]llm.ToolSpec{}, Specs()...), delegateToolSpec())
	return append(all, extraSpecs...)
}

// ToolMsg construeix el missatge "tool" per l'historial. El contingut mai
// és buit: alguns providers (400) rebutgen tool sense 'content'.
// images (data URLs) viatgen com a parts image_url (Fase A v1.0).
func ToolMsg(c llm.ToolCall, out string, images ...string) llm.Message {
	if strings.TrimSpace(out) == "" {
		out = "(sense sortida)"
	}
	return llm.Message{Role: "tool", Content: out, Images: images, ToolCallID: c.ID, Name: c.Function.Name}
}

// For aplica overrides del config i després la base.
func (p *Policy) For(name, argsJSON string) (string, string) {
	if !tools.EsNativa(name) && !isMCP(name) {
		return "deny", "eina desconeguda: " + name
	}
	if p != nil {
		if d, ok := p.Tools[name]; ok {
			switch d {
			case "allow", "ask", "deny":
				return d, "permissions del config"
			}
		}
	}
	if isMCP(name) {
		return "ask", "eina MCP: confirma"
	}
	switch name {
	case "read", "grep", "glob", "web_search", "web_fetch", "delegate", "read_image", "gh_issue", "gh_pr", "office_read", "todoread", "question", "skill":
		return "allow", ""
	case "bash_output", "bash_kill":
		// Llegir o aturar un procés que ja s'ha aprovat no torna a preguntar.
		return "allow", ""
	case "browser":
		// Obrir, llegir i capturar és com web_fetch; clicar, escriure i
		// executar JavaScript actua sobre la pàgina i demana permís.
		var a struct {
			Action string `json:"action"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		if tools.BrowserActionSegura(a.Action) {
			return "allow", ""
		}
		return "ask", "navegador: actua sobre la pàgina (" + a.Action + ")"
	case "bash", "bash_background":
		var a struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "ask", "arguments il·legibles"
		}
		var allow, deny []string
		if p != nil {
			allow, deny = p.BashAllow, p.BashDeny
		}
		return tools.ClassifyWith(a.Command, allow, deny)
	default: // write, edit
		return "ask", "escriptura: confirma"
	}
}

// Decide aplica el mode i després la política.
// mode "chat" = només lectura (estil Codex read-only): write/edit denegats,
// bash només si és a la llista segura. mode "goal" (objectiu) també és només
// lectura: primer es concreta l'objectiu, després s'executa en mode code.
// mode "code" = política normal.
func (p *Policy) Decide(mode, name, argsJSON string) (string, string) {
	// Consulta és l'alternativa sense popups: permet totes les eines natives
	// de lectura i només shell classificat com a lectura. Qualsevol altra cosa
	// es denega explícitament (mai es converteix en una aprovació).
	if mode == ModeInspect {
		switch name {
		case "write", "edit", "patch":
			return "deny", "mode consulta: no modifica fitxers"
		case "bash_background", "office_open":
			return "deny", "mode consulta: no engega processos ni programes"
		case "browser":
			if d, _ := p.For(name, argsJSON); d == "allow" {
				return "allow", ""
			}
			return "deny", "mode consulta: el navegador només mira (open, read, screenshot)"
		case "bash":
			d, reason := p.For(name, argsJSON)
			if d == "allow" {
				return "allow", ""
			}
			return "deny", "mode consulta: comanda no és lectura segura (" + reason + ")"
		default:
			if isMCP(name) {
				return "deny", "mode consulta: MCP desactivat"
			}
		}
	}
	if mode == ModeChat || mode == ModeGoal {
		què := "mode xat"
		if mode == ModeGoal {
			què = "mode objectiu"
		}
		switch name {
		case "write", "edit", "patch":
			return "deny", què + ": només lectura (passa a /mode code per editar)"
		case "bash_background", "office_open":
			return "deny", què + ": només lectura (no engega processos ni programes)"
		case "browser":
			if d, _ := p.For(name, argsJSON); d == "allow" {
				return "allow", ""
			}
			return "deny", què + ": el navegador només mira (open, read, screenshot)"
		case "bash":
			if d, reason := p.For(name, argsJSON); d != "allow" {
				return "deny", què + ": només lectura (" + reason + ")"
			}
			return "allow", ""
		default:
			if isMCP(name) {
				return "deny", què + ": només lectura (MCP desactivat)"
			}
		}
	}
	// Mode code: escriure dins del projecte és la feina de l'agent i passa
	// sol (hi ha snapshot + git per desfer). Un override explícit del config
	// (allow/ask/deny) sempre guanya a l'automatisme. Fora del projecte,
	// bash no segur i MCP continuen demanant permís com sempre.
	if (mode == ModeCode || mode == ModeAutonomous) && (name == "write" || name == "edit" || name == "patch") {
		if p != nil {
			if d, ok := p.Tools[name]; ok {
				switch d {
				case "allow", "ask", "deny":
					return autopromou(mode, d), "permissions del config"
				}
			}
			if InsideProject(p.ProjectDir, toolPath(name, argsJSON)) {
				return "allow", ""
			}
		}
	}
	d, reason := p.For(name, argsJSON)
	return autopromou(mode, d), reason
}

// autopromou deixa treballar sol el mode autònom: cap diàleg d'aprovació no
// es pot quedar esperant, perquè no hi ha ningú mirant. Només promou el
// «ask» tou (una escriptura, una instal·lació, un reinici, un esborrat amb
// abast) cap a «allow». Un «deny» no es toca mai: ni el del config, ni el
// de la llista dura del classificador (sudo, mkfs, dd if=, shutdown,
// rm -rf /, curl, wget, ssh, chmod 777), ni el d'una altra porta de mode.
// El motiu passa tal qual: quan és un deny ha de dir per què.
func autopromou(mode, decisió string) string {
	if mode == ModeAutonomous && decisió == "ask" {
		return "allow"
	}
	return decisió
}

// Exec executa una eina amb l'etiqueta de sessió per defecte (TUI,
// headless, Telegram: allà només n'hi ha una).
func Exec(name, argsJSON string) (out string, images []string, err error) {
	return ExecSess(procSession, name, argsJSON)
}

// ExecSess és el mateix dient de quina sessió ve. Importa per als
// processos de segon pla: queden etiquetats amb ella, i és el que fa que
// surtin al Terminal de la pestanya que els ha engegat i que es matin en
// tancar-la. Abans tots duien "agent" i cap de les dues coses passava.
func ExecSess(session, name, argsJSON string) (out string, images []string, err error) {
	return ExecIn(session, "", name, argsJSON)
}

// ExecIn és el mateix dient també ON: el workspace de la sessió. Amb dir
// buit es treballa al directori del procés, que és el que volen el TUI i
// el headless. Vegeu resolArgs.
func ExecIn(session, dir, name, argsJSON string) (out string, images []string, err error) {
	return ExecCtx(context.Background(), session, dir, name, argsJSON)
}

// ExecCtx és ExecIn amb el context del torn. Cancel·lar-lo atura les
// eines que poden trigar (bash i el navegador); la resta són prou
// ràpides perquè no calgui. És el que fa que Esc talli de debò una
// comanda llarga en comptes de deixar-la corrent en segon pla.
func ExecCtx(ctx context.Context, session, dir, name, argsJSON string) (out string, images []string, err error) {
	if session == "" {
		session = procSession
	}
	argsJSON = resolArgs(dir, argsJSON)
	switch name {
	case "read":
		var a struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.Read(a.Path, a.Offset, a.Limit)
		if err != nil {
			return "", nil, err
		}
		return "llegit " + a.Path + "\n" + out, nil, nil
	case "office_read":
		var oa struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &oa); err != nil {
			return "", nil, err
		}
		oout, oerr := tools.OfficeRead(oa.Path)
		if oerr != nil {
			return "", nil, oerr
		}
		if len(oout) > 8000 {
			oout = oout[:8000] + "\n… (retallat)"
		}
		return "llegit " + oa.Path + "\n" + oout, nil, nil
	case "office_open":
		var oo struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &oo); err != nil {
			return "", nil, err
		}
		oout, oerr := tools.OfficeOpen(oo.Path)
		if oerr != nil {
			return "", nil, oerr
		}
		return oout, nil, nil
	case "office_edit":
		var oe struct {
			Path    string `json:"path"`
			Op      string `json:"op"`
			Sheet   string `json:"sheet"`
			Cell    string `json:"cell"`
			Value   string `json:"value"`
			Find    string `json:"find"`
			Replace string `json:"replace"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &oe); err != nil {
			return "", nil, err
		}
		switch oe.Op {
		case "set_cell":
			out, err := tools.OfficeXlsxSet(oe.Path, oe.Sheet, oe.Cell, oe.Value)
			if err != nil {
				return "", nil, err
			}
			return out, nil, nil
		case "replace":
			out, err := tools.OfficeReplace(oe.Path, oe.Find, oe.Replace)
			if err != nil {
				return "", nil, err
			}
			return out, nil, nil
		case "append":
			out, err := tools.OfficeAppend(oe.Path, firstOf(oe.Content, oe.Value, oe.Replace))
			if err != nil {
				return "", nil, err
			}
			return out, nil, nil
		case "set_range":
			out, err := tools.OfficeXlsxSetRange(oe.Path, oe.Sheet, oe.Cell, firstOf(oe.Content, oe.Value))
			if err != nil {
				return "", nil, err
			}
			return out, nil, nil
		default:
			return "", nil, fmt.Errorf("op %q desconeguda (set_cell|set_range|replace|append)", oe.Op)
		}
	case "read_image":
		var ia struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &ia); err != nil {
			return "", nil, err
		}
		dataURL, err := tools.ReadImageDataURL(ia.Path)
		if err != nil {
			return "", nil, err
		}
		return "imatge adjunta: " + ia.Path, []string{dataURL}, nil
	case "bash":
		var a struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.BashCtx(ctx, dir, a.Command, tools.DefaultTimeout)
		return out, nil, err
	case "bash_background":
		var a struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		p, err := Procs().Start(session, procDirOr(dir), a.Command)
		if err != nil {
			return "", nil, err
		}
		// Un procés que peta de seguida (ordre inexistent) ha de dir-ho
		// ara, no d'aquí a tres passos.
		if p.Wait(400 * time.Millisecond) {
			return p.Describe() + "\n" + p.Tail(40), nil, nil
		}
		return "engegat en segon pla amb id " + p.ID + " (" + a.Command + "). Llegeix-ne la sortida amb bash_output.", nil, nil
	case "bash_output":
		var a struct {
			ID    string `json:"id"`
			Lines int    `json:"lines"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		p := Procs().Get(a.ID)
		if p == nil {
			return "", nil, fmt.Errorf("procés %q desconegut", a.ID)
		}
		n := a.Lines
		if n <= 0 || n > 400 {
			n = 120
		}
		return p.Describe() + "\n" + p.Tail(n), nil, nil
	case "bash_kill":
		var a struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &a)
		if strings.TrimSpace(a.ID) == "" {
			list := Procs().List(session)
			if len(list) == 0 {
				return "cap procés en segon pla", nil, nil
			}
			var b strings.Builder
			for _, sn := range list {
				estat := "acabat"
				if sn.Running {
					estat = "en marxa"
				}
				fmt.Fprintf(&b, "%s · %s · %s\n", sn.ID, estat, sn.Cmd)
			}
			return b.String(), nil, nil
		}
		if !Procs().Kill(a.ID) {
			return "", nil, fmt.Errorf("procés %q desconegut", a.ID)
		}
		return "aturat " + a.ID, nil, nil
	case "write":
		var a struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		n, err := tools.Write(a.Path, []byte(a.Content))
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("escrit %s (%d bytes)", a.Path, n) + apendixEdicio(a.Path), nil, nil
	case "edit":
		var a struct {
			Path string `json:"path"`
			Old  string `json:"old_string"`
			New  string `json:"new_string"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		if err := tools.Edit(a.Path, a.Old, a.New); err != nil {
			return "", nil, err
		}
		return "edit aplicat a " + a.Path + apendixEdicio(a.Path), nil, nil
	case "patch":
		var pa struct {
			Path  string `json:"path"`
			Edits []struct {
				Old       string `json:"old"`
				New       string `json:"new"`
				AfterLine int    `json:"after_line"`
			} `json:"edits"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &pa); err != nil {
			return "", nil, err
		}
		ops := make([]tools.PatchOp, 0, len(pa.Edits))
		for _, e := range pa.Edits {
			ops = append(ops, tools.PatchOp{Old: e.Old, New: e.New, AfterLine: e.AfterLine})
		}
		if err := tools.Patch(pa.Path, ops); err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("patch aplicat a %s (%d edicions)", pa.Path, len(ops)) + apendixEdicio(pa.Path), nil, nil
	case "grep":
		var a struct {
			Pattern string `json:"pattern"`
			Dir     string `json:"dir"`
			Include string `json:"include"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.Grep(a.Pattern, a.Dir, a.Include, 50)
		return out, nil, err
	case "glob":
		var a struct {
			Pattern string `json:"pattern"`
			Dir     string `json:"dir"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.Glob(a.Pattern, a.Dir, 100)
		return out, nil, err
	case "web_search":
		var a struct {
			Query string `json:"query"`
			Count int    `json:"count"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.WebSearch(a.Query, a.Count)
		return out, nil, err
	case "web_fetch":
		var a struct {
			URL      string `json:"url"`
			Find     string `json:"find"`
			MaxChars int    `json:"max_chars"`
			Raw      bool   `json:"raw"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		out, err := tools.WebFetchOpts_(a.URL, tools.WebFetchOpts{MaxChars: a.MaxChars, Find: a.Find, Raw: a.Raw})
		return out, nil, err
	case "gh_issue", "gh_pr":
		out, err := tools.GitHub(name, argsJSON)
		return out, nil, err
	case "browser":
		var ba map[string]any
		if err := json.Unmarshal([]byte(argsJSON), &ba); err != nil {
			return "", nil, err
		}
		action, _ := ba["action"].(string)
		return tools.BrowserAction(ctx, action, ba)
	case "office_create":
		var oc struct {
			Path    string `json:"path"`
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &oc); err != nil {
			return "", nil, err
		}
		out, err := tools.OfficeCreate(oc.Path, oc.Title, oc.Content)
		if err != nil {
			return "", nil, err
		}
		return out, nil, nil
	case "question":
		// Sense UI interactiva (headless/loop) no es pot triar: es guia
		// el model perquè demani per text en comptes de bloquejar-se.
		q, opts, err := tools.ParseQuestion(argsJSON)
		if err != nil {
			return "", nil, err
		}
		_ = q
		_ = opts
		return "PREGUNTA SENSE UI: no hi ha interfície per triar opcions aquí. Formula la pregunta en text a la resposta final i continua amb el teu millor criteri.", nil, nil
	case "todowrite":
		var tw struct {
			Items []struct {
				Title  string `json:"title"`
				Status string `json:"status"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &tw); err != nil {
			return "", nil, err
		}
		items := make([]tools.TodoItem, 0, len(tw.Items))
		for _, it := range tw.Items {
			items = append(items, tools.TodoItem{Title: it.Title, Status: it.Status})
		}
		tools.TodoSet(items)
		return tools.TodoRender(), nil, nil
	case "todoread":
		return tools.TodoRender(), nil, nil
	case "skill":
		var a struct {
			Nom string `json:"nom"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
			return "", nil, err
		}
		return LlegeixSkill(dir, a.Nom)
	case "delegate":
		out, err := delegateExec(argsJSON)
		return out, nil, err
	default:
		if fn, ok := extraExec[name]; ok {
			out, err := fn(argsJSON)
			return out, nil, err
		}
		return "", nil, fmt.Errorf("eina desconeguda: %s", name)
	}
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// Stepper fa un pas del loop: historial → (contingut, tool_calls).
type Stepper func(ctx context.Context, hist []llm.Message) (string, []llm.ToolCall, error)

// Loop orquestra passos fins a resposta final (per tests i headless;
// el TUI fa servir les peces per separat per poder demanar confirmacions).
type Loop struct {
	MaxSteps int
	Step     Stepper
	RunTool  func(ctx context.Context, name, argsJSON string) (string, []string, error)
	Decide   func(name, argsJSON string) (string, string)
	// Finalize, si no és nil, genera la síntesi final sense eines quan
	// s'esgota el pressupost (estil opencode). Sense, es retorna l'error
	// clàssic de límit exhaurit.
	Finalize func(ctx context.Context, hist []llm.Message) (string, error)
}

// Run executa el loop sencer. Les ask es tracten com allow (headless).
func (l *Loop) Run(ctx context.Context, task string) (string, []llm.Message, error) {
	max := l.MaxSteps
	if max <= 0 {
		max = 10
	}
	doom := &DoomTracker{}
	hist := []llm.Message{{Role: "user", Content: task}}
	for i := 0; i < max; i++ {
		content, calls, err := l.Step(ctx, hist)
		if err != nil {
			return "", hist, err
		}
		hist = append(hist, llm.Message{Role: "assistant", Content: content, ToolCalls: calls})
		if len(calls) == 0 {
			return content, hist, nil
		}
		// Primer es decideix (permís i repetició, en ordre); després les
		// lectures s'executen en paral·lel i la resta en seqüència.
		type resultat struct {
			out  string
			imgs []string
		}
		fixos := make([]string, len(calls))
		for i, c := range calls {
			// El doom es comprova abans del permís: repetir una crida
			// denegada també és encallar-se.
			repetida := doom.Note(DoomSig(c.Function.Name, c.Function.Arguments))
			dec, reason := l.Decide(c.Function.Name, c.Function.Arguments)
			if dec == "deny" {
				fixos[i] = "EINA BLOQUEJADA: " + reason
			} else if repetida {
				fixos[i] = DoomGuide
			}
		}
		res := RunCalls(calls, func(i int) bool { return fixos[i] != "" }, func(_ int, c llm.ToolCall) resultat {
			r, ri, rerr := l.RunTool(ctx, c.Function.Name, c.Function.Arguments)
			if rerr != nil {
				return resultat{out: "ERROR: " + rerr.Error()}
			}
			return resultat{out: r, imgs: ri}
		})
		for i, c := range calls {
			out, outImgs := res[i].out, res[i].imgs
			if fixos[i] != "" {
				out, outImgs = fixos[i], nil
			}
			hist = append(hist, llm.Message{
				Role: "tool", Content: out, Images: outImgs,
				ToolCallID: c.ID, Name: c.Function.Name,
			})
		}
	}
	if l.Finalize != nil {
		final, ferr := l.Finalize(ctx, append(hist, llm.Message{Role: "user", Content: FinalPrompt}))
		if ferr == nil && strings.TrimSpace(final) != "" {
			return strings.TrimSpace(final), hist, nil
		}
	}
	return "", hist, fmt.Errorf("límit de %d passos exhaurit", max)
}

// resolArgs resol les rutes relatives dels arguments d'una eina contra el
// directori de la sessió.
//
// El bug que això tanca: `bash` i les eines de fitxers treballaven al
// directori del PROCÉS, no al workspace de la sessió. A l'escriptori, una
// pestanya que havia canviat de projecte seguia llegint i executant a
// l'altre —el panell de fitxers sí que el respectava, i les eines no—, i
// amb dues pestanyes en projectes diferents la cosa era pitjor encara.
//
// Es fa aquí, sobre el JSON, i no a cada cas: són dotze llocs i n'hi
// hauria hagut prou que un s'oblidés. Les claus són sempre rutes a les
// nostres eines ("path" a read/write/edit/patch/office/read_image, "dir" a
// grep/glob); "url", "repo" i companyia no s'hi toquen.
func resolArgs(dir, argsJSON string) string {
	if strings.TrimSpace(dir) == "" || strings.TrimSpace(argsJSON) == "" {
		return argsJSON
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return argsJSON // que falli on toca, amb el seu error
	}
	canviat := false
	if p, ok := m["path"].(string); ok && strings.TrimSpace(p) != "" && !tools.IsRooted(p) {
		m["path"] = filepath.Join(dir, p)
		canviat = true
	}
	// "dir" buit o absent vol dir «el projecte»: abans era el del procés.
	if d, ok := m["dir"].(string); !ok || strings.TrimSpace(d) == "" {
		if _, hiEs := m["dir"]; hiEs || esDeDirectori(m) {
			m["dir"] = dir
			canviat = true
		}
	} else if !tools.IsRooted(d) {
		m["dir"] = filepath.Join(dir, d)
		canviat = true
	}
	if !canviat {
		return argsJSON
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return argsJSON
	}
	return string(raw)
}

// esDeDirectori diu si els arguments són d'una eina que cerca dins d'un
// directori (grep/glob): tenen "pattern" i, si no diuen "dir", han de
// començar pel projecte.
func esDeDirectori(m map[string]any) bool {
	_, te := m["pattern"]
	return te
}

// resolArgsProva exposa resolArgs als tests d'altres paquets.
func ResolArgsProva(dir, argsJSON string) string { return resolArgs(dir, argsJSON) }
