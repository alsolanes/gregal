package web

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"gregal/internal/agent"
)

// Un torn reprès després d'una decisió torna a la carpeta on treballava (la de
// la conversa), encara que la sessió del navegador hagi canviat de projecte
// (anar a Codi): si no, tot el que escrivia a la seva carpeta queia «fora del
// projecte» i demanava permís fitxer a fitxer.
func TestWindResumeKeepsTurnWorkspace(t *testing.T) {
	codi := t.TempDir()
	conversa := t.TempDir()
	pol := &agent.Policy{ProjectDir: codi, Tools: map[string]string{"write": "ask"}}
	ctx := context.WithValue(context.Background(), interactiveWindKey{}, windTurnScope{mode: agent.ModeCode, workspace: codi, policy: pol})
	fitxer := `{"path":` + quoteJSONPath(filepath.Join(conversa, "zefir", "index.html")) + `,"content":"x"}`

	before, _ := ctx.Value(interactiveWindKey{}).(windTurnScope)
	if before.policy.WindAutoAllow("write", fitxer) {
		t.Fatal("sense restaurar la carpeta, el fitxer de la conversa és fora del projecte de Codi")
	}
	after, ok := withWindWorkspaceScope(ctx, conversa).Value(interactiveWindKey{}).(windTurnScope)
	if !ok || after.workspace != conversa || after.policy.ProjectDir != conversa {
		t.Fatalf("abast reprès: %+v", after)
	}
	if !after.policy.WindAutoAllow("write", fitxer) {
		t.Fatal("dins la carpeta del torn, escriure ha de passar sol (write: ask de la plantilla)")
	}
	if after.policy.WindAutoAllow("write", `{"path":`+quoteJSONPath(filepath.Join(codi, "x.go"))+`,"content":"x"}`) {
		t.Fatal("la represa no guanya permís sobre la carpeta de Codi")
	}
	if pol.ProjectDir != codi {
		t.Fatal("no s'ha de modificar la política original")
	}
}

func quoteJSONPath(p string) string {
	b, _ := json.Marshal(p)
	return string(b)
}
