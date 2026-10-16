package officerender

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Prova real: només si hi ha LibreOffice i GREGAL_RENDER_TEST apunta a
// un fitxer (el render triga uns segons i depèn de la màquina).
func TestRenderReal(t *testing.T) {
	src := os.Getenv("GREGAL_RENDER_TEST")
	if src == "" || Available() == "" {
		t.Skip("sense GREGAL_RENDER_TEST o sense motor")
	}
	out := filepath.Join(filepath.Dir(src), "render-"+filepath.Base(src))
	os.RemoveAll(out)
	res, err := Render(context.Background(), src, Options{OutDir: out, Width: 960})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("motor=%s total=%d pàgines=%v notes=%v", res.Engine, res.Total, res.Pages, res.Notes)
	if len(res.Pages) == 0 {
		t.Fatal("cap pàgina")
	}
	if err := ContactSheet(res.Pages, nil, 3, 420, filepath.Join(out, "contact.png")); err != nil {
		t.Fatal(err)
	}
}

func TestPageList(t *testing.T) {
	got := PageList("3, 1-2,7")
	want := []int{1, 2, 3, 7}
	if len(got) != len(want) {
		t.Fatal(got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatal(got)
		}
	}
}
