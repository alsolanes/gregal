package web

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/webp"
)

type charactersManifest struct {
	Size, Fps, MaskSize int
	Files, Mask, Cap    string
	Silhouettes         map[string]string
	Caps                []string
	Sizes               map[string][2]int
	Moods               map[string]struct {
		Frames, Still int
		Loop          *bool
	}
}

// Els personatges de plastilina (tools/characters/build.py) són fulls de
// fotogrames en fila: el manifest ha de dir exactament quins fitxers hi ha i
// de quina mida, perquè els reproductors hi confien sense mirar-los.
func TestCharactersManifestMatchesSheets(t *testing.T) {
	m := loadCharactersManifest(t)
	expected := map[string]bool{"app/characters/manifest.json": true}
	for silhouette := range m.Silhouettes {
		for mood, spec := range m.Moods {
			if spec.Frames <= 0 || spec.Loop == nil || spec.Still < 0 || spec.Still >= spec.Frames {
				t.Fatalf("%s: frames=%d loop=%v still=%d", mood, spec.Frames, spec.Loop, spec.Still)
			}
			names := strings.NewReplacer("<silhouette>", silhouette, "<mood>", mood)
			size, maskSize := m.Size, m.MaskSize
			if own, ok := m.Sizes[silhouette]; ok {
				size, maskSize = own[0], own[1]
			}
			sheets := map[string]int{m.Files: size, m.Mask: maskSize}
			if slices.Contains(m.Caps, silhouette) {
				sheets[m.Cap] = maskSize
			}
			for pattern, size := range sheets {
				path := "app/characters/" + names.Replace(pattern)
				expected[path] = true
				checkSheet(t, path, spec.Frames, size)
			}
		}
	}
	err := fs.WalkDir(assets, "app/characters", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !expected[path] {
			t.Errorf("%s no és al manifest", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// El paquet minairó té la seva figura (barba fora de la màscara, la barretina
// en una màscara pròpia perquè cada minairó la tenyeixi del seu color) i, amb
// ella i els objectes de la feina (142), tots els fulls junts han de cabre en 2 MB.
// El pelut (155), a 256 px, té el seu propi pressupost.
func TestCharactersMinairoWithinBudget(t *testing.T) {
	m := loadCharactersManifest(t)
	if family := m.Silhouettes["minairo"]; family != "minairo" {
		t.Fatalf("el manifest no té el minairó: %q", family)
	}
	if !slices.Equal(m.Caps, []string{"minairo"}) || m.Cap != "<silhouette>/<mood>.cap.webp" {
		t.Fatalf("la barretina del minairó no té màscara pròpia: caps=%v cap=%q", m.Caps, m.Cap)
	}
	total := 0
	err := fs.WalkDir(assets, "app/characters", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasPrefix(path, "app/characters/pelut/") {
			raw, readErr := assets.ReadFile(path)
			total += len(raw)
			return readErr
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if total > 2_000_000 {
		t.Errorf("els personatges fan %d bytes, més de 2 MB", total)
	}
}

// La web els demana a /app/: el reproductor (characters.js), el manifest i
// els fulls, amb un tipus que el navegador accepti.
func TestCharactersServedToTheWeb(t *testing.T) {
	mux := goalTestServer(t).Hub().mux()
	for path, contentType := range map[string]string{
		"/app/characters.js":                  "text/javascript",
		"/app/characters/manifest.json":       "application/json",
		"/app/characters/pebble/idle.webp":    "image/webp",
		"/app/characters/curl/done.mask.webp": "image/webp",
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), contentType) || rec.Body.Len() == 0 {
			t.Errorf("%s: status=%d type=%q bytes=%d", path, rec.Code, rec.Header().Get("Content-Type"), rec.Body.Len())
		}
	}
	page := httptest.NewRecorder()
	mux.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/vents/", nil))
	if !strings.Contains(page.Body.String(), `src="/app/characters.js?fgv=`) {
		t.Error("la pàgina no carrega characters.js")
	}
}

func loadCharactersManifest(t *testing.T) charactersManifest {
	t.Helper()
	raw, err := assets.ReadFile("app/characters/manifest.json")
	if err != nil {
		t.Fatalf("manifest no incrustat: %v", err)
	}
	var m charactersManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if m.Size <= 0 || m.Size > 128 || m.MaskSize <= 0 || m.Fps <= 0 || len(m.Silhouettes) == 0 || len(m.Moods) == 0 {
		t.Fatalf("manifest incomplet: %+v", m)
	}
	return m
}

func checkSheet(t *testing.T, path string, frames, size int) {
	t.Helper()
	sheet, err := assets.ReadFile(path)
	if err != nil {
		t.Fatalf("falta %s: %v", path, err)
	}
	cfg, err := webp.DecodeConfig(bytes.NewReader(sheet))
	if err != nil {
		t.Fatalf("%s no és WebP: %v", path, err)
	}
	if cfg.Width != frames*size || cfg.Height != size {
		t.Fatalf("%s fa %dx%d, el manifest diu %d fotogrames de %d", path, cfg.Width, cfg.Height, frames, size)
	}
}

// El pelut (155) té el pèl a 256 px perquè es vegi fi a la pantalla d'un mòbil,
// amb la màscara a 128; tots els seus fulls junts no passen d'1,2 MB.
func TestCharactersPelutAt256WithinBudget(t *testing.T) {
	m := loadCharactersManifest(t)
	if m.Silhouettes["pelut"] != "pelut" || m.Sizes["pelut"] != [2]int{256, 128} {
		t.Fatalf("el pelut no és al manifest a 256 px: %q %v", m.Silhouettes["pelut"], m.Sizes["pelut"])
	}
	total := 0
	err := fs.WalkDir(assets, "app/characters/pelut", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			raw, readErr := assets.ReadFile(path)
			total += len(raw)
			return readErr
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if total > 1_200_000 {
		t.Errorf("els fulls del pelut fan %d bytes, més d'1,2 MB", total)
	}
}
