package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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

// maxEsperaProces és el màxim que bash_output pot esperar en una crida: el
// mateix ordre que un test llarg, i prou curt perquè un procés penjat no
// deixi el torn quiet massa estona.
const maxEsperaProces = 300

// esperaProces és el wait/until de bash_output. Torna una línia que diu
// com ha acabat l'espera, perquè el model no torni a esperar a cegues.
func esperaProces(ctx context.Context, p *procs.Proc, segons float64, until string) string {
	if segons <= 0 {
		segons = 60
	}
	if segons > maxEsperaProces {
		segons = maxEsperaProces
	}
	t0 := time.Now()
	motiu := p.WaitFor(ctx, time.Duration(segons*float64(time.Second)), until)
	fet := time.Since(t0).Round(100 * time.Millisecond)
	switch motiu {
	case "matched":
		return fmt.Sprintf("(waited %s: the output contains %q)\n", fet, until)
	case "exited":
		return fmt.Sprintf("(waited %s: the process finished)\n", fet)
	case "timeout":
		return fmt.Sprintf("(waited %s: still running; wait again or stop it with bash_kill)\n", fet)
	}
	return ""
}
