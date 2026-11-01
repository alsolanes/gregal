package web

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Els projectes que demana la persona no es deixen a l'arrel del repo del
// Gregal: tenen la seva carpeta (~/gregal/projectes/<nom>). Aquí es prova que
// el vent hi treballa de debò i que el previsualitzador ho ensenya. El permís
// d'escriptura es prova a internal/agent (also_dirs_test.go), que és on viu la
// decisió.

func projectesTest(t *testing.T) (string, string) {
	t.Helper()
	proj := t.TempDir()
	repo := t.TempDir()
	// El repo es reconeix pel go.mod del mòdul gregal.
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module gregal\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GREGAL_PROJECTES", proj)
	return proj, repo
}

// Amb la sessió apuntant al repo del Gregal, el vent treballa a la carpeta de
// projectes: una web no acaba barrejada amb el codi del servei.
func TestVentTreballaAlaCarpetaDeProjectes(t *testing.T) {
	proj, repo := projectesTest(t)
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.mu.Lock()
	s.cwd = repo
	s.mu.Unlock()

	scope := s.windLoopScope()
	if scope.workspace != proj {
		t.Fatalf("el vent hauria de treballar a %s, treballa a %s", proj, scope.workspace)
	}
	if scope.policy.ProjectDir != proj {
		t.Fatalf("ProjectDir hauria de ser %s, és %s", proj, scope.policy.ProjectDir)
	}
	// El repo continua essent escrivible: una feina sobre el Gregal no es trenca.
	trobat := false
	for _, d := range scope.policy.AlsoDirs {
		if d == repo {
			trobat = true
		}
	}
	if !trobat {
		t.Fatalf("el repo hauria de quedar a AlsoDirs: %+v", scope.policy.AlsoDirs)
	}
	if _, err := os.Stat(proj); err != nil {
		t.Fatalf("la carpeta de projectes s'hauria de crear sola: %v", err)
	}
}

// Si la persona ha triat una carpeta (un repo de debò), el vent hi treballa.
func TestVentRespectaLaCarpetaTriada(t *testing.T) {
	_, _ = projectesTest(t)
	altre := t.TempDir()
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.mu.Lock()
	s.cwd = altre
	s.mu.Unlock()
	if scope := s.windLoopScope(); scope.workspace != altre {
		t.Fatalf("hauria de treballar a %s, treballa a %s", altre, scope.workspace)
	}
}

// El previsualitzador publica les dues carpetes i enllaça els projectes.
func TestPreviewPublicaLaCarpetaDeProjectes(t *testing.T) {
	proj, repo := projectesTest(t)
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s, _ := companionTestServer(t)
	s.mu.Lock()
	s.cwd = repo
	s.mu.Unlock()
	if err := os.MkdirAll(filepath.Join(proj, "joc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "joc", "index.html"), []byte("<p>joc"), 0o644); err != nil {
		t.Fatal(err)
	}

	// L'arrel llista les dues carpetes.
	cos := previewFetch(t, s, s.previewBase())
	for _, vull := range []string{"projectes", "carpeta", "joc/"} {
		if !strings.Contains(cos, vull) {
			t.Fatalf("l'índex hauria d'ensenyar %q:\n%s", vull, cos)
		}
	}
	// I la web del projecte se serveix pel seu slug.
	if got := previewFetch(t, s, s.previewBase()+"projectes/joc/index.html"); got != "<p>joc" {
		t.Fatalf("no serveix la web del projecte: %q", got)
	}
	// `/api/preview` les marca amb el nom de la carpeta, perquè l'app sàpiga
	// on és encara que el repo també tingui un index.html.
	roots := s.previewRoots()
	if len(roots) != 2 || roots[0].Slug != "projectes" || roots[1].Slug != "carpeta" {
		t.Fatalf("roots inesperats: %+v", roots)
	}
	webs := previewWebs(proj)
	if len(webs) != 1 || webs[0].Path != "joc/index.html" {
		t.Fatalf("webs del projecte: %+v", webs)
	}
}

// El brief diu la ruta exacta: el vent no ha d'endevinar on és la carpeta.
func TestBriefDiuLaRutaDelsProjectes(t *testing.T) {
	proj, _ := projectesTest(t)
	brief := windProjectesBrief(filepath.Join(proj, "joc"))
	if !strings.Contains(brief, proj) {
		t.Fatalf("el brief no diu la carpeta de projectes: %s", brief)
	}
	if !strings.Contains(brief, "WORKING FOLDER") {
		t.Fatalf("el brief no diu la carpeta del torn: %s", brief)
	}
}

func previewFetch(t *testing.T, s *Server, u string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handlePreview(rec, httptest.NewRequest("GET", u, nil))
	return rec.Body.String()
}
