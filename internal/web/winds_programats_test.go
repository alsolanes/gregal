package web

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"gregal/internal/jobs"
)

// TestProgramatsMostraRecordatorisICors: la vista de programats ha de llistar
// els recordatoris pendents amb el nom del vent i els cors amb la pròxima
// mirada; i les stats han de comptar bé batecs, parlats i silencis.
func TestProgramatsMostraRecordatorisICors(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := remindTestServer(t)
	w, ok := s.winds.ByRef("Xaloc")
	if !ok {
		t.Fatalf("no trobo el vent de prova")
	}
	// Un recordatori pendent demà.
	at := time.Now().Add(24 * time.Hour)
	if _, err := s.jobs.Save(jobs.Job{
		ID: "remind-" + w.ID + "-1999", Name: "Recordatori", Kind: "remind",
		Prompt: "passa la revisió", Enabled: true, NextAt: &at,
	}); err != nil {
		t.Fatal(err)
	}
	// Un cor amb dues mirades: una amb sortida (parlat) i una en silenci.
	if _, err := s.jobs.Save(jobs.Job{
		ID: WindJobID(w.ID), Name: "Cor de Xaloc", Kind: "event",
		Enabled: true, NextAt: &at,
	}); err != nil {
		t.Fatal(err)
	}
	for i, out := range []string{"Xaloc: va sortir un disc", "", ""} {
		status := "ok"
		if i == 2 {
			status = "error"
		}
		_ = s.jobs.AddRun(jobs.Run{ID: fmt.Sprintf("run-prog-%d", i), JobID: WindJobID(w.ID), Status: status, Output: out})
	}

	req := httptest.NewRequest("GET", "/api/winds/programats", nil)
	rec := httptest.NewRecorder()
	s.handleWindsProgramats(rec, req)
	if rec.Code != 200 {
		t.Fatalf("codi %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Reminders []struct {
			WindName string `json:"wind_name"`
			Text     string `json:"text"`
		} `json:"reminders"`
		Hearts []struct {
			WindName   string `json:"wind_name"`
			LastStatus string `json:"last_status"`
		} `json:"hearts"`
		Stats []struct {
			WindName  string `json:"wind_name"`
			Beats     int    `json:"beats"`
			Spoke     int    `json:"spoke"`
			Silent    int    `json:"silent"`
			Reminders int    `json:"reminders"`
		} `json:"stats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Reminders) != 1 || got.Reminders[0].WindName != "Xaloc" || got.Reminders[0].Text != "passa la revisió" {
		t.Errorf("recordatoris: %+v", got.Reminders)
	}
	if len(got.Hearts) != 1 || got.Hearts[0].WindName != "Xaloc" {
		t.Errorf("cors: %+v", got.Hearts)
	}
	var st *struct {
		WindName  string `json:"wind_name"`
		Beats     int    `json:"beats"`
		Spoke     int    `json:"spoke"`
		Silent    int    `json:"silent"`
		Reminders int    `json:"reminders"`
	}
	for i := range got.Stats {
		if got.Stats[i].WindName == "Xaloc" {
			st = &got.Stats[i]
		}
	}
	if st == nil {
		t.Fatalf("sense stats de Xaloc: %+v", got.Stats)
	}
	// Tres runs del cor: 2 ok (1 parlat + 1 silenci) i 1 error → tots compten com a batecs.
	if st.Beats != 3 || st.Spoke != 1 || st.Silent != 2 {
		t.Errorf("stats del cor: beats=%d spoke=%d silent=%d, calia 3/1/2", st.Beats, st.Spoke, st.Silent)
	}
}

func programatsGet(t *testing.T, s *Server) (hearts, stats int) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindsProgramats(rec, httptest.NewRequest("GET", "/api/winds/programats", nil))
	var got struct {
		Hearts []json.RawMessage `json:"hearts"`
		Stats  []json.RawMessage `json:"stats"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("codi %d: %v", rec.Code, err)
	}
	return len(got.Hearts), len(got.Stats)
}

// Amb comptes, ni el cor sense amo ni els seus batecs surten a cap compte;
// el cor amb amo, només al seu.
func TestProgramatsOnlyOwnHeartbeatWithAccounts(t *testing.T) {
	rig := sharedTestRig(t)
	sameWindTwoAccounts(t, rig)
	seedLegacyHeart(t, rig.aleix, rig.mestral.ID)
	for name, s := range map[string]*Server{"aleix": rig.aleix, "carla": rig.carla} {
		if hearts, stats := programatsGet(t, s); hearts != 0 || stats != 0 {
			t.Fatalf("%s veu un cor sense amo: %d cors, %d stats", name, hearts, stats)
		}
	}
	if _, err := rig.aleix.jobs.Save(jobs.Job{ID: WindJobID(rig.mestral.ID), Name: "Cor", Prompt: "vigila", Kind: "event",
		Enabled: true, Wind: rig.mestral.ID, WindOwner: "aleix"}); err != nil {
		t.Fatal(err)
	}
	if err := rig.aleix.jobs.AddRun(jobs.Run{ID: "run-cor-aleix", JobID: WindJobID(rig.mestral.ID), Wind: rig.mestral.ID,
		WindOwner: "aleix", Status: "ok", FinishedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if hearts, stats := programatsGet(t, rig.aleix); hearts != 1 || stats != 1 {
		t.Fatalf("l'Aleix: %d cors, %d stats", hearts, stats)
	}
	if hearts, stats := programatsGet(t, rig.carla); hearts != 0 || stats != 0 {
		t.Fatalf("la Carla veu el cor de l'Aleix: %d cors, %d stats", hearts, stats)
	}
}
