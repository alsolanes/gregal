package agent

import (
	"encoding/json"

	"gregal/internal/llm"
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

// CheckpointIMapa fa la foto d'inici de torn i el mapa del projecte alhora
// i torna la tasca amb el mapa (TascaAmbMapa). Són independents —la foto
// treballa amb un índex temporal i no toca el de l'usuari que llegeix el
// git status del mapa— i en fila sumaven ~0,4 s + 0,1-0,6 s abans del
// primer pas (al TUI, al fil de la interfície).
func CheckpointIMapa(owner, dir, mode string, convoLen int, task string, hist []llm.Message) string {
	fet := make(chan struct{})
	go func() {
		defer close(fet)
		_ = CheckpointTorn(owner, dir, mode, convoLen)
	}()
	out := TascaAmbMapa(mode, dir, task, hist)
	<-fet
	return out
}
