package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"gregal/internal/tema"
)

// Identitat Gregal — "la marea": mar de nit, escuma i arena.
// Fosc per defecte, pensat per terminals dark. Cap color genèric:
// el primari és escuma (#9FE0C4) i l'accent és arena (#DEC899).

// Els colors no porten valor a la declaració: amb un valor aquí es
// congelen en carregar-se el paquet i el tema ja no es pot canviar. Els
// omple aplicaTema(), que es crida una vegada en arrencar.
var (
	escuma lipgloss.Color // primari: mar clara
	arena  lipgloss.Color // accent: sorra
	onada  lipgloss.Color // secundari: blau llacuna
	algua  lipgloss.Color // línies d'onada tènues
	fons   lipgloss.Color

	teal  lipgloss.Color
	slate lipgloss.Color
	// ink és el text de sobre les insígnies de color: al fosc, el fons de
	// l'app; al clar, blanc. No és el "ink" del document (que és el text
	// fort), i el nom ve d'abans.
	ink   lipgloss.Color
	night lipgloss.Color
	amber lipgloss.Color
	green lipgloss.Color
	red   lipgloss.Color
	blue  lipgloss.Color
	// lila i lilaFons són els del mode objectiu (docs/identitat.md).
	lila     lipgloss.Color
	lilaFons lipgloss.Color
	// vora del selector: slate, que ja és el gris de la paleta.
	voraApagada lipgloss.Color
	// faint és el text molt apagat (pistes de tecles, versió); textPrinc
	// és el de les respostes i textFort el dels títols.
	faint     lipgloss.Color
	textPrinc lipgloss.Color
	textFort  lipgloss.Color
	// boxIdle és la vora del composer en repòs.
	boxIdle lipgloss.Color
)

var (
	logoStyle        lipgloss.Style
	dimStyle         lipgloss.Style
	faintStyle       lipgloss.Style
	waveStyle        lipgloss.Style
	userStyle        lipgloss.Style
	userBarStyle     lipgloss.Style
	userTextStyle    lipgloss.Style
	assistantStyle   lipgloss.Style
	systemStyle      lipgloss.Style
	okStyle          lipgloss.Style
	badStyle         lipgloss.Style
	warnStyle        lipgloss.Style
	branchStyle      lipgloss.Style
	mentionStyle     lipgloss.Style
	cmdStyle         lipgloss.Style
	headStyle        lipgloss.Style
	roleStyle        lipgloss.Style
	barStyle         lipgloss.Style
	projectStyle     lipgloss.Style
	dirStyle         lipgloss.Style
	modeCodeStyle    lipgloss.Style
	modeChatStyle    lipgloss.Style
	modeInspectStyle lipgloss.Style
	toolRailStyle    lipgloss.Style
	toolNameStyle    lipgloss.Style
	toolDoneStyle    lipgloss.Style
	composerCode     lipgloss.Style
	composerChat     lipgloss.Style
	modeGoalStyle    lipgloss.Style
	modeAutoStyle    lipgloss.Style
	composerGoal     lipgloss.Style
	composerAuto     lipgloss.Style
	goalTitleStyle   lipgloss.Style
	spinnerStyle     lipgloss.Style
	crestStyle       lipgloss.Style
	addStyle         lipgloss.Style
	delStyle         lipgloss.Style
	hunkStyle        lipgloss.Style
	menuHL           lipgloss.Style
)

// temaActiu és el tema que s'està pintant. El markdown i el codi el
// necessiten per triar colors; abans l'estil de glamour es calculava una
// sola vegada i en tema clar es quedava amb el fosc.
var temaActiu = tema.Fosc()

// TemaActiu és el tema en ús (per a render.go i els diffs).
func TemaActiu() tema.Tema { return temaActiu }

func init() { SetTema("fosc") }

// SetTema tria la paleta del TUI. Es crida una vegada en arrencar, abans
// de pintar res: qualsevol estil calculat abans es queda amb el tema vell,
// que és el mateix error que ja va passar amb la paleta d'ordres i
// l'idioma. El nom ve de TemesSuportats; un nom desconegut cau al fosc.
func SetTema(nom string) {
	if os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	aplicaTema(nom)
}

// aplicaTema omple els colors i reconstrueix TOTS els estils. Si algun es
// quedés fora, en tema clar sortiria un color del fosc enmig.
// aplicaTema omple els colors i reconstrueix TOTS els estils. Si algun es
// quedes fora, en tema clar sortiria un color del fosc enmig.
//
// Els valors ja no són aquí: surten d'internal/tema, que és el mateix
// paquet del qual la web treu el seu :root i els dos fronts treuen els
// colors del codi. Aquestes variables segueixen existint perquè mig
// paquet les fa servir pel nom.
func aplicaTema(nom string) {
	temaActiu = tema.Per(nom)
	t := temaActiu
	clar := t.Clar
	col := func(hex string) lipgloss.Color { return lipgloss.Color(hex) }

	escuma = col(t.Escuma)
	arena = col(t.Arena)
	onada = col(t.Onada)
	algua = col(t.Algua)
	fons = col(t.Fons)
	night = col(t.Night)
	slate = col(t.Slate)
	green = col(t.Green)
	red = col(t.Red)
	lila = col(t.Lila)
	lilaFons = col(t.LilaFons)
	faint = col(t.Faint)
	textPrinc = col(t.Text)
	textFort = col(t.Ink)
	boxIdle = col(t.BoxIdle)
	// Text de sobre les insígnies de color: al fosc, el fons de l'app;
	// al clar, blanc (les insígnies clares tenen fons fosc).
	if clar {
		ink = col(t.Night)
	} else {
		ink = col(t.Fons)
	}
	teal, amber, blue, voraApagada = escuma, arena, onada, slate

	logoStyle = lipgloss.NewStyle().Foreground(escuma).Bold(true)
	dimStyle = lipgloss.NewStyle().Foreground(slate)
	faintStyle = lipgloss.NewStyle().Foreground(faint)
	waveStyle = lipgloss.NewStyle().Foreground(algua)
	userStyle = lipgloss.NewStyle().Foreground(escuma).Bold(true)
	// El teu missatge: barra d'escuma i text sobre superfície, perquè se
	// separi del de l'agent sense llegir cap etiqueta.
	userBarStyle = lipgloss.NewStyle().Foreground(escuma).Bold(true)
	userTextStyle = lipgloss.NewStyle().Foreground(textFort).Background(night)
	assistantStyle = lipgloss.NewStyle().Foreground(textPrinc)
	systemStyle = lipgloss.NewStyle().Foreground(slate).Italic(true)
	okStyle = lipgloss.NewStyle().Foreground(green).Bold(true)
	badStyle = lipgloss.NewStyle().Foreground(red).Bold(true)
	warnStyle = lipgloss.NewStyle().Foreground(arena).Bold(true)
	branchStyle = lipgloss.NewStyle().Foreground(arena)
	mentionStyle = lipgloss.NewStyle().Foreground(arena)
	cmdStyle = lipgloss.NewStyle().Foreground(escuma).Bold(true)
	headStyle = lipgloss.NewStyle().Foreground(arena).Bold(true)
	roleStyle = lipgloss.NewStyle().Foreground(onada).Bold(true)
	barStyle = lipgloss.NewStyle().Foreground(ink).Background(escuma)
	projectStyle = lipgloss.NewStyle().Foreground(onada)
	// La carpeta de la barra d'estat: el mateix to que el projecte de la
	// capçalera, que és el que ja identifica on ets.
	dirStyle = lipgloss.NewStyle().Foreground(onada)
	modeCodeStyle = lipgloss.NewStyle().Foreground(ink).Background(arena).Bold(true).Padding(0, 1)
	modeChatStyle = lipgloss.NewStyle().Foreground(ink).Background(escuma).Bold(true).Padding(0, 1)
	modeInspectStyle = lipgloss.NewStyle().Foreground(ink).Background(onada).Bold(true).Padding(0, 1)
	modeAutoStyle = lipgloss.NewStyle().Foreground(ink).Background(teal).Bold(true).Padding(0, 1)
	toolRailStyle = lipgloss.NewStyle().Foreground(boxIdle)
	toolNameStyle = lipgloss.NewStyle().Foreground(onada).Bold(true)
	toolDoneStyle = lipgloss.NewStyle().Foreground(green).Bold(true)
	composerCode = lipgloss.NewStyle().Foreground(arena).Bold(true)
	composerChat = lipgloss.NewStyle().Foreground(escuma).Bold(true)
	modeGoalStyle = lipgloss.NewStyle().Foreground(lila).Background(lilaFons).Bold(true).Padding(0, 1)
	composerGoal = lipgloss.NewStyle().Foreground(lila).Bold(true)
	composerAuto = lipgloss.NewStyle().Foreground(teal).Bold(true)
	goalTitleStyle = lipgloss.NewStyle().Foreground(lila).Bold(true)
	spinnerStyle = lipgloss.NewStyle().Foreground(escuma).Bold(true)
	// crestStyle és l'escuma de la cresta que passa per sobre d'una etiqueta
	// mentre hi ha feina (shimmer): el mateix to que el primari, però clar.
	crestStyle = lipgloss.NewStyle().Foreground(arena).Bold(true)

	addStyle = lipgloss.NewStyle().Foreground(green)
	delStyle = lipgloss.NewStyle().Foreground(red)
	hunkStyle = lipgloss.NewStyle().Foreground(onada)

	menuHL = lipgloss.NewStyle().Foreground(ink).Background(teal).Bold(true)
}

// Logo és la marca de l'app.
func Logo() string {
	return logoStyle.Render("‹••› GREGAL")
}

// waveCrest és l'única onada de la línia d'horitzó: cim al mig, vessants
// i cua. La resta de la línia és un traç fi. Abans tota l'amplada anava
// plena de glifs d'onada esmorteïts cap als extrems i, a pantalla
// completa, es llegia com una franja de soroll; una línia i una cresta
// separen igual i es veuen com una cosa dibuixada a posta.
var waveCrest = []rune("·~≈≋≋≋≈~·")

// waveLine pinta la línia d'horitzó a l'amplada donada (chrome propi):
// traç fi amb la cresta centrada. Sempre fa exactament width cel·les.
func waveLine(width int) string {
	if width < 4 {
		return ""
	}
	out := make([]rune, width)
	for i := range out {
		out[i] = '─'
	}
	start := width/2 - len(waveCrest)/2
	for i, r := range waveCrest {
		if idx := start + i; idx >= 0 && idx < width {
			out[idx] = r
		}
	}
	return waveStyle.Render(string(out))
}

func modeBadge(mode string) string {
	if mode == "chat" {
		return modeChatStyle.Render(T("mode.chat.badge"))
	}
	if mode == "inspect" {
		return modeInspectStyle.Render(T("mode.inspect.badge"))
	}
	if mode == "goal" {
		return modeGoalStyle.Render(T("mode.goal.badge"))
	}
	if mode == "autonomous" {
		return modeAutoStyle.Render(T("mode.autonomous.badge"))
	}
	return modeCodeStyle.Render(T("mode.code.badge"))
}

// welcome és l'entrada: sis files i sense caixa. La marca amb el vent
// [45° NE], una frase, tres exemples d'instrucció i les tres tecles que
// cal saber. Abans era una targeta de tretze files amb els cinc modes
// explicats: els modes s'expliquen al selector de mode i a ?, i la
// conversa comença a dalt.
func welcome(role string, width int) string {
	brand := Logo() + " " + faintStyle.Render("[45° NE]")
	var b strings.Builder
	b.WriteString("  " + brand + "  " + dimStyle.Render(T("benvinguda.llest")) + "\n\n")
	for _, k := range []string{"benvinguda.exemple1", "benvinguda.exemple2", "benvinguda.exemple3"} {
		b.WriteString("  " + faintStyle.Render("›") + " " + dimStyle.Render(T(k)) + "\n")
	}
	b.WriteString("  " + faintStyle.Render(T("benvinguda.pista")) + roleStyle.Render(role))
	return b.String()
}

// helpBlock pinta l'ajuda agrupada: capçalera arena + ordres escuma.
func helpBlock() string {
	var b strings.Builder
	for _, g := range ajudaGrups {
		b.WriteString(headStyle.Render("≋ "+T(g.clauTitol)) + "\n")
		for _, f := range g.files {
			b.WriteString("  " + cmdStyle.Render(T(f[0])) + dimStyle.Render(" — "+T(f[1])) + "\n")
		}
	}
	return b.String()
}

// La conversa té tres veus i cadascuna es llegeix d'un cop d'ull:
//
//	▌ el que dius tu        — barra i fons propis, a l'esquerra
//	│ la feina de l'agent   — rail tènue: pensament, crides i resultats
//	  la resposta            — sense adorns, el que has vingut a llegir
//
// Abans «TU › » i «GREGAL › » eren dues etiquetes bessones i les crides
// d'eina tenien el mateix pes que la resposta: llegint de dalt a baix no
// se sabia què era una cosa i què l'altra.

// userLine pinta el teu missatge. Només fons i negreta: la barra ▌ era
// la tercera línia vertical d'una pantalla que ja té el rail de feina i
// (quan cal) la barra de scroll, i tres columnes de línies competint fan
// que no se'n miri cap.
func userLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	out := make([]string, 0, len(lines)+1)
	// Una línia en blanc abans: és el que separa un torn de l'anterior.
	out = append(out, "")
	for i, l := range lines {
		tag := "  "
		if i == 0 {
			tag = userStyle.Render(T("tu"))
		}
		out = append(out, tag+" "+userTextStyle.Render(l))
	}
	return strings.Join(out, "\n")
}

func assistantLine(s string) string { return assistantStyle.Render(s) }

// assistantMD és la resposta final: sense prefix ni rail, que és el que es
// ve a llegir. La separen del que hi ha sobre el rail de la feina i la
// barra del missatge teu.
func assistantMD(s string, width int) string {
	return renderMD(s, width)
}
func systemLine(s string) string { return systemStyle.Render("· " + s) }

// workRail posa el rail de feina a cada línia: tot el que fa l'agent
// (pensar, cridar eines, llegir-ne el resultat) queda en una sola columna
// visual, subordinada a la resposta.
func workRail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = toolRailStyle.Render("│ ") + lines[i]
	}
	return strings.Join(lines, "\n")
}

// dinsRail posa el text dins del rail de feina, dues columnes més endins
// que la fila de l'eina, i sense tocar-ne els colors: és per al diff i
// per al codi que ja vénen pintats. indentTool, en canvi, apaga el text.
func dinsRail(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = toolRailStyle.Render("│ ") + "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}

// maxFilesSortida és el que s'ensenya d'una sortida d'eina a la conversa.
// La resta és a Ctrl+L: un `go test` que peta amb quaranta línies no ha
// d'escombrar la pantalla, però les sis primeres solen dir el que cal.
const maxFilesSortida = 6

func indentTool(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := range lines {
		lines[i] = toolRailStyle.Render("│ ") + dimStyle.Render("  "+lines[i])
	}
	return strings.Join(lines, "\n")
}

// thinkLine és el pensament: al mateix rail que les eines, perquè es vegi
// que és feina i no resposta.
func thinkLine(s string) string { return workRail(dimStyle.Render("◇ " + s)) }

// toolCallLine diu què fa l'eina en llenguatge de persona: la ruta, la
// comanda, la consulta. El JSON dels arguments és per al model; a la
// pantalla «read internal/config/config.go» es llegeix d'un cop d'ull i
// «read {"path":"internal/config/config.go"}» no.
func toolCallLine(name, args string) string {
	return workRail(toolNameStyle.Render("▸ "+name) + " " + dimStyle.Render(humanArgs(name, args, 110)))
}

// humanArgs resumeix els arguments d'una crida per a la pantalla.
func humanArgs(name, args string, max int) string {
	var v map[string]any
	if json.Unmarshal([]byte(args), &v) != nil {
		return shortArgs(args, max)
	}
	str := func(k string) string {
		if s, ok := v[k].(string); ok {
			return strings.TrimSpace(s)
		}
		return ""
	}
	var out string
	switch name {
	case "read", "write", "edit", "patch", "read_image", "office_read", "office_open", "office_create":
		out = str("path")
		if name == "read" {
			if o, ok := v["offset"].(float64); ok && o > 0 {
				out += fmt.Sprintf(" des de la línia %d", int(o))
			}
		}
	case "office_edit":
		out = str("op") + " " + str("path")
		if c := str("cell"); c != "" {
			out += " " + c
		}
	case "bash", "bash_background":
		out = str("command")
	case "bash_output", "bash_kill":
		out = str("id")
	case "grep":
		out = "«" + str("pattern") + "»"
		if d := str("dir"); d != "" {
			out += " a " + d
		}
		if inc := str("include"); inc != "" {
			out += " (" + inc + ")"
		}
	case "glob":
		out = str("pattern")
		if d := str("dir"); d != "" {
			out += " a " + d
		}
	case "web_search":
		out = "«" + str("query") + "»"
	case "web_fetch":
		out = str("url")
		if f := str("find"); f != "" {
			out += " · cerca «" + f + "»"
		}
	case "browser":
		out = str("action")
		for _, k := range []string{"url", "target", "text", "key", "js"} {
			if val := str(k); val != "" {
				out += " " + val
				break
			}
		}
	case "todowrite":
		if items, ok := v["items"].([]any); ok {
			fets := 0
			for _, it := range items {
				if m, ok := it.(map[string]any); ok && m["status"] == "done" {
					fets++
				}
			}
			out = fmt.Sprintf("%d/%d passos", fets, len(items))
		}
	case "question":
		out = str("query")
	case "delegate":
		if tasks, ok := v["tasks"].([]any); ok {
			out = fmt.Sprintf("%d subagents", len(tasks))
		}
	case "gh_issue", "gh_pr":
		out = str("action")
		if n, ok := v["number"].(float64); ok {
			out += fmt.Sprintf(" #%d", int(n))
		}
	}
	if out == "" {
		return shortArgs(args, max)
	}
	out = strings.Join(strings.Fields(out), " ")
	if r := []rune(out); len(r) > max {
		out = string(r[:max-1]) + "…"
	}
	return out
}

// resultatPlegat diu si el resultat d'una eina es mostra només com a
// resum d'una línia (lectures que van bé: el cos és per al model i viu
// sencer a l'activitat, Ctrl+L). Els errors, el bash i les escriptures
// s'ensenyen: són el que la persona vol veure.
func resultatPlegat(name string, failed bool) bool {
	if failed {
		return false
	}
	switch name {
	case "read", "grep", "glob", "web_search", "web_fetch", "browser", "office_read", "todoread", "read_image", "gh_issue", "gh_pr", "delegate":
		return true
	}
	return false
}

// resumResultat és la línia de resum d'un resultat plegat.
func resumResultat(name, output string) string {
	out := strings.TrimSpace(output)
	if out == "" {
		return ""
	}
	n := strings.Count(out, "\n") + 1
	switch name {
	case "read":
		if n-1 == 1 {
			return "1 línia"
		}
		return fmt.Sprintf("%d línies", max(n-1, 0))
	case "grep", "glob":
		if strings.HasPrefix(out, "sense ") || strings.HasPrefix(out, "cap ") {
			return out
		}
		if n == 1 {
			return "1 resultat"
		}
		return fmt.Sprintf("%d resultats", n)
	case "web_search":
		return firstLineOf(out)
	case "web_fetch", "browser":
		// La primera línia és el títol; la segona, url i paraules.
		lines := strings.SplitN(out, "\n", 3)
		t := strings.TrimPrefix(lines[0], "# ")
		if len(lines) > 1 && strings.Contains(lines[1], "paraules") {
			t += " · " + strings.TrimSpace(lines[1][strings.LastIndex(lines[1], "·")+len("·"):])
		}
		return strings.TrimSpace(t)
	case "delegate":
		return fmt.Sprintf("%d línies de resum", n)
	}
	return firstLineOf(out)
}

func firstLineOf(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	if r := []rune(s); len(r) > 100 {
		return string(r[:99]) + "…"
	}
	return s
}

func toolResultLine(name, output string, failed bool) string {
	state := workRail(toolEstat(name, failed) + " " + dimStyle.Render(resumResultat(name, output)))
	if cos := toolResultCos(name, output, failed); cos != "" {
		return state + "\n" + cos
	}
	return state
}

// toolEstat és la marca i el nom de l'eina segons com hagi anat.
func toolEstat(name string, failed bool) string {
	if failed {
		return badStyle.Render("✗ " + name)
	}
	return toolDoneStyle.Render("✓ " + name)
}

// toolEstatLine és la fila d'una eina ja acabada: marca, nom, de què
// anava i com ha anat, tot en una. És la que substitueix la fila «▸ read
// x.go» quan arriba el resultat, perquè una eina ocupi una fila i no dues.
func toolEstatLine(name, args, output string, failed bool) string {
	fila := toolEstat(name, failed)
	if a := humanArgs(name, args, 90); a != "" {
		fila += " " + dimStyle.Render(a)
	}
	if r := resumResultat(name, output); r != "" && resultatPlegat(name, failed) {
		fila += faintStyle.Render("  · " + r)
	}
	return workRail(fila)
}

// toolResultCos és la sortida que s'ensenya sota la fila. Les lectures
// que van bé no en tenen: el cos és per al model i viu sencer a Ctrl+L.
func toolResultCos(name, output string, failed bool) string {
	if resultatPlegat(name, failed) {
		return ""
	}
	body := strings.TrimRight(capText(output, 700), "\n")
	if !failed {
		// Les escriptures confirmen amb una línia que repeteix el que ja
		// diu la fila de l'eina («edit aplicat a x.go») i, a sota, hi ha
		// el diff. El que SÍ cal ensenyar és el que vingui després: la
		// sortida del hook post-edit i els diagnòstics.
		if resta, retallat := senseConfirmacio(name, body); retallat {
			body = resta
		}
	}
	if strings.TrimSpace(body) == "" {
		return ""
	}
	files := strings.Split(body, "\n")
	if len(files) > maxFilesSortida {
		resta := len(files) - maxFilesSortida
		body = strings.Join(files[:maxFilesSortida], "\n")
		return indentTool(body) + "\n" + workRail(faintStyle.Render(fmt.Sprintf("  … %d %s (Ctrl+L)", resta, plural(resta, "fila més", "files més"))))
	}
	return indentTool(body)
}

// plural tria la forma segons el nombre. «1 fitxers · 1 errors» era el
// detall petit que feia semblar el resum escrit per una màquina.
func plural(n int, un, molts string) string {
	if n == 1 {
		return un
	}
	return molts
}

// senseConfirmacio treu la primera línia d'una sortida d'escriptura
// quan només és l'acusament de rebut. Torna la resta i si ha retallat.
func senseConfirmacio(name, body string) (string, bool) {
	switch name {
	case "write", "edit", "patch", "office_edit", "office_create":
	default:
		return body, false
	}
	primera, resta := body, ""
	if i := strings.Index(body, "\n"); i >= 0 {
		primera, resta = body[:i], body[i+1:]
	}
	for _, p := range []string{"escrit ", "edit aplicat", "patch aplicat", "creat ", "substitu", "afegit", "posat "} {
		if strings.HasPrefix(strings.TrimSpace(primera), p) {
			return strings.TrimSpace(resta), true
		}
	}
	return body, false
}

// diffBlock pinta un diff unificat compacte (verd +, vermell −, blau @@).
// old/new són el contingut abans/després; cap a maxLines línies.
func diffBlock(path, old, new string, maxLines int) string {
	return hunkStyle.Render("≋ diff "+path) + "\n" + diffCos(path, old, new, maxLines)
}

// diffCos és el diff sense capçalera: dins del rail, sota la fila de
// l'edit, la ruta ja hi és i repetir-la només ocupa una fila.
func diffCos(path, old, new string, maxLines int) string {
	ops := diffLines(splitLines(old), splitLines(new))
	if maxLines > 0 && len(ops) > maxLines {
		ops = append(ops[:maxLines], diffOp{kind: 't'})
	}
	// El codi es pinta d'una sola passada i després es reparteix per
	// línies: així chroma veu el context del bloc (cadenes i comentaris
	// de diverses línies) i no cada línia aïllada.
	cos := make([]string, len(ops))
	for i, o := range ops {
		cos[i] = o.text
	}
	if pintat, ok := tema.PintaFitxerANSI(TemaActiu(), path, strings.Join(cos, "\n")); ok {
		if parts := strings.Split(pintat, "\n"); len(parts) == len(cos) {
			cos = parts
		}
	}
	var b strings.Builder
	for i, o := range ops {
		switch o.kind {
		case 't':
			b.WriteString(faintStyle.Render("  … (retallat)") + "\n")
		case ' ':
			// Context sense marge propi: el │ xocava amb el rail de feina,
			// i dues columnes de línies verticals seguides no diuen res.
			b.WriteString("  " + cos[i] + "\n")
		case '-':
			b.WriteString(delStyle.Render("− ") + cos[i] + "\n")
		case '+':
			b.WriteString(addStyle.Render("+ ") + cos[i] + "\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

type diffOp struct {
	kind byte // ' ', '-', '+', 't' (tall)
	text string
}

// diffLines alinea per prefix/sufix comú i marca el mig com canvi.
// No és un LCS complet: per a preview de TUI n'hi ha prou i és O(n).
func diffLines(a, b []string) []diffOp {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	var ops []diffOp
	ctx0 := pre - 3
	if ctx0 < 0 {
		ctx0 = 0
	}
	for _, l := range a[ctx0:pre] {
		ops = append(ops, diffOp{' ', l})
	}
	for _, l := range a[pre : len(a)-suf] {
		ops = append(ops, diffOp{'-', l})
	}
	for _, l := range b[pre : len(b)-suf] {
		ops = append(ops, diffOp{'+', l})
	}
	tail := 0
	for tail < suf && tail < 3 {
		ops = append(ops, diffOp{' ', a[len(a)-suf+tail]})
		tail++
	}
	return ops
}

// splitLines parteix en línies; "" = cap línia (no una línia buida).
// Un sol \n final no compta com a línia (evita el "+" fantasma al diff).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	ls := strings.Split(s, "\n")
	if len(ls) > 0 && ls[len(ls)-1] == "" {
		ls = ls[:len(ls)-1]
	}
	return ls
}

// ajudaGrups és /help com a dades: la sintaxi de l'ordre a l'esquerra i
// una clau de traducció a la dreta. Escrit inline no es podia ni llegir
// ni traduir.
var ajudaGrups = []struct {
	clauTitol string
	files     [][2]string // {clau de sintaxi, clau de descripció}
}{
	{"ajuda.agent", [][2]string{
		{"help.agent.cmd", "help.agent"},
		{"help.parallel.cmd", "help.parallel"},
		{"help.rewind.cmd", "help.rewind"},
		{"help.checkpoints.cmd", "help.checkpoints"},
		{"help.diff.cmd", "help.diff"},
		{"help.graf.cmd", "help.graf"},
		{"help.compact.cmd", "help.compact"},
		{"help.verify.cmd", "help.verify"},
	}},
	{"ajuda.xat", [][2]string{
		{"help.entercode.cmd", "help.entercode"},
		{"help.enterchat.cmd", "help.enterchat"},
		{"help.altenter.cmd", "help.altenter"},
		{"help.forceagent.cmd", "help.forceagent"},
		{"help.mention.cmd", "help.mention"},
		{"help.readbash.cmd", "help.readbash"},
		{"help.write.cmd", "help.write"},
		{"help.edit.cmd", "help.edit"},
	}},
	{"ajuda.sessio", [][2]string{
		{"help.roles.cmd", "help.roles"},
		{"help.model.cmd", "help.model"},
		{"help.mode.cmd", "help.mode"},
		{"help.reviewer.cmd", "help.reviewer"},
		{"help.sessions.cmd", "help.sessions"},
		{"help.perms.cmd", "help.perms"},
		{"help.clear.cmd", "help.clear"},
	}},
	{"ajuda.modes", [][2]string{
		{"mode.code.badge", "mode.code.desc"},
		{"mode.chat.badge", "mode.chat.desc"},
		{"mode.inspect.badge", "mode.inspect.desc"},
		{"mode.goal.badge", "mode.goal.desc"},
		{"mode.autonomous.badge", "mode.autonomous.desc"},
	}},
	{"ajuda.tecles", [][2]string{
		{"help.shifttab.cmd", "help.shifttab"},
		{"help.ctrlt.cmd", "help.ctrlt"},
		{"help.ctrll.cmd", "help.ctrll"},
		{"help.bang.cmd", "help.bang"},
		{"help.queue.cmd", "help.queue"},
		{"help.answer.cmd", "help.answer"},
	}},
	{"ajuda.teves", [][2]string{
		{"help.user.cmd", "help.user"},
	}},
}
