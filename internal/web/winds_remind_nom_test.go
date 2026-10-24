package web

import (
	"strings"
	"testing"
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
