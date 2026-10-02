// Package flow són grafs d'execució d'agents: dir tu quins passos té una
// feina i què fa cadascun, en comptes de deixar-ho tot a la decisió del
// model pas a pas.
//
// Per què. El loop d'ara és una línia: li dónes una tasca i el model tria
// cada pas. Va bé per a una feina d'una vegada. No va bé per a un
// procediment que vols repetir IGUAL cada cop —revisar un PR, preparar un
// lliurament, migrar un mòdul— on el valor és que els passos siguin
// explícits, es vegin, i es puguin tornar a executar.
//
// Què NO és. A la literatura es barregen tres coses amb el mateix nom:
//
//   - graf d'execució: qui actua ara i què li passa a l'estat. Això.
//   - DAG d'experiments: quina versió va produir quin resultat (lineage).
//   - graf de coneixement: entitats i relacions amb procedència.
//
// Un graf d'execució no és cap dels altres dos, i tenir-lo no et dona
// memòria. Val la pena quan el procediment es repeteix, quan vols veure on
// ha fallat, o quan hi ha passos independents que poden anar alhora; per a
// una pregunta d'una vegada, el loop de sempre és millor i més barat.
package flow

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Kind és què fa un node.
type Kind string

const (
	// KindAgent engega l'agent amb una tasca (el model tria les eines).
	KindAgent Kind = "agent"
	// KindTool executa una eina concreta sense passar pel model: barat,
	// determinista i el que vols per a «executa els tests» o «llegeix X».
	KindTool Kind = "tool"
	// KindNote no fa res: text a l'estat. Serveix per fixar instruccions
	// o constants que després fan servir altres nodes.
	KindNote Kind = "note"
)

// Node és un pas del graf.
type Node struct {
	ID    string `json:"id"`
	Kind  Kind   `json:"kind"`
	Title string `json:"title"`

	// Task (agent) i Args (tool) admeten plantilles {{clau}} amb valors de
	// l'estat: així un pas fa servir el que ha produït l'anterior.
	Task     string `json:"task,omitempty"`
	Mode     string `json:"mode,omitempty"`      // code|chat (agent)
	MaxSteps int    `json:"max_steps,omitempty"` // 0 = el del config
	Tool     string `json:"tool,omitempty"`      // nom de l'eina (tool)
	Args     string `json:"args,omitempty"`      // JSON d'arguments (tool)
	Text     string `json:"text,omitempty"`      // contingut (note)

	// Out és la clau de l'estat on va el resultat. Buida: es desa a la
	// clau "<id>" igualment, perquè sempre es pugui referenciar.
	Out string `json:"out,omitempty"`

	// X i Y són on cau al llenç de l'editor. El motor no se'ls mira: són
	// de la vista, però viuen amb el graf perquè es desin juntes.
	X int `json:"x"`
	Y int `json:"y"`
}

// Edge uneix dos nodes. When buit vol dir «sempre»; si no, és una condició
// que es mira contra l'estat (vegeu Cond).
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	When string `json:"when,omitempty"`
}

// Flow és el graf sencer.
type Flow struct {
	Name  string `json:"name"`
	Desc  string `json:"desc,omitempty"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

var idRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Validate comprova que el graf es pugui executar. Val més dir-ho aquí que
// petar a mig camí amb mig procediment fet.
func (f *Flow) Validate() error {
	if strings.TrimSpace(f.Name) == "" {
		return fmt.Errorf("el flux necessita un nom")
	}
	if len(f.Nodes) == 0 {
		return fmt.Errorf("el flux %q no té cap pas", f.Name)
	}
	vist := map[string]Node{}
	for _, n := range f.Nodes {
		if !idRe.MatchString(n.ID) {
			return fmt.Errorf("id de pas invàlid: %q (lletres, xifres, - i _)", n.ID)
		}
		if _, hi := vist[n.ID]; hi {
			return fmt.Errorf("dos passos amb el mateix id: %q", n.ID)
		}
		switch n.Kind {
		case KindAgent:
			if strings.TrimSpace(n.Task) == "" {
				return fmt.Errorf("el pas %q és d'agent i no diu què ha de fer", n.ID)
			}
			if n.Mode != "" && n.Mode != "code" && n.Mode != "chat" {
				return fmt.Errorf("el pas %q té mode %q (ha de ser code o chat)", n.ID, n.Mode)
			}
		case KindTool:
			if strings.TrimSpace(n.Tool) == "" {
				return fmt.Errorf("el pas %q és d'eina i no diu quina", n.ID)
			}
		case KindNote:
			// Sense requisits: una nota buida és una nota buida.
		default:
			return fmt.Errorf("el pas %q té una mena desconeguda: %q", n.ID, n.Kind)
		}
		vist[n.ID] = n
	}
	for _, e := range f.Edges {
		if _, hi := vist[e.From]; !hi {
			return fmt.Errorf("una fletxa surt de %q, que no existeix", e.From)
		}
		if _, hi := vist[e.To]; !hi {
			return fmt.Errorf("una fletxa va a %q, que no existeix", e.To)
		}
		if e.From == e.To {
			return fmt.Errorf("el pas %q s'apunta a ell mateix", e.From)
		}
		if _, err := ParseCond(e.When); err != nil {
			return fmt.Errorf("condició de %q→%q: %w", e.From, e.To, err)
		}
	}
	if _, err := f.Start(); err != nil {
		return err
	}
	return nil
}

// Start és el pas per on es comença: l'únic sense fletxes que hi entrin.
func (f *Flow) Start() (string, error) {
	entrada := map[string]int{}
	for _, n := range f.Nodes {
		entrada[n.ID] = 0
	}
	for _, e := range f.Edges {
		entrada[e.To]++
	}
	var arrels []string
	for id, n := range entrada {
		if n == 0 {
			arrels = append(arrels, id)
		}
	}
	sort.Strings(arrels)
	switch len(arrels) {
	case 1:
		return arrels[0], nil
	case 0:
		return "", fmt.Errorf("tots els passos tenen una fletxa d'entrada: no se sap per on començar (hi ha un cicle?)")
	default:
		return "", fmt.Errorf("hi ha %d passos sense entrada (%s): només pot haver-hi un principi", len(arrels), strings.Join(arrels, ", "))
	}
}

// Node cerca un pas per id.
func (f *Flow) Node(id string) (Node, bool) {
	for _, n := range f.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

// Next dona els passos que segueixen un node, en ordre, amb la condició
// complerta contra l'estat.
//
// nomesExplicites canvia què val després d'un pas que ha fallat: llavors
// només compten les fletxes amb condició escrita. Una fletxa sense condició
// és el camí de quan tot va bé, i seguir-la després d'un error voldria dir
// passar-li al pas següent un resultat que no existeix —exactament la mena
// de silenci que fa que després no sàpigues on s'ha trencat. Per continuar
// malgrat un error, s'ha de dir amb una condició (per exemple «x.error»).
func (f *Flow) Next(id string, st State, nomesExplicites bool) ([]string, error) {
	var out []string
	for _, e := range f.Edges {
		if e.From != id {
			continue
		}
		if nomesExplicites && strings.TrimSpace(e.When) == "" {
			continue
		}
		c, err := ParseCond(e.When)
		if err != nil {
			return nil, err
		}
		if c.Eval(st) {
			out = append(out, e.To)
		}
	}
	return out, nil
}

var tmplRe = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}`)

// Expand substitueix {{clau}} pels valors de l'estat. Una clau que no hi és
// es deixa buida: val més un buit que un {{x}} literal arribant al model.
func Expand(s string, st State) string {
	return tmplRe.ReplaceAllStringFunc(s, func(m string) string {
		k := tmplRe.FindStringSubmatch(m)[1]
		return st[k]
	})
}
