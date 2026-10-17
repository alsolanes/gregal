package main

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// serveShutdown tanca el servidor un sol cop, passi el que passi primer:
// SIGTERM/Ctrl+C o, amb --lifeline, que es tanqui l'stdin.
//
// La línia de vida és per a les apps que engeguen el backend com a procés
// fill (Android, i també serveix a l'escriptori): l'app li deixa l'stdin
// obert i, si mor (el sistema la mata, una caiguda), el pipe es tanca i el
// backend surt net en comptes de quedar orfe gastant bateria amb el port i
// el lock agafats.
type serveShutdown struct {
	once  sync.Once
	steps []func()
	exit  func(int)
}

func newServeShutdown(steps ...func()) *serveShutdown {
	return &serveShutdown{steps: steps, exit: os.Exit}
}

// run executa els passos en ordre i surt amb codi 0.
func (s *serveShutdown) run() {
	s.once.Do(func() {
		for _, step := range s.steps {
			if step != nil {
				step()
			}
		}
		s.exit(0)
	})
}

// watchSignals tanca en rebre SIGTERM o Interrupt.
func (s *serveShutdown) watchSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		s.run()
	}()
}

// watchLifeline tanca quan l'stdin arriba al final (el pare ha mort o l'ha
// tancat per demanar una aturada neta).
func (s *serveShutdown) watchLifeline(stdin io.Reader) {
	go func() {
		_, _ = io.Copy(io.Discard, stdin)
		s.run()
	}()
}

// cancelGrace és el marge perquè els torns cancel·lats deixin el seu final
// al registre abans de sortir. Si no hi arriben, el proper arrencada els
// tanca igualment (CloseOrphanRuns).
const cancelGrace = 500 * time.Millisecond
