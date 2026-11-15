package web

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gregal/internal/agent"
	"gregal/internal/jobs"
)

func TestRoutineDaysAndWhen(t *testing.T) {
	if d, err := routineDays("weekdays", nil); err != nil || len(d) != 5 {
		t.Fatalf("weekdays: %v %v", d, err)
	}
	if d, err := routineDays("days", []string{"mon", "Dijous", "mon"}); err != nil || len(d) != 2 || d[0] != 1 || d[1] != 4 {
		t.Fatalf("days: %v %v", d, err)
	}
	if _, err := routineDays("days", []string{"funday"}); err == nil {
		t.Fatal("unknown day must fail")
	}
	if d, err := routineDays("daily", nil); err != nil || d != nil {
		t.Fatalf("daily: %v %v", d, err)
	}
}

func TestDailyJobOnlyRunsOnItsDays(t *testing.T) {
	loc := time.UTC
	// Dissabte 21/11/2026 a les 09:00: una rutina de feiners a les 08:00 no
	// toca fins dilluns 23 a les 08:00.
	sat := time.Date(2026, 11, 21, 9, 0, 0, 0, loc)
	last := time.Date(2026, 11, 20, 8, 0, 5, 0, loc) // ja va córrer divendres
	j := jobs.Job{Kind: "daily", Time: "08:00", Days: []int{1, 2, 3, 4, 5}, Enabled: true, LastAt: &last}
	got := jobs.NextDue(j, sat)
	if want := time.Date(2026, 11, 23, 8, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("next due %v, want %v", got, want)
	}
	// Sense dies: demà a les 08:00 com sempre.
	j.Days = nil
	j.LastAt = &sat
	if got := jobs.NextDue(j, sat); !got.Equal(time.Date(2026, 11, 22, 8, 0, 0, 0, loc)) {
		t.Fatalf("every day: %v", got)
	}
}

func TestRoutineToolCreatesListsAndStops(t *testing.T) {
	s := rutinaTestServer(t)
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, time.Local)
	out, err := s.routineTool(routineArgs{Wind: "xaloc", Action: "create", Task: "Busca les 5 notícies més importants de Catalunya", Time: "08:00", Repeat: "weekdays"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !(strings.Contains(out, "feiners a les 08:00") || strings.Contains(out, "weekdays at 08:00")) || !strings.Contains(out, "Mon 16 Nov 08:00") {
		t.Fatalf("create: %s", out)
	}
	list, _ := s.routineTool(routineArgs{Wind: "xaloc", Action: "list"}, now)
	if !strings.Contains(list, "rutina-") || !strings.Contains(list, "notícies") {
		t.Fatalf("list: %s", list)
	}
	if _, err := s.routineTool(routineArgs{Wind: "xaloc", Action: "create", Task: "x"}, now); err == nil {
		t.Fatal("create without time must fail")
	}
	stop, err := s.routineTool(routineArgs{Wind: "xaloc", Action: "stop"}, now)
	if err != nil || !strings.Contains(stop, "Aturada") {
		t.Fatalf("stop: %s %v", stop, err)
	}
	if list, _ := s.routineTool(routineArgs{Wind: "xaloc", Action: "list"}, now); list != "No tens cap rutina." {
		t.Fatalf("after stop: %s", list)
	}
}

func TestPublishLinkPrivateDetection(t *testing.T) {
	for link, want := range map[string]bool{
		"http://192.168.1.20:8097/share/x/": true,
		"http://100.101.5.4:8097/share/x/":  true, // Tailscale
		"http://127.0.0.1:8097/share/x/":    true,
		"http://gregal:8097/share/x/":       true,
		"http://pc.local:8097/share/x/":     true,
		"https://gregal.example.cat/share/x/": false,
		"http://8.8.8.8/share/x/":           false,
	} {
		if got := linkIsPrivate(link); got != want {
			t.Errorf("%s: %v, want %v", link, got, want)
		}
	}
}

func TestWindPublishInsideProjectesNeedsNoApproval(t *testing.T) {
	pd := t.TempDir()
	t.Setenv("GREGAL_PROJECTES", pd)
	site := filepath.Join(pd, "minyons")
	pol := &agent.Policy{ProjectDir: t.TempDir()} // un altre projecte obert
	args := `{"dir":` + strconv.Quote(site) + `,"title":"Minyons"}`
	got := windProjectesPolicy(pol, "publish", args)
	if got.ProjectDir != projectesDir() || !got.WindAutoAllow("publish", args) {
		t.Fatalf("publish of a site in the winds' projects folder must pass alone (project %q)", got.ProjectDir)
	}
	outside := `{"dir":` + strconv.Quote(filepath.Join(t.TempDir(), "x")) + `}`
	if windProjectesPolicy(pol, "publish", outside).WindAutoAllow("publish", outside) {
		t.Fatal("a folder outside the projects must still ask")
	}
}

func TestWindBashInProjectesNeedsNoApproval(t *testing.T) {
	pd := t.TempDir()
	t.Setenv("GREGAL_PROJECTES", pd)
	pol := &agent.Policy{ProjectDir: t.TempDir()}
	dir := filepath.ToSlash(filepath.Join(pd, "rodoreda"))
	args := `{"command":` + strconv.Quote(`cd "`+dir+`" 2>/dev/null || mkdir -p "`+dir+`" && cd "`+dir+`"`) + `}`
	if got := windProjectesPolicy(pol, "bash", args); got.ProjectDir != projectesDir() || !got.WindAutoAllow("bash", args) {
		t.Fatalf("making the site folder in the projects must pass alone (project %q)", got.ProjectDir)
	}
	bad := `{"command":` + strconv.Quote(`cd "`+dir+`" && rm -rf /`) + `}`
	if windProjectesPolicy(pol, "bash", bad).WindAutoAllow("bash", bad) {
		t.Fatal("a destructive command must still ask")
	}
}
