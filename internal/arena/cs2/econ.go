package cs2

import (
	"fmt"
	"sort"
	"strings"
)

// Economia de CS2 (regles competitives).
const (
	startMoney   = 800
	otMoney      = 10000
	maxMoney     = 16000
	winElim      = 3250
	winBomb      = 3500 // T per explosió, CT per desactivació
	plantBonus   = 800  // a cada T si planten i perden
	plantReward  = 300
	defuseReward = 300
	kevlarCost   = 650
	helmetCost   = 1000
	kitCost      = 400
)

// lossBonus: escala 1400/1900/2400/2900/3400 (nivell 0..4). Regla CS2: el
// nivell puja amb cada derrota i baixa un graó (no torna a zero) amb cada
// victòria; cada meitat comença al nivell 1.
func lossBonus(level int) int {
	if level < 0 {
		level = 0
	}
	if level > 4 {
		level = 4
	}
	return 1400 + 500*level
}

type weapon struct {
	ID     string
	Name   string
	Cost   int
	Power  float64
	Class  string // pistol, smg, rifle, awp
	Side   int    // 0 tots, 1 T, 2 CT
	Reward int
	HS     float64
}

var weapons = map[string]weapon{
	"glock":  {"glock", "Glock-18", 0, 0.56, "pistol", 1, 300, 0.58},
	"usp":    {"usp", "USP-S", 0, 0.58, "pistol", 2, 300, 0.62},
	"p250":   {"p250", "P250", 300, 0.64, "pistol", 0, 300, 0.55},
	"deagle": {"deagle", "Desert Eagle", 700, 0.72, "pistol", 0, 300, 0.6},
	"mac10":  {"mac10", "MAC-10", 1050, 0.78, "smg", 1, 600, 0.36},
	"mp9":    {"mp9", "MP9", 1250, 0.79, "smg", 2, 600, 0.36},
	"galil":  {"galil", "Galil AR", 1800, 0.9, "rifle", 1, 300, 0.45},
	"famas":  {"famas", "FAMAS", 2050, 0.9, "rifle", 2, 300, 0.45},
	"ak47":   {"ak47", "AK-47", 2700, 1.0, "rifle", 1, 300, 0.52},
	"m4a1s":  {"m4a1s", "M4A1-S", 2900, 0.98, "rifle", 2, 300, 0.48},
	"awp":    {"awp", "AWP", 4750, 1.06, "awp", 0, 100, 0.08},
}

func nadeCost(n string, side int) int {
	switch n {
	case "smoke", "he":
		return 300
	case "flash":
		return 200
	case "molotov":
		if side == sideCT {
			return 500
		}
		return 400
	}
	return 0
}

const (
	sideT  = 1
	sideCT = 2
)

func sidePistol(side int) string {
	if side == sideCT {
		return "usp"
	}
	return "glock"
}
func sideRifle(side int) string {
	if side == sideCT {
		return "m4a1s"
	}
	return "ak47"
}
func sideCheapRifle(side int) string {
	if side == sideCT {
		return "famas"
	}
	return "galil"
}
func sideSMG(side int) string {
	if side == sideCT {
		return "mp9"
	}
	return "mac10"
}

// Player: un jugador amb atributs (idèntics al seu mirall de l'altre equip).
type Player struct {
	ID     string
	Handle string
	Role   string
	Team   int
	Slot   int
	Aim    float64
	Util   float64
	Move   float64
	Clutch float64

	Money   int
	Primary string
	Pistol  string
	Armor   int // 0, 1 armilla, 2 armilla+casc
	Kit     bool
	Nades   []string

	K, D, HS, FK, Clutches, Plants, Defuses, MultiK, Dmg int
}

func (p *Player) weapon(side int) weapon {
	if p.Primary != "" {
		return weapons[p.Primary]
	}
	if p.Pistol != "" {
		return weapons[p.Pistol]
	}
	return weapons[sidePistol(side)]
}

func (p *Player) strip() {
	p.Primary, p.Pistol, p.Armor, p.Kit, p.Nades = "", "", 0, false, nil
}

func (p *Player) addMoney(v int) {
	p.Money += v
	if p.Money > maxMoney {
		p.Money = maxMoney
	}
	if p.Money < 0 {
		p.Money = 0
	}
}

func (p *Player) count(n string) int {
	c := 0
	for _, x := range p.Nades {
		if x == n {
			c++
		}
	}
	return c
}

func (p *Player) take(n string) bool {
	for i, x := range p.Nades {
		if x == n {
			p.Nades = append(p.Nades[:i], p.Nades[i+1:]...)
			return true
		}
	}
	return false
}

// fullCost: el que costa la compra completa d'aquest jugador ara.
func fullCost(p *Player, side int) int {
	c := 0
	if p.Primary == "" || weapons[p.Primary].Class == "smg" {
		c += weapons[sideRifle(side)].Cost
	}
	if p.Armor < 2 {
		c += helmetCost
		if p.Armor == 1 {
			c -= kevlarCost
		}
	}
	c += 500
	if side == sideCT && !p.Kit {
		c += kitCost / 2
	}
	return c
}

func teamFullCost(ps []*Player, side int) int {
	c := 0
	for _, p := range ps {
		c += fullCost(p, side)
	}
	return c
}

func teamMoney(ps []*Player) int {
	s := 0
	for _, p := range ps {
		s += p.Money
	}
	return s
}

func canFullBuy(ps []*Player, side int) bool {
	return teamMoney(ps) >= teamFullCost(ps, side)-400
}

func utilWish(side int, plan string) []string {
	if side == sideT {
		switch plan {
		case "none":
			return nil
		case "flash_heavy":
			return []string{"flash", "flash", "smoke"}
		case "full":
			return []string{"smoke", "flash", "molotov", "flash"}
		default:
			return []string{"smoke", "flash"}
		}
	}
	switch plan {
	case "save":
		return nil
	case "heavy":
		return []string{"molotov", "smoke", "flash", "he"}
	default:
		return []string{"flash", "smoke"}
	}
}

type buyItem struct {
	ID      string   `json:"id"`
	Weapon  string   `json:"weapon"`
	Armor   int      `json:"armor"`
	Kit     bool     `json:"kit,omitempty"`
	Nades   []string `json:"nades,omitempty"`
	Spent   int      `json:"spent"`
	Money   int      `json:"money"`
	Dropped bool     `json:"dropped,omitempty"`
}

// buy: compra de l'equip segons el nivell. Torna què porta cadascú.
func buy(ps []*Player, side int, level, util string, awp bool) []buyItem {
	// Deixar armes (només en compra completa): els rics paguen el rifle dels pobres.
	dropped := map[string]bool{}
	if level == "full" {
		rifle := weapons[sideRifle(side)].Cost
		order := append([]*Player(nil), ps...)
		sort.SliceStable(order, func(i, j int) bool { return order[i].Money > order[j].Money })
		for _, needy := range ps {
			if needy.Primary != "" && weapons[needy.Primary].Class != "smg" {
				continue
			}
			if needy.Money >= rifle+kevlarCost {
				continue
			}
			for _, d := range order {
				if d == needy {
					continue
				}
				if d.Money-fullCost(d, side)-(awpExtra(d, awp)) >= rifle {
					d.Money -= rifle
					needy.Money += rifle
					dropped[needy.ID] = true
					break
				}
			}
		}
	}
	out := make([]buyItem, 0, len(ps))
	for _, p := range ps {
		before := p.Money
		buyOne(p, side, level, util, awp)
		out = append(out, buyItem{ID: p.ID, Weapon: p.weapon(side).ID, Armor: p.Armor, Kit: p.Kit, Nades: append([]string(nil), p.Nades...), Spent: before - p.Money, Money: p.Money, Dropped: dropped[p.ID]})
	}
	return out
}

func awpExtra(p *Player, awp bool) int {
	if awp && p.Role == "AWPer" && p.Primary != "awp" {
		return weapons["awp"].Cost - weapons["ak47"].Cost
	}
	return 0
}

func (p *Player) pay(c int) bool {
	if p.Money >= c {
		p.Money -= c
		return true
	}
	return false
}

func (p *Player) setPrimary(w string) bool {
	if p.pay(weapons[w].Cost) {
		p.Primary = w
		return true
	}
	return false
}

func (p *Player) armorTo(lvl int) {
	if p.Armor >= lvl {
		return
	}
	if lvl == 2 {
		c := helmetCost
		if p.Armor == 1 {
			c = helmetCost - kevlarCost
		}
		if p.pay(c) {
			p.Armor = 2
			return
		}
	}
	if p.Armor == 0 && p.pay(kevlarCost) {
		p.Armor = 1
	}
}

func (p *Player) buyNades(side int, wish []string, budget int) {
	for _, n := range wish {
		if len(p.Nades) >= 4 || (n == "flash" && p.count("flash") >= 2) || (n != "flash" && p.count(n) >= 1) {
			continue
		}
		c := nadeCost(n, side)
		if c <= budget && p.pay(c) {
			budget -= c
			p.Nades = append(p.Nades, n)
		}
	}
}

func buyOne(p *Player, side int, level, util string, awp bool) {
	hasGun := p.Primary != "" && weapons[p.Primary].Class != "smg"
	switch level {
	case "eco":
		return
	case "half":
		budget := p.Money * 45 / 100
		if budget > 2100 {
			budget = 2100
		}
		start := p.Money
		spent := func() int { return start - p.Money }
		if p.Primary == "" {
			smg := weapons[sideSMG(side)].Cost
			if budget >= smg+kevlarCost {
				p.setPrimary(sideSMG(side))
			} else if p.Pistol == "" && budget >= 300+kevlarCost {
				if p.pay(300) {
					p.Pistol = "p250"
				}
			}
		}
		if budget-spent() >= kevlarCost {
			p.armorTo(1)
		}
		p.buyNades(side, []string{"flash", "smoke"}, budget-spent())
		return
	case "force":
		if !hasGun {
			rifle, cheap, smg := sideRifle(side), sideCheapRifle(side), sideSMG(side)
			switch {
			case p.Money >= weapons[rifle].Cost+kevlarCost:
				p.setPrimary(rifle)
			case p.Money >= weapons[cheap].Cost+kevlarCost:
				p.setPrimary(cheap)
			case p.Primary == "" && p.Money >= weapons[smg].Cost+kevlarCost:
				p.setPrimary(smg)
			case p.Primary == "" && p.Pistol == "" && p.Money >= weapons["deagle"].Cost+kevlarCost:
				if p.pay(700) {
					p.Pistol = "deagle"
				}
			case p.Primary == "" && p.Pistol == "" && p.Money >= 300+kevlarCost:
				if p.pay(300) {
					p.Pistol = "p250"
				}
			}
		}
		p.armorTo(2)
		p.buyNades(side, utilWish(side, util), p.Money)
		return
	}
	// full
	if awp && p.Role == "AWPer" && p.Primary != "awp" && p.Money >= weapons["awp"].Cost+kevlarCost {
		p.setPrimary("awp")
	} else if !hasGun {
		if !p.setPrimary(sideRifle(side)) || p.Money < kevlarCost {
			if p.Primary == sideRifle(side) && p.Money < kevlarCost {
				// sense armilla amb un rifle: millor un galil/famas amb armilla
				p.Money += weapons[p.Primary].Cost
				p.Primary = ""
			}
			if p.Primary == "" {
				buyOne(p, side, "force", util, awp)
				return
			}
		}
	}
	p.armorTo(2)
	if side == sideCT && !p.Kit && p.Money >= kitCost+300 {
		p.pay(kitCost)
		p.Kit = true
	}
	p.buyNades(side, utilWish(side, util), p.Money)
}

func money(v int) string {
	s := fmt.Sprintf("%d", v)
	if len(s) > 3 {
		s = s[:len(s)-3] + "," + s[len(s)-3:]
	}
	return "$" + s
}

func gearLine(p *Player, side int) string {
	var b []string
	w := p.weapon(side)
	b = append(b, w.Name)
	switch p.Armor {
	case 1:
		b = append(b, "kevlar")
	case 2:
		b = append(b, "kevlar+helmet")
	}
	if p.Kit {
		b = append(b, "kit")
	}
	return strings.Join(b, ", ")
}
