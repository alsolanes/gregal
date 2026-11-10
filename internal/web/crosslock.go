package web

import (
	"path/filepath"
	"sync"

	"gregal/internal/companion"
	"gregal/internal/service"
)

// crossLock és un candau que val dins del procés i ENTRE processos del
// mateix perfil. Gregal i Gregal Jocs poden córrer alhora sobre les mateixes
// dades (service.SetProduct), i alguns fitxers els escriuen tots dos: la
// safata «Avui» i les veus proactives dels vents (la lliga hi apunta el
// resultat de cada partit amb RecordWindNews). Amb només el mutex del
// procés, un llegir-modificar-escriure de l'un podia esborrar el que l'altre
// acabava d'afegir. Es fa servir com un sync.Mutex (Lock/Unlock); el
// blocatge de fitxer és un per perfil (<dades>/<name>), prou curt per no
// notar-se.
type crossLock struct {
	name   string
	mu     sync.Mutex
	unlock func()
}

func (l *crossLock) Lock() {
	l.mu.Lock()
	l.unlock = companion.LockFile(filepath.Join(service.DataDir(), l.name))
}

// ReadLock és per a qui només llegeix: l'escriptura és atòmica (temporal +
// rename), així que n'hi ha prou amb el mutex del procés, sense tocar el
// disc (una lectura no ha de crear el fitxer de blocatge).
func (l *crossLock) ReadLock()   { l.mu.Lock() }
func (l *crossLock) ReadUnlock() { l.mu.Unlock() }

func (l *crossLock) Unlock() {
	if u := l.unlock; u != nil {
		l.unlock = nil
		u()
	}
	l.mu.Unlock()
}
