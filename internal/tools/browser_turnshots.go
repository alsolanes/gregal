package tools

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Captures del torn: el que ha fet l'agent al navegador, per ensenyar-ho al
// fil al costat de la resposta (com «torna amb captures» de Grok). Qui porta
// el torn hi posa un col·lector al context; cada acció del navegador que
// canvia el que es veu hi deixa una captura petita (la mateixa de la
// reproducció, B13). Sense col·lector no es fa res.

// TurnShot és una captura del torn: la imatge (JPEG), on era i què feia.
type TurnShot struct {
	Image  []byte
	URL    string
	Title  string
	Action string
	At     time.Time
}

// TurnShotsMax és quantes captures es guarden per torn: la primera, les
// últimes i, si n'hi ha més, es descarten les del mig.
const TurnShotsMax = 6

// TurnShots col·lecciona les captures d'un torn.
type TurnShots struct {
	mu    sync.Mutex
	shots []TurnShot
}

type turnShotsKey struct{}

// WithTurnShots posa el col·lector al context del torn.
func WithTurnShots(ctx context.Context, c *TurnShots) context.Context {
	return context.WithValue(ctx, turnShotsKey{}, c)
}

func turnShotsFrom(ctx context.Context) *TurnShots {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(turnShotsKey{}).(*TurnShots)
	return c
}

// Shots torna les captures, de la més vella a la més nova.
func (c *TurnShots) Shots() []TurnShot {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]TurnShot(nil), c.shots...)
}

// add guarda una captura. Una de la mateixa pàgina que l'anterior per a una
// acció que no hi ha canviat res (llegir) la substitueix; si se'n passa,
// cau la segona (la primera diu on ha començat).
func (c *TurnShots) add(s TurnShot) {
	if c == nil || len(s.Image) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if n := len(c.shots); n > 0 && c.shots[n-1].URL == s.URL && !turnShotChanges(s.Action) {
		c.shots[n-1] = s
		return
	}
	c.shots = append(c.shots, s)
	if len(c.shots) > TurnShotsMax {
		c.shots = append(c.shots[:1], c.shots[2:]...)
	}
}

// turnShotSkip: accions que no ensenyen res nou (llegir, mirar l'estat).
var turnShotSkip = map[string]bool{
	"read": true, "text": true, "find": true, "eval": true, "snapshot": true, "tabs": true,
	"close": true, "status": true, "cookies": true, "console": true, "network": true,
	"handoff": true, "ask_user": true, "wait": true,
}

// turnShotChanges: accions que canvien la pàgina (una captura nova encara
// que sigui la mateixa adreça).
func turnShotChanges(action string) bool {
	switch action {
	case "click", "type", "fill", "select", "press", "scroll", "drag", "batch", "back", "forward", "upload", "check":
		return true
	}
	return false
}

// noteTurnShot captura la pàgina després d'una acció, si el torn en vol.
func noteTurnShot(ctx context.Context, b *Browser, action string) {
	c := turnShotsFrom(ctx)
	action = strings.ToLower(strings.TrimSpace(action))
	if c == nil || b == nil || turnShotSkip[action] {
		return
	}
	view, ok := b.computerView()
	if !ok {
		return
	}
	img := b.computerShot(view)
	if len(img) == 0 {
		return
	}
	url, _ := b.evalString("location.href")
	title, _ := b.evalString("document.title")
	url = SafePageURL(url)
	if url == "" {
		return // about:blank, pàgines internes
	}
	c.add(TurnShot{Image: img, URL: url, Title: clipRunes(strings.TrimSpace(title), 120), Action: action, At: time.Now()})
}
