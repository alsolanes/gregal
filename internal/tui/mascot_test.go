package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
)

// Amb animacions, la mascota respira en repòs: dos ticks canvien el frame
// a la vista, i el tick sempre torna una comanda (la cadena no mor). Sense
// animacions (el defecte) es veu estàtica, i no canvia entre ticks.
func TestMascotaRepòs(t *testing.T) {
	m := planTestModel(t)
	if countFrames(m.View()) != mascotFrames[0] {
		t.Fatal("la mascota estàtica ha de ser visible sense animacions")
	}
	m.animacions = true
	v0 := m.View()
	mod, cmd := m.Update(streamTickMsg{})
	if cmd == nil {
		t.Fatal("el tick en repòs ha de rearmar-se (tickIdle)")
	}
	m = mod.(Model)
	v1 := m.View()
	frames0 := countFrames(v0)
	frames1 := countFrames(v1)
	if frames0 == frames1 {
		t.Fatalf("la vista no canvia entre ticks (spin mort?)")
	}
}

func countFrames(v string) string {
	for _, f := range mascotFrames {
		if strings.Contains(v, f) {
			return f
		}
	}
	return ""
}

// /copy sense respostes no copia res (avís honest, sense errors).
func TestCopyBuit(t *testing.T) {
	m := planTestModel(t)
	mod, _ := m.runCommand("/copy")
	got := mod.(Model)
	if len(got.lines) == 0 {
		t.Fatal("cal una línia de resposta")
	}
	last := got.lines[len(got.lines)-1]
	if !strings.Contains(last, "Res a copiar") {
		t.Fatalf("línia=%q", last)
	}
}

// La roda del ratolí ve activada de sèrie (és com s'espera llegir una
// conversa); /mouse la desactiva per poder seleccionar text del terminal, i
// la torna a activar.
func TestMouseCommuta(t *testing.T) {
	m := planTestModel(t)
	m.cfgPath = filepath.Join(t.TempDir(), "config.yaml")
	if !m.mouseOn {
		t.Fatal("el ratolí ha de venir activat (el programa arrenca amb WithMouseCellMotion)")
	}
	mod, cmd := m.runCommand("/mouse")
	got := mod.(Model)
	if got.mouseOn || cmd == nil {
		t.Fatal("el primer /mouse ha de desactivar")
	}
	mod2, cmd2 := got.runCommand("/mouse")
	if !mod2.(Model).mouseOn || cmd2 == nil {
		t.Fatal("el segon /mouse ha de tornar a activar")
	}
}

// /mouse es desa al config: en reobrir, la tria (roda o selecció)
// continua. Sense això cada reinici tornava la roda i no es podia
// seleccionar text "altre cop".
func TestMouseEsDesa(t *testing.T) {
	m := planTestModel(t)
	m.cfg.Roles["code"] = m.cfg.Roles["chat"]
	m.cfg.Roles["reviewer"] = m.cfg.Roles["chat"]
	m.cfgPath = filepath.Join(t.TempDir(), "config.yaml")
	mod, _ := m.runCommand("/mouse off")
	if mod.(Model).mouseOn {
		t.Fatal("/mouse off ha de desactivar")
	}
	raw, err := os.ReadFile(m.cfgPath)
	if err != nil {
		t.Fatalf("el config s'ha de desar: %v", err)
	}
	if !strings.Contains(string(raw), "mouse:") {
		t.Fatalf("al fitxer hi ha d'haver la clau mouse:\n%s", raw)
	}
	cfg2, _, err := config.Load(m.cfgPath)
	if err != nil {
		t.Fatalf("recarregar: %v", err)
	}
	if cfg2.MouseOn() {
		t.Fatal("l'off s'ha de conservar en desar i carregar")
	}
	mod, _ = mod.(Model).runCommand("/mouse on")
	if !mod.(Model).mouseOn {
		t.Fatal("/mouse on ha de reactivar")
	}
}
