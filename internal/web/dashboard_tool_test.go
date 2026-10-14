package web

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestDashboardSaveCreaIAfegeix(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := hubTestServer(t)
	s.cwd = t.TempDir()
	args := `{"dashboard":"Demo sales","plots":[{"title":"Revenue","type":"line","labels":["Jan","Feb"],"series":[{"label":"EUR","values":[10,20]}]},{"title":"Regions","type":"bar","labels":["N","S"],"series":[{"label":"EUR","values":[3,4]}]}]}`
	out, err := s.dashboardSave(args)
	if err != nil || !strings.Contains(out, "Demo sales") {
		t.Fatalf("save: %v %q", err, out)
	}
	// Segon cop, mateix nom (majúscules diferents): reaprofita el tauler.
	if _, err := s.dashboardSave(strings.Replace(args, "Demo sales", "demo SALES", 1)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.plotStorePath())
	if err != nil {
		t.Fatal(err)
	}
	var lib plotLibrary
	if err := json.Unmarshal(raw, &lib); err != nil {
		t.Fatal(err)
	}
	boards := 0
	for _, b := range lib.Boards {
		if strings.EqualFold(b.Name, "demo sales") {
			boards++
		}
	}
	if boards != 1 || len(lib.Plots) != 4 {
		t.Fatalf("boards=%d plots=%d", boards, len(lib.Plots))
	}
	// replace buida el tauler abans d'afegir.
	if _, err := s.dashboardSave(strings.Replace(args, `"plots"`, `"replace":true,"plots"`, 1)); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(s.plotStorePath())
	_ = json.Unmarshal(raw, &lib)
	if len(lib.Plots) != 2 {
		t.Fatalf("replace: plots=%d", len(lib.Plots))
	}
	if _, err := s.dashboardSave(`{"dashboard":"x","plots":[{"title":"bad","type":"pie","labels":["a"],"series":[{"label":"v","values":[1]}]}]}`); err == nil {
		t.Fatal("invalid chart type must be rejected")
	}
}
