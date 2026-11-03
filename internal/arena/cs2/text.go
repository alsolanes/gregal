package cs2

import (
	"fmt"
	"strings"
)

// texts: frases curtes dels esdeveniments (català o anglès).
type texts struct{ ca bool }

func textsFor(lang string) *texts { return &texts{ca: lang == "ca"} }

func (x *texts) pick(ca, en string) string {
	if x.ca {
		return ca
	}
	return en
}

func (x *texts) roundStart(n int, t, ct string) string {
	return fmt.Sprintf(x.pick("Ronda %d: %s (T) contra %s (CT)", "Round %d: %s (T) vs %s (CT)"), n, t, ct)
}

var buyCa = map[string]string{"eco": "estalvia (eco)", "half": "fa mitja compra", "force": "força la compra", "full": "fa compra completa"}
var buyEn = map[string]string{"eco": "saves (eco)", "half": "half buys", "force": "force buys", "full": "full buys"}

func (x *texts) buy(team, level string, spent int) string {
	if x.ca {
		return fmt.Sprintf("%s %s (%s)", team, buyCa[level], money(spent))
	}
	return fmt.Sprintf("%s %s (%s)", team, buyEn[level], money(spent))
}

func (x *texts) move(h, zone string) string {
	return fmt.Sprintf(x.pick("%s cap a %s", "%s to %s"), h, zone)
}

var utilCa = map[string]string{"smoke": "boira", "flash": "flaix d'escuma", "molotov": "bassal de sabó", "he": "globus d'aigua"}
var utilEn = map[string]string{"smoke": "fog cloud", "flash": "foam flash", "molotov": "soap slick", "he": "water balloon"}

func (x *texts) util(h, kind, zone string) string {
	if x.ca {
		return fmt.Sprintf("%s llança %s a %s", h, utilCa[kind], zone)
	}
	return fmt.Sprintf("%s throws a %s at %s", h, utilEn[kind], zone)
}

func (x *texts) kill(k, v, w string, hs bool) string {
	s := fmt.Sprintf(x.pick("%s remulla %s (%s)", "%s splashed %s (%s)"), k, v, w)
	if hs {
		s += x.pick(", esquitxada a la cara", ", face splash")
	}
	return s
}

func (x *texts) plant(h, site string) string {
	return fmt.Sprintf(x.pick("%s planta el Megaglobus a %s", "%s plants the Mega Balloon at %s"), h, site)
}

func (x *texts) defuse(h string, ninja bool) string {
	if ninja {
		return fmt.Sprintf(x.pick("%s el buida d'amagat! (ninja)", "%s ninja drains it!"), h)
	}
	return fmt.Sprintf(x.pick("%s buida el Megaglobus", "%s drains the Mega Balloon"), h)
}

func (x *texts) explode() string { return x.pick("El Megaglobus esclata: tothom xop!", "The Mega Balloon bursts: everyone soaked!") }

var reasonCa = map[string]string{"elim": "tots remullats", "bomb": "esclat del globus", "defuse": "globus buidat", "time": "temps"}
var reasonEn = map[string]string{"elim": "all splashed", "bomb": "balloon burst", "defuse": "balloon drained", "time": "time"}

var tagCa = map[string]string{"pistol": "ronda de pistoles d'aigua", "clutch": "clutch", "ace": "ACE", "4k": "4 remullats", "eco_win": "victòria en eco", "ninja": "ninja", "last_second": "a l'últim segon"}
var tagEn = map[string]string{"pistol": "water-pistol round", "clutch": "clutch", "ace": "ACE", "4k": "4 splashes", "eco_win": "eco win", "ninja": "ninja drain", "last_second": "last second"}

func (x *texts) roundEnd(team, reason string, tags []string, clutch int) string {
	var t []string
	for _, g := range tags {
		s := tagEn[g]
		if x.ca {
			s = tagCa[g]
		}
		if g == "clutch" {
			s = fmt.Sprintf("clutch 1v%d", clutch)
		}
		t = append(t, s)
	}
	extra := ""
	if len(t) > 0 {
		extra = " — " + strings.Join(t, ", ")
	}
	if x.ca {
		return fmt.Sprintf("%s guanya la ronda (%s)%s", team, reasonCa[reason], extra)
	}
	return fmt.Sprintf("%s win the round (%s)%s", team, reasonEn[reason], extra)
}

func (x *texts) half(n, h int, s [2]int) string {
	if n == h {
		return fmt.Sprintf(x.pick("Mitja part: %d-%d. Canvi de banda", "Halftime: %d-%d. Sides switch"), s[0], s[1])
	}
	if n == 2*h {
		return fmt.Sprintf(x.pick("%d-%d: pròrroga! $10.000 per jugador", "%d-%d: overtime! $10,000 each"), s[0], s[1])
	}
	return fmt.Sprintf(x.pick("Canvi de banda a la pròrroga (%d-%d)", "Overtime side switch (%d-%d)"), s[0], s[1])
}

func (x *texts) matchEnd(team string, s [2]int) string {
	return fmt.Sprintf(x.pick("%s guanya el partit %d-%d", "%s win the match %d-%d"), team, s[0], s[1])
}
