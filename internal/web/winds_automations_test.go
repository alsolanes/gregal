package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/companion"
	"gregal/internal/jobs"
	"gregal/internal/llm"
)

func automationsGet(t *testing.T, s *Server, query string) (items []AutomationItem, counts AutomationCounts) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleWindsAutomations(rec, httptest.NewRequest(http.MethodGet, "/api/winds/automations"+query, nil))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Items  []AutomationItem `json:"items"`
		Counts AutomationCounts `json:"counts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Items, out.Counts
}

// Una sola llista amb tot el que els vents faran sols: recordatoris,
// seguiments, vigilàncies i rutines, ordenada pel proper moment.
func TestAutomationsUnifiedList(t *testing.T) {
	s := rutinaTestServer(t)
	if _, err := s.winds.Save(companion.Wind{Name: "Garbí", Role: "esports"}); err != nil {
		t.Fatal(err)
	}
	soon := time.Now().Add(30 * time.Minute)
	later := time.Now().Add(48 * time.Hour)
	latest := later.Add(time.Hour)
	mustSave := func(j jobs.Job) {
		t.Helper()
		if _, err := s.jobs.Save(j); err != nil {
			t.Fatal(err)
		}
	}
	mustSave(jobs.Job{ID: "remind-xaloc-1", Name: "Recordatori Xaloc", Prompt: "Rega les plantes", Kind: "remind", Enabled: true, NextAt: &later, Wind: "xaloc"})
	mustSave(jobs.Job{ID: "remind-xaloc-2", Name: "Recordatori Xaloc", Prompt: "Com ha anat l'examen?", Kind: "remind", Enabled: true, NextAt: &soon, Wind: "xaloc", Followup: true})
	// El cor de Garbí, creat per l'app antiga sense Wind.
	mustSave(jobs.Job{ID: "wind-garbi", Name: "Cor", Prompt: "vigila", Kind: "event", Enabled: true, NextAt: &latest})
	mustSave(jobs.Job{ID: "rutina-xaloc-1", Name: "Discos nous", Prompt: "mira discos", Kind: "daily", Time: "08:00", Enabled: false, Wind: "xaloc"})
	// Ja ha sonat: és història.
	past := time.Now().Add(-time.Hour)
	mustSave(jobs.Job{ID: "remind-xaloc-3", Prompt: "vell", Kind: "remind", Enabled: false, NextAt: &past, LastAt: &past, Wind: "xaloc"})
	// D'un vent que no és d'aquest compte i un job d'administrador.
	mustSave(jobs.Job{ID: "remind-fantasma-1", Prompt: "x", Kind: "remind", Enabled: true, NextAt: &soon})
	mustSave(jobs.Job{ID: "admin-backup", Prompt: "backup", Kind: "daily", Time: "03:00", Enabled: true})

	items, counts := automationsGet(t, s, "")
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID+":"+it.Kind)
	}
	want := "remind-xaloc-2:followup,remind-xaloc-1:reminder,wind-garbi:watch,rutina-xaloc-1:routine"
	if strings.Join(ids, ",") != want {
		t.Fatalf("ordre/tipus = %v, vull %s", ids, want)
	}
	if items[2].WindName != "Garbí" || items[2].Title != "esports" || !items[2].CanPause {
		t.Fatalf("vigilància = %+v", items[2])
	}
	if counts.Total != 4 || counts.Today != 1 || counts.Paused != 1 || counts.Week != 3 {
		t.Fatalf("comptes = %+v", counts)
	}
	only, _ := automationsGet(t, s, "?wind=garbi")
	if len(only) != 1 || only[0].ID != "wind-garbi" {
		t.Fatalf("filtre per vent = %+v", only)
	}
}

// L'esborrat de /rutines/delete només toca automatitzacions pròpies: abans
// qualsevol job sense vent (el de l'administrador) hi passava.
func TestAutomationDeleteOnlyOwn(t *testing.T) {
	s := rutinaTestServer(t)
	if _, err := s.jobs.Save(jobs.Job{ID: "admin-backup", Prompt: "backup", Kind: "daily", Time: "03:00", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if code := rutinaPost(t, s, "/api/winds/rutines/delete", map[string]any{"id": "admin-backup"}, nil); code != 404 {
		t.Fatalf("esborrar job d'admin: %d", code)
	}
	if _, err := s.jobs.Save(jobs.Job{ID: "wind-xaloc", Prompt: "vigila", Kind: "event", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if code := rutinaPost(t, s, "/api/winds/rutines/delete", map[string]any{"id": "wind-xaloc"}, nil); code != 200 {
		t.Fatalf("esborrar el cor propi: %d", code)
	}
}

// Seguiments: el vent els pot oferir pel seu compte, però amb mesura; i si
// l'usuari els apaga per a aquest vent, l'eina els refusa.
func TestFollowupGuards(t *testing.T) {
	s := rutinaTestServer(t)
	at := time.Now().Add(26 * time.Hour)
	j, err := saveWindReminderFull(s, "xaloc", at.Format(time.RFC3339), "Com ha anat l'entrevista?", "", 0, true)
	if err != nil || !j.Followup {
		t.Fatalf("primer seguiment: %v %+v", err, j)
	}
	if _, err := saveWindReminderFull(s, "xaloc", at.Add(time.Hour).Format(time.RFC3339), "I ara?", "", 0, true); err == nil {
		t.Fatal("dos seguiments a una hora de distància haurien de fallar")
	}
	for i := 1; i <= 2; i++ {
		if _, err := saveWindReminderFull(s, "xaloc", at.Add(time.Duration(i)*24*time.Hour).Format(time.RFC3339), "seguiment", "", 0, true); err != nil {
			t.Fatalf("seguiment %d: %v", i, err)
		}
	}
	if _, err := saveWindReminderFull(s, "xaloc", at.Add(5*24*time.Hour).Format(time.RFC3339), "un de més", "", 0, true); err == nil {
		t.Fatal("el quart seguiment pendent hauria de fallar")
	}
	// Un recordatori demanat per l'usuari no té aquest límit.
	if _, err := saveWindReminderFull(s, "xaloc", at.Add(time.Hour).Format(time.RFC3339), "Rega", "", 0, false); err != nil {
		t.Fatalf("recordatori normal: %v", err)
	}
	acts := remindActionsSince(s.remindOwner(), "xaloc", time.Now().Add(-time.Minute))
	var follow int
	for _, a := range acts {
		if a["kind"] == "followup" && a["id"] != "" {
			follow++
		}
	}
	if follow < 3 {
		t.Fatalf("les targetes de seguiment han de dur id i tipus: %v", acts)
	}

	w, _ := s.winds.ByID("xaloc")
	w.Followups = "off"
	if _, err := s.winds.Save(w); err != nil {
		t.Fatal(err)
	}
	// Un editor antic que no envia el camp no el torna a encendre.
	w.Followups = ""
	w.Role = "Música"
	if _, err := s.winds.Save(w); err != nil {
		t.Fatal(err)
	}
	w, _ = s.winds.ByID("xaloc")
	if w.FollowupsOn() {
		t.Fatal("el desat sense camp ha reactivat els seguiments")
	}
	if windFollowupBrief(w) != "" {
		t.Fatal("amb els seguiments apagats la consigna no els ha d'oferir")
	}
	if _, err := saveWindReminderFull(s, "xaloc", at.Add(9*24*time.Hour).Format(time.RFC3339), "x", "", 0, true); err == nil {
		t.Fatal("amb els seguiments apagats l'eina els ha de refusar")
	}
}

func TestSavedFromText(t *testing.T) {
	text := "Aquí tens el pressupost: [Pressupost obra.pdf](/api/winds/doc?name=Pressupost+obra.pdf) i la web [Cinc llibres](https://srv/share/abc123/).\n" +
		"![Portada](https://img.example/cover.jpg)\n" +
		"Font: [Reuters](https://www.reuters.com/x)\n" +
		"```chart\n{\"title\":\"Vendes\",\"type\":\"bar\",\"labels\":[\"a\"],\"series\":[{\"label\":\"x\",\"values\":[1]}]}\n```\n" +
		"```js\nconst a = '[no](https://dins.codi/)'\n```"
	got := savedFromText(text)
	kinds := map[string]string{}
	for _, it := range got {
		kinds[it.Kind] = it.Title
	}
	if len(got) != 5 || kinds["doc"] != "Pressupost obra.pdf" || kinds["web"] != "Cinc llibres" || kinds["image"] != "Portada" || kinds["link"] != "Reuters" || kinds["chart"] != "Vendes" {
		t.Fatalf("desat = %+v", got)
	}
}

// La llista de desat ve de les converses, amb l'autor, sense duplicats i
// sense documents que ja no existeixen.
func TestSavedCollectsFromChats(t *testing.T) {
	s := rutinaTestServer(t)
	w, _ := s.winds.ByID("xaloc")
	if err := saveWindHistory(s, w, []llm.Message{
		{Role: "user", Content: "fes-me un pdf", Ts: 1},
		{Role: "assistant", Content: "Fet: [Informe.pdf](/api/winds/doc?name=Informe.pdf)", Ts: 2},
		{Role: "assistant", Content: "Torna'l a mirar: [Informe.pdf](/api/winds/doc?name=Informe.pdf) i [Esborrat.pdf](/api/winds/doc?name=Esborrat.pdf)", Ts: 3},
		{Role: "user", Content: "[meu](https://user.link/)", Ts: 4},
	}); err != nil {
		t.Fatal(err)
	}
	dir := s.windDocsDir()
	if err := mkdirWrite(dir, "Informe.pdf"); err != nil {
		t.Fatal(err)
	}
	windHistories.Delete(historyKey(s, "wind", "xaloc"))
	items, err := s.collectSaved("")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Kind != "doc" || items[0].WindName != "Xaloc" || items[0].Ts != 3 {
		t.Fatalf("desat = %+v", items)
	}
}

func mkdirWrite(dir, name string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte("%PDF-1.4"), 0o644)
}

func TestSavedURLFor(t *testing.T) {
	if got := savedURLFor("doc", "http://srv:8240/api/winds/doc?name=A.pdf", "http://x"); got != "/api/winds/doc?name=A.pdf" {
		t.Fatalf("doc = %s", got)
	}
	if got := savedURLFor("web", "/share/abc/", "http://srv:8240"); got != "http://srv:8240/share/abc/" {
		t.Fatalf("web = %s", got)
	}
	if got := savedURLFor("link", "https://a.b/c", "http://srv"); got != "https://a.b/c" {
		t.Fatalf("link = %s", got)
	}
}

func TestStaleRemindLessonsAreDropped(t *testing.T) {
	body := "# Lliçons\n- Recorda que l'hora del servidor és UTC.\n- No invoquis l'eina `remind`: retorna \"eina desconeguda\".\n- Si promets avisar l'usuari en una hora concreta, crea el recordatori amb una eina de calendari.\n- Per instal·lar dependències fes servir npm ci.\n"
	got := lessonLines(body)
	if len(got) != 2 || strings.Contains(strings.Join(got, " "), "remind") {
		t.Fatalf("lliçons = %v", got)
	}
}
