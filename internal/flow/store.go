package flow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Els grafs viuen al projecte, a .gregal/flows/, no a la carpeta de
// l'usuari: un procediment («com es revisa un PR aquí», «com es prepara un
// lliurament») és del repositori i ha de viatjar amb ell, per a tothom que
// hi treballi. És el mateix criteri que .gregal/memory.md.
const Dir = ".gregal/flows"

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slug converteix un nom en nom de fitxer.
func Slug(nom string) string {
	s := slugRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(nom)), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "flux"
	}
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

// DirFor retorna la carpeta de grafs d'un projecte.
func DirFor(projecte string) string { return filepath.Join(projecte, filepath.FromSlash(Dir)) }

// Desa escriu el graf. Valida abans: un graf trencat al disc és una trampa
// per a demà.
func Desa(projecte string, f *Flow) (string, error) {
	if err := f.Validate(); err != nil {
		return "", err
	}
	dir := DirFor(projecte)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, Slug(f.Name)+".json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Carrega llegeix un graf pel nom (o pel seu slug).
func Carrega(projecte, nom string) (*Flow, error) {
	path := filepath.Join(DirFor(projecte), Slug(nom)+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no hi ha cap flux %q a %s", nom, Dir)
	}
	var f Flow
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("el flux %q no es pot llegir: %w", nom, err)
	}
	return &f, nil
}

// Esborra treu un graf.
func Esborra(projecte, nom string) error {
	path := filepath.Join(DirFor(projecte), Slug(nom)+".json")
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("no hi ha cap flux %q", nom)
	}
	return os.Remove(path)
}

// Resum és una entrada del llistat.
type Resum struct {
	Nom    string `json:"name"`
	Slug   string `json:"slug"`
	Desc   string `json:"desc,omitempty"`
	Passos int    `json:"steps"`
}

// Llista els grafs del projecte, per ordre alfabètic. Un fitxer il·legible
// se salta en comptes de tombar el llistat sencer.
func Llista(projecte string) ([]Resum, error) {
	fitxers, err := filepath.Glob(filepath.Join(DirFor(projecte), "*.json"))
	if err != nil {
		return nil, err
	}
	out := []Resum{}
	for _, p := range fitxers {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var f Flow
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		out = append(out, Resum{Nom: f.Name, Slug: strings.TrimSuffix(filepath.Base(p), ".json"),
			Desc: f.Desc, Passos: len(f.Nodes)})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Nom) < strings.ToLower(out[j].Nom) })
	return out, nil
}
