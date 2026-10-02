package tools

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Journal registra l'estat previ dels fitxers tocats per Write/Edit
// per desfer (rewind) els canvis de la sessió. Només en memòria.
//
// A més de l'original de primer toc (per al rewind total), desa un
// esdeveniment per CADA escriptura amb la imatge prèvia: això permet
// checkpoints per pas (RewindTo) — tornar a qualsevol punt, no només
// al principi. Només cobreix Write/Edit; bash pot modificar fitxers
// per fora (sed -i, etc.) i aquests canvis no es desfan.
type Journal struct {
	mu     sync.Mutex
	orig   map[string]*[]byte // nil = no existia
	order  []string
	events []Event
	// marks: propietari -> seq -> llargada de conversa (-1 = sense marca).
	// El journal és un de sol per procés i el web hi té diverses sessions
	// alhora: sense el propietari, la marca de la sessió B es llegia en
	// rebobinar la A i li retallava la conversa a una llargada que no era
	// seva. El TUI i el Telegram, que només en tenen una, fan servir "".
	marks map[string]map[int]int
}

// Event és una escriptura amb la imatge prèvia (per desfer-la).
type Event struct {
	Seq  int    // 1, 2, 3… (checkpoint N = "estat després de l'esdeveniment N")
	Op   string // "write" o "edit"
	Path string
	At   time.Time
	// Before: contingut previ (nil = el fitxer no existia).
	Before *[]byte
}

// NewJournal crea un journal buit (per tests).
func NewJournal() *Journal { return &Journal{orig: map[string]*[]byte{}} }

// Active és el journal global: Write/Edit hi desen l'original.
var Active = NewJournal()

// Snap desa l'original una sola vegada (la primera que es toca) i registra
// l'esdeveniment amb la imatge prèvia (per als checkpoints per pas).
// S'ha de cridar ABANS de modificar (Write/Edit ja ho fan).
func (j *Journal) Snap(path string) { j.SnapOp(path, "write") }

// SnapOp és Snap amb l'operació explícita.
func (j *Journal) SnapOp(path, op string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	raw, err := os.ReadFile(path)
	var before *[]byte
	if err == nil {
		cp := append([]byte(nil), raw...)
		before = &cp
	}
	j.events = append(j.events, Event{Seq: len(j.events) + 1, Op: op, Path: path, At: time.Now(), Before: before})
	if _, ok := j.orig[path]; ok {
		return
	}
	j.orig[path] = before
	j.order = append(j.order, path)
}

// Files retorna els fitxers tocats, en ordre de primer toc.
func (j *Journal) Files() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.order...)
}

// Checkpoints retorna els esdeveniments (un per escriptura), en ordre.
// Checkpoint N = "estat després de l'esdeveniment N".
func (j *Journal) Checkpoints() []Event {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]Event(nil), j.events...)
}

// Seq retorna el darrer número d'esdeveniment (0 = cap canvi).
func (j *Journal) Seq() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.events)
}

// Rewind restaura originals (o esborra els que no existien), en ordre
// invers, i buida el journal. No esborra directoris creats.
func (j *Journal) Rewind() (string, error) { return j.RewindTo(0) }

// MarkConvo desa la llargada de conversa del seq actual: el frontend la
// crida en tancar cada torn (amb len(convo)). RewindTo la fa servir per
// saber fins on retallar l'historial.
func (j *Journal) MarkConvo(n int) { j.MarkConvoFor("", n) }

// MarkConvoFor és MarkConvo per a una sessió concreta.
func (j *Journal) MarkConvoFor(owner string, n int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.marks == nil {
		j.marks = map[string]map[int]int{}
	}
	if j.marks[owner] == nil {
		j.marks[owner] = map[int]int{}
	}
	j.marks[owner][len(j.events)] = n
}

// ConvoLenAt torna la llargada de conversa marcada per a seq (el marcatge
// més proper per sota si no n'hi ha d'exacte; 0 si seq <= 0 o no hi ha
// cap marca). -1 = sense informació (no retallar).
func (j *Journal) ConvoLenAt(seq int) int { return j.ConvoLenAtFor("", seq) }

// ConvoLenAtFor és ConvoLenAt mirant només les marques d'una sessió.
func (j *Journal) ConvoLenAtFor(owner string, seq int) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	if seq <= 0 {
		return 0
	}
	meves := j.marks[owner]
	best, ok := -1, false
	for s := range meves {
		if s <= seq && (!ok || s > best) {
			best, ok = s, true
		}
	}
	if !ok {
		return -1
	}
	return meves[best]
}

// RewindTo desfà els esdeveniments amb Seq > seq (en ordre invers,
// restaurant cada imatge prèvia) i deixa el journal a l'estat del
// checkpoint seq. seq=0 ho desfà tot. Fora de rang → error.
func (j *Journal) RewindTo(seq int) (string, error) { return j.RewindFiltered(seq, nil) }

// RewindFiltered és RewindTo però desfent només els esdeveniments el path
// dels quals passa el filtre keep (nil = tots). Amb diverses sessions
// obertes sobre projectes diferents, el journal és únic per procés: el
// filtre evita que desfer a una pestanya toqui els fitxers de l'altra.
// Els esdeveniments que el filtre deixa fora es conserven i es renumeren.
func (j *Journal) RewindFiltered(seq int, keep func(string) bool) (string, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if seq < 0 || seq > len(j.events) {
		return "", fmt.Errorf("checkpoint %d inexistent (0-%d)", seq, len(j.events))
	}
	var kept, gone []string
	var errs []string
	var survivors []Event
	for i := len(j.events) - 1; i >= seq; i-- {
		if keep != nil && !keep(j.events[i].Path) {
			survivors = append(survivors, j.events[i])
			continue
		}
		p := j.events[i].Path
		if j.events[i].Before == nil {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				errs = append(errs, p+": "+err.Error())
			} else {
				gone = append(gone, p)
			}
			continue
		}
		if err := os.WriteFile(p, *j.events[i].Before, 0o644); err != nil {
			errs = append(errs, p+": "+err.Error())
		} else {
			kept = append(kept, p)
		}
	}
	j.events = j.events[:seq]
	// Els esdeveniments que el filtre ha deixat fora sobreviuen, en ordre
	// original, amb Seq renumerat.
	for i := len(survivors) - 1; i >= 0; i-- {
		e := survivors[i]
		e.Seq = len(j.events) + 1
		j.events = append(j.events, e)
	}
	for _, meves := range j.marks {
		for ms := range meves {
			if ms > seq {
				delete(meves, ms)
			}
		}
	}
	// Reconstrueix primer-toc amb el que queda (per a futurs rewinds).
	j.orig = map[string]*[]byte{}
	j.order = nil
	for _, e := range j.events {
		if _, ok := j.orig[e.Path]; ok {
			continue
		}
		j.orig[e.Path] = e.Before
		j.order = append(j.order, e.Path)
	}
	var parts []string
	if len(kept) > 0 {
		parts = append(parts, "restaurats: "+strings.Join(kept, ", "))
	}
	if len(gone) > 0 {
		parts = append(parts, "esborrats: "+strings.Join(gone, ", "))
	}
	if len(errs) > 0 {
		return strings.Join(parts, " · "), fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	if len(parts) == 0 {
		return "res a desfer", nil
	}
	return strings.Join(parts, " · "), nil
}
