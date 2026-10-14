package repomap

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func troba(t *testing.T, syms []Symbol, disp, kind string) Symbol {
	t.Helper()
	for _, s := range syms {
		if s.Display() == disp && s.Kind == kind {
			return s
		}
	}
	t.Fatalf("no hi ha %s (%s) a %+v", disp, kind, syms)
	return Symbol{}
}

func TestExtractGo(t *testing.T) {
	src := `package x

// Policy decideix.
type Policy struct{ A int }

type Doer interface{ Do() }

type Mode string

const (
	MaxX = 3
	minY = 4
)

var Global, _ = 1, 2

func New(a int, b string) (*Policy, error) {
	return nil, nil
}

func (p *Policy) Decide(mode,
	name string) string { return "" }

func (l List[T]) Len() int { return 0 }
`
	syms := extractGo("x.go", []byte(src))
	s := troba(t, syms, "Policy", "struct")
	if s.Line != 4 || !s.Exported || s.Sig != "type Policy struct" {
		t.Fatalf("Policy: %+v", s)
	}
	troba(t, syms, "Doer", "interface")
	if m := troba(t, syms, "Mode", "type"); m.Sig != "type Mode string" {
		t.Fatalf("Mode: %+v", m)
	}
	troba(t, syms, "MaxX", "const")
	if c := troba(t, syms, "minY", "const"); c.Exported {
		t.Fatal("minY no és exportada")
	}
	troba(t, syms, "Global", "var")
	if f := troba(t, syms, "New", "func"); f.Line != 17 || f.Sig != "func New(a int, b string) (*Policy, error)" {
		t.Fatalf("New: %+v", f)
	}
	d := troba(t, syms, "Policy.Decide", "method")
	if d.Recv != "Policy" || d.Line != 21 || d.Sig != "func (p *Policy) Decide(mode, name string) string" {
		t.Fatalf("Decide: %+v", d)
	}
	troba(t, syms, "List.Len", "method")
	for _, s := range syms {
		if s.Name == "_" {
			t.Fatal("el blank identifier no és un símbol")
		}
	}
}

func TestExtractGoAmbErrors(t *testing.T) {
	// Un fitxer a mig editar: el que el parser ha llegit abans de l'error hi és.
	syms := extractGo("x.go", []byte("package x\n\nfunc Bona() {}\n\nfunc Dolenta( {\n"))
	troba(t, syms, "Bona", "func")
}

func TestExtractJSTS(t *testing.T) {
	src := `import x from "y";
export function carrega(a, b) {
}
async function privada() {}
export default class Panell extends Base {
  metode() {}
}
export interface Opcions { a: number }
export type Id = string | number;
export const enum Color { Red }
export const pinta = async (x: number): Promise<void> => {
const curta = x => x * 2;
export const VERSIO = "1";
let intern = 3;
`
	syms := Extract("a.ts", []byte(src))
	if f := troba(t, syms, "carrega", "func"); !f.Exported || f.Line != 2 {
		t.Fatalf("carrega: %+v", f)
	}
	if f := troba(t, syms, "privada", "func"); f.Exported {
		t.Fatal("privada no és exportada")
	}
	troba(t, syms, "Panell", "class")
	troba(t, syms, "Opcions", "interface")
	troba(t, syms, "Id", "type")
	troba(t, syms, "Color", "enum")
	troba(t, syms, "pinta", "func")
	troba(t, syms, "curta", "func")
	troba(t, syms, "VERSIO", "var")
	for _, s := range syms {
		if s.Name == "metode" || s.Name == "intern" {
			t.Fatalf("no és de nivell superior: %+v", s)
		}
	}
}

func TestExtractPython(t *testing.T) {
	src := `"""Mòdul.

class Fals:
"""
import os

class Motor(Base):
    """Doc."""
    def arrenca(self, x):
        def interna():
            pass
    async def _atura(self):
        pass

def principal(argv):
    pass

async def _ajuda():
    pass
`
	syms := Extract("m.py", []byte(src))
	troba(t, syms, "Motor", "class")
	troba(t, syms, "Motor.arrenca", "method")
	if m := troba(t, syms, "Motor._atura", "method"); m.Exported {
		t.Fatal("_atura és privat")
	}
	if f := troba(t, syms, "principal", "func"); f.Recv != "" || f.Sig != "def principal(argv)" {
		t.Fatalf("principal: %+v", f)
	}
	troba(t, syms, "_ajuda", "func")
	for _, s := range syms {
		if s.Name == "Fals" || s.Name == "interna" {
			t.Fatalf("no hauria de sortir: %+v", s)
		}
	}
}

func TestExtractJava(t *testing.T) {
	src := `package a;
@Deprecated
public final class Servei {
    public static class Intern {}
    private void fes() {}
}
interface Repo {}
public enum Estat { A }
public record Punt(int x, int y) {}
`
	syms := Extract("Servei.java", []byte(src))
	if c := troba(t, syms, "Servei", "class"); !c.Exported {
		t.Fatal("Servei és públic")
	}
	troba(t, syms, "Intern", "class")
	troba(t, syms, "Repo", "interface")
	troba(t, syms, "Estat", "enum")
	troba(t, syms, "Punt", "record")
}

func TestExtractCSharp(t *testing.T) {
	src := `namespace App.Core
{
    [Serializable]
    public sealed partial class Gestor : IGestor
    {
        public void Fes() {}
    }
    internal interface IGestor {}
    public readonly struct Mida {}
    public enum Nivell { Baix }
    public record class Persona(string Nom);
}
`
	syms := Extract("Gestor.cs", []byte(src))
	troba(t, syms, "Gestor", "class")
	troba(t, syms, "IGestor", "interface")
	troba(t, syms, "Mida", "struct")
	troba(t, syms, "Nivell", "enum")
	troba(t, syms, "Persona", "record")
}

func TestExtractRust(t *testing.T) {
	src := `use std::fmt;

pub struct Pila<T> { v: Vec<T> }
enum Color { Vermell }
pub trait Forma { fn area(&self) -> f64; }
pub type Res<T> = Result<T, Error>;
pub const MAX: usize = 3;

impl<T> Pila<T> {
    pub fn nova() -> Self { todo!() }
    fn len(&self) -> usize { 0 }
}

impl fmt::Display for Color {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result { Ok(()) }
}

pub async fn principal() {}
`
	syms := Extract("lib.rs", []byte(src))
	if s := troba(t, syms, "Pila", "struct"); !s.Exported {
		t.Fatal("Pila és pub")
	}
	troba(t, syms, "Color", "enum")
	troba(t, syms, "Forma", "trait")
	troba(t, syms, "Res", "type")
	troba(t, syms, "MAX", "const")
	troba(t, syms, "Pila", "impl")
	troba(t, syms, "Pila.nova", "method")
	troba(t, syms, "Pila.len", "method")
	if x := troba(t, syms, "Color", "impl"); x.Sig != "impl fmt::Display for Color" {
		t.Fatalf("impl: %+v", x)
	}
	troba(t, syms, "Color.fmt", "method")
	troba(t, syms, "principal", "func")
}

func TestParts(t *testing.T) {
	for in, want := range map[string]string{
		"MapaProjecte":                    "mapa projecte",
		"HTTPServer":                      "http server",
		"llista_fitxers":                  "llista fitxers",
		"parseJSON2Doc":                   "parse json2 doc",
		"internal/agent/mapa_projecte.go": "internal agent mapa projecte go",
	} {
		if got := strings.Join(Parts(in), " "); got != want {
			t.Errorf("Parts(%q)=%q, volia %q", in, got, want)
		}
	}
}

func escriu(t *testing.T, dir, rel, s string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCacheInvalidacioPerMtime(t *testing.T) {
	Reset()
	dir := t.TempDir()
	escriu(t, dir, "a.go", "package a\n\nfunc Primera() {}\n")
	escriu(t, dir, "b.go", "package a\n\nfunc Altra() {}\n")
	fitxers := []string{"a.go", "b.go"}
	s1 := Build(dir, fitxers, Options{})
	if s1.Parsed != 2 || len(s1.Files) != 2 {
		t.Fatalf("en fred: parsed=%d files=%d", s1.Parsed, len(s1.Files))
	}
	s2 := Build(dir, fitxers, Options{})
	if s2.Parsed != 0 || len(s2.Files) != 2 {
		t.Fatalf("en calent no s'ha de rellegir res: parsed=%d", s2.Parsed)
	}
	escriu(t, dir, "a.go", "package a\n\nfunc Segona() {}\n")
	futur := time.Now().Add(time.Minute)
	if err := os.Chtimes(filepath.Join(dir, "a.go"), futur, futur); err != nil {
		t.Fatal(err)
	}
	s3 := Build(dir, fitxers, Options{})
	if s3.Parsed != 1 {
		t.Fatalf("només a.go ha canviat: parsed=%d", s3.Parsed)
	}
	hits, _ := SearchSnapshot(s3, Query{Name: "Primera"})
	if len(hits) != 0 {
		t.Fatal("Primera ja no existeix")
	}
	if hits, _ := SearchSnapshot(s3, Query{Name: "Segona"}); len(hits) != 1 {
		t.Fatal("falta Segona")
	}
	// Un fitxer esborrat surt de l'índex.
	os.Remove(filepath.Join(dir, "b.go"))
	s4 := Build(dir, []string{"a.go"}, Options{})
	if len(s4.Files) != 1 || s4.Files[0].Path != "a.go" {
		t.Fatalf("b.go hauria d'haver sortit: %+v", s4.Files)
	}
}

func TestBuildSaltaVendorIGrans(t *testing.T) {
	Reset()
	dir := t.TempDir()
	escriu(t, dir, "vendor/x/x.go", "package x\n\nfunc Vendor() {}\n")
	escriu(t, dir, "gran.go", "package a\n\nfunc Gran() {}\n"+strings.Repeat("// farciment\n", 100))
	escriu(t, dir, "ok.go", "package a\n\nfunc Ok() {}\n")
	escriu(t, dir, "notes.txt", "func NoCodi() {}\n")
	s := Build(dir, []string{"gran.go", "notes.txt", "ok.go", "vendor/x/x.go"}, Options{MaxFileSize: 200})
	if len(s.Files) != 1 || s.Files[0].Path != "ok.go" {
		t.Fatalf("només ok.go: %+v", s.Files)
	}
}

func fixtureRank(t *testing.T) *Snapshot {
	t.Helper()
	Reset()
	dir := t.TempDir()
	escriu(t, dir, "a/alfa.go", "package a\n\nfunc Alfa() {}\nfunc Beta() {}\n")
	escriu(t, dir, "b/render.go", "package b\n\nfunc PintaMapa() {}\ntype Pintor struct{}\n")
	escriu(t, dir, "c/llista.go", "package c\n\nfunc LlistaFitxers() {}\n")
	escriu(t, dir, "c/llista_test.go", "package c\n\nfunc TestLlistaFitxers() {}\nfunc ajudaTest() {}\n")
	vell := time.Now().Add(-72 * time.Hour)
	for _, f := range []string{"a/alfa.go", "b/render.go", "c/llista.go", "c/llista_test.go"} {
		os.Chtimes(filepath.Join(dir, filepath.FromSlash(f)), vell, vell)
	}
	return Build(dir, []string{"a/alfa.go", "b/render.go", "c/llista.go", "c/llista_test.go"}, Options{})
}

func primeraLinia(t *testing.T, r Ranked) string {
	t.Helper()
	ls := strings.Split(strings.TrimSpace(r.Text), "\n")
	if len(ls) < 2 {
		t.Fatalf("secció buida: %q", r.Text)
	}
	return ls[1]
}

func TestRankIdentificadorPrimer(t *testing.T) {
	snap := fixtureRank(t)
	r := Rank(snap, "Fix the bug in PintaMapa so it handles empty maps", RankOptions{})
	if !strings.HasPrefix(primeraLinia(t, r), "b/render.go: PintaMapa(f)") {
		t.Fatalf("render.go hauria de ser primer:\n%s", r.Text)
	}
	if !r.Strong {
		t.Fatal("PintaMapa és un identificador literal")
	}
	// Per parts (snake/camel): «llista de fitxers» → LlistaFitxers.
	r = Rank(snap, "la llista de fitxers surt desordenada", RankOptions{})
	if !strings.HasPrefix(primeraLinia(t, r), "c/llista.go:") || r.Strong {
		t.Fatalf("llista.go hauria de ser primer (i no fort):\n%s", r.Text)
	}
	// El test només surt perquè té a veure amb la tasca.
	if !strings.Contains(r.Text, "c/llista_test.go: TestLlistaFitxers(f)") {
		t.Fatalf("falta el test rellevant:\n%s", r.Text)
	}
	r = Rank(snap, "PintaMapa", RankOptions{})
	if strings.Contains(r.Text, "llista_test.go") {
		t.Fatalf("un test sense relació no hi ha de ser:\n%s", r.Text)
	}
}

func TestRankModificatsPugen(t *testing.T) {
	snap := fixtureRank(t)
	r := Rank(snap, "something unrelated", RankOptions{Modified: ModifiedFromPorcelain("## main\n M c/llista.go\n?? b/\n")})
	l := primeraLinia(t, r)
	if !strings.HasPrefix(l, "b/render.go:") && !strings.HasPrefix(l, "c/llista.go:") {
		t.Fatalf("els tocats haurien d'anar primer:\n%s", r.Text)
	}
	if strings.HasPrefix(strings.Split(r.Text, "\n")[3], "a/alfa.go") == false {
		t.Fatalf("alfa.go hauria d'anar l'últim:\n%s", r.Text)
	}
}

func TestRankPressupost(t *testing.T) {
	Reset()
	dir := t.TempDir()
	var fitxers []string
	for i := 0; i < 200; i++ {
		rel := filepath.ToSlash(filepath.Join("p", "f"+strings.Repeat("x", i%7)+string(rune('a'+i%26))+string(rune('a'+i/26))+".go"))
		var b strings.Builder
		b.WriteString("package p\n\n")
		for j := 0; j < 30; j++ {
			b.WriteString("func FuncioAmbNomLlarg" + string(rune('A'+j%26)) + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "() {}\n")
		}
		escriu(t, dir, rel, b.String())
		fitxers = append(fitxers, rel)
	}
	snap := Build(dir, fitxers, Options{Budget: 10 * time.Second})
	for _, tok := range []int{100, 300, DefaultTokens} {
		r := Rank(snap, "anything", RankOptions{Tokens: tok})
		if len(r.Text) > tok*4 {
			t.Fatalf("pressupost %d tokens: %d caràcters", tok, len(r.Text))
		}
		if r.Files == 0 {
			t.Fatalf("pressupost %d: cap fitxer", tok)
		}
	}
}

func TestRankSenseCodi(t *testing.T) {
	Reset()
	dir := t.TempDir()
	escriu(t, dir, "README.md", "# hola\n")
	snap := Build(dir, []string{"README.md"}, Options{})
	if r := Rank(snap, "README", RankOptions{}); r.Text != "" {
		t.Fatalf("sense codi no hi ha secció: %q", r.Text)
	}
}

func TestSearch(t *testing.T) {
	snap := fixtureRank(t)
	hits, err := SearchSnapshot(snap, Query{Name: "llista"})
	if err != nil || len(hits) != 2 || hits[0].Path != "c/llista.go" {
		t.Fatalf("subcadena: %v %+v", err, hits)
	}
	out := FormatHits(hits, snap, 0)
	if !strings.Contains(out, "c/llista.go:3 func LlistaFitxers — func LlistaFitxers()") {
		t.Fatalf("format: %q", out)
	}
	if hits, _ := SearchSnapshot(snap, Query{Name: "^Pint", Regex: true}); len(hits) != 2 {
		t.Fatalf("regex: %+v", hits)
	}
	if hits, _ := SearchSnapshot(snap, Query{Kind: "struct"}); len(hits) != 1 || hits[0].Name != "Pintor" {
		t.Fatalf("kind: %+v", hits)
	}
	if hits, _ := SearchSnapshot(snap, Query{File: "**/*_test.go"}); len(hits) != 2 {
		t.Fatalf("glob: %+v", hits)
	}
	if hits, _ := SearchSnapshot(snap, Query{File: "alfa.go", Name: "beta"}); len(hits) != 1 {
		t.Fatalf("glob pel nom: %+v", hits)
	}
	if hits, _ := SearchSnapshot(snap, Query{Name: "a", Exported: true, File: "c/*"}); len(hits) != 2 {
		t.Fatalf("exportats: %+v", hits)
	}
	if _, err := SearchSnapshot(snap, Query{Name: "(", Regex: true}); err == nil {
		t.Fatal("regex invàlida")
	}
	if _, err := SearchSnapshot(snap, Query{Kind: "potato"}); err == nil {
		t.Fatal("kind desconegut")
	}
	if out := FormatHits(hits[:0], snap, 0); !strings.HasPrefix(out, "no symbols found") {
		t.Fatalf("buit: %q", out)
	}
	many := make([]Hit, 60)
	if out := FormatHits(many, snap, 10); !strings.Contains(out, "50 more matches") {
		t.Fatalf("topall: %q", out)
	}
}

// arrelRepo és l'arrel del mòdul (on hi ha go.mod), a partir d'aquest fitxer.
func arrelRepo(t testing.TB) string {
	_, aqui, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("sense runtime.Caller")
	}
	for d := filepath.Dir(aqui); ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		if filepath.Dir(d) == d {
			t.Skip("no trobo go.mod")
		}
	}
}

// TestMesuraRepo mesura l'índex sobre aquest mateix repositori: en fred,
// en calent, i la mida de la secció pintada. Només informa.
func TestMesuraRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("mesura llarga")
	}
	dir := arrelRepo(t)
	t0 := time.Now()
	fitxers, _ := ListFiles(dir)
	llista := time.Since(t0)
	Reset()
	fred := Build(dir, fitxers, Options{Budget: time.Minute})
	Reset()
	fred300 := Build(dir, fitxers, Options{})
	calent := Build(dir, fitxers, Options{Budget: time.Minute})
	calent2 := Build(dir, fitxers, Options{Budget: time.Minute})
	nsym := 0
	for _, f := range calent.Files {
		nsym += len(f.Symbols)
	}
	t1 := time.Now()
	r := Rank(calent, "Add a symbols section to MapaProjecte and keep SenseMapa stripping it", RankOptions{})
	rank := time.Since(t1)
	t.Logf("fitxers llistats=%d (ls-files %v); amb símbols=%d, símbols=%d", len(fitxers), llista, len(calent.Files), nsym)
	t.Logf("fred sense límit: %v (parsed=%d); fred amb 300ms: %v (parsed=%d, parcial=%v)", fred.Elapsed, fred.Parsed, fred300.Elapsed, fred300.Parsed, fred300.Partial)
	t.Logf("calent (després del fred parcial): %v parsed=%d; calent2: %v parsed=%d", calent.Elapsed, calent.Parsed, calent2.Elapsed, calent2.Parsed)
	t.Logf("rànquing+pintat: %v; secció: %d bytes ≈ %d tokens, %d fitxers", rank, len(r.Text), len(r.Text)/4, r.Files)
	if len(r.Text) > DefaultTokens*4 {
		t.Fatalf("la secció passa del pressupost: %d", len(r.Text))
	}
}

func BenchmarkBuildCalent(b *testing.B) {
	dir := arrelRepo(b)
	fitxers, _ := ListFiles(dir)
	Build(dir, fitxers, Options{Budget: time.Minute})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Build(dir, fitxers, Options{Budget: time.Minute})
	}
}
