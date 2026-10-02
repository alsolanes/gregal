package web

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
)

// ErrForaArrel és la resposta a qualsevol camí que surti del que li toca a
// l'usuari. El missatge no diu mai què hi ha a fora.
var ErrForaArrel = errors.New("aquest camí no és a la teva carpeta")

// Allow comprova que un camí és dins d'una de les arrels de l'usuari.
//
// Regles:
//   - usuari nil (servidor sense autenticació, ús local) → tot permès, com
//     abans d'aquesta feina;
//   - usuari sense arrels → res permès (només pot xatejar);
//   - la comprovació es fa sobre el camí REAL (enllaços resolts), amb el
//     separador al davant, perquè `/home/x/carla2` no passi com a
//     `/home/x/userb`.
func (u *User) Allow(path string) error {
	if u == nil || u.Tothom {
		return nil
	}
	if len(u.Roots) == 0 {
		return ErrForaArrel
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("camí buit")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	real := realPath(abs)
	for _, root := range u.Roots {
		rr := realPath(root)
		// El separador al davant evita que `/…/carla2` passi com a
		// `/…/userb`. Amb l'arrel `/` (l'amo del servidor) el prefix ja
		// és el separador i no se n'ha d'afegir cap altre.
		prefix := rr
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		if real == rr || strings.HasPrefix(real, prefix) {
			return nil
		}
	}
	return ErrForaArrel
}

// Allowed és Allow per a qui només vol un booleà.
func (u *User) Allowed(path string) bool { return u.Allow(path) == nil }

// Arrel torna l'arrel principal de l'usuari (on s'obre per defecte), o ""
// si no en té cap.
func (u *User) Arrel() string {
	if u == nil || len(u.Roots) == 0 {
		return ""
	}
	return u.Roots[0]
}

// Arrels torna una còpia de les arrels per poder-les enviar al client.
func (u *User) Arrels() []string {
	if u == nil {
		return nil
	}
	out := make([]string, len(u.Roots))
	copy(out, u.Roots)
	return out
}

// dinsArrel diu si `path` és dins `root`, sense resoldre enllaços. Serveix
// per al cas invers: comprovar si un camí del client està sota una arrel
// abans de resoldre'l.
func dinsArrel(path, root string) bool {
	if path == root {
		return true
	}
	return strings.HasPrefix(path, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator))
}

// realPath resol enllaços simbòlics fins on pugui. Si el camí no existeix
// (un fitxer nou, per exemple), resol el primer directori pare que existeixi
// i hi afegeix la resta del camí (resol symlinks i àlies 8.3 a Windows).
func realPath(p string) string {
	clean := filepath.Clean(p)
	if r, err := filepath.EvalSymlinks(clean); err == nil {
		return r
	}
	curr := clean
	var parts []string
	for {
		parent := filepath.Dir(curr)
		if parent == curr || parent == "." || parent == "" {
			break
		}
		parts = append([]string{filepath.Base(curr)}, parts...)
		if r, err := filepath.EvalSymlinks(parent); err == nil {
			return filepath.Join(append([]string{r}, parts...)...)
		}
		curr = parent
	}
	return clean
}

// PathRelatiu torna el camí relatiu a l'arrel de l'usuari, amb l'arrel
// (sagnada) al davant. És el que es pot ensenyar a la interfície sense
// filtrar l'estructura del disc d'un altre.
func (u *User) PathRelatiu(path string) string {
	if u == nil {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	for _, root := range u.Roots {
		rr := realPath(root)
		if dinsArrel(realPath(abs), rr) {
			rel, err := filepath.Rel(rr, realPath(abs))
			if err != nil || rel == "." {
				return filepath.Base(rr)
			}
			return filepath.Join(filepath.Base(rr), rel)
		}
	}
	return filepath.Base(abs)
}

// guardPath és el guard per als handlers: si l'usuari no hi pot anar,
// respon 403 i torna cert (el handler ha de plegar).
func (s *Server) guardPath(w http.ResponseWriter, path string) bool {
	if s == nil || s.user == nil {
		return false
	}
	if err := s.user.Allow(path); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return true
	}
	return false
}
