package ooxml

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// DeckSpec descriu una presentació sencera.
type DeckSpec struct {
	Title  string      `json:"title"`
	Theme  string      `json:"theme"`
	Slides []SlideSpec `json:"slides"`
}

// SlideSpec és una diapositiva. Layout: title, section, content,
// two_columns, comparison, big_number, image, title_only, blank,
// closing (o el nom/índex d'un layout de la plantilla).
type SlideSpec struct {
	Layout     string     `json:"layout"`
	Title      string     `json:"title"`
	Subtitle   string     `json:"subtitle"`
	Body       string     `json:"body"`
	Bullets    []string   `json:"bullets"`
	Left       string     `json:"left"`
	Right      string     `json:"right"`
	LeftTitle  string     `json:"left_title"`
	RightTitle string     `json:"right_title"`
	Number     string     `json:"number"`
	Caption    string     `json:"caption"`
	Image      string     `json:"image"`
	Table      [][]string `json:"table"`
	Chart      *ChartSpec `json:"chart"`
	Notes      string     `json:"notes"`
}

// ChartSpec és un gràfic natiu (editable a PowerPoint).
type ChartSpec struct {
	Type       string        `json:"type"` // bar, column, line, pie, doughnut
	Title      string        `json:"title"`
	Categories []string      `json:"categories"`
	Series     []ChartSeries `json:"series"`
	NumFormat  string        `json:"num_format"`
}

// ChartSeries és una sèrie de dades.
type ChartSeries struct {
	Name   string    `json:"name"`
	Values []float64 `json:"values"`
}

// ParseDeckSpec llegeix l'especificació JSON (o una llista de
// diapositives sola).
func ParseDeckSpec(s string) (DeckSpec, error) {
	var d DeckSpec
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") {
		err := json.Unmarshal([]byte(s), &d.Slides)
		return d, err
	}
	err := json.Unmarshal([]byte(s), &d)
	return d, err
}

// DeckFromMarkdown converteix markdown en diapositives: cada «# » o
// «## » obre una diapositiva; la primera amb només títol (i un
// paràgraf) es fa portada.
func DeckFromMarkdown(title, md string) DeckSpec {
	var d DeckSpec
	d.Title = title
	var cur *SlideSpec
	var body []string
	flush := func() {
		if cur != nil {
			cur.Body = strings.TrimSpace(strings.Join(body, "\n"))
			d.Slides = append(d.Slides, *cur)
		}
		body = nil
	}
	for _, l := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "# ") || strings.HasPrefix(t, "## ") {
			flush()
			cur = &SlideSpec{Layout: "content", Title: strings.TrimSpace(strings.TrimLeft(t, "# "))}
			continue
		}
		if cur == nil {
			cur = &SlideSpec{Layout: "content", Title: title}
		}
		body = append(body, l)
	}
	flush()
	if title != "" && (len(d.Slides) == 0 || d.Slides[0].Title != title) {
		d.Slides = append([]SlideSpec{{Layout: "title", Title: title}}, d.Slides...)
	} else if len(d.Slides) > 0 {
		d.Slides[0].Layout = "title"
		d.Slides[0].Subtitle = d.Slides[0].Body
		d.Slides[0].Body = ""
	}
	return d
}

// BuildSlides afegeix les diapositives de l'especificació. Torna avisos
// (layouts que no s'han trobat, imatges que falten…).
func (p *Pptx) BuildSlides(spec DeckSpec) ([]string, error) {
	var warns []string
	for i, s := range spec.Slides {
		w, err := p.buildSlide(s)
		if err != nil {
			return warns, fmt.Errorf("diapositiva %d: %w", i+1, err)
		}
		for _, x := range w {
			warns = append(warns, fmt.Sprintf("diapositiva %d: %s", i+1, x))
		}
	}
	return warns, nil
}

func (s SlideSpec) bodyMD() string {
	if len(s.Bullets) > 0 {
		var b strings.Builder
		for _, x := range s.Bullets {
			t := strings.TrimRight(x, " ")
			ind := len(t) - len(strings.TrimLeft(t, " "))
			t = strings.TrimSpace(t)
			t = strings.TrimPrefix(strings.TrimPrefix(t, "- "), "• ")
			b.WriteString(strings.Repeat("  ", ind/2) + "- " + t + "\n")
		}
		if s.Body != "" {
			return s.Body + "\n\n" + b.String()
		}
		return b.String()
	}
	return s.Body
}

func (p *Pptx) buildSlide(s SlideSpec) ([]string, error) {
	var warns []string
	kind := strings.ToLower(strings.TrimSpace(s.Layout))
	if kind == "" {
		switch {
		case s.Number != "":
			kind = "big_number"
		case s.Left != "" || s.Right != "":
			kind = "two_columns"
		case s.Image != "" && (s.Body != "" || len(s.Bullets) > 0):
			kind = "image"
		default:
			kind = "content"
		}
	}
	l, err := p.PickLayout(kind)
	if err != nil {
		return nil, err
	}
	slide := p.AddSlide(l, -1)
	phs := map[string][]PhInfo{}
	for _, ph := range l.Phs {
		key := ph.Type
		switch key {
		case "ctrTitle":
			key = "title"
		case "obj", "subTitle":
			key = "body"
		}
		phs[key] = append(phs[key], ph)
	}
	setPh := func(ph PhInfo, md string, o TextOpts) error {
		sp, err := p.PlaceholderShape(slide, ph)
		if err != nil {
			return err
		}
		SetText(sp, md, o)
		return nil
	}
	if s.Title != "" {
		if t := phs["title"]; len(t) > 0 {
			if err := setPh(t[0], s.Title, TextOpts{NoBullet: true}); err != nil {
				return nil, err
			}
		} else if kind != "blank" {
			p.AddTextBox(slide, Box{mX, titleY, contW, titleH}, s.Title, TextOpts{Size: 3200, Bold: true, Color: p.Theme.Dark})
		}
	}
	bodies := phs["body"]
	// Contingut principal: on va (rectangle) i què hi ha.
	mainBox := Box{mX, bodyY, contW, bodyH}
	if len(bodies) > 0 && bodies[0].Box.CX > 0 {
		mainBox = bodies[0].Box
	}
	visual := s.Table != nil || s.Chart != nil || (s.Image != "")
	switch kind {
	case "title", "closing", "section":
		sub := firstNonEmpty(s.Subtitle, s.bodyMD())
		if sub != "" {
			if len(bodies) > 0 {
				setPh(bodies[0], sub, TextOpts{NoBullet: true})
			} else {
				p.AddTextBox(slide, Box{mX, SlideH / 2, contW, 1000000}, sub, TextOpts{Size: 2000})
			}
		}
	case "big_number":
		cap := firstNonEmpty(s.Caption, s.Subtitle, s.bodyMD())
		if len(bodies) >= 2 && l.Name == "Big Number" {
			setPh(bodies[0], s.Number, TextOpts{NoBullet: true})
			if cap != "" {
				setPh(bodies[1], cap, TextOpts{NoBullet: true})
			}
		} else {
			// Plantilla sense layout de xifra: quadres de text propis.
			p.AddTextBox(slide, Box{mX, 2000000, contW, 1900000}, s.Number, TextOpts{Size: 8800, Bold: true, Color: p.Theme.Accent, Align: "ctr"})
			if cap != "" {
				p.AddTextBox(slide, Box{mX + 914400, 4000000, contW - 1828800, 1200000}, cap, TextOpts{Size: 2000, Color: p.Theme.Muted, Align: "ctr"})
			}
		}
	case "two_columns", "comparison":
		left := firstNonEmpty(s.Left, s.bodyMD())
		right := s.Right
		var lph, rph []PhInfo
		if kind == "comparison" && len(bodies) >= 4 {
			lph, rph = []PhInfo{bodies[0], bodies[1]}, []PhInfo{bodies[2], bodies[3]}
		} else if len(bodies) >= 2 {
			lph, rph = []PhInfo{bodies[0]}, []PhInfo{bodies[1]}
		}
		fill := func(phl []PhInfo, head, md string, fallback Box) Box {
			if len(phl) == 2 {
				if head != "" {
					setPh(phl[0], head, TextOpts{NoBullet: true})
				}
				if md != "" {
					setPh(phl[1], md, TextOpts{})
				}
				return phl[1].Box
			}
			if head != "" {
				md = "**" + head + "**\n\n" + md
			}
			if len(phl) == 1 {
				if md != "" {
					setPh(phl[0], md, TextOpts{})
				}
				return phl[0].Box
			}
			if md != "" {
				p.AddTextBox(slide, fallback, md, TextOpts{Size: 1800})
			}
			return fallback
		}
		fill(lph, s.LeftTitle, left, Box{mX, bodyY, halfW, bodyH})
		if visual && right == "" {
			rb := Box{mX + halfW + gap, bodyY, halfW, bodyH}
			if len(rph) > 0 && rph[len(rph)-1].Box.CX > 0 {
				rb = rph[len(rph)-1].Box
			}
			if s.RightTitle != "" && len(rph) == 2 {
				setPh(rph[0], s.RightTitle, TextOpts{NoBullet: true})
			}
			if w, err := p.addVisual(slide, s, rb); err != nil {
				return nil, err
			} else if w != "" {
				warns = append(warns, w)
			}
		} else {
			fill(rph, s.RightTitle, right, Box{mX + halfW + gap, bodyY, halfW, bodyH})
		}
	case "image":
		var picBox, txtBox Box
		for _, ph := range l.Phs {
			if ph.Type == "pic" {
				picBox = ph.Box
			}
		}
		if picBox.CX == 0 {
			picBox = Box{mX, bodyY, contW * 58 / 100, bodyH}
		}
		if len(bodies) > 0 {
			txtBox = bodies[0].Box
			if md := s.bodyMD(); md != "" {
				setPh(bodies[0], md, TextOpts{})
			}
		} else if md := s.bodyMD(); md != "" {
			txtBox = Box{picBox.X + picBox.CX + gap, bodyY, SlideW - mX - (picBox.X + picBox.CX + gap), bodyH}
			p.AddTextBox(slide, txtBox, md, TextOpts{Size: 1800})
		}
		if s.Image != "" {
			if err := p.AddPicture(slide, s.Image, picBox); err != nil {
				warns = append(warns, "imatge: "+err.Error())
			}
		}
		if s.Table != nil || s.Chart != nil {
			s2 := s
			s2.Image = ""
			if w, err := p.addVisual(slide, s2, picBox); err == nil && w != "" {
				warns = append(warns, w)
			}
		}
	case "blank", "title_only":
		md := s.bodyMD()
		box := Box{mX, bodyY, contW, bodyH}
		if kind == "blank" {
			box = Box{mX, mX, contW, SlideH - 2*mX}
		}
		if visual {
			if md != "" {
				p.AddTextBox(slide, Box{box.X, box.Y, box.CX, 700000}, md, TextOpts{Size: 1800})
				box.Y += 800000
				box.CY -= 800000
			}
			w, err := p.addVisual(slide, s, box)
			if err != nil {
				return nil, err
			}
			if w != "" {
				warns = append(warns, w)
			}
		} else if md != "" {
			p.AddTextBox(slide, box, md, TextOpts{Size: 2000})
		}
	default: // content
		md := s.bodyMD()
		if visual {
			box := mainBox
			if md != "" {
				// Text curt a dalt, visual a sota.
				if len(bodies) > 0 {
					sp, _ := p.PlaceholderShape(slide, bodies[0])
					SetText(sp, md, TextOpts{})
					setGeom(sp, Box{box.X, box.Y, box.CX, 900000})
				}
				box.Y += 1000000
				box.CY -= 1000000
			}
			w, err := p.addVisual(slide, s, box)
			if err != nil {
				return nil, err
			}
			if w != "" {
				warns = append(warns, w)
			}
		} else if md != "" {
			if len(bodies) > 0 {
				setPh(bodies[0], md, TextOpts{})
			} else {
				p.AddTextBox(slide, mainBox, md, TextOpts{Size: 2000})
			}
		}
	}
	if s.Notes != "" {
		if err := p.SetNotes(slide, s.Notes); err != nil {
			warns = append(warns, err.Error())
		}
	}
	return warns, nil
}

// setGeom fixa la posició/mida d'una forma (sobreescrivint l'herència).
func setGeom(sp *Node, b Box) {
	pr := sp.Child("p:spPr")
	if pr == nil {
		pr = El("p:spPr")
		sp.InsertAt(1, pr)
	}
	if x := pr.Child("a:xfrm"); x != nil {
		x.Remove()
	}
	pr.Prepend(El("a:xfrm").Add(
		El("a:off", "x", strconv.Itoa(b.X), "y", strconv.Itoa(b.Y)),
		El("a:ext", "cx", strconv.Itoa(b.CX), "cy", strconv.Itoa(b.CY))))
}

func (p *Pptx) addVisual(slide string, s SlideSpec, b Box) (string, error) {
	switch {
	case s.Table != nil:
		return "", p.AddTable(slide, s.Table, b)
	case s.Chart != nil:
		return "", p.AddChart(slide, *s.Chart, b)
	case s.Image != "":
		if err := p.AddPicture(slide, s.Image, b); err != nil {
			return "imatge: " + err.Error(), nil
		}
	}
	return "", nil
}

// ---------- gràfics ----------

// AddChart afegeix un gràfic natiu (barres, columnes, línies, pastís).
func (p *Pptx) AddChart(slide string, c ChartSpec, b Box) error {
	if len(c.Series) == 0 || len(c.Categories) == 0 {
		return fmt.Errorf("el gràfic necessita categories i almenys una sèrie")
	}
	for _, s := range c.Series {
		if len(s.Values) != len(c.Categories) {
			return fmt.Errorf("la sèrie %q té %d valors i hi ha %d categories", s.Name, len(s.Values), len(c.Categories))
		}
	}
	part := p.Pkg.NextPartName("ppt/charts", "chart", ".xml")
	p.Pkg.Put(part, []byte(chartXML(c, p.Theme)))
	p.Pkg.SetOverride(part, ctChart)
	rid := p.Pkg.AddRel(slide, RelChart, part)
	tree, err := p.slideTree(slide)
	if err != nil {
		return err
	}
	id := p.nextShapeID(tree)
	frag, err := ParseFragment(`<p:graphicFrame xmlns:p="` + nsP + `" xmlns:a="` + nsA + `" xmlns:r="` + nsR + `">` +
		fmt.Sprintf(`<p:nvGraphicFramePr><p:cNvPr id="%d" name="Chart %d"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm>`, id, id, b.X, b.Y, b.CX, b.CY) +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="` + rid + `"/></a:graphicData></a:graphic></p:graphicFrame>`)
	if err != nil {
		return err
	}
	gf := frag[0]
	for _, a := range []string{"xmlns:p", "xmlns:a", "xmlns:r"} {
		gf.Del(a)
	}
	tree.Append(gf)
	return nil
}

func chartXML(c ChartSpec, t Theme) string {
	palette := []string{t.Primary, t.Accent, t.Accent2, t.Muted, t.Dark, "A5A5A5"}
	typ := strings.ToLower(c.Type)
	ncat := len(c.Categories)
	labels := ncat*len(c.Series) <= 12 // més punts: les etiquetes se solapen
	colL := func(i int) string { return colName(i) }
	catRef := func() string {
		var b strings.Builder
		b.WriteString(fmt.Sprintf(`<c:cat><c:strRef><c:f>Sheet1!$A$2:$A$%d</c:f><c:strCache><c:ptCount val="%d"/>`, ncat+1, ncat))
		for i, cat := range c.Categories {
			b.WriteString(fmt.Sprintf(`<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, Esc(cat)))
		}
		b.WriteString(`</c:strCache></c:strRef></c:cat>`)
		return b.String()
	}
	numFmt := c.NumFormat
	if numFmt == "" {
		numFmt = "General"
	}
	series := func(i int, s ChartSeries, pie bool) string {
		col := colL(i + 1)
		var b strings.Builder
		b.WriteString(fmt.Sprintf(`<c:ser><c:idx val="%d"/><c:order val="%d"/><c:tx><c:strRef><c:f>Sheet1!$%s$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>%s</c:v></c:pt></c:strCache></c:strRef></c:tx>`, i, i, col, Esc(s.Name)))
		clr := palette[i%len(palette)]
		switch {
		case pie:
			b.WriteString(`<c:spPr><a:ln w="12700"><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></a:ln></c:spPr>`)
			for j := range c.Categories {
				b.WriteString(fmt.Sprintf(`<c:dPt><c:idx val="%d"/><c:bubble3D val="0"/><c:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:ln w="12700"><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></a:ln></c:spPr></c:dPt>`, j, palette[j%len(palette)]))
			}
		case typ == "line":
			b.WriteString(fmt.Sprintf(`<c:spPr><a:ln w="31750" cap="rnd"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:round/></a:ln></c:spPr><c:marker><c:symbol val="circle"/><c:size val="6"/><c:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:ln><a:noFill/></a:ln></c:spPr></c:marker>`, clr, clr))
		default:
			b.WriteString(fmt.Sprintf(`<c:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:ln><a:noFill/></a:ln></c:spPr><c:invertIfNegative val="0"/>`, clr))
		}
		pos := "outEnd"
		if pie {
			pos = "bestFit"
		} else if typ == "line" {
			pos = "t"
		}
		b.WriteString(fmt.Sprintf(`<c:dLbls><c:numFmt formatCode="%s" sourceLinked="0"/><c:spPr><a:noFill/><a:ln><a:noFill/></a:ln></c:spPr><c:txPr><a:bodyPr/><a:lstStyle/><a:p><a:pPr><a:defRPr sz="1400" b="1"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:defRPr></a:pPr><a:endParaRPr lang="ca-ES"/></a:p></c:txPr><c:dLblPos val="%s"/><c:showLegendKey val="0"/><c:showVal val="%d"/><c:showCatName val="0"/><c:showSerName val="0"/><c:showPercent val="%d"/><c:showBubbleSize val="0"/></c:dLbls>`,
			Esc(numFmt), map[bool]string{true: "FFFFFF", false: t.Text}[pie], pos, b2i(!pie && labels), b2i(pie && ncat <= 6)))
		b.WriteString(catRef())
		b.WriteString(fmt.Sprintf(`<c:val><c:numRef><c:f>Sheet1!$%s$2:$%s$%d</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="%d"/>`, col, col, ncat+1, ncat))
		for j, v := range s.Values {
			b.WriteString(fmt.Sprintf(`<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, j, strconv.FormatFloat(v, 'f', -1, 64)))
		}
		b.WriteString(`</c:numCache></c:numRef></c:val>`)
		if typ == "line" {
			b.WriteString(`<c:smooth val="0"/>`)
		}
		b.WriteString(`</c:ser>`)
		return b.String()
	}
	txt := func(sz int, color string) string {
		return fmt.Sprintf(`<c:txPr><a:bodyPr/><a:lstStyle/><a:p><a:pPr><a:defRPr sz="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill></a:defRPr></a:pPr><a:endParaRPr lang="ca-ES"/></a:p></c:txPr>`, sz, color)
	}
	axes := func() string {
		return `<c:catAx><c:axId val="111"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="` + map[bool]string{true: "l", false: "b"}[typ == "bar"] + `"/><c:numFmt formatCode="General" sourceLinked="0"/><c:majorTickMark val="none"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:spPr><a:ln w="9525"><a:solidFill><a:srgbClr val="BFBFBF"/></a:solidFill></a:ln></c:spPr>` + txt(1400, t.Muted) + `<c:crossAx val="222"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/><c:noMultiLvlLbl val="0"/></c:catAx>` +
			`<c:valAx><c:axId val="222"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="` + map[bool]string{true: "b", false: "l"}[typ == "bar"] + `"/><c:majorGridlines><c:spPr><a:ln w="6350"><a:solidFill><a:srgbClr val="E5E5E5"/></a:solidFill></a:ln></c:spPr></c:majorGridlines><c:numFmt formatCode="` + Esc(numFmt) + `" sourceLinked="0"/><c:majorTickMark val="none"/><c:minorTickMark val="none"/><c:tickLblPos val="nextTo"/><c:spPr><a:ln><a:noFill/></a:ln></c:spPr>` + txt(1300, t.Muted) + `<c:crossAx val="111"/><c:crosses val="autoZero"/><c:crossBetween val="between"/></c:valAx>`
	}
	var plot strings.Builder
	pie := typ == "pie" || typ == "doughnut"
	switch typ {
	case "line":
		plot.WriteString(`<c:lineChart><c:grouping val="standard"/><c:varyColors val="0"/>`)
		for i, s := range c.Series {
			plot.WriteString(series(i, s, false))
		}
		plot.WriteString(`<c:marker val="1"/><c:axId val="111"/><c:axId val="222"/></c:lineChart>` + axes())
	case "pie":
		plot.WriteString(`<c:pieChart><c:varyColors val="1"/>` + series(0, c.Series[0], true) + `<c:firstSliceAng val="0"/></c:pieChart>`)
	case "doughnut":
		plot.WriteString(`<c:doughnutChart><c:varyColors val="1"/>` + series(0, c.Series[0], true) + `<c:firstSliceAng val="0"/><c:holeSize val="55"/></c:doughnutChart>`)
	default:
		dir := "col"
		if typ == "bar" {
			dir = "bar"
		}
		plot.WriteString(`<c:barChart><c:barDir val="` + dir + `"/><c:grouping val="clustered"/><c:varyColors val="0"/>`)
		for i, s := range c.Series {
			plot.WriteString(series(i, s, false))
		}
		plot.WriteString(`<c:gapWidth val="80"/><c:overlap val="-10"/><c:axId val="111"/><c:axId val="222"/></c:barChart>` + axes())
	}
	title := `<c:autoTitleDeleted val="1"/>`
	if c.Title != "" {
		title = `<c:title><c:tx><c:rich><a:bodyPr/><a:lstStyle/><a:p><a:pPr><a:defRPr sz="1600" b="1"><a:solidFill><a:srgbClr val="` + t.Dark + `"/></a:solidFill></a:defRPr></a:pPr><a:r><a:rPr lang="ca-ES" sz="1600" b="1"><a:solidFill><a:srgbClr val="` + t.Dark + `"/></a:solidFill></a:rPr><a:t>` + Esc(c.Title) + `</a:t></a:r></a:p></c:rich></c:tx><c:overlay val="0"/></c:title><c:autoTitleDeleted val="0"/>`
	}
	legend := ""
	if pie || len(c.Series) > 1 {
		legend = `<c:legend><c:legendPos val="b"/><c:overlay val="0"/>` + txt(1400, t.Text) + `</c:legend>`
	}
	return xmlHead + `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="` + nsA + `" xmlns:r="` + nsR + `"><c:roundedCorners val="0"/><c:chart>` + title +
		`<c:plotArea><c:layout/>` + plot.String() + `<c:spPr><a:noFill/><a:ln><a:noFill/></a:ln></c:spPr></c:plotArea>` + legend + `<c:plotVisOnly val="1"/><c:dispBlanksAs val="gap"/></c:chart>` +
		`<c:spPr><a:noFill/><a:ln><a:noFill/></a:ln></c:spPr><c:txPr><a:bodyPr/><a:lstStyle/><a:p><a:pPr><a:defRPr><a:latin typeface="` + t.BodyFont + `"/></a:defRPr></a:pPr><a:endParaRPr lang="ca-ES"/></a:p></c:txPr></c:chartSpace>`
}

// colName torna la lletra de columna (1 → A).
func colName(i int) string {
	s := ""
	for i > 0 {
		i--
		s = string(rune('A'+i%26)) + s
		i /= 26
	}
	return s
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
