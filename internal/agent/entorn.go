package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Fets de l'entorn per al system prompt.
//
// Mesurat amb el model real sobre un projecte Go a Windows: de dotze
// passos, SET se'n van anar buscant on era el compilador. `go version`
// fallava —el toolchain no era al PATH—, i a partir d'aquí l'agent va
// provar `which go.exe`, `ls /c/Go`, `find /c -maxdepth 3 -name go.exe`,
// `echo $PATH`… fins a trobar-lo a «C:\Program Files\Go\bin» al pas dotze,
// amb el pressupost ja esgotat. La feina va sortir bé, però per sort.
//
// No és una cosa que l'agent pugui deduir: o li ho diem, o ho busca. Dir-ho
// costa unes quantes crides a LookPath un sol cop per procés; buscar-ho li
// costa mitja tasca. Per això també diem quan una eina hi és però NO és al
// PATH: és el cas que fa perdre els passos, perquè `go version` falla
// encara que el compilador estigui instal·lat dos directoris més enllà.

// einaTrobada és una eina del sistema i on és.
type einaTrobada struct {
	Nom    string
	Ruta   string
	AlPath bool
}

// einesConegudes són les que un agent de programació fa servir de seguida.
// Per a cadascuna, els llocs on mirar si no és al PATH.
var einesConegudes = map[string][]string{
	"go":     {`C:\Program Files\Go\bin\go.exe`, `C:\Go\bin\go.exe`, "/usr/local/go/bin/go", "/usr/lib/go/bin/go", "~/sdk/go/bin/go"},
	"node":   {`C:\Program Files\nodejs\node.exe`, "/usr/local/bin/node", "/usr/bin/node"},
	"python": {"/usr/bin/python3", "/usr/local/bin/python3"},
	"java":   {`C:\Program Files\Java`, "/usr/lib/jvm"},
	"git":    {`C:\Program Files\Git\cmd\git.exe`, "/usr/bin/git"},
	"docker": {`C:\Program Files\Docker\Docker\resources\bin\docker.exe`, "/usr/bin/docker"},
	"rustc":  {"/usr/local/bin/rustc"},
}

var (
	einesUnCop sync.Once
	einesCache []einaTrobada
)

// Eines cerca les eines conegudes. Es calcula un sol cop per procés: la
// resposta no canvia i cada torn la demanaria altra vegada.
func Eines() []einaTrobada {
	einesUnCop.Do(func() { einesCache = cercaEines() })
	return einesCache
}

func cercaEines() []einaTrobada {
	noms := make([]string, 0, len(einesConegudes))
	for n := range einesConegudes {
		noms = append(noms, n)
	}
	sort.Strings(noms)
	out := make([]einaTrobada, 0, len(noms))
	for _, nom := range noms {
		if ruta, err := exec.LookPath(nom); err == nil {
			out = append(out, einaTrobada{Nom: nom, Ruta: ruta, AlPath: true})
			continue
		}
		// El cas que ens interessa: hi és, però `nom` sol no funciona.
		for _, cand := range einesConegudes[nom] {
			if r := resolCandidat(nom, cand); r != "" {
				out = append(out, einaTrobada{Nom: nom, Ruta: r})
				break
			}
		}
	}
	return out
}

// resolCandidat accepta tant un executable com un directori contenidor
// (java i els JDK: «C:\Program Files\Java\jdk-21\bin\java.exe»).
// També entén ~ (és on viu el go de l'Strix: ~/sdk/go/bin/go).
func resolCandidat(nom, cand string) string {
	if strings.HasPrefix(cand, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			cand = filepath.Join(home, strings.TrimPrefix(cand, "~/"))
		}
	}
	st, err := os.Stat(cand)
	if err != nil {
		return ""
	}
	if !st.IsDir() {
		return cand
	}
	entrades, err := os.ReadDir(cand)
	if err != nil {
		return ""
	}
	bin := nom
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	for _, e := range entrades {
		exe := filepath.Join(cand, e.Name(), "bin", bin)
		if _, err := os.Stat(exe); err == nil {
			return exe
		}
	}
	return ""
}

// FetsEntorn descriu el sistema i les eines per al system prompt. Cadena
// buida si no hi ha res a dir.
func FetsEntorn() string {
	var b strings.Builder
	fmt.Fprintf(&b, "SISTEMA: %s/%s", runtime.GOOS, runtime.GOARCH)
	if sh := shellPerDefecte(); sh != "" {
		b.WriteString(" · les ordres de bash s'executen amb " + sh)
	}
	// Des d'on corren: sense dir-ho, el model posava «cd <camí absolut> &&»
	// davant de cada ordre (i un `cd /$(pwd)` fallit gastava un pas sencer
	// al banc A/B). Cada ordre és un procés nou: un cd no dura.
	b.WriteString(" · cada ordre de bash comença al directori del projecte: no cal fer cd, i un cd no es manté a l'ordre següent")
	b.WriteString("\n")
	eines := Eines()
	if len(eines) == 0 {
		return b.String()
	}
	b.WriteString("EINES DEL SISTEMA (comprovades en arrencar; no en dedueixis cap més):\n")
	for _, e := range eines {
		b.WriteString(descriuEina(e) + "\n")
	}
	return b.String()
}

// descriuEina és la línia d'una eina. La que importa és la segona: hi és
// però `nom` sol falla, que és el cas que fa perdre els passos.
func descriuEina(e einaTrobada) string {
	if e.AlPath {
		return "- " + e.Nom + ": al PATH"
	}
	// Barres normals, no %q: a Windows el %q doblava les contrabarres i el
	// que llegia el model era «C:\\Program Files\\Go\\bin». Les ordres van
	// a bash, que entén les barres normals.
	return fmt.Sprintf("- %s: hi és a %s però NO és al PATH — afegeix-l'hi tu a l'ordre (export PATH=\"%s:$PATH\") en comptes de buscar-lo",
		e.Nom, filepath.ToSlash(e.Ruta), filepath.ToSlash(filepath.Dir(e.Ruta)))
}

// shellPerDefecte diu amb què s'executen les ordres, que a Windows no és
// evident: és Git Bash, no cmd ni PowerShell, i l'agent escrivia ordres de
// cmd que fallaven.
func shellPerDefecte() string {
	if runtime.GOOS != "windows" {
		return "sh"
	}
	if _, err := exec.LookPath("bash"); err == nil {
		return "bash (Git Bash), no cmd ni PowerShell"
	}
	return ""
}

// dies són els noms catalans, en l'ordre de time.Weekday (diumenge = 0).
var dies = [7]string{"diumenge", "dilluns", "dimarts", "dimecres", "dijous", "divendres", "dissabte"}

// DataAra diu quin dia és. NO l'hora, i és a posta.
//
// Halogen desa el punt de represa del seu KV cache AL FINAL del system prompt
// (HALOGEN_PROMPT_CACHE=2, el seu defecte). Qualsevol canvi en aquest prompt
// invalida el punt i obliga a re-llegir tota la conversa: amb cache, un torn
// de seguida a 100k costa ~2 s; sense, ~88 s. Una hora dins del prompt
// canviava cada minut i destruïa aquesta cache a cada torn; la data canvia un
// cop al dia. L'hora exacta, si cal, la dona `date`, que és a la llista de
// lectura segura.
func DataAra() string {
	now := time.Now()
	return fmt.Sprintf("DATA D'ARA: %s (%s). Si et cal l'hora exacta, crida bash amb `date`.",
		now.Format("2006-01-02"), dies[now.Weekday()])
}
