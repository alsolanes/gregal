package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gregal/internal/tools"
)

func TestHandleCheckpointsIRewindTo(t *testing.T) {
	s := goalTestServer(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(a, []byte("v0"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools.Active.SnapOp(a, "write")
	if err := os.WriteFile(a, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	tools.Active.SnapOp(a, "write")
	if err := os.WriteFile(a, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer tools.Active.Rewind()

	req := httptest.NewRequest(http.MethodGet, "/api/checkpoints", nil)
	rec := httptest.NewRecorder()
	s.handleCheckpoints(rec, req)
	if rec.Code != 200 {
		t.Fatalf("checkpoints=%d", rec.Code)
	}
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("n=%d", len(list))
	}
	rec = postJSON(t, s.handleRewindTo, `{"seq":1}`)
	if rec.Code != 200 {
		t.Fatalf("rewind-to=%d: %s", rec.Code, rec.Body.String())
	}
	raw, _ := os.ReadFile(a)
	if string(raw) != "v1" {
		t.Fatalf("a=%q, volia v1", raw)
	}
	// Seq inexistent → 400.
	if rec := postJSON(t, s.handleRewindTo, `{"seq":99}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("seq99=%d, volia 400", rec.Code)
	}
	// Sense seq → 400.
	if rec := postJSON(t, s.handleRewindTo, `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("buit=%d, volia 400", rec.Code)
	}
}
