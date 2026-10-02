package web

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gregal/internal/config"
)

// Un .xlsx anomenat «EWEC EDH Project Plan.xlsx» es desava a tmp amb els
// espais a dins. La ruta es dona a l'agent com a «@ruta» i el lector de
// mencions talla al primer espai: el model rebia mitja ruta i responia, amb
// raó, que el fitxer no existia. Al disc no hi pot haver espais.
func TestOfficePujatSenseEspaisALaRuta(t *testing.T) {
	s := goalTestServer(t)
	cos, _ := json.Marshal(map[string]string{
		"name": "EWEC EDH Project Plan.docx",
		"data": base64.StdEncoding.EncodeToString(testOfficeDocxBytes()),
	})
	w := httptest.NewRecorder()
	s.handleOfficeUpload(w, httptest.NewRequest("POST", "/api/office/upload", strings.NewReader(string(cos))))
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var res struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	json.Unmarshal(w.Body.Bytes(), &res)
	defer os.Remove(res.Path)
	if strings.Contains(res.Path, " ") {
		t.Fatalf("la ruta al disc no pot dur espais: %q", res.Path)
	}
	// El nom bonic es conserva per ensenyar-lo i per baixar-lo.
	if res.Name != "EWEC EDH Project Plan.docx" {
		t.Fatalf("el nom que es veu ha de ser l'original: %q", res.Name)
	}
	// I la ruta que es dona a l'agent ha de sobreviure sencera al lector.
	sortida := s.expandMentions("Revisa @"+res.Path, s.cwd)
	if strings.Contains(sortida, "no such file") || strings.Contains(sortida, "cannot find") {
		t.Fatalf("l'agent no pot llegir el que li hem passat:\n%s", sortida)
	}
}

func TestSenseEspais(t *testing.T) {
	casos := map[string]string{
		"EWEC EDH Project Plan.xlsx": "EWEC_EDH_Project_Plan.xlsx",
		"pressupost, final.xlsx":     "pressupost__final.xlsx",
		`el "bo".docx`:               "el__bo_.docx",
		"acció.pptx":                 "acció.pptx", // els accents es queden
	}
	for in, vol := range casos {
		if got := senseEspais(in); got != vol {
			t.Errorf("senseEspais(%q) = %q, volia %q", in, got, vol)
		}
	}
}

// Amb cometes, una menció admet espais: a Windows les rutes en tenen.
func TestMencioAmbCometes(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "un fitxer amb espais.txt")
	if err := os.WriteFile(p, []byte("hola\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: &config.Config{}, cwd: dir}
	out := s.expandMentions(`mira @"`+p+`" i digues què hi ha`, s.cwd)
	if !strings.Contains(out, "hola") {
		t.Fatalf("no ha llegit el fitxer:\n%s", out)
	}
	// Sense cometes, es talla; el que importa és que no peti i que ho digui.
	if out2 := s.expandMentions("mira @"+p, s.cwd); strings.Contains(out2, "hola") {
		t.Skip("sense cometes també funciona; cap problema")
	}
}
