package tools

// Lectura de la pàgina per l'arbre d'accessibilitat (Accessibility.getFullAXTree),
// com fan Playwright MCP, Chrome DevTools MCP i Claude in Chrome: cada element
// amb el seu rol, nom i estat, i els interactius amb una ref estable (e12)
// lligada al node del DOM (backendDOMNodeId). Res de marcar el DOM ni de
// selectors: la ref sobreviu mentre el node existeix i les accions hi van
// directes (DOM.getContentQuads, DOM.resolveNode).
//
// Després d'una acció no cal tornar a llegir: es retorna només el que ha
// canviat respecte de l'última lectura (diff de línies), que és una fracció
// dels tokens d'una lectura sencera.
//
// Si el transport no sap fer l'arbre (CDP fals dels tests, navegadors
// antics), Snapshot cau a la lectura de sempre (markdown + [n]).

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// axState és la memòria de refs d'un navegador.
type axState struct {
	active bool        // l'última lectura era d'accessibilitat
	doc    string      // URL sense fragment: si canvia, refs noves
	next   int         // propera ref
	byNode map[int]int // backendDOMNodeId → ref
	refs   map[int]axRef
	last   []string // línies de l'última lectura (per al diff)
	url    string   // URL completa de l'última lectura
	title  string
}

// axRef és el que se sap d'una ref sense tornar a preguntar al navegador
// (el permís d'acció irreversible es decideix amb això).
type axRef struct {
	node int
	role string
	name string
}

type axValue struct {
	Value any `json:"value"`
}

func (v *axValue) str() string {
	if v == nil {
		return ""
	}
	switch x := v.Value.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

type axNode struct {
	NodeID     string   `json:"nodeId"`
	Ignored    bool     `json:"ignored"`
	Role       *axValue `json:"role"`
	Name       *axValue `json:"name"`
	Value      *axValue `json:"value"`
	Properties []struct {
		Name  string  `json:"name"`
		Value axValue `json:"value"`
	} `json:"properties"`
	ChildIDs []string `json:"childIds"`
	Backend  int      `json:"backendDOMNodeId"`
}

func (n *axNode) prop(name string) (string, bool) {
	for _, p := range n.Properties {
		if p.Name == name {
			return p.Value.str(), true
		}
	}
	return "", false
}

// Rols on es pot actuar: porten ref.
var axInteractive = map[string]bool{
	"button": true, "link": true, "textbox": true, "searchbox": true, "combobox": true,
	"checkbox": true, "radio": true, "switch": true, "slider": true, "spinbutton": true,
	"tab": true, "menuitem": true, "menuitemcheckbox": true, "menuitemradio": true,
	"option": true, "treeitem": true, "listbox": true, "textarea": true, "gridcell": false,
	"columnheader": false, "disclosuretriangle": true, "PopUpButton": true,
}

// Rols fulla: el text de dins ja és el seu nom; no cal baixar-hi.
var axLeaf = map[string]bool{
	"button": true, "link": true, "textbox": true, "searchbox": true, "checkbox": true,
	"radio": true, "switch": true, "slider": true, "spinbutton": true, "tab": true,
	"menuitem": true, "menuitemcheckbox": true, "menuitemradio": true, "option": true,
	"img": true, "image": true, "heading": true, "textarea": true, "PopUpButton": true,
}

// Rols que no diuen res per si sols: es travessen sense línia.
var axPassThrough = map[string]bool{
	"generic": true, "none": true, "presentation": true, "group": true, "paragraph": true,
	"Section": true, "LayoutTable": true, "LayoutTableRow": true, "LayoutTableCell": true,
	"list": true, "listitem": true, "ListMarker": true, "InlineTextBox": true, "LineBreak": true,
	"emphasis": true, "strong": true, "Abbr": true, "time": true, "code": true, "blockquote": true,
	"figure": true, "Canvas": true, "article": true, "Pre": true, "superscript": true, "subscript": true,
	"label": true, "LabelText": true, "DescriptionList": true, "term": true, "definition": true,
	"mark": true, "insertion": true, "deletion": true, "contentinfo": false,
}

// Marques: surten encara que no tinguin nom, perquè situen el model.
var axLandmark = map[string]bool{
	"banner": true, "navigation": true, "main": true, "contentinfo": true, "complementary": true,
	"form": true, "search": true, "dialog": true, "alertdialog": true, "alert": true,
	"table": true, "grid": true, "tablist": true, "menu": true, "menubar": true, "tree": true,
	"toolbar": true, "status": true, "tabpanel": true, "region": false,
}

const (
	axMaxLines = 450
	axMaxChars = 16000
	axTextMax  = 180
)

// axSnapshot llegeix l'arbre i el pinta. Torna ok=false si el transport no
// en sap (aleshores es fa la lectura de sempre).
func (b *Browser) axSnapshot() (lines []string, ok bool, err error) {
	raw, err := b.call("Accessibility.getFullAXTree", map[string]any{})
	if err != nil {
		return nil, false, nil
	}
	var res struct {
		Nodes []axNode `json:"nodes"`
	}
	if json.Unmarshal(raw, &res) != nil || len(res.Nodes) == 0 {
		return nil, false, nil
	}
	href, _ := b.evalString("location.href")
	title, _ := b.evalString("document.title")
	frames := b.axFrames()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ax == nil {
		b.ax = &axState{}
	}
	ax := b.ax
	doc := href
	if u, e := url.Parse(href); e == nil {
		u.Fragment = ""
		doc = u.String()
	}
	if doc != ax.doc || ax.byNode == nil {
		ax.doc, ax.next = doc, 0
		ax.byNode, ax.refs = map[int]int{}, map[int]axRef{}
		ax.last = nil
	}
	ax.url, ax.title, ax.active = href, title, true
	byID := make(map[string]*axNode, len(res.Nodes))
	for i := range res.Nodes {
		byID[res.Nodes[i].NodeID] = &res.Nodes[i]
	}
	root := &res.Nodes[0]
	w := &axWriter{ax: ax, byID: byID}
	w.walk(root, 0, "")
	// Els iframes: molts webs (jocs, intranets, webmails) hi posen el
	// contingut. Cada marc té el seu arbre; les refs hi funcionen igual
	// perquè el node és del mateix procés.
	for _, f := range frames {
		w.flushText()
		head := "- iframe " + strconv.Quote(axClip(f.url, 120))
		if f.nodes == nil {
			w.lines = append(w.lines, head+" (another site: not readable as text; use screenshot and click x/y)")
			continue
		}
		w.lines = append(w.lines, head)
		fw := &axWriter{ax: ax, byID: make(map[string]*axNode, len(f.nodes))}
		for i := range f.nodes {
			fw.byID[f.nodes[i].NodeID] = &f.nodes[i]
		}
		fw.walk(&f.nodes[0], 1, "")
		fw.flushText()
		w.lines = append(w.lines, fw.lines...)
	}
	return w.lines, true, nil
}

type axFrame struct {
	url   string
	nodes []axNode // nil: un marc d'un altre lloc (procés a part)
}

// axFrames llegeix els arbres dels iframes (fins a 8).
func (b *Browser) axFrames() []axFrame {
	raw, err := b.call("Page.getFrameTree", nil)
	if err != nil {
		return nil
	}
	type frameNode struct {
		Frame struct {
			ID  string `json:"id"`
			URL string `json:"url"`
		} `json:"frame"`
		Children []json.RawMessage `json:"childFrames"`
	}
	var tree struct {
		FrameTree frameNode `json:"frameTree"`
	}
	if json.Unmarshal(raw, &tree) != nil {
		return nil
	}
	var out []axFrame
	var visit func(children []json.RawMessage)
	visit = func(children []json.RawMessage) {
		for _, c := range children {
			if len(out) >= 8 {
				return
			}
			var f frameNode
			if json.Unmarshal(c, &f) != nil || f.Frame.URL == "" || f.Frame.URL == "about:blank" {
				continue
			}
			fr := axFrame{url: f.Frame.URL}
			if r, err := b.call("Accessibility.getFullAXTree", map[string]any{"frameId": f.Frame.ID}); err == nil {
				var res struct {
					Nodes []axNode `json:"nodes"`
				}
				if json.Unmarshal(r, &res) == nil && len(res.Nodes) > 0 {
					fr.nodes = res.Nodes
				}
			}
			out = append(out, fr)
			visit(f.Children)
		}
	}
	visit(tree.FrameTree.Children)
	return out
}

type axWriter struct {
	ax    *axState
	byID  map[string]*axNode
	lines []string
	text  []string // text pendent de la mateixa tirada (s'ajunta en una línia)
	depth int
}

func (w *axWriter) flushText() {
	if len(w.text) == 0 {
		return
	}
	t := strings.Join(w.text, " ")
	w.text = nil
	t = strings.Join(strings.Fields(t), " ")
	if t == "" {
		return
	}
	w.lines = append(w.lines, strings.Repeat("  ", w.depth)+"- text: "+axClip(t, axTextMax*2))
}

func axClip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ref torna la ref del node (la mateixa mentre el document no canviï).
func (w *axWriter) ref(n *axNode, role, name string) int {
	if n.Backend == 0 {
		return 0
	}
	r, ok := w.ax.byNode[n.Backend]
	if !ok {
		w.ax.next++
		r = w.ax.next
		w.ax.byNode[n.Backend] = r
	}
	w.ax.refs[r] = axRef{node: n.Backend, role: role, name: name}
	return r
}

// walk pinta un node i els fills. parentName evita repetir el text que ja
// és el nom del pare (un enllaç «Inici» amb un text «Inici» a dins).
func (w *axWriter) walk(n *axNode, depth int, parentName string) {
	if n == nil {
		return
	}
	role := n.Role.str()
	name := strings.Join(strings.Fields(n.Name.str()), " ")
	children := func(d int, pname string) {
		for _, id := range n.ChildIDs {
			w.walk(w.byID[id], d, pname)
		}
	}
	if n.Ignored {
		children(depth, parentName)
		return
	}
	switch role {
	case "RootWebArea", "WebArea":
		children(depth, parentName)
		w.flushText()
		return
	case "StaticText":
		if name != "" && name != parentName {
			w.depth = depth
			w.text = append(w.text, name)
		}
		return
	}
	focusable, _ := n.prop("focusable")
	editable, _ := n.prop("editable")
	interactive := axInteractive[role] || (editable != "" && editable != "false" && role != "StaticText") ||
		(focusable == "true" && name != "" && !axPassThrough[role] && !axLandmark[role] && role != "heading" && role != "img")
	if !interactive && axPassThrough[role] && !(role == "group" && name != "") {
		children(depth, parentName)
		return
	}
	if role == "row" && !w.hasInteractive(n) {
		cells := w.rowText(n)
		if cells != "" {
			w.flushText()
			w.lines = append(w.lines, strings.Repeat("  ", depth)+"- row: "+axClip(cells, axTextMax*2))
		}
		return
	}
	if !interactive && !axLandmark[role] && name == "" && role != "img" && role != "image" {
		children(depth, parentName)
		return
	}
	if (role == "img" || role == "image") && name == "" {
		return
	}
	// L'etiqueta d'un camp ja és el seu nom: «text: Nom» + «textbox "Nom"»
	// sobra.
	if name != "" && len(w.text) > 0 && strings.TrimSpace(w.text[len(w.text)-1]) == name {
		w.text = w.text[:len(w.text)-1]
	}
	w.flushText()
	var b strings.Builder
	b.WriteString(strings.Repeat("  ", depth))
	b.WriteString("- " + role)
	if name != "" {
		b.WriteString(" " + strconv.Quote(axClip(name, axTextMax)))
	}
	if interactive {
		if r := w.ref(n, role, name); r > 0 {
			fmt.Fprintf(&b, " [ref=e%d]", r)
		}
	}
	for _, p := range []string{"checked", "pressed", "selected", "expanded", "disabled", "required", "invalid", "level", "readonly", "haspopup"} {
		v, ok := n.prop(p)
		if !ok || v == "" || v == "false" {
			continue
		}
		switch {
		case p == "level":
			b.WriteString(" level=" + v)
		case p == "invalid" && v != "true":
			b.WriteString(" invalid=" + v)
		case p == "haspopup":
			b.WriteString(" popup=" + v)
		case v == "true":
			b.WriteString(" " + p)
		default:
			b.WriteString(" " + p + "=" + v)
		}
	}
	if v := strings.TrimSpace(n.Value.str()); v != "" && v != name {
		b.WriteString(" value=" + strconv.Quote(axClip(v, axTextMax)))
	}
	w.lines = append(w.lines, b.String())
	if axLeaf[role] {
		return
	}
	children(depth+1, name)
	w.flushText()
}

// hasInteractive diu si algun descendent porta ref.
func (w *axWriter) hasInteractive(n *axNode) bool {
	for _, id := range n.ChildIDs {
		c := w.byID[id]
		if c == nil {
			continue
		}
		if !c.Ignored && axInteractive[c.Role.str()] {
			return true
		}
		if w.hasInteractive(c) {
			return true
		}
	}
	return false
}

// rowText és una fila de taula en una línia: «cel·la | cel·la | …».
func (w *axWriter) rowText(n *axNode) string {
	var cells []string
	for _, id := range n.ChildIDs {
		c := w.byID[id]
		if c == nil {
			continue
		}
		t := strings.Join(strings.Fields(c.Name.str()), " ")
		if t == "" {
			t = w.plainText(c)
		}
		cells = append(cells, t)
	}
	out := strings.Join(cells, " | ")
	if strings.Trim(out, " |") == "" {
		return ""
	}
	return out
}

func (w *axWriter) plainText(n *axNode) string {
	if n.Role.str() == "StaticText" {
		return n.Name.str()
	}
	var parts []string
	for _, id := range n.ChildIDs {
		if c := w.byID[id]; c != nil {
			if t := strings.TrimSpace(w.plainText(c)); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, " ")
}

// axRender posa capçalera, filtre (find), abast (scope) i topall.
func axRender(urlS, title string, lines []string, find string, total bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Page: %s\nURL: %s\n", title, urlS)
	if nota := NotaAntiRobots(title, hostDe(urlS)); nota != "" {
		b.WriteString(nota + "\n")
	}
	sel := lines
	if f := strings.ToLower(strings.TrimSpace(find)); f != "" {
		sel = nil
		for _, l := range lines {
			if strings.Contains(strings.ToLower(l), f) {
				sel = append(sel, strings.TrimLeft(l, " "))
			}
		}
		fmt.Fprintf(&b, "Lines matching %q: %d of %d\n", find, len(sel), len(lines))
	}
	b.WriteString("\n")
	n, chars := 0, 0
	for i, l := range sel {
		if n >= axMaxLines || chars+len(l) > axMaxChars {
			fmt.Fprintf(&b, "… %d more lines (use find:\"…\" or scope:\"eN\" to read a part)\n", len(sel)-i)
			break
		}
		b.WriteString(l + "\n")
		n++
		chars += len(l) + 1
	}
	if total && len(sel) == 0 {
		b.WriteString("(empty page)\n")
	}
	b.WriteString("\nAct with the ref: click/type/select/hover target:\"e12\".")
	return b.String()
}

// axScope retalla les línies al subarbre d'una ref.
func axScope(lines []string, ref int) []string {
	tag := fmt.Sprintf("[ref=e%d]", ref)
	for i, l := range lines {
		if !strings.Contains(l, tag) {
			continue
		}
		ind := len(l) - len(strings.TrimLeft(l, " "))
		out := []string{l}
		for _, m := range lines[i+1:] {
			if len(m)-len(strings.TrimLeft(m, " ")) <= ind {
				break
			}
			out = append(out, m)
		}
		return out
	}
	return nil
}

// axDiff compara dues lectures: línies que han aparegut (+) i desaparegut (-).
func axDiff(prev, cur []string) (added, removed []string) {
	// LCS per línies: les pàgines són de centenars de línies, n*m va bé.
	n, m := len(prev), len(cur)
	if n*m > 600*600 {
		seen := map[string]int{}
		for _, l := range prev {
			seen[l]++
		}
		for _, l := range cur {
			if seen[l] > 0 {
				seen[l]--
				continue
			}
			added = append(added, l)
		}
		have := map[string]int{}
		for _, l := range cur {
			have[l]++
		}
		for _, l := range prev {
			if have[l] > 0 {
				have[l]--
				continue
			}
			removed = append(removed, l)
		}
		return
	}
	dp := make([][]int32, n+1)
	for i := range dp {
		dp[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if prev[i] == cur[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case prev[i] == cur[j]:
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			removed = append(removed, prev[i])
			i++
		default:
			added = append(added, cur[j])
			j++
		}
	}
	removed = append(removed, prev[i:]...)
	added = append(added, cur[j:]...)
	return
}

// AXRead és l'acció read: arbre sencer (o filtrat), i el desa per al diff.
func (b *Browser) AXRead(find string, scope int) (string, bool, error) {
	lines, ok, err := b.axSnapshot()
	if err != nil || !ok {
		return "", ok, err
	}
	b.mu.Lock()
	b.ax.last = lines
	u, t := b.ax.url, b.ax.title
	b.mu.Unlock()
	if scope > 0 {
		if part := axScope(lines, scope); part != nil {
			lines = part
		}
	}
	return axRender(u, t, lines, find, true), true, nil
}

// AXChanges és el que una acció retorna: el que ha canviat a la pàgina.
// Una pàgina nova (o un canvi gran) es retorna sencera.
func (b *Browser) AXChanges() string {
	b.mu.Lock()
	var prev []string
	var prevDoc string
	if b.ax != nil {
		prev, prevDoc = b.ax.last, b.ax.doc
	}
	b.mu.Unlock()
	lines, ok, _ := b.axSnapshot()
	if !ok {
		return ""
	}
	b.mu.Lock()
	b.ax.last = lines
	u, t, doc := b.ax.url, b.ax.title, b.ax.doc
	b.mu.Unlock()
	if prev == nil || doc != prevDoc {
		return "New page.\n" + axRender(u, t, lines, "", true)
	}
	added, removed := axDiff(prev, lines)
	if len(added) == 0 && len(removed) == 0 {
		return "Page unchanged (" + t + ")."
	}
	if len(added)+len(removed) > 140 || len(added)+len(removed) > len(lines)*6/10 {
		return "Page changed a lot.\n" + axRender(u, t, lines, "", true)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Changes on %s (+%d -%d lines):\n", t, len(added), len(removed))
	shown := 0
	for _, l := range removed {
		if shown >= 30 {
			break
		}
		out.WriteString("- " + strings.TrimPrefix(strings.TrimLeft(l, " "), "- ") + "\n")
		shown++
	}
	for _, l := range added {
		if shown >= 90 {
			fmt.Fprintf(&out, "… and %d more (read to see everything)\n", len(added)+len(removed)-shown)
			break
		}
		out.WriteString("+ " + strings.TrimPrefix(strings.TrimLeft(l, " "), "- ") + "\n")
		shown++
	}
	return strings.TrimRight(out.String(), "\n")
}

// axRefOf interpreta «e12», «12», «[12]», «ref=e12».
func axRefOf(target string) (int, bool) {
	t := strings.TrimSpace(target)
	t = strings.TrimPrefix(strings.Trim(t, "[]"), "ref=")
	t = strings.TrimPrefix(t, "e")
	n, err := strconv.Atoi(t)
	return n, err == nil && n > 0
}

// refNode torna el node d'una ref de la lectura d'accessibilitat.
func (b *Browser) refNode(target string) (axRef, bool) {
	n, ok := axRefOf(target)
	if !ok {
		return axRef{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ax == nil || !b.ax.active {
		return axRef{}, false
	}
	r, ok := b.ax.refs[n]
	return r, ok
}

// RefInfo diu el rol i el nom d'una ref (per al permís, sense CDP).
func (b *Browser) RefInfo(target string) (role, name string, ok bool) {
	r, ok := b.refNode(target)
	return r.role, r.name, ok
}

// ---- accions sobre nodes ----

// nodePoint fa visible el node i torna el centre del seu primer quad.
func (b *Browser) nodePoint(node int) (float64, float64, error) {
	_, _ = b.call("DOM.scrollIntoViewIfNeeded", map[string]any{"backendNodeId": node})
	raw, err := b.call("DOM.getContentQuads", map[string]any{"backendNodeId": node})
	if err != nil {
		return 0, 0, err
	}
	var res struct {
		Quads [][]float64 `json:"quads"`
	}
	if json.Unmarshal(raw, &res) != nil {
		return 0, 0, fmt.Errorf("browser: element sense posició")
	}
	for _, q := range res.Quads {
		if len(q) < 8 {
			continue
		}
		x := (q[0] + q[2] + q[4] + q[6]) / 4
		y := (q[1] + q[3] + q[5] + q[7]) / 4
		if q[2]-q[0] != 0 || q[5]-q[1] != 0 {
			return x, y, nil
		}
	}
	return 0, 0, fmt.Errorf("browser: l'element no es veu (sense mida)")
}

// callOn executa fn amb this = el node i torna el valor.
func (b *Browser) callOn(node int, fn string, args ...any) (json.RawMessage, error) {
	raw, err := b.call("DOM.resolveNode", map[string]any{"backendNodeId": node})
	if err != nil {
		return nil, fmt.Errorf("browser: l'element ja no hi és (llegeix la pàgina de nou): %v", err)
	}
	var obj struct {
		Object struct {
			ObjectID string `json:"objectId"`
		} `json:"object"`
	}
	if json.Unmarshal(raw, &obj) != nil || obj.Object.ObjectID == "" {
		return nil, fmt.Errorf("browser: l'element ja no hi és (llegeix la pàgina de nou)")
	}
	defer b.call("Runtime.releaseObject", map[string]any{"objectId": obj.Object.ObjectID})
	cargs := make([]map[string]any, len(args))
	for i, a := range args {
		cargs[i] = map[string]any{"value": a}
	}
	res, err := b.call("Runtime.callFunctionOn", map[string]any{
		"objectId": obj.Object.ObjectID, "functionDeclaration": fn, "arguments": cargs,
		"returnByValue": true, "awaitPromise": true,
	})
	if err != nil {
		return nil, err
	}
	var r struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(res, &r); err != nil {
		return nil, err
	}
	if r.ExceptionDetails != nil {
		return nil, fmt.Errorf("browser: JavaScript: %s", truncateInline(r.ExceptionDetails.Text, 200))
	}
	return r.Result.Value, nil
}

func (b *Browser) callOnString(node int, fn string, args ...any) (string, error) {
	v, err := b.callOn(node, fn, args...)
	if err != nil {
		return "", err
	}
	var s string
	if json.Unmarshal(v, &s) == nil {
		return s, nil
	}
	return string(v), nil
}

// mouse envia un clic real (moure, prémer, deixar anar).
func (b *Browser) mouse(x, y float64, button string, count int, modifiers int) error {
	if button == "" {
		button = "left"
	}
	if count <= 0 {
		count = 1
	}
	if _, err := b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y, "modifiers": modifiers}); err != nil {
		return err
	}
	for c := 1; c <= count; c++ {
		for _, tp := range []string{"mousePressed", "mouseReleased"} {
			if _, err := b.call("Input.dispatchMouseEvent", map[string]any{"type": tp, "x": x, "y": y, "button": button, "clickCount": c, "modifiers": modifiers}); err != nil {
				return err
			}
		}
	}
	return nil
}

// jsHit diu si el punt (x, y) cau sobre l'element o a dins (no tapat).
// Dins d'un iframe, (x, y) són del marc de dalt: es passen al del document.
const jsHit = `function(x, y) {
  let w = window;
  try { while (w.frameElement) { const r = w.frameElement.getBoundingClientRect(); x -= r.left + w.frameElement.clientLeft; y -= r.top + w.frameElement.clientTop; w = w.parent; } } catch (err) { return true; }
  const e = document.elementFromPoint(x, y); return !!e && (e === this || this.contains(e) || e.contains(this)); }`

// ClickRef fa clic a una ref. Si una altra cosa la tapa (un bàner, una
// capa), fa el clic per JavaScript i ho diu.
func (b *Browser) ClickRef(r axRef, button string, count, modifiers int) (string, error) {
	x, y, err := b.nodePoint(r.node)
	what := axDescribe(r)
	if err != nil {
		if _, e2 := b.callOn(r.node, `function() { this.scrollIntoView({block: 'center'}); this.click(); }`); e2 != nil {
			return "", err
		}
		return "clicked " + what + " (by script: " + err.Error() + ")", nil
	}
	if hit, _ := b.callOnString(r.node, jsHit, x, y); hit == "false" {
		if _, err := b.callOn(r.node, `function() { this.click(); }`); err != nil {
			return "", err
		}
		return "clicked " + what + " (it was covered by another element, so by script)", nil
	}
	if err := b.mouse(x, y, button, count, modifiers); err != nil {
		return "", err
	}
	return "clicked " + what, nil
}

func axDescribe(r axRef) string {
	if r.name == "" {
		return r.role
	}
	return r.role + " " + strconv.Quote(axClip(r.name, 60))
}

// jsCredential: camps on l'agent no escriu mai (contrasenyes, targetes,
// codis d'un sol ús). Ho ha de fer la persona.
const jsCredential = `function() {
  const t = String(this.type || '').toLowerCase();
  const ac = String(this.autocomplete || (this.getAttribute && this.getAttribute('autocomplete')) || '').toLowerCase();
  const nm = String((this.name || '') + ' ' + (this.id || '')).toLowerCase();
  if (t === 'password' || /cc-|one-time-code|current-password|new-password/.test(ac) || /card.?num|cardnumber|cvc|cvv|iban/.test(nm)) return 'blocked';
  return 'ok';
}`

// ErrCredentialField diu per què l'agent no hi escriu.
var ErrCredentialField = fmt.Errorf("browser: this is a password, card or one-time-code field: Gregal never types credentials. Ask the user to type it themselves, then continue")

// jsFocusClear enfoca el camp i en selecciona el contingut perquè el text
// nou el substitueixi (així React i companyia veuen un input normal).
const jsFocusClear = `function(clear) {
  this.scrollIntoView({block: 'center'});
  this.focus();
  if (!clear) return 'ok';
  try {
    if (typeof this.select === 'function') { this.select(); return 'ok'; }
  } catch (e) {}
  if ('value' in this) { this.value = ''; this.dispatchEvent(new Event('input', {bubbles: true})); return 'ok'; }
  if (this.isContentEditable) { const r = document.createRange(); r.selectNodeContents(this); const s = getSelection(); s.removeAllRanges(); s.addRange(r); }
  return 'ok';
}`

// TypeRef escriu en una ref (substituint el que hi hagi, si clear).
func (b *Browser) TypeRef(r axRef, text string, clear, enter bool) (string, error) {
	if v, _ := b.callOnString(r.node, jsCredential); v == "blocked" {
		return "", ErrCredentialField
	}
	if _, err := b.callOn(r.node, jsFocusClear, clear); err != nil {
		return "", err
	}
	if text != "" {
		if _, err := b.call("Input.insertText", map[string]any{"text": text}); err != nil {
			return "", err
		}
	} else if clear {
		_ = b.Press("Backspace")
	}
	msg := fmt.Sprintf("typed %q into %s", truncateInline(text, 60), axDescribe(r))
	if enter {
		if err := b.Press("Enter"); err != nil {
			return "", err
		}
		msg += " + Enter"
	}
	return msg, nil
}

// jsSelect tria una opció d'un <select> per valor o pel text.
const jsSelect = `function(want) {
  if (this.tagName !== 'SELECT') return 'not-select';
  const w = String(want).trim().toLowerCase();
  const opts = Array.from(this.options);
  const o = opts.find(o => o.value === want) || opts.find(o => o.text.trim().toLowerCase() === w) || opts.find(o => o.text.toLowerCase().includes(w));
  if (!o) return 'none:' + opts.map(o => o.text.trim()).slice(0, 40).join(' | ');
  this.value = o.value; o.selected = true;
  this.dispatchEvent(new Event('input', {bubbles: true}));
  this.dispatchEvent(new Event('change', {bubbles: true}));
  return 'ok:' + o.text.trim();
}`

// SelectRef tria una opció. Els desplegables fets a mà (ARIA) no són
// <select>: es clica i es tria l'opció amb un altre clic.
func (b *Browser) SelectRef(r axRef, option string) (string, error) {
	v, err := b.callOnString(r.node, jsSelect, option)
	if err != nil {
		return "", err
	}
	switch {
	case v == "not-select":
		return "", fmt.Errorf("browser: %s is not a native <select>: click it and then click the option", axDescribe(r))
	case strings.HasPrefix(v, "none:"):
		return "", fmt.Errorf("browser: no option %q; options: %s", option, strings.TrimPrefix(v, "none:"))
	}
	return "selected " + strconv.Quote(strings.TrimPrefix(v, "ok:")) + " in " + axDescribe(r), nil
}

// HoverRef posa el ratolí a sobre (menús que s'obren en passar-hi).
func (b *Browser) HoverRef(r axRef) (string, error) {
	x, y, err := b.nodePoint(r.node)
	if err != nil {
		return "", err
	}
	if _, err := b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y}); err != nil {
		return "", err
	}
	return "hovering " + axDescribe(r), nil
}

// DragRef arrossega d'una ref a una altra amb passos intermedis.
func (b *Browser) DragRef(from, to axRef) (string, error) {
	x1, y1, err := b.nodePoint(from.node)
	if err != nil {
		return "", err
	}
	x2, y2, err := b.nodePoint(to.node)
	if err != nil {
		return "", err
	}
	steps := []map[string]any{
		{"type": "mouseMoved", "x": x1, "y": y1},
		{"type": "mousePressed", "x": x1, "y": y1, "button": "left", "clickCount": 1},
	}
	for i := 1; i <= 8; i++ {
		f := float64(i) / 8
		steps = append(steps, map[string]any{"type": "mouseMoved", "x": x1 + (x2-x1)*f, "y": y1 + (y2-y1)*f, "button": "left", "buttons": 1})
	}
	steps = append(steps, map[string]any{"type": "mouseReleased", "x": x2, "y": y2, "button": "left", "clickCount": 1})
	for _, s := range steps {
		if _, err := b.call("Input.dispatchMouseEvent", s); err != nil {
			return "", err
		}
	}
	return "dragged " + axDescribe(from) + " onto " + axDescribe(to), nil
}

// UploadRef posa fitxers en un <input type=file>.
func (b *Browser) UploadRef(r axRef, paths []string) (string, error) {
	if _, err := b.call("DOM.setFileInputFiles", map[string]any{"files": paths, "backendNodeId": r.node}); err != nil {
		return "", fmt.Errorf("browser: upload: %v (the target must be the file input; if the page hides it, click the upload button first and read again)", err)
	}
	_, _ = b.callOn(r.node, `function() { this.dispatchEvent(new Event('input', {bubbles: true})); this.dispatchEvent(new Event('change', {bubbles: true})); }`)
	return fmt.Sprintf("attached %d file(s) to %s", len(paths), axDescribe(r)), nil
}
