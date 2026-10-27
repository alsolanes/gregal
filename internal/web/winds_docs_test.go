package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/tools"
)

// Els vents poden crear documents a la seva carpeta (i editar els que
// l'usuari ha adjuntat), però enlloc més.
func TestWindDocDecide(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, args, want string
	}{
		{"pdf_create", `{"path":"informe.pdf","content":"# Hola"}`, "allow"},
		{"office_create", `{"path":"carta.docx"}`, "allow"},
		{"pdf_create", `{"path":"` + filepath.ToSlash(filepath.Join(t.TempDir(), "fora.pdf")) + `"}`, "deny"},
		{"pdf_create", `{"path":"../escapa.pdf"}`, "deny"},
		{"pdf_edit", `{"path":"informe.pdf","op":"extract","pages":"1","out":"p1.pdf"}`, "allow"},
		{"pdf_edit", `{"path":"informe.pdf","op":"split"}`, "allow"},
		{"pdf_edit", `{"path":"` + filepath.ToSlash(filepath.Join(os.TempDir(), "gregal-office-abc-x.pdf")) + `","op":"stamp","text":"OK"}`, "allow"},
		{"pdf_edit", `{"path":"` + filepath.ToSlash(filepath.Join(os.TempDir(), "altre.pdf")) + `","op":"stamp","text":"OK"}`, "deny"},
		{"pdf_create", `{}`, "deny"},
	}
	for _, c := range cases {
		if got, why := windDocDecide(dir, c.name, c.args); got != c.want {
			t.Errorf("%s %s: %s (%s), volia %s", c.name, c.args, got, why, c.want)
		}
	}
	if !windDocTool("pdf_create") || windDocTool("write") || windDocTool("bash") {
		t.Fatal("només les eines de documents")
	}
}

func TestWindDocLinksAndDownload(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	s := goalTestServer(t)
	start := time.Now()
	ctx := s.windDocsCtx(context.Background())
	dir := windDocsOf(ctx)
	if dir == "" {
		t.Fatal("sense carpeta de documents")
	}
	tools.Active = tools.NewJournal()
	if _, err := tools.PdfCreate(ctx, tools.PdfCreateOpts{Path: filepath.Join(dir, "Pressupost obra.pdf"), Title: "Pressupost", Content: "Total: 1.200 €"}); err != nil {
		t.Fatal(err)
	}
	links := s.windDocLinks(start, "Aquí el tens.")
	if !strings.Contains(links, "📄 [Pressupost obra.pdf](/api/winds/doc?name=Pressupost+obra.pdf)") {
		t.Fatalf("enllaç: %q", links)
	}
	// Un document d'abans del torn no surt; un que ja hi és a la resposta, tampoc.
	if got := s.windDocLinks(time.Now().Add(time.Hour), ""); got != "" {
		t.Fatalf("documents antics: %q", got)
	}
	if got := s.windDocLinks(start, "ja hi és: /api/winds/doc?name=Pressupost+obra.pdf"); got != "" {
		t.Fatalf("repetit: %q", got)
	}

	rec := httptest.NewRecorder()
	s.handleWindDoc(rec, httptest.NewRequest(http.MethodGet, "/api/winds/doc?name=Pressupost+obra.pdf", nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(rec.Body.String(), "%PDF") {
		t.Fatalf("baixada: %d %s", rec.Code, rec.Header())
	}
	for _, bad := range []string{"../secret.pdf", "..%2Fsecret.pdf", ".amagat", ""} {
		rec = httptest.NewRecorder()
		s.handleWindDoc(rec, httptest.NewRequest(http.MethodGet, "/api/winds/doc?name="+bad, nil))
		if rec.Code == 200 {
			t.Fatalf("%q no s'hauria de servir", bad)
		}
	}
}
