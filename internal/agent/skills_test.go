package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// L'eina `skill` obre la del projecte i, si el nom no hi és, diu quines
// hi ha: un nom mal escrit no ha de costar un pas perdut.
func TestEinaSkill(t *testing.T) {
	dir := t.TempDir()
	sk := filepath.Join(dir, ".gregal", "skills")
	if err := os.MkdirAll(sk, 0o755); err != nil {
		t.Fatal(err)
	}
	cos := "---\nnom: desplegament\ndescripcio: com es desplega això\n---\nAmb make deploy i prou.\n"
	if err := os.WriteFile(filepath.Join(sk, "desplegament.md"), []byte(cos), 0o600); err != nil {
		t.Fatal(err)
	}

	out, _, err := ExecIn("", dir, "skill", `{"nom":"desplegament"}`)
	if err != nil {
		t.Fatalf("hauria d'obrir-la: %v", err)
	}
	if !strings.Contains(out, "make deploy") {
		t.Fatalf("el cos ha de sortir sencer: %q", out)
	}

	// Una integrada, sense projecte que la tapi.
	if out, _, err := ExecIn("", dir, "skill", `{"nom":"sessions"}`); err != nil || !strings.Contains(out, "sessio-") {
		t.Fatalf("la integrada de sessions ha de sortir: %v %q", err, out)
	}

	_, _, err = ExecIn("", dir, "skill", `{"nom":"desplegamnt"}`)
	if err == nil || !strings.Contains(err.Error(), "desplegament") {
		t.Fatalf("un nom que no hi és ha de dir quines hi ha: %v", err)
	}
}

// L'índex de skills entra al prompt de tots els fronts (tots componen el
// context del projecte amb ContextProjecte).
func TestElContextDelProjectePortaLIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("Regla d'or."), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := ContextProjecte(dir)
	if !strings.Contains(ctx, "Regla d'or.") {
		t.Fatalf("la memòria del projecte hi ha de ser:\n%s", ctx)
	}
	for _, vol := range []string{"SKILLS", "sessions:", "configuracio:", "`skill`"} {
		if !strings.Contains(ctx, vol) {
			t.Fatalf("falta %q a l'índex:\n%s", vol, ctx)
		}
	}
	// El cos NO hi és: al prompt hi va l'índex i prou.
	if strings.Contains(ctx, "events.jsonl") {
		t.Fatal("el cos de les skills no pot anar al prompt")
	}
}

// L'eina és de lectura: no demana permís i el menú de permisos no
// l'ofereix (denegar-la només trencaria el que sap de si mateix).
func TestSkillEsDeLectura(t *testing.T) {
	if d, _ := DefaultPolicy().For("skill", `{"nom":"gregal"}`); d != "allow" {
		t.Fatalf("hauria de ser allow: %s", d)
	}
}
