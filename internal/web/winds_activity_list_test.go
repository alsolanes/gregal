package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
)

func activityGet(t *testing.T, s *Server, query string) (int, []ActivityItem) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindsActivity(rec, httptest.NewRequest(http.MethodGet, "/api/winds/activity"+query, nil))
	if rec.Code != 200 {
		return rec.Code, nil
	}
	var out struct {
		Items []ActivityItem `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return rec.Code, out.Items
}

func activityKeys(items []ActivityItem) string {
	var keys []string
	for _, it := range items {
		keys = append(keys, it.Kind+":"+it.Ref)
	}
	return strings.Join(keys, ",")
}

// seedActivity deixa a s una cosa de cada mena, del vent wind, amb hores
// fixes: base-1h la feina … base-5h el recordatori.
func seedActivity(t *testing.T, s *Server, wind companion.Wind, owner, tag string, base time.Time) {
	t.Helper()
	at := func(h int) time.Time { return base.Add(-time.Duration(h) * time.Hour) }
	// Les feines només es llisten si la carpeta és dins les arrels del compte.
	ws := t.TempDir()
	s.user.Roots = append(s.user.Roots, ws)
	rec := windTaskRecord{ID: "task-" + tag, Task: "Fes una web " + tag, Workspace: ws, Created: at(1), Status: "done",
		Agents: map[string]string{"build-1": wind.Name}}
	if err := writeWindJSON(filepath.Join(s.windStateDir(), "wind-tasks", rec.ID+".json"), rec); err != nil {
		t.Fatal(err)
	}
	novetatsMu.Lock()
	saveNovetats(s, append(loadNovetats(s), windNovetat{Ts: at(2).Unix(), Wind: wind.ID, Name: wind.Name, Source: "cor",
		Text: "Ha nevat " + tag, ID: "n-" + tag, Owner: owner}))
	novetatsMu.Unlock()
	routine := jobs.Job{ID: "rutina-" + tag, Name: "Discos " + tag, Kind: "daily", Time: "08:00", Enabled: true, Wind: wind.ID, WindOwner: owner}
	heartAt, remindAt := at(4), at(5)
	for _, j := range []jobs.Job{
		routine,
		{ID: "wind-" + wind.ID, Name: "Cor", Prompt: "vigila", Kind: "event", Enabled: true, LastAt: &heartAt, LastStatus: "ok", Wind: wind.ID, WindOwner: owner},
		{ID: "remind-" + wind.ID + "-" + tag, Prompt: "Rega " + tag, Kind: "remind", LastAt: &remindAt, LastStatus: "ok", Wind: wind.ID, WindOwner: owner},
	} {
		if _, err := s.jobs.Save(j); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.jobs.AddRun(jobs.Run{ID: "run-" + tag, Wind: wind.ID, WindOwner: owner, JobID: routine.ID, JobName: routine.Name,
		StartedAt: at(3).Add(-time.Minute), FinishedAt: at(3), Status: "ok"}); err != nil {
		t.Fatal(err)
	}
}

// Una sola llista amb tot el que els vents han fet, de la més nova a la més
// vella: feines delegades, novetats, runs de rutines i els cors i
// recordatoris que han sonat; «before» pagina.
func TestActivityMergesKindsNewestFirst(t *testing.T) {
	rig := sharedTestRig(t)
	s := rig.aleix
	base := time.Now().Add(-time.Hour).Truncate(time.Second)
	seedActivity(t, s, rig.mestral, "aleix", "a", base)

	_, items := activityGet(t, s, "")
	want := "task:task-a,news:n-a,routine:rutina-a,heartbeat:wind-mestral,reminder:remind-mestral-a"
	if got := activityKeys(items); got != want {
		t.Fatalf("activitat = %s\nvull %s", got, want)
	}
	for _, it := range items {
		if it.Wind != rig.mestral.ID || it.Title == "" || it.At.IsZero() {
			t.Fatalf("element incomplet: %+v", it)
		}
	}
	if items[0].Status != "done" || items[1].Status != "unread" || items[2].Status != "ok" {
		t.Fatalf("estats = %+v", items[:3])
	}

	// Pàgina següent: el client passa l'«at» del darrer element.
	_, page := activityGet(t, s, "?limit=2")
	if activityKeys(page) != "task:task-a,news:n-a" {
		t.Fatalf("pàgina 1 = %s", activityKeys(page))
	}
	_, next := activityGet(t, s, "?limit=2&before="+url.QueryEscape(page[1].At.Format(time.RFC3339Nano)))
	if activityKeys(next) != "routine:rutina-a,heartbeat:wind-mestral" {
		t.Fatalf("pàgina 2 = %s", activityKeys(next))
	}
	// També en segons unix, com «des_de» de les novetats.
	_, unix := activityGet(t, s, "?before="+strconv.FormatInt(base.Add(-3*time.Hour).Unix(), 10))
	if activityKeys(unix) != "heartbeat:wind-mestral,reminder:remind-mestral-a" {
		t.Fatalf("before unix = %s", activityKeys(unix))
	}

	// Només un vent: el que no en té res, llista buida (no null).
	_, only := activityGet(t, s, "?wind="+rig.mestral.ID)
	if len(only) != 5 {
		t.Fatalf("filtre per vent = %s", activityKeys(only))
	}
	_, none := activityGet(t, s, "?wind="+rig.secret.ID)
	if none == nil || len(none) != 0 {
		t.Fatalf("vent sense activitat = %+v", none)
	}
}

// Els elements d'un altre compte no surten mai, ni quan el seu vent té el
// mateix id que un de propi.
func TestActivityOnlyOwnAccount(t *testing.T) {
	rig := sharedTestRig(t)
	carlaMestral, err := rig.carla.winds.Save(companion.Wind{Name: "Mestral", Role: "mar"})
	if err != nil {
		t.Fatal(err)
	}
	if carlaMestral.ID != rig.mestral.ID {
		t.Fatalf("el test vol el mateix id als dos comptes: %s / %s", carlaMestral.ID, rig.mestral.ID)
	}
	base := time.Now().Add(-time.Hour).Truncate(time.Second)
	seedActivity(t, rig.carla, carlaMestral, "carla", "c", base)

	if _, items := activityGet(t, rig.aleix, ""); len(items) != 0 {
		t.Fatalf("l'Aleix veu activitat de la Carla: %s", activityKeys(items))
	}
	_, mine := activityGet(t, rig.carla, "")
	if len(mine) != 5 {
		t.Fatalf("la Carla no veu la seva: %s", activityKeys(mine))
	}
}

func TestActivityLimitAndBadBefore(t *testing.T) {
	rig := sharedTestRig(t)
	s := rig.aleix
	now := time.Now().Truncate(time.Second)
	novetatsMu.Lock()
	var list []windNovetat
	for i := 0; i < 250; i++ {
		list = append(list, windNovetat{Ts: now.Add(-time.Duration(i) * time.Minute).Unix(), Wind: rig.mestral.ID,
			Name: "Mestral", Source: "cor", Text: "veu", ID: "n-" + strconv.Itoa(i), Owner: "aleix"})
	}
	// saveNovetats en guarda 100: prou per veure el límit per defecte.
	saveNovetats(s, list[:100])
	novetatsMu.Unlock()
	routine := jobs.Job{ID: "rutina-x", Name: "Discos", Kind: "interval", IntervalH: 1, Enabled: true, Wind: rig.mestral.ID, WindOwner: "aleix"}
	if _, err := s.jobs.Save(routine); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 150; i++ {
		at := now.Add(-time.Duration(i) * time.Second)
		if err := s.jobs.AddRun(jobs.Run{ID: "run-" + strconv.Itoa(1000+i), Wind: rig.mestral.ID, WindOwner: "aleix", JobID: routine.ID,
			JobName: routine.Name, StartedAt: at, FinishedAt: at, Status: "ok"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, items := activityGet(t, s, ""); len(items) != 50 {
		t.Fatalf("per defecte = %d, vull 50", len(items))
	}
	if _, items := activityGet(t, s, "?limit=1000"); len(items) != 200 {
		t.Fatalf("limit=1000 = %d, vull 200", len(items))
	}
	if _, items := activityGet(t, s, "?limit=3"); len(items) != 3 {
		t.Fatalf("limit=3 = %d", len(items))
	}
	for _, bad := range []string{"ahir", "2026-13-01", "12abc"} {
		if code, _ := activityGet(t, s, "?before="+bad); code != http.StatusBadRequest {
			t.Fatalf("before=%s → %d, vull 400", bad, code)
		}
	}
}
