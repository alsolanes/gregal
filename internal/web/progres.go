package web

import (
	"encoding/json"
	"regexp"
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
// surten de seguida. Un segon: amb tres, el comptador i el raonament en viu
// es quedaven quiets prou estona per semblar penjats. Variable perquè els
// tests l'escurcin.
var intervalProgres = time.Second

// progresTorn emet l'event `progress`: el senyal de vida del torn perquè la
// UI pugui dir "esperant el model 2m47s" o "el model raona (12k caràcters)"
// en comptes de quedar-se muda. Vist en viu el 2026-10-13: després d'un
// web_fetch fallit, 2m47s sense res mentre el model generava (o era a la
// cua del proveïdor), i l'usuari no sabia si l'agent s'havia penjat.
//
// Els comptadors de la crida al model els alimenta llm.StreamProgress
// (cada delta, també en mode recuperació, on el text no surt fins al
// final). Les actualitzacions que només canvien comptadors van com a molt
// cada intervalProgres (vegeu recordProgress).
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
	cua     string // final del raonament de la crida en curs
	einaNom string // eina que el model està escrivint
	einaFi  string // final dels arguments parcials, on es busca el fitxer
	fitxer  string // path o file dels arguments, quan ja és sencer
}

func nouProgresTorn(emit func(string, any)) *progresTorn {
	return &progresTorn{emit: emit, ara: time.Now}
}

// iniciModel marca l'inici d'una crida al model: petició enviada, cap byte.
func (p *progresTorn) iniciModel() {
	p.mu.Lock()
	p.fase, p.estat, p.eines, p.cua = faseModel, estatEsperant, "", ""
	p.einaNom, p.einaFi, p.fitxer = "", "", ""
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

// cuaRaonament és el màxim de caràcters del final del raonament que viatgen
// dins l'event (prou per llegir què pensa el model, poc per al registre).
const cuaRaonament = 280

// raonament és el llm.WithReasoningText de la crida: guarda el final del que
// el model pensa. No emet res: surt amb el següent delta de comptadors.
func (p *progresTorn) raonament(s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fase != faseModel {
		return
	}
	r := []rune(p.cua + s)
	if len(r) > cuaRaonament {
		r = r[len(r)-cuaRaonament:]
	}
	p.cua = string(r)
}

// finestraEina és quants bytes del final dels arguments es guarden per
// trobar-hi el fitxer: prou per a un "path": "…" partit entre deltes, sense
// guardar ni tornar a recórrer els 67k d'un write sencer a cada delta.
const finestraEina = 512

// clauFitxer troba un "path" o "file" amb el valor ja tancat. Dins d'un
// string JSON les cometes van escapades (\"path\"), així que un "path" que
// surti dins del contingut no hi encaixa.
var clauFitxer = regexp.MustCompile(`"(?:path|file)"\s*:\s*("(?:[^"\\]|\\.)*")`)

// eina és el llm.WithToolText de la crida: apunta quina eina s'escriu i, en
// quant apareix sencer, el fitxer objectiu. No emet res: surt amb el
// següent delta de comptadors.
func (p *progresTorn) eina(nom, args string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fase != faseModel {
		return
	}
	if nom != "" && nom != p.einaNom {
		p.einaNom, p.einaFi, p.fitxer = nom, "", ""
	}
	fi := p.einaFi + args
	if m := clauFitxer.FindAllStringSubmatch(fi, -1); m != nil {
		var f string
		if json.Unmarshal([]byte(m[len(m)-1][1]), &f) == nil && f != "" {
			p.fitxer = f
		}
	}
	if len(fi) > finestraEina {
		fi = fi[len(fi)-finestraEina:]
	}
	p.einaFi = fi
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
	p.cua = ""
	p.einaNom, p.einaFi, p.fitxer = "", "", ""
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
		// Només mentre la crida encara només raona: en escriure text o una
		// eina, el raonament ja no és el que passa.
		if p.cua != "" && p.comptes[1] == 0 && p.comptes[2] == 0 {
			ev["tail"] = p.cua
		}
		if p.einaNom != "" {
			ev["tool_name"] = p.einaNom
		}
		if p.fitxer != "" {
			ev["tool_target"] = p.fitxer
		}
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
