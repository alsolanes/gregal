package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// arenaEventCfg: el nom i la data de la competició tal com surten a la web de
// Gregal Jocs. L'organitzador (admin) ho canvia; es desa a settings.json com
// {"event": {...}}, de manera que el nom de l'empresa no és mai al codi.
type arenaEventCfg struct {
	Name    string `json:"name"`
	Tagline string `json:"tagline"`
	Date    int64  `json:"date"` // unix; 0 = sense data
	Place   string `json:"place"`
	// LogoHash: set when the organiser uploaded a logo (GET /api/arena/logo).
	LogoHash string `json:"logo_hash,omitempty"`
}

const (
	arenaEventDefaultName    = "Gregal Prompt Cup"
	arenaEventDefaultTagline = "Write the prompt. The model plays. The best instructions win."
)

func (s *Server) arenaEvent() arenaEventCfg {
	var st struct {
		Event arenaEventCfg `json:"event"`
	}
	arenaRead(filepath.Join(s.arenaDir(), "settings.json"), &st)
	c := st.Event
	if strings.TrimSpace(c.Name) == "" {
		c.Name = arenaEventDefaultName
	}
	if strings.TrimSpace(c.Tagline) == "" {
		c.Tagline = arenaEventDefaultTagline
	}
	c.LogoHash = s.arenaLogoHash()
	return c
}

func (s *Server) saveArenaEvent(c arenaEventCfg) arenaEventCfg {
	clip := func(v string, n int) string {
		v = strings.TrimSpace(v)
		if r := []rune(v); len(r) > n {
			v = string(r[:n])
		}
		return v
	}
	c.Name, c.Tagline, c.Place = clip(c.Name, 60), clip(c.Tagline, 140), clip(c.Place, 80)
	c.LogoHash = ""
	if c.Date < 0 {
		c.Date = 0
	}
	arenaMu.Lock()
	path := filepath.Join(s.arenaDir(), "settings.json")
	raw := map[string]json.RawMessage{}
	arenaRead(path, &raw)
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	b, _ := json.Marshal(c)
	raw["event"] = b
	arenaWrite(path, raw)
	arenaMu.Unlock()
	return s.arenaEvent()
}

// arenaTotals: les xifres de la portada (tots els partits, no només els
// recents que veu cadascú).
func arenaTotals(sums []arenaSum) map[string]int {
	players := map[string]bool{}
	t := map[string]int{}
	for i := range sums {
		m := &sums[i].M
		t["matches"]++
		if m.Kind == "official" {
			t["official"]++
		}
		if m.Stats != nil {
			t["decisions"] += m.Stats.Decisions
		}
		for _, o := range m.Owners {
			if o != "" && !strings.HasPrefix(o, "bot:") {
				players[o] = true
			}
		}
	}
	t["players"] = len(players)
	return t
}

// ---- the company logo ----
//
// The organiser can put the company's logo in the site header. It lives on
// the server (<arena>/logo.txt, a data: URL), never in the code, so the same
// build can carry any company's branding. GET /api/arena returns only its
// hash (event.logo_hash); the site fetches the image once per hash from
// GET /api/arena/logo.

const arenaLogoMax = 200 << 10 // characters of the data: URL

var arenaLogoPrefixes = []string{"data:image/png;base64,", "data:image/jpeg;base64,", "data:image/webp;base64,", "data:image/svg+xml;base64,"}

func (s *Server) arenaLogoPath() string { return filepath.Join(s.arenaDir(), "logo.txt") }

func (s *Server) arenaLogo() string {
	raw, err := os.ReadFile(s.arenaLogoPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func (s *Server) arenaLogoHash() string {
	l := s.arenaLogo()
	if l == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(l))
	return hex.EncodeToString(sum[:6])
}

// saveArenaLogo stores the logo ("" removes it).
func (s *Server) saveArenaLogo(logo string) error {
	logo = strings.TrimSpace(logo)
	if logo == "" {
		if err := os.Remove(s.arenaLogoPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if len(logo) > arenaLogoMax {
		return fmt.Errorf("the logo is too big (max %d KB as an image file)", arenaLogoMax*3/4>>10)
	}
	ok := false
	for _, p := range arenaLogoPrefixes {
		if strings.HasPrefix(logo, p) {
			if _, err := base64.StdEncoding.DecodeString(logo[len(p):]); err != nil {
				return fmt.Errorf("the logo is not a valid image")
			}
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("the logo must be a PNG, JPEG, WebP or SVG image")
	}
	if err := os.MkdirAll(s.arenaDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.arenaLogoPath(), []byte(logo), 0o644)
}

// handleArenaLogo: GET → {logo} ("" when there is none).
func (s *Server) handleArenaLogo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"logo": s.arenaLogo()})
}
