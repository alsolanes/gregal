package agent

import (
	"path/filepath"
	"testing"

	"gregal/internal/config"
)

func TestUIModeIAccessCobreixenTotsElsModes(t *testing.T) {
	for mode, want := range map[string]string{
		ModeChat: UIModeAsk, ModeInspect: UIModeAsk,
		ModePlan: UIModePlan, ModeGoal: UIModePlan,
		ModeCode: UIModeAct, ModeAutonomous: UIModeAct, ModeDebug: UIModeAct,
	} {
		if got := UIMode(mode); got != want {
			t.Errorf("UIMode(%s) = %s, want %s", mode, got, want)
		}
	}
	if AccessOf(ModeAutonomous, true) != AccessFull || AccessOf(ModeCode, true) != AccessAsk || AccessOf(ModeCode, false) != AccessEdits {
		t.Fatal("AccessOf")
	}
	if ModeFor(UIModeAct, AccessFull) != ModeAutonomous || ModeFor(UIModeAct, AccessAsk) != ModeCode || ModeFor(UIModePlan, AccessFull) != ModePlan || ModeFor(UIModeAsk, AccessFull) != ModeChat {
		t.Fatal("ModeFor")
	}
}

func TestAccessFromWordAcceptaElsNomsAntics(t *testing.T) {
	for w, want := range map[string]string{
		"manual": AccessEdits, "accept": AccessEdits, "review": AccessEdits,
		"auto": AccessFull, "full": AccessFull, "ask": AccessAsk, "edits": AccessEdits, "xyz": "",
	} {
		if got := AccessFromWord(w); got != want {
			t.Errorf("AccessFromWord(%q) = %q, want %q", w, got, want)
		}
	}
}

func TestModePlaEsNomesLectura(t *testing.T) {
	if !ValidMode(ModePlan) || ModeFromWord("planifica") != ModePlan || ModeFromWord("plan") != ModePlan {
		t.Fatal("plan ha de ser un mode vàlid")
	}
	dir := t.TempDir()
	p := &Policy{ProjectDir: dir}
	args := `{"path":"` + filepath.ToSlash(filepath.Join(dir, "a.go")) + `","content":"x"}`
	if d, _ := p.Decide(ModePlan, "write", args); d != "deny" {
		t.Fatalf("plan write = %s, want deny", d)
	}
	if d, _ := p.Decide(ModePlan, "bash", `{"command":"rm -rf build"}`); d != "deny" {
		t.Fatalf("plan bash = %s, want deny", d)
	}
	if d, _ := p.Decide(ModePlan, "read", `{"path":"a.go"}`); d != "allow" {
		t.Fatalf("plan read = %s, want allow", d)
	}
}

func TestDemanaAbansDeCanviar(t *testing.T) {
	dir := t.TempDir()
	args := `{"path":"` + filepath.ToSlash(filepath.Join(dir, "a.go")) + `","content":"x"}`
	edits := &Policy{ProjectDir: dir}
	if d, _ := edits.Decide(ModeCode, "write", args); d != "allow" {
		t.Fatalf("edits: write dins = %s, want allow", d)
	}
	ask := PolicyFromConfig(&config.Config{Permissions: config.PermissionsCfg{Access: "ask"}}, dir)
	if !ask.AskEdits {
		t.Fatal("permissions.access: ask ha d'arribar a la política")
	}
	if d, _ := ask.Decide(ModeCode, "write", args); d != "ask" {
		t.Fatalf("ask: write dins = %s, want ask", d)
	}
	if d, _ := ask.Decide(ModeAutonomous, "write", args); d != "allow" {
		t.Fatalf("autonomia total: write dins = %s, want allow", d)
	}
	if !delegatePolicy(config.PermissionsCfg{Access: "ask"}, "").AskEdits {
		t.Fatal("els subagents també han de demanar")
	}
}
