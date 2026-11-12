package tools

import (
	"strings"
	"sync"
	"time"
)

// Vista en directe del navegador de Gregal quan no hi ha cap ordinador remot
// (mode local: el navegador d'aquest servidor és l'ordinador). El visor del
// mòbil i de la web demana una captura cada segon; no s'engega cap navegador
// per fer-la: si Gregal no en té cap d'obert, ho diu.

var (
	liveMu    sync.Mutex
	liveScope string
	liveAt    time.Time
)

// noteLiveScope recorda l'últim navegador que ha fet servir l'agent.
func noteLiveScope(scope string) {
	liveMu.Lock()
	liveScope, liveAt = scope, time.Now()
	liveMu.Unlock()
}

// LiveShot és una captura actual (JPEG) de la pestanya on treballa l'agent,
// amb l'adreça i el títol. ok=false si no hi ha cap navegador obert.
func LiveShot() (img []byte, url, title string, used time.Time, ok bool) {
	liveMu.Lock()
	scope, at := liveScope, liveAt
	liveMu.Unlock()
	browserMu.Lock()
	b := browserSessions[scope]
	if scope == "" || b == nil {
		b = browserInst
	}
	browserMu.Unlock()
	if b == nil || !b.alive() {
		return nil, "", "", at, false
	}
	view, vok := b.computerView()
	if !vok {
		return nil, "", "", at, false
	}
	img = b.computerShot(view)
	if len(img) == 0 {
		return nil, "", "", at, false
	}
	url, _ = b.evalString("location.href")
	title, _ = b.evalString("document.title")
	return img, strings.TrimSpace(url), clipRunes(strings.TrimSpace(title), 120), at, true
}
