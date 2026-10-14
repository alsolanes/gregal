package agent

import (
	"encoding/json"

	"gregal/internal/tools"
)

// checkpointAbansDe fa una foto git del workspace abans d'una ordre de shell
// que no és de només lectura: és el que permet desfer el que bash canvia per
// fora del journal (sed -i, rm, generadors). Les de lectura (ls, git status,
// go test…) no en fan, que no costi res als torns que només miren.
// Fora de git o amb git.checkpoints: off no fa res.
func checkpointAbansDe(owner, dir, name, argsJSON string) {
	if name != "bash" && name != "bash_background" {
		return
	}
	var a struct {
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(argsJSON), &a) != nil || a.Command == "" {
		return
	}
	if d, _ := tools.ClassifyWith(a.Command, nil, nil); d == "allow" {
		return
	}
	// L'error queda a GitCPs.LastError(): un log aquí trencaria el TUI.
	_, _ = tools.GitCPs.Snapshot(owner, dir, "step", "before: "+a.Command, -1)
}

// CheckpointTorn fa la foto d'inici de torn (convoLen = llargada de la
// conversa abans del missatge nou). Els modes de només lectura no en fan.
// L'error és informatiu (la web el registra al log): el torn continua.
func CheckpointTorn(owner, dir, mode string, convoLen int) error {
	if mode == ModeChat || mode == ModeInspect || mode == ModeGoal {
		return nil
	}
	_, err := tools.GitCPs.Snapshot(owner, dir, "turn", "before turn", convoLen)
	return err
}
