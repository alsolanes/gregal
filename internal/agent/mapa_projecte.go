package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gregal/internal/llm"
)

// Context del projecte al començament del torn.
//
// Mesurat al banc A/B contra opencode (mateix model, mateixes tasques): les
// primeres dues o tres voltes de Gregal eren orientar-se —pwd, ls, find,
// llegir TASK.md— i opencode se les estalviava perquè el seu prompt ja duu
// l'arbre del projecte. Cada volta és una espera sencera del model, i amb
// un model local és el que més pesa. Aquí el torn arrenca amb el llistat de
// fitxers, l'estat de git i els fitxers petits que la tasca anomena.
//
// Va enganxat al missatge de la tasca i no al system prompt: el system és
// el prefix que el KV cache dels models locals reutilitza, i un llistat
// que canvia quan l'agent crea un fitxer l'invalidaria a cada torn. I no
// es repeteix en una sessió llarga si no ha canviat (MarcaContext).

// MarcaContext encapçala el bloc: serveix per no tornar-lo a posar igual.
const MarcaContext = "[mapa del projecte"

const (
	maxFitxersLlista = 150
	maxAdjunt        = 8 << 10  // bytes per fitxer anomenat
	maxAdjuntsTotal  = 16 << 10 // bytes de tots els adjunts
	maxAdjunts       = 3
	maxEstatGit      = 30
)

// MapaProjecte torna el bloc de context per a la tasca, o "" si no hi
// ha res a dir o si és igual a l'últim que ja porta l'historial.
func MapaProjecte(dir, tasca string, hist []llm.Message) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		}
	}
	if dir == "" {
		return ""
	}
	fitxers, git := llistaFitxers(dir)
	if len(fitxers) == 0 {
		return ""
	}
	var cos strings.Builder
	cos.WriteString(resumFitxers(fitxers))
	if git {
		if e := estatGit(dir); e != "" {
			cos.WriteString("\nEstat de git:\n" + e)
		}
	}
	empremta := empremtaDe(cos.String())
	adjunts := adjuntsAnomenats(dir, tasca, fitxers)
	// Si l'historial ja porta aquest mateix llistat i la tasca no anomena
	// cap fitxer nou, no cal repetir-lo: en una sessió de vint torns serien
	// milers de tokens iguals.
	if adjunts == "" && jaHiEs(hist, empremta) {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s] Directori de treball: %s. Llistat pres en començar el torn: fes-lo servir en comptes de ls/find/pwd, i torna a mirar el disc si l'has canviat.\n",
		MarcaContext, empremta, filepath.ToSlash(dir))
	b.WriteString(cos.String())
	b.WriteString(adjunts)
	return b.String()
}

// TascaAmbMapa és el que criden els clients en començar un torn: el mapa
// només als modes que treballen amb el projecte (el xat conversa).
func TascaAmbMapa(mode, dir, tasca string, hist []llm.Message) string {
	switch mode {
	case ModeCode, ModeAutonomous, ModeInspect, ModeGoal, "":
		return AmbMapaProjecte(dir, tasca, hist)
	}
	return tasca
}

// SenseMapa treu el bloc del mapa d'un missatge, per pintar-lo o titular-lo.
func SenseMapa(s string) string {
	if i := strings.Index(s, "\n\n"+MarcaContext); i >= 0 {
		return s[:i]
	}
	return s
}

// AmbMapaProjecte afegeix el context a la tasca (o la torna igual).
func AmbMapaProjecte(dir, tasca string, hist []llm.Message) string {
	if c := MapaProjecte(dir, tasca, hist); c != "" {
		return tasca + "\n\n" + c
	}
	return tasca
}

// llistaFitxers torna els fitxers del projecte (camins relatius, barres
// normals) i si és un repositori git. Amb git mana el que git veu (sense
// els ignorats); sense, un recorregut que salta el que no és del projecte.
func llistaFitxers(dir string) ([]string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		var fs []string
		for _, l := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				fs = append(fs, l)
			}
		}
		sort.Strings(fs)
		return fs, true
	}
	var out []string
	salta := map[string]bool{".git": true, "node_modules": true, "vendor": true, "__pycache__": true,
		"dist": true, "build": true, "target": true, ".venv": true, "venv": true, ".idea": true, ".vscode": true}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != dir && (salta[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= 5000 {
			return filepath.SkipAll
		}
		if rel, err := filepath.Rel(dir, p); err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, false
}

// resumFitxers és la llista sencera si és curta; si no, els fitxers de
// l'arrel i cada carpeta amb quants fitxers té.
func resumFitxers(fitxers []string) string {
	var b strings.Builder
	if len(fitxers) <= maxFitxersLlista {
		fmt.Fprintf(&b, "Fitxers (%d):\n", len(fitxers))
		for _, f := range fitxers {
			b.WriteString(f + "\n")
		}
		return b.String()
	}
	arrel := []string{}
	carpetes := map[string]int{}
	for _, f := range fitxers {
		if i := strings.IndexByte(f, '/'); i >= 0 {
			carpetes[f[:i]]++
		} else {
			arrel = append(arrel, f)
		}
	}
	noms := make([]string, 0, len(carpetes))
	for c := range carpetes {
		noms = append(noms, c)
	}
	sort.Strings(noms)
	fmt.Fprintf(&b, "Fitxers (%d, massa per llistar-los tots; fes servir glob/grep per entrar a les carpetes):\n", len(fitxers))
	for _, c := range noms {
		fmt.Fprintf(&b, "%s/ (%d)\n", c, carpetes[c])
	}
	for i, f := range arrel {
		if i >= maxFitxersLlista {
			fmt.Fprintf(&b, "… i %d fitxers més a l'arrel\n", len(arrel)-i)
			break
		}
		b.WriteString(f + "\n")
	}
	return b.String()
}

// estatGit és la branca i els canvis sense desar (retallat).
func estatGit(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain", "--branch")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	linies := strings.Split(strings.TrimRight(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n"), "\n")
	if len(linies) == 1 {
		return linies[0] + " (net, sense canvis)\n"
	}
	if len(linies) > maxEstatGit+1 {
		n := len(linies) - (maxEstatGit + 1)
		linies = append(linies[:maxEstatGit+1], fmt.Sprintf("… i %d canvis més", n))
	}
	return strings.Join(linies, "\n") + "\n"
}

// adjuntsAnomenats inclou els fitxers petits que la tasca anomena pel seu
// camí exacte: si la tasca diu «llegeix TASK.md», la primera volta del
// model era llegir-lo. Només fitxers de text, petits i dins del projecte.
func adjuntsAnomenats(dir, tasca string, fitxers []string) string {
	var b strings.Builder
	total, n := 0, 0
	for _, f := range fitxers {
		if n >= maxAdjunts {
			break
		}
		if !anomenat(tasca, f) {
			continue
		}
		dades, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
		if err != nil || len(dades) == 0 || len(dades) > maxAdjunt || total+len(dades) > maxAdjuntsTotal || !esText(dades) {
			continue
		}
		total += len(dades)
		n++
		fmt.Fprintf(&b, "\n--- %s (contingut en començar el torn; no cal tornar-lo a llegir si no l'has canviat) ---\n%s", f, dades)
		if !strings.HasSuffix(string(dades), "\n") {
			b.WriteString("\n")
		}
		b.WriteString("--- fi de " + f + " ---\n")
	}
	return b.String()
}

// anomenat diu si la tasca conté el camí com a paraula sencera (no
// «TASK.md» dins de «MYTASK.md»). Els noms massa curts o sense punt no
// compten: «go» o «main» sortirien a qualsevol frase.
func anomenat(tasca, f string) bool {
	if len(f) < 4 || !strings.Contains(filepath.Base(f), ".") {
		return false
	}
	for i := 0; ; {
		j := strings.Index(tasca[i:], f)
		if j < 0 {
			return false
		}
		a, z := i+j, i+j+len(f)
		if (a == 0 || !esNomChar(tasca[a-1])) && (z == len(tasca) || !esNomChar(tasca[z])) {
			return true
		}
		i = a + 1
	}
}

// esNomChar: el punt no compta, perquè «llegeix TASK.md.» acaba la frase.
func esNomChar(c byte) bool {
	return c == '_' || c == '-' || c == '/' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// esText descarta binaris (un NUL als primers bytes).
func esText(b []byte) bool {
	n := len(b)
	if n > 1024 {
		n = 1024
	}
	for _, c := range b[:n] {
		if c == 0 {
			return false
		}
	}
	return true
}

func empremtaDe(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:4])
}

// jaHiEs diu si l'historial ja porta un context amb aquesta empremta.
func jaHiEs(hist []llm.Message, empremta string) bool {
	marca := MarcaContext + " " + empremta + "]"
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].Role == "user" && strings.Contains(hist[i].Content, marca) {
			return true
		}
	}
	return false
}
