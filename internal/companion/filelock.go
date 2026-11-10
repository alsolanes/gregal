package companion

import (
	"os"
	"path/filepath"
)

// LockFile pren un blocatge exclusiu ENTRE processos sobre path (el crea si
// cal) i torna la funció que l'allibera. És el mateix blocatge que fa servir
// winds.json, per a altres fitxers que dos servidors del mateix perfil
// (Gregal i Gregal Jocs) poden llegir-modificar-escriure alhora. Si no es
// pot blocar (sistema de fitxers sense blocatges, disc de només lectura),
// continua sense blocar: mai no trenca qui el crida.
func LockFile(path string) (unlock func()) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return func() {}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() {}
	}
	if err := lockWindFile(f); err != nil {
		_ = f.Close()
		return func() {}
	}
	return func() { unlockDisc(f) }
}
