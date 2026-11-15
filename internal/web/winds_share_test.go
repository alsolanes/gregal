package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/companion"
)

const sharePlan = "Aquí tens el pla:\n\n```gregal-plan\n{\"title\":\"Dissabte a Girona\",\"steps\":[{\"time\":\"10:00\",\"text\":\"Tren\"}]}\n```\n\n![Girona](/api/winds/image?id=0123456789abcdef01234567.jpg)\n\n[el pressupost](/api/winds/doc?name=pressupost.xlsx)"

func putWindImage(t *testing.T, s *Server, name string) {
	t.Helper()
	if err := os.MkdirAll(s.windImagesDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.windImagesDir(), name), []byte("JPEGDATA"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestShareTitleFromCardOrFirstLine(t *testing.T) {
	if got := shareTitle("", sharePlan); got != "Dissabte a Girona" {
		t.Fatalf("títol de la targeta: %q", got)
	}
	if got := shareTitle("", "## Menú de la setmana\n\nDilluns: crema"); got != "Menú de la setmana" {
		t.Fatalf("primera línia: %q", got)
	}
	if got := shareTitle("  El meu  ", sharePlan); got != "El meu" {
		t.Fatalf("el del client mana: %q", got)
	}
}

// L'enllaç públic és una web de només lectura amb el text, la foto copiada i
// cap enllaç a documents privats; no compta per al límit de webs.
func TestWindClipPublishesReadOnlyPage(t *testing.T) {
	rig := sharedTestRig(t)
	putWindImage(t, rig.aleix, "0123456789abcdef01234567.jpg")
	b, _ := json.Marshal(map[string]string{"text": sharePlan, "from": "Gregal"})
	code, out := sharedCall(t, rig.aleix, (*Server).handleWindClip, "POST", "http://192.168.1.20:8080/api/winds/clip", string(b))
	if code != 200 {
		t.Fatalf("clip: %d %v", code, out)
	}
	if out["title"] != "Dissabte a Girona" || out["private"] != true {
		t.Fatalf("títol i avís d'adreça de casa: %v", out)
	}
	if !strings.HasPrefix(out["link"].(string), "http://192.168.1.20:8080/share/") {
		t.Fatalf("enllaç: %v", out["link"])
	}
	id := out["id"].(string)
	m, err := readShareMeta(id)
	if err != nil || !m.Clip || m.Kind != "site" {
		t.Fatalf("meta: %+v %v", m, err)
	}
	page := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/share/"+id+"/", nil)
	req.SetPathValue("id", id)
	req.SetPathValue("path", "")
	handleShareSitePage(page, req)
	html := page.Body.String()
	for _, want := range []string{"Dissabte a Girona", "/app/md.js", "/app/clip-view.js", `img/0123456789abcdef01234567.jpg`, "el pressupost"} {
		if !strings.Contains(html, want) {
			t.Fatalf("la pàgina no porta %q", want)
		}
	}
	if strings.Contains(html, "/api/winds/") || strings.Contains(html, "</script><script>alert") {
		t.Fatalf("la pàgina té rutes privades: %s", html)
	}
	img := httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/share/"+id+"/img/0123456789abcdef01234567.jpg", nil)
	req.SetPathValue("id", id)
	req.SetPathValue("path", "img/0123456789abcdef01234567.jpg")
	handleShareSitePage(img, req)
	if img.Body.String() != "JPEGDATA" {
		t.Fatalf("la foto no s'ha copiat: %d", img.Code)
	}
	for _, x := range listShares(rig.aleix.usuari()) {
		if x.ID == id && !x.Clip {
			t.Fatal("el clip ha de quedar marcat")
		}
	}
}

// Un text amb «</script>» no surt de l'script de dades.
func TestClipPageEscapesText(t *testing.T) {
	page := clipPage("<b>t</b>", "", "", "hola </script><script>alert(1)</script>")
	if strings.Contains(page, "</script><script>alert") || strings.Contains(page, "<b>t</b>") {
		t.Fatalf("no escapa: %s", page)
	}
}

// Enviar a una persona de l'app: li arriba una còpia (amb la foto), l'avís al
// seu Gregal, i ningú més no ho veu.
func TestWindSendToAnotherAccount(t *testing.T) {
	rig := sharedTestRig(t)
	if _, err := rig.carla.winds.Save(companion.Wind{Name: "Gregal", Persona: "Ajuda."}); err != nil {
		t.Fatal(err)
	}
	putWindImage(t, rig.aleix, "0123456789abcdef01234567.jpg")
	b, _ := json.Marshal(map[string]any{"to": []string{"carla", "nobody", "aleix"}, "text": sharePlan, "from": "Gregal"})
	code, out := sharedCall(t, rig.aleix, (*Server).handleWindSend, "POST", "/", string(b))
	if code != 200 {
		t.Fatalf("send: %d %v", code, out)
	}
	if sent := out["sent"].([]any); len(sent) != 1 || sent[0] != "carla" {
		t.Fatalf("només a la Carla (ni a un desconegut ni a un mateix): %v", sent)
	}
	code, out = sharedCall(t, rig.carla, (*Server).handleWindInbox, "GET", "/", "")
	items := out["items"].([]any)
	if code != 200 || len(items) != 1 || out["unread"].(float64) != 1 {
		t.Fatalf("safata de la Carla: %d %v", code, out)
	}
	it := items[0].(map[string]any)
	if it["from"] != "aleix" || it["title"] != "Dissabte a Girona" || strings.Contains(it["text"].(string), "/api/winds/doc") {
		t.Fatalf("element: %v", it)
	}
	// I surt a Desat, amb el text per pintar-lo.
	_, saved := sharedCall(t, rig.carla, (*Server).handleWindsSaved, "GET", "/api/winds/saved", "")
	if first := saved["items"].([]any)[0].(map[string]any); first["kind"] != "received" || first["from"] != "aleix" || first["unread"] != true || !strings.Contains(first["text"].(string), "gregal-plan") {
		t.Fatalf("Desat de la Carla: %v", first)
	}
	// La foto de l'Aleix es veu des del compte de la Carla.
	rec := httptest.NewRecorder()
	rig.carla.handleWindImage(rec, httptest.NewRequest("GET", "/api/winds/image?id=0123456789abcdef01234567.jpg", nil))
	if rec.Code != 200 || rec.Body.String() != "JPEGDATA" {
		t.Fatalf("foto rebuda: %d", rec.Code)
	}
	// L'avís al seu Gregal.
	nov := loadNovetats(rig.carla)
	if len(nov) == 0 || !strings.Contains(nov[len(nov)-1].Text, "Dissabte a Girona") {
		t.Fatalf("novetat: %+v", nov)
	}
	// El Pere no hi veu res.
	_, out = sharedCall(t, rig.pere, (*Server).handleWindInbox, "GET", "/", "")
	if len(out["items"].([]any)) != 0 {
		t.Fatalf("el Pere veu coses d'altres: %v", out)
	}
	// Llegit i oblidat.
	id := it["id"].(string)
	if code, _ := sharedCall(t, rig.carla, (*Server).handleWindInbox, "POST", "/", `{"id":"`+id+`","action":"read"}`); code != 200 {
		t.Fatal("read")
	}
	_, out = sharedCall(t, rig.carla, (*Server).handleWindInbox, "GET", "/", "")
	if out["unread"].(float64) != 0 {
		t.Fatalf("llegit: %v", out)
	}
	if code, _ := sharedCall(t, rig.pere, (*Server).handleWindInbox, "POST", "/", `{"id":"`+id+`","action":"forget"}`); code != 404 {
		t.Fatal("el Pere no pot esborrar res de la Carla")
	}
	sharedCall(t, rig.carla, (*Server).handleWindInbox, "POST", "/", `{"id":"`+id+`","action":"forget"}`)
	_, out = sharedCall(t, rig.carla, (*Server).handleWindInbox, "GET", "/", "")
	if len(out["items"].([]any)) != 0 {
		t.Fatalf("oblidat: %v", out)
	}
}

func TestWindSendNeedsAccounts(t *testing.T) {
	s := goalTestServer(t)
	code, _ := sharedCall(t, s, (*Server).handleWindSend, "POST", "/", `{"to":["x"],"text":"hola"}`)
	if code != 404 {
		t.Fatalf("sense comptes: %d", code)
	}
}
