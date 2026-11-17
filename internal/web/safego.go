package web

import (
	"log"
	"runtime/debug"
)

// recoverGo és el «defer» de qualsevol goroutine de fons: un pànic en una
// goroutine no el recupera net/http i tomba tot el servidor (i amb ell tots
// els vents, les rutines i els clients connectats). Es registra amb la pila i
// la resta segueix.
func recoverGo(name string) {
	if r := recover(); r != nil {
		log.Printf("pànic a %s: %v\n%s", name, r, debug.Stack())
	}
}
