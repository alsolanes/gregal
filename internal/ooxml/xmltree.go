// Package ooxml manipula paquets Office Open XML (.docx/.pptx/.xlsx i
// les seves plantilles) sense dependències externes.
//
// Regla d'or: un part es llegeix i s'escriu amb l'arbre d'aquest fitxer,
// que conserva els prefixos de namespace tal com venen (w:, a:, p:,
// mc:Ignorable…). encoding/xml.Encoder NO serveix per tornar a escriure
// OOXML: converteix «w:p» en <p xmlns="w"> i «xml:space» en
// «_xml:space», i el Word ja no obre el document.
package ooxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Kind diu quin tipus de node és.
type Kind uint8

const (
	ElemNode Kind = iota
	TextNode
	ProcInstNode
	CommentNode
	DirectiveNode
)

// Attr és un atribut amb el nom qualificat tal com era a l'original
// («w:val», «xml:space», «xmlns:w», «r»).
type Attr struct {
	Name  string
	Value string
}

// Node és un element, text o instrucció d'un part XML. Name és el nom
// qualificat amb prefix («w:p»). Text val per a TextNode, ProcInst
// (target + " " + contingut), Comment i Directive.
type Node struct {
	Kind     Kind
	Name     string
	Attrs    []Attr
	Children []*Node
	Text     string
	Parent   *Node
}

// Parse llegeix un part XML i en torna el node document (Kind Elem amb
// Name buit) que conté la declaració, comentaris i l'element arrel.
func Parse(data []byte) (*Node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	root := &Node{Kind: ElemNode}
	cur := root
	for {
		tok, err := dec.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xml invàlid: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &Node{Kind: ElemNode, Name: qname(t.Name), Parent: cur}
			for _, a := range t.Attr {
				n.Attrs = append(n.Attrs, Attr{Name: qname(a.Name), Value: a.Value})
			}
			cur.Children = append(cur.Children, n)
			cur = n
		case xml.EndElement:
			if cur.Parent == nil || qname(t.Name) != cur.Name {
				return nil, fmt.Errorf("xml invàlid: tancament </%s> inesperat", qname(t.Name))
			}
			cur = cur.Parent
		case xml.CharData:
			cur.Children = append(cur.Children, &Node{Kind: TextNode, Text: string(t), Parent: cur})
		case xml.ProcInst:
			cur.Children = append(cur.Children, &Node{Kind: ProcInstNode, Name: t.Target, Text: string(t.Inst), Parent: cur})
		case xml.Comment:
			cur.Children = append(cur.Children, &Node{Kind: CommentNode, Text: string(t), Parent: cur})
		case xml.Directive:
			cur.Children = append(cur.Children, &Node{Kind: DirectiveNode, Text: string(t), Parent: cur})
		}
	}
	if cur != root {
		return nil, fmt.Errorf("xml invàlid: element <%s> sense tancar", cur.Name)
	}
	if root.Root() == nil {
		return nil, fmt.Errorf("xml invàlid: sense element arrel")
	}
	return root, nil
}

// MustParse és Parse per a XML intern conegut (plantilles pròpies).
func MustParse(s string) *Node {
	n, err := Parse([]byte(s))
	if err != nil {
		panic(err)
	}
	return n
}

// ParseFragment llegeix un o més elements sense arrel comuna (per
// exemple «<w:p>…</w:p><w:p>…</w:p>») i els torna desenganxats.
func ParseFragment(s string) ([]*Node, error) {
	doc, err := Parse([]byte("<frag>" + s + "</frag>"))
	if err != nil {
		return nil, err
	}
	kids := doc.Root().Children
	for _, k := range kids {
		k.Parent = nil
	}
	return kids, nil
}

func qname(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	return n.Space + ":" + n.Local
}

// Root torna l'element arrel d'un node document.
func (n *Node) Root() *Node {
	for _, c := range n.Children {
		if c.Kind == ElemNode {
			return c
		}
	}
	return nil
}

// Bytes serialitza el node (si és el document, amb la declaració).
func (n *Node) Bytes() []byte {
	var b bytes.Buffer
	if n.Kind == ElemNode && n.Name == "" {
		for _, c := range n.Children {
			write(&b, c)
		}
		return b.Bytes()
	}
	write(&b, n)
	return b.Bytes()
}

// String és Bytes com a cadena.
func (n *Node) String() string { return string(n.Bytes()) }

func write(b *bytes.Buffer, n *Node) {
	switch n.Kind {
	case TextNode:
		escText(b, n.Text)
	case ProcInstNode:
		b.WriteString("<?")
		b.WriteString(n.Name)
		if n.Text != "" {
			b.WriteByte(' ')
			b.WriteString(n.Text)
		}
		b.WriteString("?>")
	case CommentNode:
		b.WriteString("<!--")
		b.WriteString(n.Text)
		b.WriteString("-->")
	case DirectiveNode:
		b.WriteString("<!")
		b.WriteString(n.Text)
		b.WriteString(">")
	case ElemNode:
		b.WriteByte('<')
		b.WriteString(n.Name)
		for _, a := range n.Attrs {
			b.WriteByte(' ')
			b.WriteString(a.Name)
			b.WriteString(`="`)
			escAttr(b, a.Value)
			b.WriteByte('"')
		}
		if len(n.Children) == 0 {
			b.WriteString("/>")
			return
		}
		b.WriteByte('>')
		for _, c := range n.Children {
			write(b, c)
		}
		b.WriteString("</")
		b.WriteString(n.Name)
		b.WriteByte('>')
	}
}

func escText(b *bytes.Buffer, s string) {
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		default:
			if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
				continue // caràcters de control: XML 1.0 no els admet
			}
			b.WriteRune(r)
		}
	}
}

func escAttr(b *bytes.Buffer, s string) {
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '"':
			b.WriteString("&quot;")
		case '\n':
			b.WriteString("&#xA;")
		case '\r':
			b.WriteString("&#xD;")
		case '\t':
			b.WriteString("&#x9;")
		default:
			if r < 0x20 {
				continue
			}
			b.WriteRune(r)
		}
	}
}

// Esc escapa text per incrustar-lo en XML construït a mà.
func Esc(s string) string {
	var b bytes.Buffer
	escAttr(&b, s)
	return b.String()
}

// ---------- navegació ----------

// Local torna el nom sense prefix.
func (n *Node) Local() string {
	if i := strings.IndexByte(n.Name, ':'); i >= 0 {
		return n.Name[i+1:]
	}
	return n.Name
}

// Is diu si el node és l'element amb aquest nom qualificat.
func (n *Node) Is(name string) bool { return n != nil && n.Kind == ElemNode && n.Name == name }

// Get torna el valor d'un atribut (nom qualificat) o "".
func (n *Node) Get(name string) string {
	for _, a := range n.Attrs {
		if a.Name == name {
			return a.Value
		}
	}
	return ""
}

// Has diu si l'atribut existeix.
func (n *Node) Has(name string) bool {
	for _, a := range n.Attrs {
		if a.Name == name {
			return true
		}
	}
	return false
}

// Set posa (o afegeix) un atribut.
func (n *Node) Set(name, val string) {
	for i, a := range n.Attrs {
		if a.Name == name {
			n.Attrs[i].Value = val
			return
		}
	}
	n.Attrs = append(n.Attrs, Attr{name, val})
}

// Del treu un atribut.
func (n *Node) Del(name string) {
	out := n.Attrs[:0]
	for _, a := range n.Attrs {
		if a.Name != name {
			out = append(out, a)
		}
	}
	n.Attrs = out
}

// Child torna el primer fill element amb aquest nom.
func (n *Node) Child(name string) *Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Kind == ElemNode && c.Name == name {
			return c
		}
	}
	return nil
}

// Elems torna els fills element (amb nom, si se'n dona).
func (n *Node) Elems(name ...string) []*Node {
	var out []*Node
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Kind != ElemNode {
			continue
		}
		if len(name) == 0 || c.Name == name[0] {
			out = append(out, c)
		}
	}
	return out
}

// Path segueix una cadena de fills («p:cSld», «p:spTree»…).
func (n *Node) Path(names ...string) *Node {
	cur := n
	for _, nm := range names {
		cur = cur.Child(nm)
		if cur == nil {
			return nil
		}
	}
	return cur
}

// Find torna tots els descendents (en ordre de document) amb el nom.
func (n *Node) Find(name string) []*Node {
	var out []*Node
	n.Walk(func(x *Node) bool {
		if x != n && x.Kind == ElemNode && x.Name == name {
			out = append(out, x)
		}
		return true
	})
	return out
}

// First torna el primer descendent amb el nom.
func (n *Node) First(name string) *Node {
	var hit *Node
	n.Walk(func(x *Node) bool {
		if hit != nil {
			return false
		}
		if x != n && x.Kind == ElemNode && x.Name == name {
			hit = x
			return false
		}
		return true
	})
	return hit
}

// Walk recorre en preordre; si f torna false, no baixa als fills.
func (n *Node) Walk(f func(*Node) bool) {
	if !f(n) {
		return
	}
	for _, c := range append([]*Node(nil), n.Children...) {
		c.Walk(f)
	}
}

// InnerText concatena tot el text descendent.
func (n *Node) InnerText() string {
	var b strings.Builder
	n.Walk(func(x *Node) bool {
		if x.Kind == TextNode {
			b.WriteString(x.Text)
		}
		return true
	})
	return b.String()
}

// SetText substitueix els fills per un sol node de text.
func (n *Node) SetText(s string) {
	n.Children = []*Node{{Kind: TextNode, Text: s, Parent: n}}
	if s == "" {
		n.Children = nil
	}
}

// ---------- mutació ----------

// Index torna la posició del fill dins el pare (o -1).
func (n *Node) Index() int {
	if n.Parent == nil {
		return -1
	}
	for i, c := range n.Parent.Children {
		if c == n {
			return i
		}
	}
	return -1
}

// Append afegeix fills al final.
func (n *Node) Append(kids ...*Node) {
	for _, k := range kids {
		k.Parent = n
	}
	n.Children = append(n.Children, kids...)
}

// Prepend afegeix fills al principi.
func (n *Node) Prepend(kids ...*Node) { n.InsertAt(0, kids...) }

// InsertAt insereix fills a la posició i.
func (n *Node) InsertAt(i int, kids ...*Node) {
	for _, k := range kids {
		k.Parent = n
	}
	if i < 0 {
		i = 0
	}
	if i > len(n.Children) {
		i = len(n.Children)
	}
	nc := make([]*Node, 0, len(n.Children)+len(kids))
	nc = append(nc, n.Children[:i]...)
	nc = append(nc, kids...)
	nc = append(nc, n.Children[i:]...)
	n.Children = nc
}

// InsertBefore posa els nodes just abans de n (germans).
func (n *Node) InsertBefore(kids ...*Node) {
	if i := n.Index(); i >= 0 {
		n.Parent.InsertAt(i, kids...)
	}
}

// InsertAfter posa els nodes just després de n (germans).
func (n *Node) InsertAfter(kids ...*Node) {
	if i := n.Index(); i >= 0 {
		n.Parent.InsertAt(i+1, kids...)
	}
}

// Remove treu el node del pare.
func (n *Node) Remove() {
	if i := n.Index(); i >= 0 {
		p := n.Parent
		p.Children = append(p.Children[:i], p.Children[i+1:]...)
		n.Parent = nil
	}
}

// Replace posa els nodes donats al lloc de n.
func (n *Node) Replace(kids ...*Node) {
	if i := n.Index(); i >= 0 {
		p := n.Parent
		n.Remove()
		p.InsertAt(i, kids...)
	}
}

// Clone fa una còpia profunda (desenganxada).
func (n *Node) Clone() *Node {
	c := &Node{Kind: n.Kind, Name: n.Name, Text: n.Text}
	if len(n.Attrs) > 0 {
		c.Attrs = append([]Attr(nil), n.Attrs...)
	}
	for _, k := range n.Children {
		kc := k.Clone()
		kc.Parent = c
		c.Children = append(c.Children, kc)
	}
	return c
}

// El crea un element amb atributs en parells nom, valor.
func El(name string, attrs ...string) *Node {
	n := &Node{Kind: ElemNode, Name: name}
	for i := 0; i+1 < len(attrs); i += 2 {
		n.Attrs = append(n.Attrs, Attr{attrs[i], attrs[i+1]})
	}
	return n
}

// Txt crea un node de text.
func Txt(s string) *Node { return &Node{Kind: TextNode, Text: s} }

// Add afegeix fills i torna el mateix node (per encadenar).
func (n *Node) Add(kids ...*Node) *Node {
	n.Append(kids...)
	return n
}

// EnsureChild torna el fill amb el nom o el crea. order dona l'ordre
// d'esquema dels germans: el nou es col·loca abans del primer germà que
// hi aparegui més tard (OOXML és estricte amb l'ordre dels fills).
func (n *Node) EnsureChild(name string, order []string) *Node {
	if c := n.Child(name); c != nil {
		return c
	}
	c := El(name)
	pos := len(n.Children)
	if rank := indexOf(order, name); rank >= 0 {
		for i, k := range n.Children {
			if k.Kind == ElemNode && indexOf(order, k.Name) > rank {
				pos = i
				break
			}
		}
	}
	n.InsertAt(pos, c)
	return c
}

func indexOf(xs []string, s string) int {
	for i, x := range xs {
		if x == s {
			return i
		}
	}
	return -1
}
