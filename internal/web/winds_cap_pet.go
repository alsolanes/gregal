package web

import (
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// La mascota també és una criatura de companyia, com un tamagotchi: té
// alegria, energia i gana, puja de nivell quan la cuides, jugues amb ella o
// li encarregues feina, i guanya roba per posar-se. Mai no castiga: si
// passes dies sense obrir l'app, l'alegria baixa fins a un mínim i prou
// (no es posa malalta ni marxa). Les animacions són locals a l'app; aquí
// només hi ha l'estat i les frases.
//
//	POST /api/winds/cap/play {act, lang} → {pet, say, level_up, unlocked}
//
// act: "pet" (mimar), "feed:<galeta|poma|cafe>", "game:<id>:<punts>",
// "sleep" (becaina) o "wear:<peça>" ("wear:" la treu).

type capPet struct {
	Happy   int    `json:"happy"`  // 0-100
	Energy  int    `json:"energy"` // 0-100: jugar en gasta, descansar en recupera
	Full    int    `json:"full"`   // 0-100: menjar
	XP      int    `json:"xp"`
	Outfit  string `json:"outfit"`
	Played  int    `json:"played"`
	Updated int64  `json:"updated"`
}

const (
	capHappyFloor = 25 // l'alegria no baixa d'aquí: res de castigar
	capXPPerLevel = 100
)

// capOutfits: la roba. Les de nivell es guanyen cuidant-la; les de
// temporada surten soles el seu mes.
var capOutfits = []struct {
	ID    string
	Level int
	Month time.Month // 0 = tot l'any
}{
	{"llacet", 2, 0}, {"gorra", 3, 0}, {"bufanda", 4, 0}, {"ulleres", 5, 0},
	{"barret", 6, 0}, {"auriculars", 7, 0}, {"corona", 8, 0},
	{"flors", 1, time.April}, {"carbassa", 1, time.October}, {"gorro", 1, time.December},
}

func (p capPet) level() int { return 1 + p.XP/capXPPerLevel }

func capOutfitsFor(level int, month time.Month) []string {
	out := []string{}
	for _, o := range capOutfits {
		if (o.Month == 0 && level >= o.Level) || (o.Month != 0 && o.Month == month) {
			out = append(out, o.ID)
		}
	}
	return out
}

func clampPet(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// tick fa passar el temps des de l'última vegada: l'energia es recupera, la
// gana creix i l'alegria baixa a poc a poc fins al mínim.
func (p *capPet) tick(now time.Time) {
	if p.Updated == 0 {
		*p = capPet{Happy: 70, Energy: 80, Full: 60, Outfit: p.Outfit, XP: p.XP, Updated: now.Unix()}
		return
	}
	h := float64(now.Unix()-p.Updated) / 3600
	if h <= 0 {
		return
	}
	p.Energy = clampPet(p.Energy + int(12*h))
	p.Full = clampPet(p.Full - int(5*h))
	if p.Happy > capHappyFloor {
		p.Happy = max(capHappyFloor, p.Happy-int(3*h))
	}
	p.Updated = now.Unix()
}

func (p capPet) json(now time.Time) map[string]any {
	lv := p.level()
	return map[string]any{
		"happy": p.Happy, "energy": p.Energy, "full": p.Full, "xp": p.XP,
		"level": lv, "level_xp": p.XP % capXPPerLevel, "level_need": capXPPerLevel,
		"outfit": p.Outfit, "outfits": capOutfitsFor(lv, now.Month()), "played": p.Played,
	}
}

func capPick(ca bool, catalan, english []string) string {
	if ca {
		return catalan[rand.Intn(len(catalan))]
	}
	return english[rand.Intn(len(english))]
}

// capPlay aplica una acció i torna què diu la mascota.
func capPlay(p *capPet, act string, ca bool, now time.Time) (string, error) {
	p.tick(now)
	kind, arg, _ := strings.Cut(act, ":")
	switch kind {
	case "pet":
		p.Happy = clampPet(p.Happy + 6)
		p.XP += 2
		return capPick(ca,
			[]string{"Hihi, fa pessigolles! 💛", "Mmm, més, més!", "Ets el meu humà preferit.", "Ronc ronc… (no sóc un gat, però gairebé)"},
			[]string{"Hehe, that tickles! 💛", "Mmm, more, more!", "You're my favourite human.", "Purr… (I'm not a cat, but almost)"}), nil
	case "feed":
		if p.Full >= 95 {
			return capPick(ca, []string{"Ja estic ple! Si en menjo més, rodolaré 🫃"}, []string{"I'm full! One more bite and I'll roll away 🫃"}), nil
		}
		switch arg {
		case "galeta":
			p.Full, p.Happy = clampPet(p.Full+25), clampPet(p.Happy+4)
		case "poma":
			p.Full, p.Happy, p.Energy = clampPet(p.Full+20), clampPet(p.Happy+3), clampPet(p.Energy+5)
		case "cafe":
			p.Full, p.Energy = clampPet(p.Full+5), clampPet(p.Energy+25)
			p.XP += 4
			return capPick(ca, []string{"☕ Uau! Ara podria organitzar tres colles alhora!"}, []string{"☕ Whoa! Now I could run three crews at once!"}), nil
		default:
			return "", errConst("unknown snack")
		}
		p.XP += 4
		return capPick(ca,
			[]string{"Ñam ñam! 😋", "Boníssim, gràcies!", "Això sí que és un bon esmorzar de cap de gabinet."},
			[]string{"Yum yum! 😋", "Delicious, thanks!", "Now that's a chief of staff's breakfast."}), nil
	case "game":
		if p.Energy < 15 {
			return capPick(ca, []string{"Estic rebentat… em deixes fer una becaina abans? 😴"}, []string{"I'm worn out… can I take a nap first? 😴"}), nil
		}
		game, rawScore, _ := strings.Cut(arg, ":")
		score, _ := strconv.Atoi(rawScore)
		if game == "" || score < 0 {
			return "", errConst("bad game")
		}
		score = min(score, 30)
		p.Energy = clampPet(p.Energy - 15)
		p.Happy = clampPet(p.Happy + 10 + min(score, 10))
		p.XP += min(10+score*2, 40)
		p.Played++
		if score >= 10 {
			return capPick(ca, []string{"Quina partida! Ets un crac 🏆", "Una altra, una altra!"}, []string{"What a game! You're a star 🏆", "Again, again!"}), nil
		}
		return capPick(ca, []string{"M'ho he passat d'allò més bé!", "La propera et guanyo, ja veuràs 😤"}, []string{"That was so much fun!", "I'll beat you next time, you'll see 😤"}), nil
	case "sleep":
		p.Energy = clampPet(p.Energy + 40)
		p.Happy = clampPet(p.Happy + 2)
		return capPick(ca, []string{"Zzz… Ja estic com nou ✨"}, []string{"Zzz… Good as new ✨"}), nil
	case "wear":
		if arg == "" {
			p.Outfit = ""
			return capPick(ca, []string{"Au natural!"}, []string{"Au naturel!"}), nil
		}
		for _, o := range capOutfitsFor(p.level(), now.Month()) {
			if o == arg {
				p.Outfit = arg
				return capPick(ca, []string{"Com em queda? 😎", "Vaig a la moda!"}, []string{"How do I look? 😎", "So stylish!"}), nil
			}
		}
		return "", errConst("outfit locked")
	}
	return "", errConst("unknown act")
}

// capCheer: encarregar-li feina també la fa feliç (és el seu ofici).
func capCheer(p *capPet, now time.Time) {
	p.tick(now)
	p.Happy = clampPet(p.Happy + 4)
	p.XP += 6
}

func (s *Server) handleCapPlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Act  string `json:"act"`
		Lang string `json:"lang"`
	}
	if !decodeSmall(w, r, &req) {
		return
	}
	now := time.Now()
	capMu.Lock()
	defer capMu.Unlock()
	st := s.loadCap()
	before := st.Pet.level()
	beforeOutfits := capOutfitsFor(before, now.Month())
	say, err := capPlay(&st.Pet, strings.TrimSpace(req.Act), req.Lang == "ca", now)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.saveCap(st)
	var unlocked []string
	had := map[string]bool{}
	for _, o := range beforeOutfits {
		had[o] = true
	}
	for _, o := range capOutfitsFor(st.Pet.level(), now.Month()) {
		if !had[o] {
			unlocked = append(unlocked, o)
		}
	}
	writeJSON(w, map[string]any{"pet": st.Pet.json(now), "say": say, "level_up": st.Pet.level() > before, "unlocked": unlocked})
}
