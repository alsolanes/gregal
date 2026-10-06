package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gregal/internal/procs"
	"gregal/internal/shell"
)

// Límits de seguretat per defecte.
const (
	MaxOutputChars = 8000
	// DefaultTimeout eren 30s, i es quedaven curts per a la primera cosa
	// que fa qualsevol agent en un projecte: compilar. Mesurat en aquest
	// mateix repositori amb la cau buida, `go build ./...` triga 36s —amb
	// la cau calenta, 9s—, o sigui que la primera compilació d'una sessió
	// es tallava sempre, i el que arribava al model era un timeout sec del
	// qual no es pot saber si el codi està trencat o si només ha faltat
	// temps. `npm install` i un `cargo build` en fred són del mateix ordre.
	//
	// Dos minuts cobreixen compilar i els tests d'un paquet. El que és
	// llarg de mena —la suite sencera, un servidor— continua sent feina de
	// bash_background, que no bloqueja el torn.
	DefaultTimeout = 120 * time.Second
)

// Read retorna línies numerades d'un fitxer (offset 1-based, limit 0 = tot).
func Read(path string, offset, limit int) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(raw), "\n")
	if offset < 1 {
		offset = 1
	}
	if offset > len(lines) {
		return "", fmt.Errorf("%s: només té %d línies", path, len(lines))
	}
	end := len(lines)
	if limit > 0 && offset-1+limit < end {
		end = offset - 1 + limit
	}
	var b strings.Builder
	for i := offset - 1; i < end; i++ {
		fmt.Fprintf(&b, "%d|%s\n", i+1, lines[i])
	}
	return truncate(b.String(), MaxOutputChars), nil
}

// Write escriu contingut sencer (crea el directori pare). Retorna bytes.
func Write(path string, content []byte) (int, error) {
	Active.SnapOp(path, "write")
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return 0, err
		}
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return 0, err
	}
	return len(content), nil
}

// Edit substitueix un bloc exacte; exigeix UNA sola ocurrència.
//
// Els finals de línia no compten. Un fitxer sortit d'un git amb
// core.autocrlf a Windows —aquest repositori mateix— té \r\n, i el model
// escriu el bloc amb \n, que és el que fa tothom. La comparació exacta
// fallava sempre i el que rebia era «bloc no trobat»: ni una pista, així
// que el que feia era tornar-hi igual o reescriure el fitxer sencer.
// Quan cal el segon intent, el fitxer es desa amb els finals de línia que
// tenia: passar-lo a \n faria un diff de totes les línies.
func Edit(path, old, new string) error {
	Active.SnapOp(path, "edit")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(raw)
	switch n := strings.Count(text, old); {
	case n == 1:
		return os.WriteFile(path, []byte(strings.Replace(text, old, new, 1)), 0o644)
	case n > 1:
		return fmt.Errorf("bloc ambigu a %s (%d ocurrències): posa-hi més context perquè n'hi hagi una de sola", path, n)
	}
	pla, oldPla, newPla := senseCR(text), senseCR(old), senseCR(new)
	switch n := strings.Count(pla, oldPla); {
	case n == 1:
		nou := strings.Replace(pla, oldPla, newPla, 1)
		if strings.Contains(text, "\r\n") {
			nou = strings.ReplaceAll(nou, "\n", "\r\n")
		}
		return os.WriteFile(path, []byte(nou), 0o644)
	case n > 1:
		return fmt.Errorf("bloc ambigu a %s (%d ocurrències): posa-hi més context perquè n'hi hagi una de sola", path, n)
	}
	return fmt.Errorf("bloc no trobat a %s: %s", path, perQueNoHiEs(pla, oldPla))
}

// senseCR normalitza els finals de línia per comparar.
func senseCR(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// perQueNoHiEs mira per què ha fallat la cerca i ho diu. «bloc no trobat» a
// seques no deixa saber si el text no hi és, si hi és amb un altre sagnat o
// si te l'has inventat, i cadascun es resol d'una manera diferent.
func perQueNoHiEs(text, old string) string {
	retalla := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	if strings.Contains(retalla(text), retalla(old)) {
		return "el text hi és però amb uns altres espais o sagnat — torna a llegir el fitxer amb read i copia'l tal com surt"
	}
	// Si alguna línia prou llarga del bloc sí que hi és, el que balla és el
	// context del voltant; si no n'hi ha cap, el bloc no és d'aquest fitxer.
	// Es miren totes, i no només la més llarga: la més llarga pot ser
	// justament la que t'has inventat.
	for _, l := range strings.Split(old, "\n") {
		if l = strings.TrimSpace(l); len(l) >= 8 && strings.Contains(text, l) {
			return "la línia " + strconv.Quote(escurça(l, 60)) + " hi és, però el bloc de context que l'envolta no — torna a llegir el fitxer amb read"
		}
	}
	return "no hi ha res semblant en aquest fitxer — comprova que sigui el fitxer bo i llegeix-lo amb read abans d'editar"
}

func escurça(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// Grep cerca pattern (substring literal) línia a línia sota dir.
// include filtra per glob de nom de fitxer (p. ex. "*.go"; buit = tot).
// Omet .git, binaris i dirs pesants. Topall de 50 coincidències.
func Grep(pattern, dir, include string, maxMatches int) (string, error) {
	if strings.TrimSpace(pattern) == "" {
		return "", fmt.Errorf("grep: pattern buit")
	}
	if dir == "" {
		dir = "."
	}
	if maxMatches <= 0 || maxMatches > 200 {
		maxMatches = 50
	}
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // continua amb la resta
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "dist" || base == "target" {
				return filepath.SkipDir
			}
			return nil
		}
		if include != "" {
			if ok, _ := filepath.Match(include, d.Name()); !ok {
				return nil
			}
		}
		raw, err := os.ReadFile(path)
		if err != nil || bytes.IndexByte(raw, 0) >= 0 {
			return nil // il·legible o binari
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, pattern) {
				out = append(out, fmt.Sprintf("%s:%d:%s", path, i+1, strings.TrimSpace(line)))
				if len(out) >= maxMatches {
					return errStop
				}
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStop) {
		return "", err
	}
	if len(out) == 0 {
		return "sense coincidències", nil
	}
	return strings.Join(out, "\n"), nil
}

var errStop = errors.New("topall")

// Glob llista fitxers sota dir que casen amb pattern. Suporta ** recursiu
// (p. ex. **/*.go, **/go.mod, internal/**/*.go), com esperen els agents.
func Glob(pattern, dir string, max int) (string, error) {
	if strings.TrimSpace(pattern) == "" {
		return "", fmt.Errorf("glob: pattern buit")
	}
	if dir == "" {
		dir = "."
	}
	if max <= 0 || max > 500 {
		max = 100
	}
	var matches []string
	if !strings.Contains(pattern, "**") {
		var err error
		matches, err = filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return "", err
		}
	} else {
		rx, err := doublestarRegexp(filepath.ToSlash(pattern))
		if err != nil {
			return "", err
		}
		err = filepath.WalkDir(dir, func(full string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if d.IsDir() {
				switch d.Name() {
				case ".git", "node_modules", "dist", "target":
					if full != dir {
						return filepath.SkipDir
					}
				}
				return nil
			}
			rel, err := filepath.Rel(dir, full)
			if err == nil && rx.MatchString(filepath.ToSlash(rel)) {
				matches = append(matches, full)
				if len(matches) >= max {
					return errStop
				}
			}
			return nil
		})
		if err != nil && !errors.Is(err, errStop) {
			return "", err
		}
	}
	if len(matches) > max {
		matches = matches[:max]
	}
	if len(matches) == 0 {
		return "sense coincidències", nil
	}
	return strings.Join(matches, "\n"), nil
}

// doublestarRegexp converteix un glob amb separadors / a regexp.
// **/ també casa zero directoris, de manera que **/go.mod troba ./go.mod.
func doublestarRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i += 2
				if i < len(pattern) && pattern[i] == '/' {
					b.WriteString("(?:.*/)?")
					i++
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
				i++
			}
		case '?':
			b.WriteString("[^/]")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
			i++
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
func PreviewEdit(old, new string) string {
	return "--- vell\n" + old + "\n+++ nou\n" + new
}

// prefixosSegurs són ordres de només lectura (o compilació/prova del
// projecte, que és la feina de l'agent i es desfà amb git). Van amb el
// mateix criteri que abans: coincidència exacta o prefix.
var prefixosSegurs = []string{
	"ls", "cat ", "head ", "tail ", "wc ", "file ", "stat ", "pwd",
	"echo ", "du ", "df ", "git status", "git diff", "git log", "git show ",
	"git branch", "go version", "gofmt ", "grep ", "rg ", "find ",
	// Compilar, provar i inspeccionar el projecte: sense això l'agent no
	// pot tancar cap cicle (escriu → comprova → arregla) sense un popup
	// per passa. `go run`/`python3`/`node` executen codi del projecte,
	// que és el que ja fan `go test` i els scripts de npm/gradle.
	"go build", "go test", "go vet", "go run ", "go list ",
	"python3 ", "node ", "npm run ", "npm test ", "npm ls ",
	"./gradlew ", "gradle ",
	// Informació del sistema: només mira. `date -s` i `hostname nou` sí
	// que canvien coses, però tots dos demanen root i `sudo` és a la
	// llista de denegats, així que l'agent (que corre com l'usuari) no
	// hi pot arribar. Sense `date`, el mode xat no podia ni saber quin
	// dia és i responia de memòria.
	"date", "uname", "hostname", "whoami", "id", "uptime", "free",
	"nproc", "lsblk", "locale", "lsb_release", "which ",
	"sha256sum ", "md5sum ",
	// Contenidors i serveis, només lectura.
	"docker ps", "docker images",
	"systemctl status", "systemctl is-active", "systemctl is-enabled",
	// Crear directoris: reversible i sense contingut.
	"mkdir ",
	// Windows segures (dir/type com ls/cat)
	"dir", "type ", "where ", "tasklist",
}

// prefixSegur diu si una ordre simple (sense operadors de shell) és de
// la llista segura.
func prefixSegur(lower string, extraAllow []string) bool {
	for _, p := range append(prefixosSegurs, extraAllow...) {
		if lower == strings.TrimSpace(p) || strings.HasPrefix(lower, p) {
			// "ls"/"dir" sols també valen; evita "less"/"directory".
			if (p == "ls" || p == "dir") && !(lower == strings.TrimSpace(p) || strings.HasPrefix(lower, p+" ")) {
				continue
			}
			return true
		}
	}
	return false
}

// cometesTancades rebutja ordres amb cometes sense tancar: partir-les per
// operadors podria canviar-ne el sentit (un `&&` dins de cometes no separa).
func cometesTancades(s string) bool {
	var simple, doble bool
	esc := false
	for _, r := range s {
		if esc {
			esc = false
			continue
		}
		switch r {
		case '\\':
			esc = true
		case '\'':
			if !doble {
				simple = !simple
			}
		case '"':
			if !simple {
				doble = !doble
			}
		}
	}
	return !simple && !doble
}

// senseRedirSegura treu les redireccions que no escriuen enlloc: fondre
// stderr a stdout (`2>&1`, `>&2`) o llençar (`>/dev/null`). La resta de
// `>`/`<` escriu o llegeix fitxers i continua demanant permís.
func senseRedirSegura(s string) string {
	for _, r := range []string{"2>&1", "1>/dev/null", "2>/dev/null", ">/dev/null", ">&2"} {
		s = strings.ReplaceAll(s, r, " ")
	}
	return s
}

// etapaSegura classifica un tros sense encadenaments ni canonades.
func etapaSegura(etapa string, extraAllow []string) (string, string) {
	e := strings.TrimSpace(senseRedirSegura(strings.ToLower(strings.TrimSpace(etapa))))
	if e == "" {
		return "ask", "etapa buida"
	}
	// `cd` sol no destrueix res: el que vingui després es classifica a part.
	if e == "cd" || strings.HasPrefix(e, "cd ") {
		return "allow", ""
	}
	if strings.ContainsAny(e, "><&\r\n`") || strings.Contains(e, "$(") {
		if strings.ContainsAny(e, "\r\n`") || strings.Contains(e, "$(") {
			return "ask", "operador de shell: confirma"
		}
		if strings.Contains(e, "&") {
			return "ask", "operador &: confirma"
		}
		return "ask", "redirecció a fitxer: confirma"
	}
	if prefixSegur(e, extraAllow) {
		return "allow", ""
	}
	return "ask", "fora de la llista segura"
}

// composta classifica cadenes (`&&`, `||`, `;`) i canonades (`|`): passen
// soles si TOTES les etapes són segures. Això cobreix el dia a dia
// (`cd dir && go build`, `go test ./... 2>&1 | tail -20`) sense obrir la
// porta a res nou: cada etapa passa pel mateix sedàs que una ordre sola,
// i les negacions denegades es comproven abans, sobre la comanda sencera.
func composta(cmd string, extraAllow []string) (string, string) {
	if !cometesTancades(cmd) {
		return "ask", "operador de shell: confirma"
	}
	parts := []string{cmd}
	for _, sep := range []string{"&&", "||", ";"} {
		var trossos []string
		for _, p := range parts {
			trossos = append(trossos, strings.Split(p, sep)...)
		}
		parts = trossos
	}
	for _, p := range parts {
		for _, etapa := range strings.Split(p, "|") {
			if d, rao := etapaSegura(etapa, extraAllow); d != "allow" {
				if len(parts) > 1 || strings.Contains(p, "|") {
					return "ask", "«" + strings.TrimSpace(etapa) + "»: " + rao
				}
				return d, rao
			}
		}
	}
	return "allow", ""
}

// ReplaceLines substitueix les línies start..end (1-based, inclusives) per repl.
func ReplaceLines(content string, start, end int, repl []string) (string, error) {
	lines := strings.Split(content, "\n")
	// Traiem l'últim buit fantasma del split si el fitxer acaba en \n.
	trailing := false
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
		trailing = true
	}
	if start < 1 || end < start || end > len(lines) {
		return "", fmt.Errorf("rang %d-%d fora de %d línies", start, end, len(lines))
	}
	out := append(append([]string{}, lines[:start-1]...), repl...)
	out = append(out, lines[end:]...)
	res := strings.Join(out, "\n")
	if trailing {
		res += "\n"
	}
	return res, nil
}

// caminsCritics són els arbres que l'agent no pot espatllar: les còpies, les
// dades vives de Nextcloud, el sistema i la seva pròpia configuració. Això
// últim no és un detall: un agent que es pot reescriure les regles de
// permisos ja no és un agent de confiança.
var caminsCritics = []string{
	"/mnt/backups", "/mnt/backups2", "/srv/storage",
	"/etc", "/boot", "/usr", "/bin", "/sbin", "/lib", "/var/lib",
	"~/.ssh", "~/.gnupg", "~/.config/gregal", "~/.hermes",
}

// verbsEscriptura són els verbs que poden fer mal si apunten a un camí
// crític. Els de només llegir (cat, ls, grep, rsync…) no hi són a posta.
var verbsEscriptura = []string{
	"rm ", "mv ", "truncate", "shred", "dd ", "mkfs", "mkswap",
	"chmod ", "chown ", "cp ", "ln ", "install ", "tee ", "sed -i",
}

// camiCriticA retorna el camí crític que apareix a la comanda (ja en
// minúscules) com a mot sencer, o "" si no n'hi ha cap.
func camiCriticA(lower string) string {
	for _, c := range caminsCritics {
		if conteCami(lower, c) {
			return c
		}
	}
	return ""
}

// conteCami diu si lower conté cami delimitat: al davant, començament o un
// separador (mai un caràcter de camí, perquè `./bin` no és `/bin`); al
// darrere, final, separador o `/` (així `/mnt/backups` atrapa també
// `/mnt/backups/current`, i `/etc/hosts` compta com a `/etc`).
func conteCami(lower, cami string) bool {
	const davant = "./-_~abcdefghijklmnopqrstuvwxyz0123456789$"
	const darrere = "/ \t\"';&|<>()`$=:"
	for i := 0; i+len(cami) <= len(lower); {
		j := strings.Index(lower[i:], cami)
		if j < 0 {
			return false
		}
		pos := i + j
		end := pos + len(cami)
		okDavant := pos == 0 || !strings.ContainsRune(davant, rune(lower[pos-1]))
		okDarrere := end == len(lower) || strings.ContainsRune(darrere, rune(lower[end]))
		if okDavant && okDarrere {
			return true
		}
		i = end
	}
	return false
}

// escripturaSobreCritic diu si la comanda escriu o destrueix dins d'algun
// camí crític. Només mira què ve DESPRÉS del verb o de la redirecció: un
// `cat /etc/hosts > /tmp/x` llegeix /etc i no hi escriu, i ha de passar.
// Si una ordre barreja lectura i escriptura, mana la part d'escriptura.
func escripturaSobreCritic(lower string) bool {
	base := senseRedirSegura(lower)
	// Redirecció que escriu: el destí és el que ve després del «>».
	if i := strings.Index(base, ">"); i >= 0 && camiCriticA(base[i:]) != "" {
		return true
	}
	// `find … -delete` porta el camí abans del verb: mira tota l'ordre.
	if strings.Contains(base, "-delete") && camiCriticA(base) != "" {
		return true
	}
	for _, v := range verbsEscriptura {
		for i := 0; ; {
			j := strings.Index(base[i:], v)
			if j < 0 {
				break
			}
			fi := i + j + len(v)
			if camiCriticA(base[fi:]) != "" {
				return true
			}
			i = fi
		}
	}
	return false
}

// Classifica una comanda: allow (lectura segura), deny (perillosa), ask (resta).
func Classify(cmd string) (string, string) {
	return ClassifyWith(cmd, nil, nil)
}

// ClassifyWith afegeix prefixos allow i subcadenes deny del config.
func ClassifyWith(cmd string, extraAllow, extraDeny []string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	// `rm -rf /` (l'arrel, sola o amb més trossos) no té marxa enrere: es
	// denega. `rm -rf /tmp/x` o `rm -rf ./build` són esborrats acotats i
	// passen a demanar permís com qualsevol altra escriptura.
	if i := strings.Index(lower, "rm -rf /"); i >= 0 {
		rest := lower[i+len("rm -rf /"):]
		if rest == "" || strings.ContainsAny(rest[:1], " 	;&|\r\n") {
			return "deny", "patró bloquejat: rm -rf /"
		}
	}
	// La llista dura té dues famílies: PATRONS prohibits (subcadenes que mai
	// no s'executen, perquè fan mal directament: sudo, mkfs, curl…) i CAMINS
	// crítics (on no s'hi escriu ni s'hi esborra — vegeu caminsCritics). Cap
	// mode ni cap override del config les relaxa.
	deny := append([]string{
		"sudo", "mkfs", "dd if=", "shutdown", "reboot",
		":(){", "/dev/sd", "curl", "wget", "ssh ", "chmod 777", "mkswap",
		// Windows perilloses (també bloquegen al TUI/serve de Windows)
		"format ", "del /f", "del /s", "rmdir /s", "rd /s", "remove-item",
		"diskpart", "cipher /w", "takeown", "icacls ",
	}, extraDeny...)
	for _, d := range deny {
		if strings.Contains(lower, strings.ToLower(d)) {
			return "deny", "patró bloquejat: " + d
		}
	}
	// Camins crítics: val en tots els modes. Llegir-los (cat, ls, grep, rsync
	// en sec…) és legítim i sovint necessari; escriure-hi o esborrar-hi, mai.
	if c := camiCriticA(lower); c != "" && escripturaSobreCritic(lower) {
		return "deny", "camí crític protegit: no s'hi escriu ni s'hi esborra (" + c + ")"
	}
	// Els prefixes de lectura només són segurs si la shell executa una única
	// ordre. Redireccions, canonades i encadenaments poden convertir `echo`,
	// `cat` o `git status` en una escriptura: composta() parteix la comanda
	// i exigeix que TOTES les etapes siguin segures.
	return composta(cmd, extraAllow)
}

// shellCommand tria l'intèrpret amb internal/shell (sh; a Windows el sh del
// Git si hi és, si no cmd). Un sol criteri per a l'eina bash i per al
// terminal de processos llargs: abans cadascun feia la seva.
func shellCommand(cmd string) *exec.Cmd {
	name, args := shell.Argv(shell.NormalitzaPathsWindows(cmd))
	return exec.Command(name, args...)
}

// Bash executa una comanda al directori del procés.
func Bash(cmd string, timeout time.Duration) (string, error) { return BashIn("", cmd, timeout) }

// BashIn és el mateix dient ON s'executa. Amb dir buit, el directori del
// procés (TUI, headless, Telegram: només hi ha un projecte). El web hi
// passa el workspace de la sessió: sense això, una pestanya que havia
// canviat de projecte seguia executant les ordres a l'altre.
func BashIn(dir, cmd string, timeout time.Duration) (string, error) {
	return BashCtx(context.Background(), dir, cmd, timeout)
}

// BashCtx és BashIn amb context de l'usuari: cancel·lar-lo mata la
// comanda. Sense això, Esc durant un `go test` de dos minuts no aturava
// res —el procés seguia i el TUI esperava igual— i Ctrl+C tancava el
// programa sencer perquè no hi havia cap cancel·lació a què agafar-se.
func BashCtx(ctx context.Context, dir, cmd string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c := shellCommand(cmd)
	c.Dir = dir
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := procs.Run(ctx, c)
	text := truncate(out.String(), MaxOutputChars)
	if ctx.Err() == context.Canceled {
		return text, fmt.Errorf("aturada per l'usuari")
	}
	if ctx.Err() == context.DeadlineExceeded {
		// «timeout (30s)» sol no diu què fer, i el que fa el model és
		// tornar-hi igual i tornar-se a menjar els trenta segons. La
		// sortida d'abans del tall també hi va: sovint ja diu prou.
		return text, fmt.Errorf("timeout (%s): la comanda encara corria i s'ha tallat. No la repeteixis igual; si és llarga de mena (build, suite de tests, servidor) engega-la amb bash_background i llegeix-la amb bash_output", timeout)
	}
	if err != nil {
		return text, fmt.Errorf("exit: %v\n%s", err, text)
	}
	return text, nil
}

// GitBranch retorna la branca git de dir ("" si no és repo).
func GitBranch(dir string) string {
	out, err := gitOut(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	if b := strings.TrimSpace(out); b != "" && b != "HEAD" {
		return b
	}
	return ""
}

// GitDiff retorna l'estat i tots els canvis del repo: unstaged, staged i
// fitxers nous. Torna "" si no hi ha repo o no hi ha canvis.
func GitDiff(dir string) string {
	status, err := gitOut(dir, "status", "--short")
	if err != nil || strings.TrimSpace(status) == "" {
		return ""
	}

	stat, statErr := gitOut(dir, "diff", "HEAD", "--stat")
	diff, diffErr := gitOut(dir, "diff", "HEAD")
	if statErr != nil || diffErr != nil { // repo encara sense HEAD
		stagedStat, _ := gitOut(dir, "diff", "--cached", "--stat")
		workStat, _ := gitOut(dir, "diff", "--stat")
		stat = stagedStat + workStat
		stagedDiff, _ := gitOut(dir, "diff", "--cached")
		workDiff, _ := gitOut(dir, "diff")
		diff = stagedDiff + workDiff
	}

	var untracked strings.Builder
	rawNames, _ := gitOut(dir, "ls-files", "--others", "--exclude-standard", "-z")
	for _, rel := range strings.Split(rawNames, "\x00") {
		if rel == "" {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if readErr != nil {
			continue
		}
		untracked.WriteString("\n--- /dev/null\n+++ b/")
		untracked.WriteString(rel)
		if bytes.IndexByte(raw, 0) >= 0 {
			untracked.WriteString("\n(binary nou)\n")
			continue
		}
		untracked.WriteString("\n@@ fitxer nou @@\n")
		untracked.WriteString(truncate(string(raw), 2000))
		if len(raw) == 0 || raw[len(raw)-1] != '\n' {
			untracked.WriteByte('\n')
		}
	}

	body := status + "\n" + stat + "\n" + diff + untracked.String()
	return truncate("```diff\n"+body+"\n```", 8000)
}

func gitOut(dir string, args ...string) (string, error) {
	full := append([]string{"-C", dir}, args...)
	c := exec.Command("git", full...)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	if err := c.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncat)"
}
