package web

import (
	"strings"
	"testing"

	"gregal/internal/agent"
	"gregal/internal/companion"
)

// TestRemindAmbNomEnComptesDId: el model passa el nom del vent a l'eina
// remind; ha de programar l'avís igualment (id o nom, tot val).
func TestRemindAmbNomEnComptesDId(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	w, ok := s.winds.ByRef("Xaloc")
	if !ok {
		t.Fatalf("no trobo el vent de prova")
	}
	_, err := saveWindReminder(s, w.Name, "+2h", "recorda'm prendre l'aigua")
	if err != nil {
		t.Fatalf("remind per nom: %v", err)
	}
	found := false
	list, _ := s.jobs.List()
	for _, j := range list {
		if strings.HasPrefix(j.ID, "remind-"+w.ID) {
			found = true
		}
	}
	if !found {
		t.Errorf("no s'ha desat cap remind amb l'id real del vent")
	}
}

// TestRemindEnSessioMultiusuariUsaElsVentsDeLaSessio: l'eina remind es
// registra un sol cop en arrencar (amb el Server base), però quan un usuari
// parla amb el seu vent en una sessió clonada per SessionFor, s.winds és el
// de l'usuari. El context del torn ha de portar aquell Server perquè remind
// trobi el vent i en desi el propietari.
func TestRemindEnSessioMultiusuariUsaElsVentsDeLaSessio(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	root := remindTestServer(t)
	root.registerRemindTool()
	// Un Server a part (no una còpia de *root, que duplicaria el mutex) que
	// comparteix la cua de feines, com fa clone().
	sess := remindTestServer(t)
	sess.jobs = root.jobs
	sess.user = &User{Name: "aleix"}
	sess.winds = companion.NewWindStore(t.TempDir())
	w, err := sess.winds.Save(companion.Wind{Name: "Ponent", Color: "#EACF87"})
	if err != nil {
		t.Fatal(err)
	}
	// El base no té Ponent (com passa en multiusuari quan cada usuari té el seu windStateDir).
	ctx := sess.withRemindServer(t.Context())
	out, _, err := agent.ExecCtx(ctx, "test", t.TempDir(), "remind", `{"wind":"`+w.ID+`","at":"+2h","text":"Surt el llibre de Rowe"}`)
	if err != nil || !strings.Contains(out, "Fet:") {
		t.Fatalf("remind en sessió d'usuari ha fallat: out=%q err=%v", out, err)
	}
	list, _ := sess.jobs.List()
	if len(list) == 0 || list[0].WindOwner != "aleix" || list[0].Wind != w.ID {
		t.Fatalf("job mal associat a l'usuari: %+v", list)
	}
}
