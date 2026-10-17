package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gregal/internal/agent"
	"gregal/internal/config"
	"gregal/internal/llm"
	"gregal/internal/session"
)

// Opcions del mode -p que el fan servir com claude -p, codex exec o
// opencode run dins d'scripts i de CI:
//
//   - La tirada es desa com una sessió més (la llista de /resume del TUI i
//     de la web), i `gregal -c -p "…"` la continua, o `--resume NOM` una
//     de concreta. Abans cada -p començava de zero i no en quedava res.
//   - El que arriba per stdin s'afegeix a la tasca: `git diff | gregal -p
//     "revisa això"`. Amb `-p -` la tasca sencera ve de stdin.
//   - `--model` tria el model d'aquesta tirada sense tocar el config:
//     `proveidor/model` o només el model (amb el proveïdor del rol).
type headlessExtra struct {
	// Desa la conversa com a sessió (el CLI sempre, tret de --no-session;
	// els tests no).
	Desa     bool
	Continua bool
	Resume   string
	Model    string
}

// maxStdin és el màxim que s'adjunta de stdin: un diff o un log llarg hi
// caben; un fitxer binari o un volcat sencer, no.
const maxStdin = 256 << 10

// tascaAmbStdin afegeix a la tasca el que arriba per una canonada o un
// fitxer redirigit. Un terminal no es llegeix: el -p d'abans no esperava
// res de stdin i no ha de quedar penjat.
func tascaAmbStdin(task string, in *os.File) (string, error) {
	if in == nil {
		return task, nil
	}
	st, err := in.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice != 0 {
		if task == "-" {
			return "", errors.New("-p -: no input on stdin")
		}
		return task, nil
	}
	// Sense dades (stdin tancat o /dev/null) no hi ha res a afegir.
	raw, err := io.ReadAll(io.LimitReader(in, maxStdin+1))
	if err != nil {
		return task, nil
	}
	tallat := len(raw) > maxStdin
	if tallat {
		raw = raw[:maxStdin]
	}
	dades := strings.TrimRight(strings.ToValidUTF8(string(raw), "�"), "\r\n")
	if task == "-" {
		if strings.TrimSpace(dades) == "" {
			return "", errors.New("-p -: stdin is empty")
		}
		return dades, nil
	}
	if strings.TrimSpace(dades) == "" {
		return task, nil
	}
	nota := ""
	if tallat {
		nota = fmt.Sprintf(" (truncated to the first %d KB)", maxStdin>>10)
	}
	return task + "\n\nInput piped on stdin" + nota + ":\n<stdin>\n" + dades + "\n</stdin>", nil
}

// aplicaModel posa el model triat als rols del torn principal (code, chat
// i think, si hi són), que són els que el router pot triar. «p/m» amb p un
// proveïdor del config canvia també el proveïdor; si no, el text sencer és
// el model (els d'OpenRouter porten barra: «anthropic/claude-…»).
func aplicaModel(cfg *config.Config, sel string) error {
	sel = strings.TrimSpace(sel)
	if sel == "" {
		return nil
	}
	prov, model := "", sel
	if i := strings.Index(sel, "/"); i > 0 {
		if _, ok := cfg.Providers[sel[:i]]; ok {
			prov, model = sel[:i], sel[i+1:]
		}
	}
	if strings.TrimSpace(model) == "" {
		return fmt.Errorf("--model %q: no model name", sel)
	}
	canviats := 0
	for _, nom := range []string{"code", "chat", "think"} {
		r, ok := cfg.Roles[nom]
		if !ok {
			continue
		}
		if prov != "" {
			r.Provider = prov
		}
		r.Model = model
		cfg.Roles[nom] = r
		canviats++
	}
	if canviats == 0 {
		return fmt.Errorf("--model: the configuration has no code or chat role")
	}
	return nil
}

// mateixaCarpeta compara dos camins de workspace (a Windows sense
// distingir majúscules).
func mateixaCarpeta(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// sessioPrevia carrega la conversa que continua aquesta tirada: la de
// --resume, o amb -c la més recent desada en aquesta carpeta (del -p, del
// TUI o de la web). Torna el nom, que es reutilitza en desar.
func sessioPrevia(dir, cwd string, ex headlessExtra) (string, []llm.Message, error) {
	nom := strings.TrimSpace(ex.Resume)
	if nom == "" && ex.Continua {
		infos, err := session.List(dir)
		if err != nil {
			return "", nil, err
		}
		for _, in := range infos {
			if mateixaCarpeta(in.Workspace, cwd) {
				nom = in.Name
				break
			}
		}
		if nom == "" {
			return "", nil, fmt.Errorf("-c: no saved session for %s", cwd)
		}
	}
	if nom == "" {
		return "", nil, nil
	}
	s, err := session.Load(dir, nom)
	if err != nil {
		return "", nil, err
	}
	// Les sessions compactades del TUI duen el resum a part.
	return s.Name, agent.AmbResum(s.Convo, s.Compacted), nil
}

// desaSessioHeadless desa la conversa de la tirada. Si continuava una
// sessió, n'actualitza el mateix fitxer.
func desaSessioHeadless(dir, nom, cwd, role string, convo []llm.Message) (string, error) {
	if len(convo) == 0 {
		return "", nil
	}
	if nom == "" {
		nom = session.NewName()
	}
	path, err := session.SaveSession(dir, session.Session{Name: nom, Role: role, Workspace: cwd, Convo: convo})
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(filepath.Base(path), ".json"), nil
}
