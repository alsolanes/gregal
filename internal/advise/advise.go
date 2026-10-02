// Package advise detecta vies de millora (dependències, toolchain,
// higiene del repo, backend empaquetat) i les torna com a propostes.
//
// Principis: només lectura (mai modifica res), mai falla dur (cada
// detector té timeout i recuperació de pànic; sense xarxa o sense eines
// torna "no-disponible", no un error), i funciona sense cap configuració.
package advise

import (
	"context"
	"sort"
	"time"
)

// Severity ordena per importància: un "avis" demana atenció,
// un "suggeriment" és millora opcional, "info" és constatació.
type Severity string

const (
	Info        Severity = "info"
	Suggeriment Severity = "suggeriment"
	Avis        Severity = "avis"
)

// Proposal és una millora proposada, amb caducitat: passats 30 dies
// es considera rància i la botiga la poda.
type Proposal struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Severity  Severity  `json:"severity"`
	Title     string    `json:"title"`
	Detail    string    `json:"detail"`
	Effort    string    `json:"effort,omitempty"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Status del detector: "ok" (res a dir), "proposta" (ha generat
// propostes), "no-aplica" (no hi ha els fitxers que necessita) o
// "no-disponible" (falta xarxa, eina o permís; mai és un error).
type Outcome struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	N      int    `json:"proposals"`
}

// Report és el resultat d'una passada completa.
type Report struct {
	At        time.Time  `json:"at"`
	Proposals []Proposal `json:"proposals"`
	Outcomes  []Outcome  `json:"outcomes"`
}

type detector func(ctx context.Context, root string) ([]Proposal, Outcome)

var detectors = []struct {
	name   string
	online bool // cert si necessita xarxa
	fn     detector
}{
	{"go-toolchain", false, checkGoToolchain},
	{"npm-lock", false, checkNpmLock},
	{"backend-fresc", false, checkBackendFresc},
	{"higiene-repo", false, checkHigieneRepo},
	{"go-desfasat", true, checkGoDesfasat},
	{"npm-desfasat", true, checkNpmDesfasat},
}

// Run passa tots els detectors sobre root (normalment l'arrel del repo).
// Si online és fals, els detectors de xarxa s'ometen. No torna mai error:
// cada detector està aïllat amb timeout i recuperació de pànic.
func Run(ctx context.Context, root string, online bool) Report {
	rep := Report{At: time.Now().UTC()}
	seen := map[string]bool{}
	for _, d := range detectors {
		if d.online && !online {
			rep.Outcomes = append(rep.Outcomes, Outcome{Name: d.name, Status: "no-aplica", Reason: "omet: mode fora de línia"})
			continue
		}
		props, out := safeRun(ctx, d.name, d.fn, root)
		out.N = len(props)
		if len(props) > 0 && out.Status == "ok" {
			out.Status = "proposta"
		}
		rep.Outcomes = append(rep.Outcomes, out)
		for _, p := range props {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			rep.Proposals = append(rep.Proposals, p)
		}
	}
	sort.Slice(rep.Proposals, func(i, j int) bool {
		if rep.Proposals[i].Severity != rep.Proposals[j].Severity {
			return sevRank(rep.Proposals[i].Severity) < sevRank(rep.Proposals[j].Severity)
		}
		return rep.Proposals[i].ID < rep.Proposals[j].ID
	})
	return rep
}

func safeRun(ctx context.Context, name string, fn detector, root string) (props []Proposal, out Outcome) {
	defer func() {
		if rec := recover(); rec != nil {
			props = nil
			out = Outcome{Name: name, Status: "no-disponible", Reason: "error intern del detector"}
		}
	}()
	dctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	return fn(dctx, root)
}

func sevRank(s Severity) int {
	switch s {
	case Avis:
		return 0
	case Suggeriment:
		return 1
	default:
		return 2
	}
}

// newProposal construeix una proposta amb caducitat a 30 dies.
func newProposal(kind string, sev Severity, title, detail, effort, source string) Proposal {
	now := time.Now().UTC()
	sum := fnvID(kind + "\x00" + title)
	return Proposal{
		ID:        kind + "-" + sum,
		Kind:      kind,
		Severity:  sev,
		Title:     title,
		Detail:    detail,
		Effort:    effort,
		Source:    source,
		CreatedAt: now,
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}
}

// fnvID és un hash curt i estable per deduplicar propostes.
func fnvID(s string) string {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime
	}
	const hex = "0123456789abcdef"
	out := make([]byte, 12)
	for i := 11; i >= 0; i-- {
		out[i] = hex[h&0xf]
		h >>= 4
	}
	return string(out)
}
