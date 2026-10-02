package flow

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Runner és el que sap executar un pas de debò. El motor no coneix ni el
// model ni les eines: així es prova sense cap proveïdor i el TUI, la web i
// el mode -p hi endollen el mateix agent que fan servir per a tot.
type Runner interface {
	// Agent engega l'agent amb una tasca i torna la resposta final.
	Agent(ctx context.Context, task, mode string, maxSteps int) (string, error)
	// Tool executa una eina amb arguments JSON i torna la sortida.
	Tool(ctx context.Context, name, argsJSON string) (string, error)
}

// StepResult és què ha passat en un pas. Guardar-ho tot és el que permet
// saber després per què va sortir el que va sortir: és el bocí de
// «lineage» que fa que un graf valgui la pena i no sigui només un dibuix.
type StepResult struct {
	Node    string        `json:"node"`
	Kind    Kind          `json:"kind"`
	Title   string        `json:"title,omitempty"`
	Input   string        `json:"input,omitempty"`
	Output  string        `json:"output,omitempty"`
	Err     string        `json:"error,omitempty"`
	Started time.Time     `json:"started"`
	Took    time.Duration `json:"took"`
}

// RunResult és l'execució sencera.
type RunResult struct {
	Flow    string       `json:"flow"`
	Steps   []StepResult `json:"steps"`
	State   State        `json:"state"`
	Stopped string       `json:"stopped,omitempty"` // per què s'ha aturat abans d'hora
}

// Last torna l'últim pas executat (fals si no n'hi ha cap).
func (r RunResult) Last() (StepResult, bool) {
	if len(r.Steps) == 0 {
		return StepResult{}, false
	}
	return r.Steps[len(r.Steps)-1], true
}

// MaxPassos és el sostre d'execucions de node d'una tirada. Els grafs
// poden tenir cicles a posta (reintentar fins que els tests passin), i
// sense sostre un cicle mal posat et deixa l'agent donant voltes.
const MaxPassos = 100

// Opcions ajusta una execució.
type Opcions struct {
	// Estat inicial: el que sap el graf abans de començar.
	Estat State
	// MaxPassos sobreescriu el sostre (0 = MaxPassos).
	MaxPassos int
	// OnStep es crida en acabar cada pas: serveix per pintar-ho en viu.
	OnStep func(StepResult)
}

// Run executa el graf des del principi. Atura't quan no hi ha continuació,
// quan un pas falla, o quan s'esgota el sostre de passos.
//
// L'estat rep, per cada pas, la clau del node (o la seva Out) amb la
// sortida, i "<id>.error" si ha fallat; a més de "last" amb l'última
// sortida, que és el que es fa servir el 90% de les vegades.
func Run(ctx context.Context, f *Flow, r Runner, opt Opcions) (RunResult, error) {
	res := RunResult{Flow: f.Name, State: State{}}
	if err := f.Validate(); err != nil {
		return res, err
	}
	for k, v := range opt.Estat {
		res.State[k] = v
	}
	sostre := opt.MaxPassos
	if sostre <= 0 {
		sostre = MaxPassos
	}
	actual, err := f.Start()
	if err != nil {
		return res, err
	}

	for n := 0; actual != ""; n++ {
		if n >= sostre {
			res.Stopped = fmt.Sprintf("sostre de %d passos exhaurit (hi ha un cicle que no es tanca?)", sostre)
			return res, nil
		}
		if err := ctx.Err(); err != nil {
			res.Stopped = "aturat"
			return res, err
		}
		node, ok := f.Node(actual)
		if !ok {
			return res, fmt.Errorf("el pas %q ha desaparegut del graf", actual)
		}

		sr := StepResult{Node: node.ID, Kind: node.Kind, Title: node.Title, Started: time.Now()}
		var out string
		var errPas error
		switch node.Kind {
		case KindNote:
			out = Expand(node.Text, res.State)
			sr.Input = out
		case KindAgent:
			task := Expand(node.Task, res.State)
			sr.Input = task
			out, errPas = r.Agent(ctx, task, node.Mode, node.MaxSteps)
		case KindTool:
			args := Expand(node.Args, res.State)
			sr.Input = node.Tool + " " + args
			out, errPas = r.Tool(ctx, node.Tool, args)
		}
		sr.Took = time.Since(sr.Started)
		sr.Output = out
		clau := node.Out
		if clau == "" {
			clau = node.ID
		}
		res.State[clau] = out
		res.State["last"] = out
		if errPas != nil {
			sr.Err = errPas.Error()
			// L'error va a l'estat perquè una fletxa el pugui mirar: és com
			// es fa un «si falla, arregla-ho» sense sortir del graf.
			res.State[node.ID+".error"] = errPas.Error()
			res.State["last.error"] = errPas.Error()
		} else {
			delete(res.State, "last.error")
		}
		res.Steps = append(res.Steps, sr)
		if opt.OnStep != nil {
			opt.OnStep(sr)
		}

		// Després d'un error només valen les fletxes amb condició escrita:
		// vegeu Flow.Next. Sense cap, s'atura i es diu.
		seguents, err := f.Next(node.ID, res.State, errPas != nil)
		if err != nil {
			return res, err
		}
		if errPas != nil && len(seguents) == 0 {
			res.Stopped = "el pas «" + etiqueta(node) + "» ha fallat: " + errPas.Error()
			return res, errPas
		}
		if len(seguents) == 0 {
			return res, nil // final natural
		}
		if len(seguents) > 1 {
			// Diverses fletxes compleixen alhora. Podria ser paral·lel, però
			// executar-ho en paral·lel sense que ho hagis demanat canviaria
			// l'ordre dels efectes sobre el disc. Es va per la primera i es
			// diu, que és previsible.
			res.State["avis"] = fmt.Sprintf("del pas %q en surten %d camins alhora; s'ha seguit %q", node.ID, len(seguents), seguents[0])
		}
		actual = seguents[0]
	}
	return res, nil
}

func etiqueta(n Node) string {
	if strings.TrimSpace(n.Title) != "" {
		return n.Title
	}
	return n.ID
}
