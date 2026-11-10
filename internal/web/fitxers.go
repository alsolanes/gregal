package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gregal/internal/profile"
)

// fitxersPrefix són els enllaços de fitxers enviats al xat. La clau del camí és
// aleatòria i prou llarga: qui la té, té el fitxer (com els enllaços de vista
// prèvia), i així el mòbil el pot baixar sense capçaleres d'autenticació.
const fitxersPrefix = "/f/"

// maxFitxerEnviat limita què s'envia pel xat: un APK hi cap de sobres, una
// imatge de disc no.
const maxFitxerEnviat = 250 << 20

func (s *Server) fitxersDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, profile.DirName(), "fitxers")
}

// publicaFitxer copia un fitxer del disc a la carpeta d'enllaços públics i en
// torna l'adreça pública (base del servidor + clau + nom).
func (s *Server) publicaFitxer(ctx context.Context, path string) (string, string, int64, error) {
	ruta := strings.TrimSpace(path)
	if ruta == "" {
		return "", "", 0, errors.New("falta la ruta del fitxer")
	}
	if strings.HasPrefix(ruta, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			ruta = filepath.Join(home, ruta[2:])
		}
	}
	abs, err := filepath.Abs(ruta)
	if err != nil {
		return "", "", 0, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", 0, err
	}
	if !info.Mode().IsRegular() {
		return "", "", 0, errors.New("no és un fitxer normal")
	}
	if info.Size() > maxFitxerEnviat {
		return "", "", 0, errors.New("el fitxer és massa gran per enviar-lo pel xat")
	}
	clau := make([]byte, 12)
	if _, err := rand.Read(clau); err != nil {
		return "", "", 0, err
	}
	id := hex.EncodeToString(clau)
	nom := filepath.Base(abs)
	dir := filepath.Join(s.fitxersDir(), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", 0, err
	}
	if err := copiaFitxer(abs, filepath.Join(dir, nom)); err != nil {
		return "", "", 0, err
	}
	base := strings.TrimSuffix(linkBaseOf(ctx), "/")
	enllac := base + fitxersPrefix + id + "/" + nom
	return enllac, nom, info.Size(), nil
}

func copiaFitxer(des, a string) error {
	in, err := os.Open(des)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(a, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// handleFitxer serveix un fitxer enviat al xat: /f/<clau>/<nom>.
func (s *Server) handleFitxer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, fitxersPrefix)
	id, nom, ok := strings.Cut(rest, "/")
	if !ok || len(id) != 24 || len(nom) == 0 || strings.Contains(nom, "..") || strings.Contains(id, "..") {
		http.NotFound(w, r)
		return
	}
	if _, err := hex.DecodeString(id); err != nil {
		http.NotFound(w, r)
		return
	}
	ruta := filepath.Join(s.fitxersDir(), id, filepath.Base(nom))
	if info, err := os.Stat(ruta); err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(nom)+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, ruta)
}
