package web

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"gregal/internal/llm"
)

// Fases de l'event SSE `progress` (docs/api-contract.md).
const (
	faseModel    = "model"
	faseEines    = "tools"
	faseRevisio  = "review"
	faseCompacta = "compact"

	estatEsperant = "waiting"
	estatRebent   = "receiving"
	estatCorrent  = "running"
)

// intervalProgres és el mínim entre dues actualitzacions de comptadors
// d'una crida al model. Els canvis d'estat (waiting → receiving) i de fase
// surten de seguida. Variable perquè els tests l'escurcin.
var intervalProgres = 3 * time.Second

// progresTorn emet l'event `progress`: el senyal de vida del torn perquè la
// UI pugui dir "esperant el model 2m47s" o "el model raona (12k caràcters)"
// en comptes de quedar-se muda. Vist en viu el 2026-10-13: després d'un
// web_fetch fallit, 2m47s sense res mentre el model generava (o era a la
// cua del proveïdor), i l'usuari no sabia si l'agent s'havia penjat.
//
// Els comptadors de la crida al model els alimenta llm.StreamProgress
// (cada delta, també en mode recuperació, on el text no surt fins al
// final). Les actualitzacions que només canvien comptadors van com a molt
// cada intervalProgres; el volum durable queda baix (vegeu recordProgress).
type progresTorn struct {
	emit func(string, any)
	ara  func() time.Time

	mu      sync.Mutex
	fase    string
	estat   string
	des     time.Time
	eines   string
	comptes [3]int // reasoning, text, tool de la crida en curs
	emesos  [3]int // comptes de l'última emissió
	emesAt  time.Time
}

func nouProgresTorn(emit func(string, any)) *progresTorn {
	return &progresTorn{emit: emit, ara: time.Now}
}

// iniciModel marca l'inici d'una crida al model: petició enviada, cap byte.
func (p *progresTorn) iniciModel() {
	p.mu.Lock()
	p.fase, p.estat, p.eines = faseModel, estatEsperant, ""
	p.des = p.ara()
	p.comptes, p.emesos = [3]int{}, [3]int{}
	ev := p.emetLocked()
	p.mu.Unlock()
	p.emit("progress", ev)
}

// esperaModel torna a "waiting" (reintent del proveïdor o d'un stream
// interromput): el següent primer tros tornarà a dir "receiving".
func (p *progresTorn) esperaModel() {
	p.mu.Lock()
	if p.fase != faseModel || p.estat == estatEsperant {
		p.mu.Unlock()
		return
	}
	p.estat = estatEsperant
	ev := p.emetLocked()
	p.mu.Unlock()
	p.emit("progress", ev)
}

// delta és el llm.StreamProgress de la crida al model.
func (p *progresTorn) delta(kind string, n int) {
	p.mu.Lock()
	if p.fase != faseModel {
		p.mu.Unlock()
		return
	}
	canviEstat := false
	switch kind {
	case llm.ProgressFirst:
		canviEstat = p.estat != estatRebent
		p.estat = estatRebent
	case llm.ProgressReasoning:
		p.comptes[0] += n
	case llm.ProgressText:
		p.comptes[1] += n
	case llm.ProgressTool:
		p.comptes[2] += n
	}
	// Un delta sense primer tros previ (no hauria de passar) també és
	// senyal que arriben bytes.
	if kind != llm.ProgressFirst && p.estat != estatRebent {
		p.estat, canviEstat = estatRebent, true
	}
	if !canviEstat && (p.comptes == p.emesos || p.ara().Sub(p.emesAt) < intervalProgres) {
		p.mu.Unlock()
		return
	}
	ev := p.emetLocked()
	p.mu.Unlock()
	p.emit("progress", ev)
}

// fiModel tanca la crida: una última actualització només si els comptadors
// han canviat des de l'última emissió (així la UI veu el total).
func (p *progresTorn) fiModel() {
	p.mu.Lock()
	if p.fase != faseModel || p.comptes == p.emesos {
		p.mu.Unlock()
		return
	}
	ev := p.emetLocked()
	p.mu.Unlock()
	p.emit("progress", ev)
}

// faseNova emet una fase que no és del model (tools, review, compact).
func (p *progresTorn) faseNova(fase string, eines []string) {
	p.mu.Lock()
	p.fase, p.estat = fase, estatCorrent
	p.eines = strings.Join(eines, ",")
	p.des = p.ara()
	p.comptes, p.emesos = [3]int{}, [3]int{}
	ev := p.emetLocked()
	p.mu.Unlock()
	p.emit("progress", ev)
}

// emetLocked construeix el payload i apunta l'emissió. Cal tenir p.mu.
func (p *progresTorn) emetLocked() map[string]string {
	p.emesos = p.comptes
	p.emesAt = p.ara()
	ev := map[string]string{
		"phase": p.fase,
		"state": p.estat,
		"since": p.des.UTC().Format(time.RFC3339Nano),
	}
	switch p.fase {
	case faseModel:
		ev["reasoning"] = strconv.Itoa(p.comptes[0])
		ev["text"] = strconv.Itoa(p.comptes[1])
		ev["tool"] = strconv.Itoa(p.comptes[2])
	case faseEines:
		ev["tools"] = p.eines
	}
	return ev
}

// textProgres és el text curt de l'event durable (p. ex. "model ·
// receiving"): no buit perquè recordActive no el descarti.
func textProgres(ev map[string]string) string {
	fase := ev["phase"]
	if fase == "" {
		fase = "progress"
	}
	if fase == faseEines && ev["tools"] != "" {
		return fase + " · " + ev["tools"]
	}
	if ev["state"] == "" {
		return fase
	}
	return fase + " · " + ev["state"]
}
