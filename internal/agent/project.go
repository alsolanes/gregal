package agent

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"gregal/internal/tools"
)

// toolPath extreu la ruta objectiu de les eines de fitxers ("" si no n'hi ha).
func toolPath(name, argsJSON string) string {
	switch name {
	case "write", "edit", "patch", "read":
	default:
		return ""
	}
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return ""
	}
	return strings.TrimSpace(a.Path)
}

// InsideProject diu si path queda dins projectDir (contenció real, no prefix
// de cadena). Les relatives es resolen contra el projecte; "~", buides i
// escapes ".." queden fora (demanaran permís, no pas automàtic).
func InsideProject(projectDir, path string) bool {
	dir := strings.TrimSpace(projectDir)
	p := strings.TrimSpace(path)
	if dir == "" || p == "" || strings.HasPrefix(p, "~") {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	// Es resol el DIRECTORI primer i la relativa s'hi enganxa després: si
	// el directori porta un nom curt 8.3 (RUNNER~1 al CI de Windows) o un
	// enllaç, resoldre el conjunt sencer falla (el fitxer encara no existeix)
	// i el prefix comparava l'escrit amb el resolt: tot demanava permís.
	if realDir, err := filepath.EvalSymlinks(absDir); err == nil {
		absDir = realDir
	}
	abs := p
	if !tools.IsRooted(p) {
		abs = filepath.Join(absDir, p)
	} else if !filepath.IsAbs(p) {
		// Arrel sense unitat a Windows ("\Windows\...", "/etc/..."): no la
		// tractem mai com a relativa al projecte, o s'hi colaria per Join.
		if a, err := filepath.Abs(p); err == nil {
			abs = a
		}
	}
	abs = filepath.Clean(abs)
	// Resol el que existeixi: si el fitxer encara no hi és (un write a un
	// fitxer nou amb ruta absoluta via enllaç o nom curt), es resol el
	// primer avantpassat existent i s'hi reenganxa la resta. Sense això,
	// comparar l'escrit amb el resolt falla i tot demana permís.
	abs = resolveInexistent(abs)
	return abs == absDir || strings.HasPrefix(abs, absDir+string(filepath.Separator))
}

// resolveInexistent resol enllaços i noms curts fins on el disc permet:
// si el camí sencer no existeix, puja fins al primer avantpassat que sí
// que existeix i hi reenganxa la cua. Mai torna buit.
func resolveInexistent(abs string) string {
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	var cua []string
	curr := abs
	for {
		parent := filepath.Dir(curr)
		if parent == curr {
			return abs
		}
		cua = append([]string{filepath.Base(curr)}, cua...)
		if real, err := filepath.EvalSymlinks(parent); err == nil {
			return filepath.Join(append([]string{real}, cua...)...)
		}
		curr = parent
	}
}
