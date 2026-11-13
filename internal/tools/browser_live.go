package tools

import (
	"errors"
	"fmt"
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

// liveBrowser és el navegador on treballa (o ha treballat) l'agent, si és viu.
func liveBrowser() (*Browser, time.Time) {
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
		return nil, at
	}
	return b, at
}

// LiveScope és l'abast (sessió) del navegador en directe: la lease de control
// es pren per al seu propietari.
func LiveScope() string {
	liveMu.Lock()
	defer liveMu.Unlock()
	return liveScope
}

// ErrNoLiveBrowser: no hi ha cap navegador obert on actuar.
var ErrNoLiveBrowser = errors.New("browser: no page is open")

// LiveInput és el que la persona fa des del visor en directe (mòbil o web):
// un toc, text, una tecla, desplaçar o tornar enrere. x i y són fraccions
// (0–1) de la captura, que és el viewport sencer. Mai no es registra el text.
type LiveInput struct {
	Kind string  `json:"kind"` // tap, type, key, scroll, back
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Text string  `json:"text,omitempty"`
	Key  string  `json:"key,omitempty"`
	Dir  string  `json:"dir,omitempty"` // up, down
}

// liveKeys són les tecles que el visor pot prémer.
var liveKeys = map[string]bool{"Enter": true, "Tab": true, "Escape": true, "Backspace": true}

// LiveAct fa l'acció de la persona al navegador en directe.
func LiveAct(in LiveInput) error {
	b, _ := liveBrowser()
	if b == nil {
		return ErrNoLiveBrowser
	}
	view, ok := b.computerView()
	if !ok {
		return ErrNoLiveBrowser
	}
	x, y := clamp01(in.X)*view.W, clamp01(in.Y)*view.H
	switch in.Kind {
	case "tap":
		return b.mouse(x, y, "left", 1, 0)
	case "type":
		text := []rune(in.Text)
		if len(text) == 0 || len(text) > 2000 {
			return fmt.Errorf("browser: text must be 1-2000 characters")
		}
		_, err := b.call("Input.insertText", map[string]any{"text": string(text)})
		return err
	case "key":
		if !liveKeys[in.Key] {
			return fmt.Errorf("browser: key %q not allowed", in.Key)
		}
		return b.pressCombo(in.Key)
	case "scroll":
		dy := 600.0
		if in.Dir == "up" {
			dy = -dy
		}
		if in.X == 0 && in.Y == 0 {
			x, y = view.W/2, view.H/2
		}
		_, err := b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseWheel", "x": x, "y": y, "deltaX": 0, "deltaY": dy})
		return err
	case "back":
		_, err := b.evalString("history.back(), ''")
		return err
	}
	return fmt.Errorf("browser: unknown live action %q", in.Kind)
}

// LiveShot és una captura actual (JPEG) de la pestanya on treballa l'agent,
// amb l'adreça i el títol. ok=false si no hi ha cap navegador obert.
func LiveShot() (img []byte, url, title string, used time.Time, ok bool) {
	b, at := liveBrowser()
	if b == nil {
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
