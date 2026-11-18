package goal

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gregal/internal/backlog"
	"gregal/internal/llm"
)

// El mode objectiu ja no té llista pròpia: el que concreta és una tasca del
// backlog del projecte (internal/backlog). Aquí hi ha el que comparteixen
// els front-ends (web, TUI, Telegram, `gregal --goal`): llegir el bloc
// ```goal com a tasca, desar-la, trobar-la, començar-la i passar els
// objectius antics al backlog.

// RecordedBy és qui consta al log quan el mode objectiu crea una tasca.
const RecordedBy = "goal mode"

var clauBloc = regexp.MustCompile(`^([A-Za-zÀ-ÿ_ ]{2,20}):\s*(.*)$`)

// ParseItem llegeix el bloc ```goal d'una resposta com a tasca del backlog.
// Accepta les claus d'ara (id, type, title, why, criteria, out_of_scope,
// size) i les d'abans (task/tasca, context, criteris, steps/passos,
// risks/riscos): els passos i els riscos s'afegeixen al perquè.
func ParseItem(reply string) (backlog.Item, bool) {
	cos, ok := blocGoal(reply)
	if !ok {
		return backlog.Item{}, false
	}
	var b lectorBloc
	for _, linia := range strings.Split(cos, "\n") {
		b.linia(strings.TrimSpace(linia))
	}
	return b.tasca()
}

// lectorBloc llegeix el bloc línia a línia: una línia «clau: valor» canvia
// de camp, i les que la segueixen són més text del camp o elements de la
// llista (criteris, passos, riscos).
type lectorBloc struct {
	it           backlog.Item
	plan, riscos []string
	actual       string
}

func (b *lectorBloc) escalar(k string) *string {
	switch k {
	case "title":
		return &b.it.Title
	case "why":
		return &b.it.Why
	case "out_of_scope":
		return &b.it.OutOfScope
	}
	return nil
}

func (b *lectorBloc) linia(t string) {
	if t == "" {
		return
	}
	if k, v, ok := clauDeLinia(t); ok {
		b.actual = k
		b.posa(k, v)
		return
	}
	item := strings.TrimSpace(strings.TrimLeft(t, "-*•"))
	switch b.actual {
	case "criteria":
		if item != "" {
			b.it.Criteria = append(b.it.Criteria, backlog.Criterion{Text: item})
		}
	case "steps":
		b.plan = append(b.plan, item)
	case "risks":
		b.riscos = append(b.riscos, item)
	default:
		if p := b.escalar(b.actual); p != nil {
			*p = strings.TrimSpace(*p + "\n" + t)
		}
	}
}

// clauDeLinia diu si la línia obre un camp. «Steps: obre maig…» dins del
// perquè d'un bug és text, no la llista de passos del format vell (que va
// buida i amb els elements a sota).
func clauDeLinia(t string) (string, string, bool) {
	m := clauBloc.FindStringSubmatch(t)
	if m == nil || strings.HasPrefix(t, "- ") {
		return "", "", false
	}
	k, ok := clauNormal(m[1])
	v := strings.TrimSpace(m[2])
	if (k == "steps" || k == "risks") && v != "" {
		return "", "", false
	}
	return k, v, ok
}

func (b *lectorBloc) posa(k, v string) {
	switch k {
	case "id":
		b.it.ID, _ = strconv.Atoi(strings.TrimPrefix(v, "#"))
	case "type":
		b.it.Type = strings.ToLower(v)
	case "size":
		b.it.Size = strings.ToUpper(v)
	case "priority":
		b.it.Priority = strings.ToLower(v)
	default:
		if p := b.escalar(k); p != nil && v != "" {
			*p = v
		}
	}
}

// tasca tanca la lectura: el pla i els riscos van al perquè, i un tipus o
// una mida que no existeixen es deixen als valors per defecte.
func (b *lectorBloc) tasca() (backlog.Item, bool) {
	it := b.it
	if len(b.plan) > 0 {
		it.Why = strings.TrimSpace(it.Why + "\n\nPlan:\n- " + strings.Join(b.plan, "\n- "))
	}
	if len(b.riscos) > 0 {
		it.Why = strings.TrimSpace(it.Why + "\n\nRisks:\n- " + strings.Join(b.riscos, "\n- "))
	}
	if it.Title == "" {
		return backlog.Item{}, false
	}
	switch it.Type {
	case backlog.TypeFeature, backlog.TypeBug, backlog.TypeTask, backlog.TypeSpike, backlog.TypeEpic:
	default:
		it.Type = backlog.TypeFeature
	}
	if it.Size != "S" && it.Size != "M" && it.Size != "L" {
		it.Size = ""
	}
	return it, true
}

func clauNormal(k string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(k)) {
	case "id":
		return "id", true
	case "type", "tipus":
		return "type", true
	case "title", "títol", "titol", "task", "tasca":
		return "title", true
	case "why", "perquè", "context":
		return "why", true
	case "criteria", "criteris", "acceptance criteria":
		return "criteria", true
	case "out_of_scope", "out of scope", "fora d'abast":
		return "out_of_scope", true
	case "size", "mida":
		return "size", true
	case "priority", "prioritat":
		return "priority", true
	case "steps", "passos":
		return "steps", true
	case "risks", "riscos":
		return "risks", true
	}
	return "", false
}

// TurnReply és el text on buscar el bloc ```goal d'un torn: l'últim
// missatge de l'assistent del torn que en porta un, o la resposta final. El
// model sovint escriu el bloc, pregunta amb l'eina question i tanca amb una
// frase curta: mirar només la resposta final perdia la tasca (i el model
// deia «desada»). El torn comença després de l'últim missatge de la
// persona: un bloc d'un torn anterior no es torna a desar.
func TurnReply(hist []llm.Message, final string) string {
	if _, ok := blocGoal(final); ok {
		return final
	}
	for i := len(hist) - 1; i >= 0; i-- {
		switch hist[i].Role {
		case "user":
			return final
		case "assistant":
			if _, ok := blocGoal(hist[i].Content); ok {
				return hist[i].Content
			}
		}
	}
	return final
}

// Record desa la tasca del bloc ```goal al backlog del projecte de cwd. Si
// el bloc porta l'id d'una tasca que encara no s'ha començat, la revisa en
// lloc de crear-ne una altra. created diu quin dels dos ha passat.
func Record(reply, cwd string) (it backlog.Item, created bool, err error) {
	nou, ok := ParseItem(reply)
	if !ok {
		return backlog.Item{}, false, errors.New("no goal block")
	}
	st, err := backlog.Open(cwd)
	if err != nil {
		return backlog.Item{}, false, err
	}
	if nou.ID > 0 {
		if prev, err := st.Get(nou.ID); err == nil && (prev.Status == backlog.StatusProposed || prev.Status == backlog.StatusReady) {
			it, err = st.Edit(nou.ID, RecordedBy, func(i *backlog.Item) {
				i.Type, i.Title, i.Why, i.Criteria = nou.Type, nou.Title, nou.Why, nou.Criteria
				if nou.OutOfScope != "" {
					i.OutOfScope = nou.OutOfScope
				}
				if nou.Size != "" {
					i.Size = nou.Size
				}
			})
			return it, false, err
		}
	}
	// Sense id, una tasca oberta i no començada amb el mateix títol és la
	// mateixa: el model pot haver-la creat amb l'eina backlog dins del torn,
	// o repetir el bloc sense id. Es revisa en lloc de duplicar-la.
	// (Només sense id: amb un id que ja no es pot revisar, en fa una de nova.)
	if all, err := st.List(); err == nil && nou.ID == 0 {
		for _, prev := range all {
			if (prev.Status == backlog.StatusProposed || prev.Status == backlog.StatusReady) &&
				strings.EqualFold(strings.Join(strings.Fields(prev.Title), " "), strings.Join(strings.Fields(nou.Title), " ")) {
				return Record(withID(reply, prev.ID), cwd)
			}
		}
	}
	it, err = st.Create(nou, RecordedBy)
	return it, err == nil, err
}

// withID afegeix «id: n» a dalt del bloc ```goal (perquè Record el revisi).
func withID(reply string, id int) string {
	i := strings.Index(reply, obertura)
	if i < 0 {
		return reply
	}
	at := i + len(obertura)
	return reply[:at] + "\nid: " + strconv.Itoa(id) + reply[at:]
}

// Note és la línia que s'afegeix a la resposta un cop desada: la persona
// veu on ha anat a parar i el model, en el torn següent, sap l'id per
// revisar-la.
func Note(it backlog.Item, created bool) string {
	verb := "Saved to the backlog"
	if !created {
		verb = "Updated in the backlog"
	}
	return fmt.Sprintf("◆ %s as #%d (%s). To revise it, send the whole block again with `id: %d`.", verb, it.ID, it.Status, it.ID)
}

// AnnotateLast afegeix la nota a l'últim missatge de l'assistent de la
// conversa (on ja hi ha la resposta amb el bloc).
func AnnotateLast(convo []llm.Message, note string) {
	for i := len(convo) - 1; i >= 0; i-- {
		if convo[i].Role == "assistant" {
			convo[i].Content = strings.TrimRight(convo[i].Content, "\n") + "\n\n" + note
			return
		}
	}
}

// Open són les tasques obertes del projecte, la més recent primer.
func Open(cwd string) ([]backlog.Item, error) {
	st, err := backlog.Open(cwd)
	if err != nil {
		return nil, err
	}
	all, err := st.List()
	if err != nil {
		return nil, err
	}
	var out []backlog.Item
	for _, it := range all {
		if it.Status != backlog.StatusDone && it.Status != backlog.StatusDropped {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, nil
}

// Resolve troba una tasca per número («3» o «#3»); sense número, la
// oberta tocada més recentment.
func Resolve(cwd, ref string) (backlog.Item, error) {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "#")
	if ref != "" {
		id, err := strconv.Atoi(ref)
		if err != nil {
			return backlog.Item{}, fmt.Errorf("%q is not an item number", ref)
		}
		st, err := backlog.Open(cwd)
		if err != nil {
			return backlog.Item{}, err
		}
		return st.Get(id)
	}
	open, err := Open(cwd)
	if err != nil {
		return backlog.Item{}, err
	}
	if len(open) == 0 {
		return backlog.Item{}, backlog.ErrNotFound
	}
	return open[0], nil
}

// Start posa una tasca en marxa des de cwd: si encara és proposada la
// marca a punt (el «sí, executa-ho» de la persona és la confirmació), la
// passa a in_progress, apunta on es treballa i torna l'encàrrec per a
// l'agent.
func Start(cwd string, id int, who string) (backlog.Item, string, error) {
	st, err := backlog.Open(cwd)
	if err != nil {
		return backlog.Item{}, "", err
	}
	it, err := st.Get(id)
	if err != nil {
		return it, "", err
	}
	if it.Status == backlog.StatusProposed || it.Status == backlog.StatusBlocked {
		if it, err = st.Move(id, backlog.StatusReady, "", who); err != nil {
			return it, "", err
		}
	}
	if it.Status != backlog.StatusInProgress {
		if it, err = st.Move(id, backlog.StatusInProgress, "", who); err != nil {
			return it, "", err
		}
	}
	base := ""
	if it.Base == "" {
		base = backlog.Head(cwd)
	}
	if it, err = st.SetWork(id, cwd, base); err != nil {
		return it, "", err
	}
	return it, backlog.Task(it), nil
}

// Card és la tasca en text pla, per al TUI i Telegram.
func Card(it backlog.Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d · %s · %s\n%s\n", it.ID, it.Type, it.Status, it.Title)
	if it.Why != "" {
		b.WriteString("\n" + it.Why + "\n")
	}
	for n, c := range it.Criteria {
		marca := " "
		if c.Done {
			marca = "x"
		}
		fmt.Fprintf(&b, "%d. [%s] %s\n", n+1, marca, c.Text)
	}
	if it.OutOfScope != "" {
		b.WriteString("Out of scope: " + it.OutOfScope + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// ForLoop fa d'una tasca el Goal que espera el bucle de `gregal --goal`.
func ForLoop(it backlog.Item, cwd string) Goal {
	return Goal{ID: "#" + strconv.Itoa(it.ID), Title: it.Title, Body: Card(it), CWD: cwd, Status: StatusEsborrany}
}

// MigrateToBacklog passa els objectius pendents de goals.json al backlog
// del projecte de cadascun, com a tasques proposades. Els fets ja no
// diuen res. El fitxer queda com a goals.json.migrated perquè no es torni
// a migrar. Torna quants n'ha passat.
func MigrateToBacklog(dir string) (int, error) {
	llista, err := llegir(dir)
	if err != nil || len(llista) == 0 {
		return 0, err
	}
	n := 0
	var errs []string
	var queden []Goal // els que han fallat: es tornaran a provar
	for _, g := range llista {
		if g.Status == StatusFet || strings.TrimSpace(g.CWD) == "" {
			continue
		}
		if fi, err := os.Stat(g.CWD); err != nil || !fi.IsDir() {
			continue
		}
		it, ok := ParseItem("```goal\n" + g.Body + "\n```")
		if !ok {
			it = backlog.Item{Type: backlog.TypeFeature, Title: g.Title, Why: g.Body}
		}
		it.ID = 0
		st, err := backlog.Open(g.CWD)
		if err == nil {
			_, err = st.Create(it, "migrated from goals")
		}
		if err != nil {
			errs = append(errs, g.ID+": "+err.Error())
			queden = append(queden, g)
			continue
		}
		n++
	}
	if err := os.Rename(Path(dir), Path(dir)+".migrated"); err != nil {
		return n, err
	}
	if len(queden) > 0 {
		// Només els que han fallat tornen a goals.json: la propera
		// vegada no es duplica cap dels que ja hi són.
		if err := escriure(dir, queden); err != nil {
			return n, err
		}
		return n, errors.New(strings.Join(errs, "; "))
	}
	return n, nil
}
