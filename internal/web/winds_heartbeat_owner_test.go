package web

import (
	"sort"
	"strings"
	"testing"
	"time"

	"gregal/internal/jobs"
)

func heartSave(t *testing.T, s *Server, body string) (int, string) {
	t.Helper()
	code, out := sharedCall(t, s, (*Server).handleJobSave, "POST", "/api/jobs/save", body)
	if code != 200 {
		return code, ""
	}
	return code, out["job"].(map[string]any)["id"].(string)
}

func heartIDs(t *testing.T, s *Server) string {
	t.Helper()
	_, out := sharedCall(t, s, (*Server).handleJobs, "GET", "/api/jobs", "")
	var ids []string
	for _, j := range out["jobs"].([]any) {
		ids = append(ids, j.(map[string]any)["id"].(string))
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

func heartJob(t *testing.T, s *Server, id string) jobs.Job {
	t.Helper()
	list, err := s.jobs.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range list {
		if j.ID == id {
			return j
		}
	}
	t.Fatalf("no hi ha el job %s", id)
	return jobs.Job{}
}

// Dos comptes amb un vent del mateix id: cada un desa el seu cor (amb l'id
// de sempre o sense id) i n'hi ha dos, cadascun només visible i editable pel
// seu compte. El cor antic sense amo no surt a cap dels dos.
func TestHeartbeatJobPerAccount(t *testing.T) {
	rig := sharedTestRig(t)
	sameWindTwoAccounts(t, rig)
	seedLegacyHeart(t, rig.aleix, rig.mestral.ID)
	code, aleixID := heartSave(t, rig.aleix, `{"id":"wind-mestral","name":"Cor","prompt":"vigila","kind":"event","enabled":true}`)
	if code != 200 {
		t.Fatalf("l'Aleix desa el cor: %d", code)
	}
	code, carlaID := heartSave(t, rig.carla, `{"name":"Cor","prompt":"mar","kind":"event","enabled":true,"wind":"mestral"}`)
	if code != 200 {
		t.Fatalf("la Carla desa el cor: %d", code)
	}
	if aleixID != "wind-mestral@aleix" || carlaID != "wind-mestral@carla" {
		t.Fatalf("ids = %s / %s", aleixID, carlaID)
	}
	if got := heartIDs(t, rig.aleix); got != aleixID {
		t.Fatalf("l'Aleix veu %s", got)
	}
	if got := heartIDs(t, rig.carla); got != carlaID {
		t.Fatalf("la Carla veu %s", got)
	}
	// Pausar el propi no toca el de l'altre compte.
	if code, _ := heartSave(t, rig.aleix, `{"id":"`+aleixID+`","name":"Cor","prompt":"vigila","kind":"event","enabled":false}`); code != 200 {
		t.Fatalf("pausa: %d", code)
	}
	if a, c := heartJob(t, rig.aleix, aleixID), heartJob(t, rig.aleix, carlaID); a.Enabled || !c.Enabled || c.Prompt != "mar" || c.WindOwner != "carla" {
		t.Fatalf("després de pausar: %+v / %+v", a, c)
	}
}

// Ni desant ni esborrant amb l'id del cor d'un altre compte no s'hi arriba.
func TestHeartbeatJobOtherAccountCannotOverwrite(t *testing.T) {
	rig := sharedTestRig(t)
	sameWindTwoAccounts(t, rig)
	_, aleixID := heartSave(t, rig.aleix, `{"id":"wind-mestral","name":"Cor","prompt":"vigila","kind":"event","enabled":true}`)
	if code, _ := heartSave(t, rig.carla, `{"id":"`+aleixID+`","name":"Meu","prompt":"robat","kind":"event","enabled":false}`); code != 404 {
		t.Fatalf("la Carla desa sobre el cor de l'Aleix: %d", code)
	}
	if code, _ := sharedCall(t, rig.carla, (*Server).handleJobDelete, "POST", "/api/jobs/delete", `{"id":"`+aleixID+`"}`); code != 404 {
		t.Fatalf("la Carla esborra el cor de l'Aleix: %d", code)
	}
	if j := heartJob(t, rig.aleix, aleixID); !j.Enabled || j.Prompt != "vigila" || j.WindOwner != "aleix" {
		t.Fatalf("el cor de l'Aleix ha canviat: %+v", j)
	}
}

// Sense comptes, el cor és wind-<vent> com sempre.
func TestHeartbeatJobSingleUserKeepsID(t *testing.T) {
	s := rutinaTestServer(t)
	for _, body := range []string{
		`{"id":"wind-xaloc","name":"Cor","prompt":"vigila","kind":"event","enabled":true}`,
		`{"name":"Cor","prompt":"vigila","kind":"event","enabled":true,"wind":"xaloc"}`,
	} {
		if code, id := heartSave(t, s, body); code != 200 || id != "wind-xaloc" {
			t.Fatalf("%s → %d %s", body, code, id)
		}
	}
}

// El cor d'un compte desat abans amb l'id de sempre passa a l'id del compte,
// amb la seva programació; el que no té amo (o un sol usuari) no es toca.
func TestMigrateWindJobsToAccountIDs(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	store, err := jobs.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	next := time.Now().Add(time.Hour).Truncate(time.Second)
	for _, j := range []jobs.Job{
		{ID: "wind-mestral", Name: "Cor", Prompt: "vigila", Kind: "event", Enabled: true, NextAt: &next, Wind: "mestral", WindOwner: "aleix"},
		{ID: "wind-xaloc", Name: "Cor", Prompt: "vigila", Kind: "event", Enabled: true},
		{ID: "wind-garbi", Name: "Cor", Prompt: "vigila", Kind: "event", Enabled: true, Wind: "garbi"},
	} {
		if _, err := store.Save(j); err != nil {
			t.Fatal(err)
		}
	}
	migrateWindJobs(store)
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, j := range list {
		ids = append(ids, j.ID)
		if j.ID == "wind-mestral@aleix" && (j.NextAt == nil || !j.NextAt.Equal(next) || j.WindOwner != "aleix") {
			t.Fatalf("migrat sense programació: %+v", j)
		}
	}
	sort.Strings(ids)
	if got := strings.Join(ids, ","); got != "wind-garbi,wind-mestral@aleix,wind-xaloc" {
		t.Fatalf("ids = %s", got)
	}
}

// La responsabilitat del cor (pausar-lo des de «Responsabilitats») toca el
// cor del seu compte.
func TestRespHeartbeatSyncsOwnAccountJob(t *testing.T) {
	rig := sharedTestRig(t)
	sameWindTwoAccounts(t, rig)
	_, aleixID := heartSave(t, rig.aleix, `{"id":"wind-mestral","name":"Cor","prompt":"vigila","kind":"event","enabled":true}`)
	_, carlaID := heartSave(t, rig.carla, `{"id":"wind-mestral","name":"Cor","prompt":"mar","kind":"event","enabled":true}`)
	r := rig.carla.respEnsureHeartbeat(rig.mestral, heartJob(t, rig.carla, carlaID))
	r.State = respPaused
	if err := rig.carla.respSyncJob(r, false); err != nil {
		t.Fatal(err)
	}
	if a, c := heartJob(t, rig.aleix, aleixID), heartJob(t, rig.carla, carlaID); !a.Enabled || c.Enabled {
		t.Fatalf("pausa de la Carla: aleix=%v carla=%v", a.Enabled, c.Enabled)
	}
}
