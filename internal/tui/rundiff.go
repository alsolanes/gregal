package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"gregal/internal/agent"
)

// execToolWithDiff executa una eina i, si és write/edit/patch, retorna el
// diff abans/després del fitxer tocat (best-effort: sense diff si no el
// pot llegir o no hi ha canvis). És el que fa visible la feina de
// l'agent: no només "FET edit", sinó què ha canviat.
func execToolWithDiff(name, argsJSON string) (out, diff string, imgs []string, err error) {
	return execToolWithDiffIn("", name, argsJSON)
}

// execToolWithDiffIn és la variant del TUI que fixa el workspace de la
// sessió. Això és necessari després de reprendre una conversa d'un altre
// projecte: agent.ExecIn resol arguments i processos dins d'aquesta carpeta.
func execToolWithDiffIn(dir, name, argsJSON string) (out, diff string, imgs []string, err error) {
	return execToolWithDiffCtx(context.Background(), dir, name, argsJSON)
}

// execToolWithDiffCtx és la mateixa amb el context del torn: cancel·lar-lo
// atura l'eina en marxa (bash, navegador). El TUI sempre passa per aquí;
// la variant sense context queda per als tests.
func execToolWithDiffCtx(ctx context.Context, dir, name, argsJSON string) (out, diff string, imgs []string, err error) {
	path := toolFilePath(name, argsJSON)
	diffPath := path
	if dir != "" && path != "" && !filepath.IsAbs(path) {
		diffPath = filepath.Join(dir, path)
	}
	var old []byte
	if diffPath != "" {
		old, _ = os.ReadFile(diffPath)
	}
	out, imgs, err = agent.ExecCtx(ctx, "", dir, name, argsJSON)
	if diffPath != "" && err == nil {
		if nb, rerr := os.ReadFile(diffPath); rerr == nil && string(nb) != string(old) {
			diff = diffCos(diffPath, string(old), string(nb), 40)
		}
	}
	return out, diff, imgs, err
}

// toolFilePath extreu el fitxer afectat per les eines d'escriptura.
func toolFilePath(name, argsJSON string) string {
	switch name {
	case "write", "edit", "patch":
		var a struct {
			Path string `json:"path"`
		}
		if json.Unmarshal([]byte(argsJSON), &a) == nil {
			return a.Path
		}
	}
	return ""
}
