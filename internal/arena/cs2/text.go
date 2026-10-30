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

var utilCa = map[string]string{"smoke": "fum", "flash": "flaix", "molotov": "molotov", "he": "granada"}

func (x *texts) util(h, kind, zone string) string {
	if x.ca {
		return fmt.Sprintf("%s llança %s a %s", h, utilCa[kind], zone)
	}
	return fmt.Sprintf("%s throws a %s at %s", h, kind, zone)
}

func (x *texts) kill(k, v, w string, hs bool) string {
	s := fmt.Sprintf(x.pick("%s elimina %s (%s)", "%s killed %s (%s)"), k, v, w)
	if hs {
		s += x.pick(", al cap", ", headshot")
	}
	return s
}

func (x *texts) plant(h, site string) string {
	return fmt.Sprintf(x.pick("%s planta la bomba a %s", "%s plants the bomb at %s"), h, site)
}

func (x *texts) defuse(h string, ninja bool) string {
	if ninja {
		return fmt.Sprintf(x.pick("%s desactiva d'amagat! (ninja)", "%s ninja defuses!"), h)
	}
	return fmt.Sprintf(x.pick("%s desactiva la bomba", "%s defuses the bomb"), h)
}

func (x *texts) explode() string { return x.pick("La bomba explota", "The bomb explodes") }

var reasonCa = map[string]string{"elim": "eliminació", "bomb": "bomba", "defuse": "desactivació", "time": "temps"}
var reasonEn = map[string]string{"elim": "elimination", "bomb": "bomb", "defuse": "defuse", "time": "time"}

var tagCa = map[string]string{"pistol": "ronda de pistoles", "clutch": "clutch", "ace": "ACE", "4k": "4 morts", "eco_win": "victòria en eco", "ninja": "ninja", "last_second": "a l'últim segon"}
var tagEn = map[string]string{"pistol": "pistol round", "clutch": "clutch", "ace": "ACE", "4k": "4K", "eco_win": "eco win", "ninja": "ninja defuse", "last_second": "last second"}

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

func (x *texts) half(n int, s [2]int) string {
	if n == 12 {
		return fmt.Sprintf(x.pick("Mitja part: %d-%d. Canvi de banda", "Halftime: %d-%d. Sides switch"), s[0], s[1])
	}
	if n == 24 {
		return fmt.Sprintf(x.pick("%d-%d: pròrroga! $10.000 per jugador", "%d-%d: overtime! $10,000 each"), s[0], s[1])
	}
	return fmt.Sprintf(x.pick("Canvi de banda a la pròrroga (%d-%d)", "Overtime side switch (%d-%d)"), s[0], s[1])
}

func (x *texts) matchEnd(team string, s [2]int) string {
	return fmt.Sprintf(x.pick("%s guanya el partit %d-%d", "%s win the match %d-%d"), team, s[0], s[1])
}
