package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// casEdicio és una edició que fallava amb la comparació exacta, presa dels
// errors típics dels models (sobretot els petits i els locals): què hi ha al
// fitxer, què ha enviat el model, i què ha de quedar.
type casEdicio struct {
	nom      string
	fitxer   string
	old, new string
	vol      string // fitxer resultant; "" si ha de fallar
	nivell   string // nivell que ha de casar (subcadena)
	errConte string // si ha de fallar, què ha de dir l'error
}

var corpusEdicions = []casEdicio{
	{
		nom:    "Go amb tabuladors, el model escriu quatre espais",
		fitxer: "package a\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn x\n\t}\n\treturn 0\n}\n",
		old:    "    if x > 0 {\n        return x\n    }",
		new:    "    if x > 0 {\n        log.Println(x)\n        return x * 2\n    }",
		vol:    "package a\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\tlog.Println(x)\n\t\treturn x * 2\n\t}\n\treturn 0\n}\n",
		nivell: nivIndentacio,
	},
	{
		nom:    "línia nova més niada que cap d'old, en un fitxer amb tabuladors",
		fitxer: "func G() {\n\tfor _, v := range xs {\n\t\tuse(v)\n\t}\n}\n",
		old:    "    for _, v := range xs {\n        use(v)\n    }",
		new:    "    for _, v := range xs {\n        if v != nil {\n            use(v)\n        }\n    }",
		vol:    "func G() {\n\tfor _, v := range xs {\n\t\tif v != nil {\n\t\t\tuse(v)\n\t\t}\n\t}\n}\n",
		nivell: nivIndentacio,
	},
	{
		nom:    "espais al final de línia al fitxer que el model no copia",
		fitxer: "def f():  \n    x = 1   \n    return x\n",
		old:    "def f():\n    x = 1\n",
		new:    "def f():\n    x = 2\n",
		vol:    "def f():\n    x = 2\n    return x\n",
		nivell: nivEspaisFinal,
	},
	{
		nom:    "Python: bloc d'un mètode copiat sense el sagnat de la classe",
		fitxer: "class A:\n    def run(self):\n        if self.ok:\n            go()\n        stop()\n",
		old:    "if self.ok:\n    go()\nstop()",
		new:    "if self.ok:\n    go()\n    log()\nstop()",
		vol:    "class A:\n    def run(self):\n        if self.ok:\n            go()\n            log()\n        stop()\n",
		nivell: nivIndentacio,
	},
	{
		nom:    "JS a dos espais, el model en fa servir quatre",
		fitxer: "function f() {\n  if (a) {\n    b();\n  }\n}\n",
		old:    "    if (a) {\n        b();\n    }",
		new:    "    if (a) {\n        b();\n        c();\n    }",
		vol:    "function f() {\n  if (a) {\n    b();\n    c();\n  }\n}\n",
		nivell: nivIndentacio,
	},
	{
		nom:    "sagnat incoherent: només casen les línies retallades",
		fitxer: "x = [\n  1,\n      2,\n]\n",
		old:    "x = [\n1,\n2,\n]",
		new:    "x = [\n1,\n2,\n3,\n]",
		vol:    "x = [\n1,\n2,\n3,\n]\n",
		nivell: nivRetallat,
	},
	{
		nom:    "cometes tipogràfiques al fitxer, rectes al model",
		fitxer: "// Diu “hola” al món — i se’n va\nfunc Hola() {}\n",
		old:    "// Diu \"hola\" al món - i se'n va\nfunc Hola() {}",
		new:    "// Diu hola\nfunc Hola() {}",
		vol:    "// Diu hola\nfunc Hola() {}\n",
		nivell: nivUnicode,
	},
	{
		nom:    "espai no separable dins d'una cadena",
		fitxer: "msg := \"10\u00a0km\"\nfmt.Println(msg)\n",
		old:    "msg := \"10 km\"\nfmt.Println(msg)",
		new:    "msg := \"10 km\"\nfmt.Println(msg, \"!\")",
		vol:    "msg := \"10 km\"\nfmt.Println(msg, \"!\")\n",
		nivell: nivUnicode,
	},
	{
		nom:    "una línia del mig reescrita de memòria (àncores)",
		fitxer: "func Suma(xs []int) int {\n\ttotal := 0\n\tfor _, x := range xs {\n\t\ttotal += x\n\t}\n\treturn total\n}\n",
		old:    "func Suma(xs []int) int {\n\ttotal := 0\n\tfor _, v := range xs {\n\t\ttotal += x\n\t}\n\treturn total\n}",
		new:    "func Suma(xs []int) (total int) {\n\tfor _, x := range xs {\n\t\ttotal += x\n\t}\n\treturn\n}",
		vol:    "func Suma(xs []int) (total int) {\n\tfor _, x := range xs {\n\t\ttotal += x\n\t}\n\treturn\n}\n",
		nivell: nivAncores,
	},
	{
		nom:    "línies en blanc de més a l'inici i al final d'old",
		fitxer: "a\nb  \nc\n",
		old:    "\n\nb\n\n",
		new:    "\n\nB\n\n",
		vol:    "a\nB\nc\n",
		nivell: nivEspaisFinal,
	},
	{
		nom:    "bloc copiat de read amb els números de línia",
		fitxer: "package a\n\nvar x = 1\nvar y = 2\n",
		old:    "3|var x = 1\n4|var y = 2",
		new:    "3|var x = 10\n4|var y = 20",
		vol:    "package a\n\nvar x = 10\nvar y = 20\n",
		nivell: nivNumeros,
	},
	{
		nom:    "CRLF i sagnat diferent alhora",
		fitxer: "func F() {\r\n\tg()\r\n}\r\n",
		old:    "func F() {\n    g()\n}",
		new:    "func F() {\n    g()\n    h()\n}",
		vol:    "func F() {\r\n\tg()\r\n\th()\r\n}\r\n",
		nivell: nivIndentacio,
	},
	{
		nom:      "dos blocs iguals llevat del sagnat: ambigu, no se n'escull cap",
		fitxer:   "if a {\n\tx()\n}\nfunc g() {\n\t\tx()\n}\n",
		old:      "  x()",
		new:      "  y()",
		errConte: "ambigu",
	},
	{
		nom:      "línia inventada al mig: les àncores no la deixen passar",
		fitxer:   "func F() {\n\ta()\n\tb()\n}\n",
		old:      "func F() {\n\ta()\n\tlog.Fatal(\"x\")\n\tb()\n\tc()\n}",
		new:      "x",
		errConte: "fragment més semblant",
	},
	{
		nom:      "bloc inventat del tot",
		fitxer:   "package a\n\nfunc F() {}\n",
		old:      "type Servidor struct {\n\tport int\n}",
		new:      "x",
		errConte: "res semblant",
	},
}

func TestCorpusEdicionsFallides(t *testing.T) {
	for _, c := range corpusEdicions {
		t.Run(c.nom, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "f.txt")
			if err := os.WriteFile(p, []byte(c.fitxer), 0o644); err != nil {
				t.Fatal(err)
			}
			nota, err := edita(p, c.old, c.new, false, nil)
			raw, _ := os.ReadFile(p)
			if c.vol == "" {
				if err == nil {
					t.Fatalf("havia de fallar i ha deixat:\n%s", raw)
				}
				if !strings.Contains(err.Error(), c.errConte) {
					t.Errorf("l'error havia de dir %q: %v", c.errConte, err)
				}
				if string(raw) != c.fitxer {
					t.Errorf("un error no pot tocar el fitxer:\n%q", raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("no s'ha aplicat: %v", err)
			}
			if string(raw) != c.vol {
				t.Errorf("resultat:\n%q\nvolia:\n%q", raw, c.vol)
			}
			if !strings.Contains(nota, c.nivell) || !strings.Contains(nota, "aproximada") {
				t.Errorf("la nota havia de dir el nivell %q: %q", c.nivell, nota)
			}
		})
	}
}

// Una coincidència exacta no porta cap avís de nivell, però sí el diff.
func TestEditExacteSenseNotaDeNivellAmbDiff(t *testing.T) {
	p := fitxer(t, "a\nb\nc\nd\ne\n")
	nota, err := edita(p, "c", "C", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(nota, "aproximada") {
		t.Errorf("exacte no ha d'avisar: %q", nota)
	}
	if !strings.Contains(nota, "-3|c") || !strings.Contains(nota, "+3|C") || !strings.Contains(nota, " 2|b") {
		t.Errorf("el diff ha de dur el canvi i el context numerat: %q", nota)
	}
}

func TestErrorEnsenyaElFragmentMesSemblantAmbLiniesIPercentatge(t *testing.T) {
	p := fitxer(t, "uno\ndos\nfunc Calcula(a, b int) int {\n\treturn a + b\n}\ntres\n")
	err := Edit(p, "func Calcula(a, b int) int {\n\treturn a * b + 1\n\t// nota\n}", "x")
	if err == nil {
		t.Fatal("havia de fallar")
	}
	for _, s := range []string{"fragment més semblant", "%", "línies 3–", "3|func Calcula"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("l'error havia de contenir %q:\n%v", s, err)
		}
	}
}

// La guarda d'opencode: un tros molt més gran que old no s'esborra.
func TestDesproporcionat(t *testing.T) {
	if !desproporcionat("a\nb\nc\nd\ne\nf", "a\nf") {
		t.Error("sis línies per dues és desproporcionat")
	}
	if desproporcionat("a\nb\nc", "a\nx\nc") {
		t.Error("mateixa mida no és desproporcionat")
	}
	if !desproporcionat("x\n"+strings.Repeat("y", 2000)+"\nz", "x\ny\nz") {
		t.Error("molts més caràcters és desproporcionat")
	}
}

func TestPatchFuzzyAvisaIDonaDiff(t *testing.T) {
	p := fitxer(t, "func F() {\n\ta()\n\tb()\n}\n")
	nota, err := pedaça(p, []PatchOp{
		{Old: "\ta()", New: "\tA()"},
		{Old: "    b()", New: "    B()"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p)
	if string(raw) != "func F() {\n\tA()\n\tB()\n}\n" {
		t.Errorf("%q", raw)
	}
	if !strings.Contains(nota, "edició 2") || strings.Contains(nota, "edició 1:") || !strings.Contains(nota, "+3|\tB()") {
		t.Errorf("nota: %q", nota)
	}
}

func TestNormalitzaUnicode(t *testing.T) {
	if got := normalitzaUnicode("“a” ‘b’ c–d e—f g\u00a0h i…\u200b"); got != `"a" 'b' c-d e-f g h i...` {
		t.Errorf("%q", got)
	}
}
