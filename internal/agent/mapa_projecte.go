package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gregal/internal/llm"
	"gregal/internal/repomap"
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
const MarcaContext = "[project map"

// MarcaContextCA és la marca d'abans que el bloc passés a l'anglès: les
// sessions desades la porten i SenseMapa/jaHiEs l'han de continuar veient.
const MarcaContextCA = "[mapa del projecte"

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
	estat := ""
	if git {
		if estat = estatGit(dir); estat != "" {
			cos.WriteString("\nGit status:\n" + estat)
		}
	}
	// L'empremta és la del llistat i l'estat de git, no la dels símbols:
	// els símbols depenen de la tasca i, si hi entressin, el mapa es
	// repetiria a cada torn.
	empremta := empremtaDe(cos.String())
	adjunts := adjuntsAnomenats(dir, tasca, fitxers)
	simbols := seccioSimbols(dir, tasca, fitxers, estat)
	// Si l'historial ja porta aquest mateix llistat i la tasca no anomena
	// cap fitxer nou, no cal repetir-lo: en una sessió de vint torns serien
	// milers de tokens iguals.
	if adjunts == "" && jaHiEs(hist, empremta) {
		// Si la tasca anomena literalment un símbol («arregla MapaProjecte»),
		// els símbols rellevants sí que hi van, sense tornar a posar la
		// llista. La marca porta « symbols» perquè jaHiEs no el prengui pel
		// llistat sencer (si una compactació se l'endu, ha de tornar).
		if simbols.Strong {
			return fmt.Sprintf("%s %s symbols] Working directory: %s. The file list given earlier is unchanged.\n%s",
				MarcaContext, empremta, filepath.ToSlash(dir), simbols.Text)
		}
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s] Working directory: %s. File list taken at the start of the turn: use it instead of ls/find/pwd, and look at the disk again if you changed it.\n",
		MarcaContext, empremta, filepath.ToSlash(dir))
	b.WriteString(cos.String())
	if simbols.Text != "" {
		b.WriteString("\n" + simbols.Text)
	}
	b.WriteString(adjunts)
	return b.String()
}

// TokensMapaSimbols és el pressupost de la secció de símbols del mapa.
// GREGAL_REPOMAP_TOKENS el canvia (0 l'apaga).
func TokensMapaSimbols() int {
	if v := strings.TrimSpace(os.Getenv("GREGAL_REPOMAP_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return repomap.DefaultTokens
}

// seccioSimbols és el mapa de símbols ordenat per la tasca (repomap). Va
// dins del mateix bloc marcat, i per això SenseMapa també el treu. Amb un
// repositori enorme o un disc lent l'índex es talla als 300 ms i la secció
// surt amb el que hi ha; sense codi, no surt.
func seccioSimbols(dir, tasca string, fitxers []string, estat string) repomap.Ranked {
	tokens := TokensMapaSimbols()
	if tokens <= 0 {
		return repomap.Ranked{}
	}
	tocats := repomap.ModifiedFromPorcelain(estat)
	pri := make([]string, 0, len(tocats))
	for p := range tocats {
		if !strings.HasSuffix(p, "/") {
			pri = append(pri, p)
		}
	}
	sort.Strings(pri)
	snap := repomap.Build(dir, fitxers, repomap.Options{Priority: pri})
	return repomap.Rank(snap, tasca, repomap.RankOptions{Tokens: tokens, Modified: tocats})
}

// TascaAmbMapa és el que criden els clients en començar un torn: el mapa
// només als modes que treballen amb el projecte (el xat conversa).
func TascaAmbMapa(mode, dir, tasca string, hist []llm.Message) string {
	switch mode {
	case ModeCode, ModeAutonomous, ModeDebug, ModeInspect, ModeGoal, "":
		return AmbMapaProjecte(dir, tasca, hist)
	}
	return tasca
}

// SenseMapa treu el bloc del mapa d'un missatge, per pintar-lo o titular-lo.
func SenseMapa(s string) string {
	for _, marca := range []string{MarcaContext, MarcaContextCA} {
		if i := strings.Index(s, "\n\n"+marca); i >= 0 {
			return s[:i]
		}
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
// Viu a repomap perquè l'índex de símbols segueixi les mateixes regles.
func llistaFitxers(dir string) ([]string, bool) {
	return repomap.ListFiles(dir)
}

// resumFitxers és la llista sencera si és curta; si no, els fitxers de
// l'arrel i cada carpeta amb quants fitxers té.
func resumFitxers(fitxers []string) string {
	var b strings.Builder
	if len(fitxers) <= maxFitxersLlista {
		fmt.Fprintf(&b, "Files (%d):\n", len(fitxers))
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
	fmt.Fprintf(&b, "Files (%d, too many to list them all; use glob/grep to look inside the folders):\n", len(fitxers))
	for _, c := range noms {
		fmt.Fprintf(&b, "%s/ (%d)\n", c, carpetes[c])
	}
	for i, f := range arrel {
		if i >= maxFitxersLlista {
			fmt.Fprintf(&b, "… and %d more files in the root\n", len(arrel)-i)
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
		return linies[0] + " (clean, no changes)\n"
	}
	if len(linies) > maxEstatGit+1 {
		n := len(linies) - (maxEstatGit + 1)
		linies = append(linies[:maxEstatGit+1], fmt.Sprintf("… and %d more changes", n))
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
		fmt.Fprintf(&b, "\n--- %s (contents at the start of the turn; no need to read it again unless you changed it) ---\n%s", f, dades)
		if !strings.HasSuffix(string(dades), "\n") {
			b.WriteString("\n")
		}
		b.WriteString("--- end of " + f + " ---\n")
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
	marcaCA := MarcaContextCA + " " + empremta + "]"
	for i := len(hist) - 1; i >= 0; i-- {
		if hist[i].Role == "user" && (strings.Contains(hist[i].Content, marca) || strings.Contains(hist[i].Content, marcaCA)) {
			return true
		}
	}
	return false
}
