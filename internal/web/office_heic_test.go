package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// Una foto HEIC pujada des del mòbil es desa en JPEG: amb nom .heic o amb el
// .jpg que hi posa l'app d'Android quan no reconeix l'extensió.
func TestOfficeUploadConvertsHEIC(t *testing.T) {
	s := goalTestServer(t)
	raw, err := os.ReadFile("../tools/testdata/sample.heic")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"IMG_0001.HEIC", "imatge-123.jpg"} {
		body, _ := json.Marshal(map[string]string{"name": name, "data": base64.StdEncoding.EncodeToString(raw)})
		w := httptest.NewRecorder()
		s.handleOfficeUpload(w, httptest.NewRequest("POST", "/api/office/upload", bytes.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", name, w.Code, w.Body.String())
		}
		var up struct{ Name, Path string }
		json.Unmarshal(w.Body.Bytes(), &up)
		if !strings.HasSuffix(strings.ToLower(up.Path), ".jpg") {
			t.Fatalf("%s: el fitxer desat hauria de ser .jpg: %q", name, up.Path)
		}
		got, err := os.ReadFile(up.Path)
		if err != nil || len(got) < 3 || got[0] != 0xFF || got[1] != 0xD8 {
			t.Fatalf("%s: el fitxer desat no és JPEG (%v)", name, err)
		}
		os.Remove(up.Path)
	}
}
