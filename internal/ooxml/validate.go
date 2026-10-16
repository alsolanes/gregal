package ooxml

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Validate fa comprovacions estructurals que eviten la majoria de
// «el Word/PowerPoint ha trobat contingut il·legible»:
//   - cada part XML està ben format;
//   - cada relació interna apunta a un part que existeix;
//   - cada part té tipus de contingut i cada Override apunta a un part;
//   - cada r:id/r:embed/r:link usat en un part existeix als seus .rels;
//   - pptx: ids de sldIdLst únics i ≥256, diapositives amb layout;
//   - docx: ids de comentaris i canvis registrats coherents.
//
// Torna la llista de problemes (buida si tot va bé), ordenada.
func Validate(pk *Package) []string {
	var issues []string
	add := func(f string, a ...any) { issues = append(issues, fmt.Sprintf(f, a...)) }

	names := pk.Names()
	exists := map[string]bool{}
	for _, n := range names {
		exists[n] = true
	}
	for _, n := range names {
		if !isXMLPart(n) {
			continue
		}
		if _, err := pk.XML(n); err != nil {
			add("%v", err)
		}
	}
	if _, err := pk.XML(ctPart); err != nil {
		sort.Strings(issues)
		return issues
	}
	for _, n := range names {
		if n == ctPart || strings.HasSuffix(n, ".rels") {
			continue
		}
		if pk.ContentType(n) == "" {
			add("%s: sense tipus de contingut a [Content_Types].xml", n)
		}
	}
	ct, _ := pk.XML(ctPart)
	for _, o := range ct.Root().Elems("Override") {
		p := strings.TrimPrefix(o.Get("PartName"), "/")
		if !exists[p] {
			add("[Content_Types].xml: Override de %s, que no existeix", p)
		}
	}
	// Relacions.
	for _, n := range names {
		if !strings.HasSuffix(n, ".rels") {
			continue
		}
		src := relsSource(n)
		if src != "" && !exists[src] {
			add("%s: és el .rels d'un part que no existeix (%s)", n, src)
			continue
		}
		seen := map[string]bool{}
		for _, r := range pk.Rels(src) {
			if seen[r.ID] {
				add("%s: Id %s repetit", n, r.ID)
			}
			seen[r.ID] = true
			if r.Mode == "External" {
				continue
			}
			t := RelTarget(src, r.Target)
			if !exists[t] {
				add("%s: %s apunta a %s, que no existeix", n, r.ID, t)
			}
		}
	}
	// r:ids usats als parts.
	for _, n := range names {
		if !isXMLPart(n) || strings.HasSuffix(n, ".rels") || n == ctPart {
			continue
		}
		doc, err := pk.XML(n)
		if err != nil {
			continue
		}
		ids := map[string]bool{}
		for _, r := range pk.Rels(n) {
			ids[r.ID] = true
		}
		doc.Walk(func(x *Node) bool {
			if x.Kind != ElemNode {
				return true
			}
			for _, a := range x.Attrs {
				if a.Name == "r:id" || a.Name == "r:embed" || a.Name == "r:link" || a.Name == "r:pict" {
					if a.Value != "" && !ids[a.Value] {
						add("%s: <%s %s=%q> sense relació", n, x.Name, a.Name, a.Value)
					}
				}
			}
			return true
		})
	}
	main := pk.MainPart()
	switch {
	case strings.HasPrefix(main, "ppt/"):
		issues = append(issues, validatePptx(pk, main, exists)...)
	case strings.HasPrefix(main, "word/"):
		issues = append(issues, validateDocx(pk, main)...)
	}
	sort.Strings(issues)
	return dedup(issues)
}

// NewIssues torna els problemes de pk que no eren ja a orig (per a
// fitxers que vénen d'una plantilla que ja tenia defectes).
func NewIssues(pk, orig *Package) []string {
	old := map[string]bool{}
	if orig != nil {
		for _, s := range Validate(orig) {
			old[s] = true
		}
	}
	var out []string
	for _, s := range Validate(pk) {
		if !old[s] {
			out = append(out, s)
		}
	}
	return out
}

func validatePptx(pk *Package, main string, exists map[string]bool) []string {
	var issues []string
	doc, err := pk.XML(main)
	if err != nil {
		return nil
	}
	pres := doc.Root()
	seen := map[int]bool{}
	for _, s := range pres.Path("p:sldIdLst").Elems("p:sldId") {
		id, err := strconv.Atoi(s.Get("id"))
		if err != nil || id < 256 || id > 2147483647 {
			issues = append(issues, fmt.Sprintf("%s: sldId id=%q fora de rang (≥256)", main, s.Get("id")))
		}
		if seen[id] {
			issues = append(issues, fmt.Sprintf("%s: sldId id=%d repetit", main, id))
		}
		seen[id] = true
	}
	// Ordre dels fills de p:presentation (PowerPoint hi és estricte).
	order := []string{"p:sldMasterIdLst", "p:notesMasterIdLst", "p:handoutMasterIdLst", "p:sldIdLst", "p:sldSz", "p:notesSz", "p:smartTags", "p:embeddedFontLst", "p:custShowLst", "p:photoAlbum", "p:custDataLst", "p:kinsoku", "p:defaultTextStyle", "p:modifyVerifier", "p:extLst"}
	last := -1
	for _, c := range pres.Elems() {
		r := indexOf(order, c.Name)
		if r < 0 {
			continue
		}
		if r < last {
			issues = append(issues, fmt.Sprintf("%s: <%s> fora d'ordre", main, c.Name))
		}
		last = r
	}
	for _, s := range pk.ByType(main, RelSlide) {
		if len(pk.ByType(s, RelSlideLayout)) == 0 {
			issues = append(issues, fmt.Sprintf("%s: diapositiva sense layout", s))
		}
	}
	return issues
}

func validateDocx(pk *Package, main string) []string {
	var issues []string
	doc, err := pk.XML(main)
	if err != nil {
		return nil
	}
	// Comentaris: cada referència ha d'existir a comments.xml.
	cids := map[string]bool{}
	for _, c := range pk.ByType(main, RelComments) {
		if cd, err := pk.XML(c); err == nil {
			for _, x := range cd.Root().Elems("w:comment") {
				cids[x.Get("w:id")] = true
			}
		}
	}
	for _, x := range doc.Find("w:commentReference") {
		if !cids[x.Get("w:id")] {
			issues = append(issues, fmt.Sprintf("%s: commentReference %s sense comentari", main, x.Get("w:id")))
		}
	}
	// Canvis registrats: ids únics.
	seen := map[string]bool{}
	for _, tag := range []string{"w:ins", "w:del"} {
		for _, x := range doc.Find(tag) {
			id := x.Get("w:id")
			if id == "" {
				issues = append(issues, fmt.Sprintf("%s: <%s> sense w:id", main, tag))
				continue
			}
			if seen[id] {
				issues = append(issues, fmt.Sprintf("%s: w:id %s repetit en canvis registrats", main, id))
			}
			seen[id] = true
		}
	}
	// w:t no pot contenir w:delText fora de w:del, i al revés.
	for _, x := range doc.Find("w:delText") {
		in := false
		for p := x.Parent; p != nil; p = p.Parent {
			if p.Name == "w:del" || p.Name == "w:moveFrom" {
				in = true
				break
			}
		}
		if !in {
			issues = append(issues, fmt.Sprintf("%s: w:delText fora d'un w:del", main))
			break
		}
	}
	return issues
}

func relsSource(rels string) string {
	if rels == "_rels/.rels" {
		return ""
	}
	dir, file := path.Split(rels)
	dir = strings.TrimSuffix(strings.TrimSuffix(dir, "/"), "_rels")
	return dir + strings.TrimSuffix(file, ".rels")
}

func isXMLPart(n string) bool {
	e := strings.ToLower(path.Ext(n))
	return e == ".xml" || e == ".rels" // .vml sovint no és XML ben format
}

func dedup(xs []string) []string {
	var out []string
	for i, s := range xs {
		if i == 0 || s != xs[i-1] {
			out = append(out, s)
		}
	}
	return out
}
