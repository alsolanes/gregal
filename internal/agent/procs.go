package agent

import (
	"os"
	"strings"
	"sync"

	"gregal/internal/procs"
)

// H3/G8 — processos en segon pla per a l'agent. El bash normal té 30
// segons; això és el que fa possible `npm run dev`, un build sencer o una
// suite de tests llarga sense bloquejar el torn.

var (
	procOnce  sync.Once
	procStore *procs.Store
	// procSession és l'etiqueta de sessió que porten els processos que
	// engega l'agent des del TUI o el headless. El servidor web hi posa
	// l'id de la seva sessió amb StartProc.
	procSession = "agent"
)

// Procs retorna el magatzem de processos en segon pla del procés.
func Procs() *procs.Store {
	procOnce.Do(func() { procStore = procs.New() })
	return procStore
}

// procDir és on s'engeguen: el directori de treball actual.
func procDir() string {
	d, err := os.Getwd()
	if err != nil {
		return "."
	}
	return d
}

// procDirOr és el directori on s'engega un procés de segon pla: el de la
// sessió si en sap, i si no el del procés. Abans sempre el del procés, o
// sigui que un `npm run dev` d'una pestanya que havia canviat de projecte
// arrencava a l'altre.
func procDirOr(dir string) string {
	if strings.TrimSpace(dir) != "" {
		return dir
	}
	return procDir()
}
