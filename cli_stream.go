package main

import (
	"encoding/json"
	"io"
	"sync"

	"gregal/internal/agent"
)

// --output stream-json: una línia JSON per cosa que passa (pas, crida
// d'eina, resultat, avís) i, al final, el mateix objecte que --output json
// amb "type":"result". És el que fan `claude -p --output-format
// stream-json`, `codex exec --json` i `opencode run --format json`: un
// script, una CI o una extensió d'editor veuen el torn mentre passa, en
// lloc d'esperar minuts a un sol objecte.
type emissorJSONL struct {
	mu sync.Mutex
	w  io.Writer
}

// maxSortidaStream és el que es deixa de cada resultat d'eina: prou per
// veure què ha passat sense copiar un fitxer sencer a cada línia.
const maxSortidaStream = 4000

func (e *emissorJSONL) emet(v map[string]any) {
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.w.Write(append(raw, '\n'))
}

func (e *emissorJSONL) pas(step int, tools []string) {
	e.emet(map[string]any{"type": "step", "step": step, "tools": tools})
}

func (e *emissorJSONL) event(ev agent.Event) {
	switch ev.Tipus {
	case agent.EvCrida:
		e.emet(map[string]any{"type": "tool_call", "id": ev.ID, "tool": ev.Eina, "args": json.RawMessage(argsJSON(ev.EinaArgs))})
	case agent.EvResultat:
		sortida := ev.Missatge()
		tallat := false
		if r := []rune(sortida); len(r) > maxSortidaStream {
			sortida, tallat = string(r[:maxSortidaStream]), true
		}
		v := map[string]any{"type": "tool_result", "id": ev.ID, "tool": ev.Eina, "failed": ev.Fallada, "output": sortida}
		if tallat {
			v["truncated"] = true
		}
		if !ev.Executada {
			v["executed"] = false
		}
		e.emet(v)
	case agent.EvAvis:
		e.emet(map[string]any{"type": "warning", "text": ev.Missatge()})
	case agent.EvError:
		e.emet(map[string]any{"type": "error", "text": ev.Missatge()})
	case agent.EvDiff:
		e.emet(map[string]any{"type": "diff", "text": ev.Missatge()})
	case agent.EvNota:
		e.emet(map[string]any{"type": "note", "text": ev.Missatge()})
	}
}

// resultat és la darrera línia: l'objecte de --output json amb el tipus.
func (e *emissorJSONL) resultat(h headlessJSON) {
	raw, _ := json.Marshal(h)
	var v map[string]any
	if json.Unmarshal(raw, &v) != nil {
		return
	}
	v["type"] = "result"
	e.emet(v)
}

// argsJSON deixa els arguments com a objecte si són JSON vàlid, i com a
// cadena si no (un model pot enviar-ne de trencats).
func argsJSON(s string) []byte {
	if json.Valid([]byte(s)) && s != "" {
		return []byte(s)
	}
	raw, _ := json.Marshal(s)
	return raw
}
