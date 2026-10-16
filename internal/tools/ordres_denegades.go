package tools

import "strings"

// Ordres prohibides per programa, no per subcadena.
//
// Abans la llista dura es comprovava com a subcadena de tota la comanda, i
// denegava ordres de lectura innocents perquè un argument hi contenia la
// paraula: `git log --format '%h %s'` («format »), `grep -rn curl .`,
// `rg 'ssh ' docs`, `clang-format -i a.c`. Al model li arribava «patró
// bloquejat» sense cap altra sortida, i perdia passos buscant-ne una.
//
// Ara es parteix la comanda com ho faria la shell (cometes, escapades,
// operadors, substitucions) i només compta la paraula que fa d'ORDRE: la
// primera de cada etapa, la que va després d'un prefix (env, xargs,
// timeout, nohup…) o de -exec. On el codi ve dins d'una cadena (`bash -c`,
// `python -c`, `eval`, heretext), no es pot saber què és ordre i què no:
// allà es manté la comprovació per subcadena de sempre.
//
// curl i wget contra la pròpia màquina (localhost, 127.x, [::1]) ja no es
// deneguen: provar el servidor que s'està desenvolupant és del dia a dia.
// Passen a demanar permís com qualsevol ordre fora de la llista segura. A
// fora, o amb un proxy, o canalitzats cap a una shell, continuen denegats.

// programesDenegats: programa → arguments que el fan perillós ("" =
// sempre). mkfs també cobreix mkfs.ext4 i companyia.
var programesDenegats = []struct{ prog, arg, patro string }{
	{"sudo", "", "sudo"}, {"mkfs", "", "mkfs"}, {"dd", "if=", "dd if="},
	{"shutdown", "", "shutdown"}, {"reboot", "", "reboot"},
	{"curl", "", "curl"}, {"wget", "", "wget"}, {"ssh", "", "ssh "},
	{"chmod", "777", "chmod 777"}, {"mkswap", "", "mkswap"},
	// Windows perilloses (també bloquegen al TUI/serve de Windows)
	{"format", "", "format "}, {"del", "/f", "del /f"}, {"del", "/s", "del /s"},
	{"rmdir", "/s", "rmdir /s"}, {"rd", "/s", "rd /s"}, {"remove-item", "", "remove-item"},
	{"diskpart", "", "diskpart"}, {"cipher", "/w", "cipher /w"}, {"takeown", "", "takeown"},
	{"icacls", "", "icacls "},
}

// subcadenesDenegades no són programes: van a qualsevol lloc.
var subcadenesDenegades = []string{":(){", "/dev/sd"}

// prefixosOrdre deixen l'ordre de veritat més endavant a la mateixa etapa:
// totes les paraules que els segueixen es miren com si fossin ordres
// (`xargs -I X curl X`, `timeout 5 ssh h`, `env A=1 sudo x`).
var prefixosOrdre = map[string]bool{
	"env": true, "xargs": true, "nohup": true, "time": true, "exec": true,
	"command": true, "builtin": true, "nice": true, "ionice": true, "timeout": true,
	"watch": true, "stdbuf": true, "strace": true, "chroot": true, "doas": true,
	"start": true, "runas": true, "parallel": true, "-exec": true, "-execdir": true,
	"-ok": true, "-okdir": true, "then": true, "do": true, "else": true, "elif": true,
	"if": true, "while": true, "until": true, "!": true,
}

// interprets executen codi que els arriba en una cadena.
var interprets = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true, "fish": true,
	"cmd": true, "powershell": true, "pwsh": true, "python": true, "python3": true,
	"py": true, "node": true, "deno": true, "bun": true, "perl": true, "ruby": true,
	"php": true, "osascript": true, "su": true,
}

type paraulaOrdre struct {
	text   string
	ordre  bool // fa d'ordre (o pot fer-ne, després d'un prefix)
	etapa  int
	cometa bool // tenia cometes
}

// ordreDenegada torna el patró que bloqueja la comanda, o "" si no n'hi ha
// cap. Rep la comanda tal com és: les opcions de curl distingeixen
// majúscules.
func ordreDenegada(cmd string) string {
	lower := strings.ToLower(cmd)
	for _, s := range subcadenesDenegades {
		if strings.Contains(lower, s) {
			return s
		}
	}
	paraules, codiEnCadena := parteixOrdre(cmd)
	if codiEnCadena {
		for _, d := range programesDenegats {
			if strings.Contains(lower, d.patro) {
				return d.patro
			}
		}
		return ""
	}
	vars := variablesAssignades(paraules)
	xarxaLocal, shell := false, false
	for i, p := range paraules {
		if !p.ordre {
			continue
		}
		prog := nomPrograma(p.text)
		if interprets[prog] && llegeixCodiDeLEntrada(prog, argumentsEtapa(paraules, i)) {
			shell = true
		}
		for _, d := range programesDenegats {
			if prog != d.prog && !(d.prog == "mkfs" && strings.HasPrefix(prog, "mkfs.")) {
				continue
			}
			args := argumentsEtapa(paraules, i)
			if d.arg != "" && !conteArg(args, d.arg) {
				continue
			}
			if (prog == "curl" || prog == "wget") && !strings.Contains(lower, "proxy") && nomesLocal(prog, expandeixVars(args, vars)) {
				xarxaLocal = true
				continue
			}
			return d.patro
		}
	}
	// Baixar alguna cosa i passar-la a una shell no es fa mai, ni des de
	// la pròpia màquina.
	if xarxaLocal && shell {
		return "curl | sh"
	}
	return ""
}

// variablesAssignades són les assignacions que fan d'etapa sencera
// (`B=http://127.0.0.1:8080; curl $B/x`), que és com el model escriu un
// bucle de proves contra el seu servidor. Una assignació que fa de prefix
// d'una ordre (`B=x curl $B`) no compta: la shell expandeix $B abans.
func variablesAssignades(paraules []paraulaOrdre) map[string]string {
	vars := map[string]string{}
	for i, p := range paraules {
		if !p.ordre || !esAssignacio(strings.ToLower(p.text)) {
			continue
		}
		if i+1 < len(paraules) && paraules[i+1].etapa == p.etapa {
			continue
		}
		nom, valor, _ := strings.Cut(p.text, "=")
		vars[nom] = valor
	}
	return vars
}

// expandeixVars substitueix $NOM i ${NOM} per les variables conegudes. Les
// que no ho són es queden tal qual (i llavors no semblen una adreça local).
func expandeixVars(args []paraulaOrdre, vars map[string]string) []paraulaOrdre {
	if len(vars) == 0 {
		return args
	}
	out := make([]paraulaOrdre, len(args))
	for i, a := range args {
		out[i] = a
		for nom, valor := range vars {
			out[i].text = strings.ReplaceAll(out[i].text, "${"+nom+"}", valor)
			if j := strings.Index(out[i].text, "$"+nom); j >= 0 {
				fi := j + 1 + len(nom)
				if fi == len(out[i].text) || !esCaracterNom(out[i].text[fi]) {
					out[i].text = out[i].text[:j] + valor + out[i].text[fi:]
				}
			}
		}
	}
	return out
}

func esCaracterNom(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// nomPrograma és el nom d'un camí d'executable: /usr/bin/curl, curl.exe i
// C:\Windows\System32\curl.exe són curl.
func nomPrograma(s string) string {
	s = strings.ToLower(s[strings.LastIndexAny(s, `/\`)+1:])
	return strings.TrimSuffix(s, ".exe")
}

// llegeixCodiDeLEntrada: una shell sempre; un altre intèrpret només si no
// té cap script ni mòdul a executar (`python`, `python -`), perquè
// `curl … | python -m json.tool` només formata la resposta.
func llegeixCodiDeLEntrada(prog string, args []paraulaOrdre) bool {
	switch prog {
	case "sh", "bash", "zsh", "dash", "ksh", "fish", "cmd", "powershell", "pwsh", "su":
		return true
	}
	for _, a := range args {
		if a.text != "-" && !strings.HasPrefix(a.text, "-") {
			return false
		}
		if a.text == "-m" {
			return false
		}
	}
	return true
}

func argumentsEtapa(paraules []paraulaOrdre, i int) []paraulaOrdre {
	var args []paraulaOrdre
	for _, p := range paraules[i+1:] {
		if p.etapa != paraules[i].etapa {
			break
		}
		args = append(args, p)
	}
	return args
}

func conteArg(args []paraulaOrdre, arg string) bool {
	for _, a := range args {
		if t := strings.ToLower(a.text); strings.HasPrefix(t, arg) {
			return true
		}
	}
	return false
}

// parteixOrdre fa de lexer de shell. Torna les paraules amb l'etapa on són
// i si fan d'ordre, i codiEnCadena quan hi ha codi que no es pot veure
// (heretext, eval, un intèrpret amb -c/-e, $'…').
func parteixOrdre(s string) (paraules []paraulaOrdre, codiEnCadena bool) {
	if strings.Contains(s, "<<") || strings.Contains(s, "$'") {
		return nil, true
	}
	var (
		cur      strings.Builder
		enCurs   bool
		cometa   bool
		etapa    int
		esOrdre  = true // la paraula que comença ara fa d'ordre
		prefix   bool   // l'etapa porta un prefix: tot pot ser ordre
		progEtap string
	)
	tanca := func() {
		if !enCurs {
			return
		}
		w := cur.String()
		ordre := esOrdre || prefix
		paraules = append(paraules, paraulaOrdre{text: w, ordre: ordre, etapa: etapa, cometa: cometa})
		nom := nomPrograma(w)
		if esOrdre {
			progEtap = nom
		}
		lw := strings.ToLower(w)
		switch {
		case prefixosOrdre[lw] || prefixosOrdre[nom]:
			prefix = true
		case (esOrdre || prefix) && (nom == "eval" || nom == "source" || nom == "." || nom == "alias"):
			codiEnCadena = true
		case ordre && !esAssignacio(lw) && !nomNet(w):
			// Una ordre que la shell encara ha de transformar ($X, ${IFS},
			// {a,b}, comodins, ~): no se sap quin programa serà.
			codiEnCadena = true
		case interprets[progEtap] && !esOrdre && strings.HasPrefix(lw, "-") && strings.ContainsAny(lw, "ce"):
			// bash -c, python -c, node -e, perl -e, pwsh -command, cmd /c…
			codiEnCadena = true
		case interprets[progEtap] && !esOrdre && (lw == "/c" || lw == "/k"):
			codiEnCadena = true
		}
		// Una assignació (A=1 ordre) deixa l'ordre per a la paraula següent.
		if !(esOrdre && esAssignacio(lw)) {
			esOrdre = false
		}
		cur.Reset()
		enCurs, cometa = false, false
	}
	novaEtapa := func() {
		tanca()
		etapa++
		esOrdre, prefix, progEtap = true, false, ""
	}
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		c := r[i]
		switch {
		case c == '\\' && i+1 < len(r) && (r[i+1] == '\n' || r[i+1] == '\r'):
			// Continuació de línia: la shell l'esborra (cu\⏎rl és curl).
			i++
			if r[i] == '\r' && i+1 < len(r) && r[i+1] == '\n' {
				i++
			}
		case c == '\\' && i+1 < len(r):
			i++
			cur.WriteRune(r[i])
			enCurs = true
		case c == '\'':
			j := i + 1
			for j < len(r) && r[j] != '\'' {
				j++
			}
			cur.WriteString(string(r[i+1 : min(j, len(r))]))
			enCurs, cometa = true, true
			i = j
		case c == '"':
			// Dins de cometes dobles hi ha substitucions ($( ) i ` `), que
			// s'executen: allà es comença una etapa nova.
			j := i + 1
			for ; j < len(r) && r[j] != '"'; j++ {
				if r[j] == '\\' && j+1 < len(r) && strings.ContainsRune("$`\"\\\n", r[j+1]) {
					j++
					cur.WriteRune(r[j])
					continue
				}
				if r[j] == '`' || (r[j] == '$' && j+1 < len(r) && r[j+1] == '(') {
					return nil, true
				}
				cur.WriteRune(r[j])
			}
			enCurs, cometa = true, true
			i = j
		case c == ' ' || c == '\t':
			tanca()
		case c == '$' && i+1 < len(r) && r[i+1] == '{':
			// ${NOM} és part de la paraula, no un bloc { }.
			j := i + 2
			for j < len(r) && r[j] != '}' {
				j++
			}
			dins := string(r[i:min(j+1, len(r))])
			if strings.Contains(dins, "$(") || strings.Contains(dins, "`") {
				return nil, true // ${A:-$(ordre)} executa l'ordre
			}
			cur.WriteString(dins)
			enCurs = true
			i = j
		case c == '$' && i+1 < len(r) && r[i+1] == '(':
			i++
			novaEtapa()
		case strings.ContainsRune("|&;()`{}\r\n", c):
			novaEtapa()
		default:
			cur.WriteRune(c)
			enCurs = true
		}
	}
	tanca()
	return paraules, codiEnCadena
}

// nomNet: un nom o camí d'executable tal qual, sense res que la shell hagi
// d'expandir.
func nomNet(w string) bool {
	for _, c := range w {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune(`._-+/\:@%`, c)) {
			return false
		}
	}
	return true
}

func esAssignacio(w string) bool {
	i := strings.IndexByte(w, '=')
	if i <= 0 {
		return false
	}
	for _, c := range w[:i] {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// Opcions de curl i wget que porten un valor que no és una adreça, i les
// que fan sortir la petició cap a una altra màquina (proxy, resolució
// forçada) o llegeixen opcions o adreces d'un fitxer: amb aquestes no es
// pot dir que la petició és local. Distingeixen majúscules: a curl, -X és
// el mètode i -x el proxy.
var (
	curlAmbValor = paraulesSet("-H --header -d --data --data-raw --data-binary --data-urlencode --json -X --request -o --output -w --write-out -m --max-time --connect-timeout -u --user -A --user-agent -b --cookie -c --cookie-jar -F --form -T --upload-file --retry -r --range -e --referer -z --time-cond -D --dump-header")
	curlSurt     = paraulesSet("-x --proxy --preproxy --socks4 --socks4a --socks5 --socks5-hostname --connect-to --resolve -K --config --doh-url -: --next")
	wgetAmbValor = paraulesSet("-O --output-document -o --output-file -a --append-output --header --post-data --post-file --body-data --method -T --timeout -t --tries --user --password -U --user-agent -P --directory-prefix")
	wgetSurt     = paraulesSet("-e --execute -i --input-file --config -B --base")
)

func paraulesSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// nomesLocal diu si un curl/wget només parla amb la pròpia màquina: hi ha
// almenys una adreça, totes són loopback i cap opció la desvia.
func nomesLocal(prog string, args []paraulaOrdre) bool {
	ambValor, surt := curlAmbValor, curlSurt
	if prog == "wget" {
		ambValor, surt = wgetAmbValor, wgetSurt
	}
	adreces := 0
	for i := 0; i < len(args); i++ {
		a := args[i].text
		// Redireccions: `> out.json`, `2>/dev/null`.
		if r := strings.TrimLeft(a, "0123456789"); strings.HasPrefix(r, ">") || strings.HasPrefix(r, "<") {
			if strings.Trim(r, "<>") == "" {
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 && !args[i].cometa {
			nom, valor, ambIgual := strings.Cut(a, "=")
			switch {
			case surt[nom]:
				return false
			case nom == "--url":
				if !ambIgual {
					if i++; i >= len(args) {
						return false
					}
					valor = args[i].text
				}
				if !adrecaLocal(valor) {
					return false
				}
				adreces++
			case ambValor[nom] && !ambIgual:
				i++
			}
			continue
		}
		if !adrecaLocal(a) {
			return false
		}
		adreces++
	}
	return adreces > 0
}

// adrecaLocal: http(s)://localhost:8080/x, 127.0.0.1/x, [::1]:3000…
func adrecaLocal(a string) bool {
	a = strings.ToLower(a)
	if i := strings.Index(a, "://"); i >= 0 {
		esquema := a[:i]
		if esquema != "http" && esquema != "https" {
			return false
		}
		a = a[i+3:]
	}
	if i := strings.IndexAny(a, "/?#"); i >= 0 {
		a = a[:i]
	}
	if i := strings.LastIndexByte(a, '@'); i >= 0 {
		a = a[i+1:]
	}
	host := a
	if strings.HasPrefix(host, "[") {
		if j := strings.IndexByte(host, ']'); j > 0 {
			host = host[1:j]
		}
	} else if i := strings.LastIndexByte(host, ':'); i >= 0 && strings.Count(host, ":") == 1 {
		host = host[:i]
	}
	switch {
	case host == "localhost" || strings.HasSuffix(host, ".localhost"):
		return true
	case host == "::1" || host == "0.0.0.0":
		return true
	case strings.HasPrefix(host, "127."):
		for _, c := range host {
			if !(c == '.' || c >= '0' && c <= '9') {
				return false
			}
		}
		return strings.Count(host, ".") == 3
	}
	return false
}
