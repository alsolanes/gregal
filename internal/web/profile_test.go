package web

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// El perfil és de la persona: cada compte té el seu, sense límit de comptes, i
// ningú (ni un administrador) en pot llegir o escriure el d'un altre.

type profileBody struct {
	Prefs map[string]struct {
		V *string `json:"v"`
		T int64   `json:"t"`
	} `json:"prefs"`
	Keys []string `json:"keys"`
}

func profileCall(t *testing.T, s *Server, token, method, body string) (int, profileBody) {
	t.Helper()
	req := httptest.NewRequest(method, "/api/profile", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	var out profileBody
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestProfileIsPerPersonForManyAccounts(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_ACCOUNTS_DIR", t.TempDir()+"/accounts")
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")

	// Cinc persones: el perfil de cadascuna és seu.
	tokens := map[string]string{}
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("persona%d", i)
		if w := accountsPost(t, s, `{"name":"`+name+`","password":"12345678"}`); w.Code != 200 {
			t.Fatalf("create %s: %d", name, w.Code)
		}
		tok, _, err := s.users.Login(name, "12345678")
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = tok
	}
	for name, tok := range tokens {
		body := fmt.Sprintf(`{"prefs":{"gregal_tema":{"v":"tema-%s","t":1000},"gregal_idioma":{"v":"ca","t":1000}}}`, name)
		if code, _ := profileCall(t, s, tok, http.MethodPost, body); code != 200 {
			t.Fatalf("%s desa: %d", name, code)
		}
	}
	for name, tok := range tokens {
		code, p := profileCall(t, s, tok, http.MethodGet, "")
		if code != 200 {
			t.Fatalf("%s llegeix: %d", name, code)
		}
		got := p.Prefs["gregal_tema"].V
		if got == nil || *got != "tema-"+name {
			t.Fatalf("%s ha vist el perfil d'un altre: %v", name, got)
		}
		if len(p.Keys) == 0 {
			t.Fatal("la pàgina necessita la llista de claus")
		}
	}
	// L'administrador (clau de servidor) no veu el perfil de cap compte: té el seu.
	if _, p := profileCall(t, s, "admin-secret", http.MethodGet, ""); len(p.Prefs) != 0 {
		t.Fatalf("l'administrador no ha de veure perfils d'altres: %v", p.Prefs)
	}
}

func TestProfileMergeNewestWinsAndOnlyAllowedKeys(t *testing.T) {
	now := int64(10_000_000)
	str := func(v string) *string { return &v }
	cur := profileDoc{Prefs: map[string]profileEntry{"gregal_tema": {V: str("dark"), T: 5000}}}

	// Un canvi més antic no trepitja el més nou.
	cur = mergeProfile(cur, map[string]profileEntry{"gregal_tema": {V: str("light"), T: 4000}}, now)
	if *cur.Prefs["gregal_tema"].V != "dark" {
		t.Fatal("un canvi vell no pot guanyar")
	}
	// Un de més nou, sí; dos dispositius amb claus diferents no es trepitgen.
	cur = mergeProfile(cur, map[string]profileEntry{
		"gregal_tema":     {V: str("calma"), T: 6000},
		"gregal_densitat": {V: str("compacta"), T: 100},
	}, now)
	if *cur.Prefs["gregal_tema"].V != "calma" || *cur.Prefs["gregal_densitat"].V != "compacta" {
		t.Fatalf("fusió: %v", cur.Prefs)
	}
	// Esborrar (v = null) també és un canvi.
	cur = mergeProfile(cur, map[string]profileEntry{"gregal_tema": {V: nil, T: 7000}}, now)
	if e := cur.Prefs["gregal_tema"]; e.V != nil || e.T != 7000 {
		t.Fatalf("esborrat: %+v", e)
	}
	// El que no és de la llista (el token, una sessió) no hi entra mai, ni un valor enorme.
	cur = mergeProfile(cur, map[string]profileEntry{
		"gregal_token":   {V: str("secret"), T: 9000},
		"gregal_session": {V: str("abc"), T: 9000},
		"gregal_mida":    {V: str(strings.Repeat("x", 600)), T: 9000},
	}, now)
	for _, k := range []string{"gregal_token", "gregal_session", "gregal_mida"} {
		if _, ok := cur.Prefs[k]; ok {
			t.Fatalf("%s no havia d'entrar", k)
		}
	}
	// Una hora al futur llunyà no guanya per sempre: es talla a «ara».
	cur = mergeProfile(cur, map[string]profileEntry{"gregal_view": {V: str("normal"), T: 1 << 60}}, now)
	if cur.Prefs["gregal_view"].T != now {
		t.Fatalf("hora futura: %d", cur.Prefs["gregal_view"].T)
	}
}

func TestProfileKeepsNameAndAvatarColourPerAccount(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_ACCOUNTS_DIR", t.TempDir()+"/accounts")
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")
	tokens := map[string]string{}
	for _, name := range []string{"anna", "biel"} {
		if w := accountsPost(t, s, `{"name":"`+name+`","password":"12345678"}`); w.Code != 200 {
			t.Fatalf("create %s: %d", name, w.Code)
		}
		tok, _, err := s.users.Login(name, "12345678")
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = tok
	}

	body := `{"prefs":{"gregal_user_name":{"v":"Anna Puig","t":1000},"gregal_user_color":{"v":"#A8CDE2","t":1000}}}`
	if code, _ := profileCall(t, s, tokens["anna"], http.MethodPost, body); code != 200 {
		t.Fatalf("desa: %d", code)
	}
	_, p := profileCall(t, s, tokens["anna"], http.MethodGet, "")
	if v := p.Prefs["gregal_user_name"].V; v == nil || *v != "Anna Puig" {
		t.Fatalf("nom: %v", v)
	}
	if v := p.Prefs["gregal_user_color"].V; v == nil || *v != "#A8CDE2" {
		t.Fatalf("color: %v", v)
	}
	if _, other := profileCall(t, s, tokens["biel"], http.MethodGet, ""); len(other.Prefs) != 0 {
		t.Fatalf("en Biel no ha de veure el nom de l'Anna: %v", other.Prefs)
	}

	// Un nom massa llarg no s'hi desa: el que hi havia es queda.
	long := strings.Repeat("n", profileMaxValue+1)
	profileCall(t, s, tokens["anna"], http.MethodPost, `{"prefs":{"gregal_user_name":{"v":"`+long+`","t":2000}}}`)
	if _, p := profileCall(t, s, tokens["anna"], http.MethodGet, ""); p.Prefs["gregal_user_name"].V == nil || *p.Prefs["gregal_user_name"].V != "Anna Puig" {
		t.Fatal("un nom massa llarg no s'havia de desar")
	}
}

func TestProfileNeverCarriesSecretsOrDeviceState(t *testing.T) {
	for _, k := range []string{"gregal_token", "gregal_session", "gregal_draft_sessions", "gregal_resumed_conversations",
		"gregal_agent_browser_width", "gregal_preview_width", "gregal_side"} {
		if profileKeys[k] {
			t.Fatalf("%s no ha de viatjar amb el perfil", k)
		}
	}
}

func photoCall(t *testing.T, s *Server, token, method string, body []byte, want int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/profile/photo", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != want {
		t.Fatalf("%s /api/profile/photo: %d %s, vol %d", method, w.Code, w.Body.String(), want)
	}
	return w
}

func mePhoto(t *testing.T, s *Server, token string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	var me struct {
		Photo string `json:"photo"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("/api/me: %d %s", w.Code, w.Body.String())
	}
	return me.Photo
}

func TestProfilePhotoIsPerAccountSquareAndRemovable(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	t.Setenv("GREGAL_ACCOUNTS_DIR", t.TempDir()+"/accounts")
	h := testHubWithUsers(t)
	s := h.def
	s.SetToken("admin-secret")
	tokens := map[string]string{}
	for _, name := range []string{"anna", "biel"} {
		if w := accountsPost(t, s, `{"name":"`+name+`","password":"12345678"}`); w.Code != 200 {
			t.Fatalf("create %s: %d", name, w.Code)
		}
		tok, _, err := s.users.Login(name, "12345678")
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = tok
	}

	// Una foto apaïsada de 600×300: el servidor en fa un quadrat de 256.
	var raw bytes.Buffer
	if err := jpeg.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 600, 300)), nil); err != nil {
		t.Fatal(err)
	}
	w := photoCall(t, s, tokens["anna"], http.MethodPost, raw.Bytes(), 200)
	var up struct {
		Photo string `json:"photo"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &up); err != nil || !strings.HasPrefix(up.Photo, "/api/profile/photo?v=") {
		t.Fatalf("resposta: %s", w.Body.String())
	}
	// Un altre navegador de l'Anna la veu: /api/me en dona l'adreça.
	if got := mePhoto(t, s, tokens["anna"]); got != up.Photo {
		t.Fatalf("/api/me photo = %q, vol %q", got, up.Photo)
	}

	w = photoCall(t, s, tokens["anna"], http.MethodGet, nil, 200)
	img, _, err := image.Decode(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Fatalf("mida %v, vol 256×256", b)
	}

	// La foto de l'Anna no és d'en Biel.
	photoCall(t, s, tokens["biel"], http.MethodGet, nil, 404)
	if got := mePhoto(t, s, tokens["biel"]); got != "" {
		t.Fatalf("en Biel no té foto: %q", got)
	}

	// Ni un fitxer que no és una imatge ni un de més de 5 MB; la foto que hi
	// havia es queda.
	photoCall(t, s, tokens["anna"], http.MethodPost, []byte("no soc una foto"), 400)
	big := append(append([]byte{}, raw.Bytes()...), make([]byte, profilePhotoMaxBytes)...)
	photoCall(t, s, tokens["anna"], http.MethodPost, big, 400)
	photoCall(t, s, tokens["anna"], http.MethodGet, nil, 200)

	photoCall(t, s, tokens["anna"], http.MethodDelete, nil, 200)
	photoCall(t, s, tokens["anna"], http.MethodGet, nil, 404)
	if got := mePhoto(t, s, tokens["anna"]); got != "" {
		t.Fatalf("/api/me encara en dona: %q", got)
	}
}

func TestProfilePhotoTakesOnlyJPEGPNGWebPAndNoHugeImages(t *testing.T) {
	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := squarePhoto(gifBuf.Bytes()); err == nil {
		t.Fatal("a GIF must be refused: the photo takes JPEG, PNG or WebP")
	}
	// A tiny PNG whose header claims 10000×10000: refused before decoding.
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	raw := pngBuf.Bytes()
	binary.BigEndian.PutUint32(raw[16:], 10000)
	binary.BigEndian.PutUint32(raw[20:], 10000)
	binary.BigEndian.PutUint32(raw[29:], crc32.ChecksumIEEE(raw[12:29]))
	if _, err := squarePhoto(raw); err == nil || !strings.Contains(err.Error(), "pixels") {
		t.Fatalf("a 100-megapixel header must be refused: %v", err)
	}
}
