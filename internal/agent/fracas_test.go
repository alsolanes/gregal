package agent

import (
	"strings"
	"testing"

	"gregal/internal/llm"
	"gregal/internal/tools"
)

// La tirada perduda del 2026-09-28 (una tasca trivial, 146 passos, 50 min)
// va viure del bucle «corre el test, retoqueja, corre el test»: cada pas
// executava eines noves (l'antic criteri d'avenç sempre concedia
// ampliacions) però cap comprovació passava mai. El DoomTracker no la
// caçava perquè cada comanda variava una mica; aquests tests claven les
// dents que l'hi han de tallar: guia als tres fracassos seguits, zero amb
// una bash verda, edicions que no trenquen la ratxa, ni ampliació ni
// síntesi curta mentre duri, i topall interactiu d'ampliacions.

// finsAExecuta consumeix la cua de permisos del pas, alimenta el resultat
// donat i torna els events que ha emès el motor: no hi ha E/S, el motor
// només rep el que li diguem.
func finsAExecuta(t *testing.T, tn *Torn, res []Execucio) []Event {
	t.Helper()
	for i := 0; i < 10; i++ {
		p := tn.Seguent()
		if p.Ordre == OrdreAprova {
			tn.RepAprovacio(true)
			continue
		}
		if p.Ordre != OrdreExecuta {
			t.Fatalf("esperava execució, no %d", p.Ordre)
		}
		return tn.RepExecucions(res)
	}
	t.Fatal("deu permisos seguits és un bucle de la política")
	return nil
}

// bashFallit simula el que fa el client quan una ordre peteja: la sortida
// arriba amb el prefix ERROR: i el vermell de go test a sota. La comanda
// varia a cada crida per no topar amb el DoomTracker de signatures, que
// aquí no és el que es prova (i que és, de fet, el que l'incident
// esquivava: cada comanda «nova» comptava com a eina «nova»).
func bashFallit(nom string) Execucio {
	return Execucio{
		Call:    cridaProva("b-"+nom, "bash", `{"command":"go test ./`+nom+`"}`),
		Sortida: "ERROR: exit: exit status 1\n--- FAIL: Test" + nom + " (0.38s)\nFAIL\tgregal/" + nom,
	}
}

func pasBash(nom string) []llm.ToolCall {
	return []llm.ToolCall{cridaProva("c-"+nom, "bash", `{"command":"go test ./`+nom+`"}`)}
}

// Dos fracassos seguits no han de moure res; al tercer, la guia.
func TestFracasGuiaAlTercer(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 20)
	tn.Seguent()

	for i, nom := range []string{"alpha", "beta", "gamma"} {
		tn.RepPas("", pasBash(nom), nil)
		evs := finsAExecuta(t, tn, []Execucio{bashFallit(nom)})
		hist := strings.Join(histTexts(tn), "\n")
		if i < 2 && strings.Contains(hist, "COMPROVACIÓ VERMELLA") {
			t.Fatalf("amb %s encara no hi ha guia", nom)
		}
		if i == 2 {
			if !strings.Contains(hist, "COMPROVACIÓ VERMELLA REPETIDA") {
				t.Fatal("al tercer fracàs la guia hi ha de ser a l'historial")
			}
			if !hiHaAvis(evs, "3 comprovacions vermelles seguides") {
				t.Fatal("i un avís curt per pintar (la guia llarga és per al model)")
			}
		}
	}
}

// Una bash verda (una comprovació que passa) torna el comptador a zero:
// el flux legítim «vermell → arregla → verd» no s'ha de castigar mai.
func TestFracasVerdTornaAZero(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 20)
	tn.Seguent()

	tn.RepPas("", pasBash("alpha"), nil)
	finsAExecuta(t, tn, []Execucio{bashFallit("alpha")})
	tn.RepPas("", pasBash("beta"), nil)
	finsAExecuta(t, tn, []Execucio{bashFallit("beta")})

	// Verd: el test ara passa.
	tn.RepPas("", pasBash("gamma"), nil)
	verd := Execucio{Call: cridaProva("b-verd", "bash", `{"command":"go test ./gamma"}`), Sortida: "ok\tgregal/gamma\t1.2s"}
	finsAExecuta(t, tn, []Execucio{verd})

	// Dos fracassos més no han de disparar res: el comptador ha començat de zero.
	tn.RepPas("", pasBash("delta"), nil)
	finsAExecuta(t, tn, []Execucio{bashFallit("delta")})
	tn.RepPas("", pasBash("epsilon"), nil)
	finsAExecuta(t, tn, []Execucio{bashFallit("epsilon")})

	if strings.Contains(strings.Join(histTexts(tn), "\n"), "COMPROVACIÓ VERMELLA") {
		t.Fatal("amb el verd al mig no hi ha d'haver guia")
	}
}

// Una edició que triomfa entre dos vermells NO trenca la ratxa: és
// exactament el bucle de l'incident (edita → test vermell → edita → test
// vermell) i és el que el comptador d'eines noves no sabia veure.
func TestEdicionsNoTrenquenLaRatxa(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 20)
	tn.Seguent()

	for _, nom := range []string{"alpha", "beta", "gamma"} {
		// L'edició triomfa: aquesta execució és «progrés» per al criteri
		// antic d'ampliacions, i precisament per això res no parava la
		// bogeria. Cap E/S real: el motor només rep el resultat fabricat.
		tn.RepPas("", []llm.ToolCall{cridaProva("w-"+nom, "write", `{"path":"`+nom+`.go","content":"package main"}`)}, nil)
		finsAExecuta(t, tn, []Execucio{{Call: cridaProva("w2-"+nom, "write", `{}`), Sortida: "escrit"}})

		tn.RepPas("", pasBash(nom), nil)
		finsAExecuta(t, tn, []Execucio{bashFallit(nom)})
	}
	if !strings.Contains(strings.Join(histTexts(tn), "\n"), "COMPROVACIÓ VERMELLA") {
		t.Fatal("els vermells amb edicions al mig segueixen sent una ratxa")
	}
}

// Amb la ratxa de vermelles viva, el pressupost no s'amplia: el torn va
// directe a la síntesi llarga, perquè l'usuari hi és i ha de poder dir si
// continua. Això és el que hauria tallat la tirada de 146 passos al minut
// vint en comptes del cinquanta.
func TestRatxaVermellaBloquejaAmpliacio(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 3)
	tn.Seguent()

	for i, nom := range []string{"alpha", "beta", "gamma"} {
		tn.RepPas("", pasBash(nom), nil)
		finsAExecuta(t, tn, []Execucio{bashFallit(nom)})
		if i < 2 {
			tn.Seguent() // pas següent del model (2 i 3)
		}
	}

	p := tn.Seguent()
	if p.Ordre != OrdreSintesi {
		t.Fatalf("amb la suite vermella no es demana ampliació: %d", p.Ordre)
	}
	if !strings.Contains(histUltim(p), "pressupost") {
		t.Fatalf("la síntesi d'una ratxa és la llarga (estat del verd i del vermell):\n%s", histUltim(p))
	}
	if !strings.Contains(histUltim(p), "comprovacions vermelles") {
		t.Fatalf("i ha de dir quantes comprovacions porten caigudes:\n%s", histUltim(p))
	}
}

// El topall d'ampliacions interactiu: tres trossos i a parar, amb síntesi
// honesta. Més enllà no hi ha quart tros encara que el model digui
// CONTINUA: toca parlar amb qui mira la pantalla.
func TestAmpliacioInteractivaTopall(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 1)
	tn.Seguent()

	pkgs := []string{"alpha", "beta", "gamma", "delta"}
	for n := 0; n < MaxAmpliacionsInteractiu+1; n++ {
		nom := pkgs[n]
		// Pas de model amb una bash verda: cap ratxa, cap doom — només
		// el comptador de trossos.
		tn.RepPas("", pasBash(nom), nil)
		verd := Execucio{Call: cridaProva("b-"+nom, "bash", `{"command":"ls ./`+nom+`"}`), Sortida: "ok"}
		finsAExecuta(t, tn, []Execucio{verd})

		p := tn.Seguent()
		if n < MaxAmpliacionsInteractiu {
			if p.Ordre != OrdreAmplia {
				t.Fatalf("ampliació %d: esperava la pregunta, no %d", n+1, p.Ordre)
			}
			tn.RepAmpliacio("CONTINUA 1", nil)
			tn.Seguent() // obre el pas següent per a la volta que ve
			continue
		}
		if p.Ordre != OrdreSintesi {
			t.Fatalf("més enllà del topall interactiu toca síntesi, no %d", p.Ordre)
		}
		if !strings.Contains(histUltim(p), "pressupost") {
			t.Fatalf("la síntesi del topall ha de ser la llarga:\n%s", histUltim(p))
		}
		return
	}
	t.Fatal("el bucle no hauria de passar del topall")
}

// El vermell de go test sense prefix ERROR (una canonada que enmascara
// l'exit) ha de comptar com a fracàs: abans «--- FAIL» era invisible per
// SemblaFracas i el model podia tancar dient «fet».
func TestVermellGoTestSenseExit(t *testing.T) {
	if !SemblaFracas("=== RUN   TestX\n--- FAIL: TestX (0.38s)\nFAIL\nFAIL\tgregal/internal/tui") {
		t.Fatal("un vermell de go test ha de semblar fracàs")
	}
	if !SemblaFracas("ERROR: exit: exit status 1") {
		t.Fatal("l'ERROR del client sempre és fracàs")
	}
	if SemblaFracas("ok\tgregal/internal/tui\t1.2s") {
		t.Fatal("un verd no és fracàs")
	}
	if SemblaFracas("el log parla d'un error d'entrada de l'usuari") {
		t.Fatal("un log que menciona errors no és fracàs de l'eina")
	}
}

// «Revisa i afegeix X» és una tasca de canvi: la tasca de l'incident no
// entrava a TascaDeReparacio i la guia de verificació no el podia parar
// quan tancava amb la suite vermella.
func TestAfegeixEsTascaDeReparacio(t *testing.T) {
	if !TascaDeReparacio("revisa i afegeix la carpeta de treball a la barra del tui") {
		t.Fatal("«afegeix» és un canvi i es verifica com a tal")
	}
	if TascaDeReparacio("per què falla aquest test?") {
		t.Fatal("una pregunta no és una reparació")
	}
}

// Explorar no és verificar: tres grep sense coincidències (exit 1, que el
// client presenta com a «ERROR: exit…») no són una ratxa vermella, i un
// sed verd entre dos tests vermells no la trenca.
func TestExploracioNoEsRatxa(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 20)
	tn.Seguent()
	for _, nom := range []string{"alpha", "beta", "gamma", "delta"} {
		tn.RepPas("", pasBash(nom), nil)
		grep := Execucio{Call: cridaProva("g-"+nom, "bash", `{"command":"grep -rn `+nom+` internal/"}`), Sortida: "ERROR: exit: exit status 1\n"}
		finsAExecuta(t, tn, []Execucio{grep})
	}
	if strings.Contains(strings.Join(histTexts(tn), "\n"), "COMPROVACIÓ VERMELLA") {
		t.Fatal("un grep buit no és una comprovació vermella")
	}

	tn = tornProva(ModeCode, 20)
	tn.Seguent()
	for _, nom := range []string{"alpha", "beta", "gamma"} {
		tn.RepPas("", pasBash(nom), nil)
		finsAExecuta(t, tn, []Execucio{bashFallit(nom)})
		tn.RepPas("", pasBash(nom), nil)
		sed := Execucio{Call: cridaProva("s-"+nom, "bash", `{"command":"sed -i 's/a/b/' `+nom+`.go"}`), Sortida: "fet"}
		finsAExecuta(t, tn, []Execucio{sed})
	}
	if !strings.Contains(strings.Join(histTexts(tn), "\n"), "COMPROVACIÓ VERMELLA") {
		t.Fatal("un sed verd entre tests vermells no és un test verd")
	}
}

func TestEsComprovacio(t *testing.T) {
	si := []string{
		"go test ./...", "cd internal && go vet ./...", "GOFLAGS=-count=1 go test ./x",
		"timeout 60s go build ./...", "npm test", "npm run build", "python -m pytest -q",
		"pytest tests/", "make", "cargo clippy", "./gradlew assembleDebug", "go test ./... 2>&1 | tail -20",
	}
	no := []string{
		"grep -rn test internal/", "ls internal", "sed -i 's/a/b/' x.go", "cat go.mod",
		"git status", "go mod tidy", "sh -c ls", "",
	}
	for _, c := range si {
		if !EsComprovacio(c) {
			t.Errorf("%q és una comprovació", c)
		}
	}
	for _, c := range no {
		if EsComprovacio(c) {
			t.Errorf("%q no és una comprovació", c)
		}
	}
	if SemblaFracas("agent\nfailover\nweb") {
		t.Error("un ls amb una carpeta failover/ no és un vermell")
	}
	if !SemblaFracas("--- ok\nFAIL") {
		t.Error("un FAIL final de go test és un vermell")
	}
}

// Nou vermelles seguides (tres guies ignorades) en interactiu tanquen el
// torn amb síntesi encara que quedin passos al tros base.
func TestRatxaLlargaForcaSintesi(t *testing.T) {
	tools.TodoClear()
	tn := tornProva(ModeCode, 40)
	tn.Seguent()
	// Noms sense xifres: el DoomTracker normalitza els números i «./a1»,
	// «./a2»… li semblarien la mateixa ordre.
	noms := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota"}
	for i, nom := range noms {
		tn.RepPas("", pasBash(nom), nil)
		finsAExecuta(t, tn, []Execucio{bashFallit(nom)})
		if i < len(noms)-1 {
			if p := tn.Seguent(); p.Ordre != OrdrePasModel {
				t.Fatalf("amb %d vermelles encara continua: %v", i+1, p.Ordre)
			}
		}
	}
	p := tn.Seguent()
	if p.Ordre != OrdreSintesi || !tn.Esgotat() {
		t.Fatalf("a la novena vermella seguida, síntesi forçada: %v %v", p.Ordre, tn.Esgotat())
	}
	if !strings.Contains(histUltim(p), "comprovacions vermelles") {
		t.Fatalf("i és la síntesi llarga:\n%s", histUltim(p))
	}
}

// --- helpers -----------------------------------------------------------

func histTexts(t *Torn) []string {
	var out []string
	for _, m := range t.Hist {
		out = append(out, m.Content)
	}
	return out
}

func histUltim(p Pas) string {
	if len(p.Hist) == 0 {
		return ""
	}
	return p.Hist[len(p.Hist)-1].Content
}

func hiHaAvis(evs []Event, text string) bool {
	for _, e := range evs {
		if e.Tipus == EvAvis && strings.Contains(e.Missatge(), text) {
			return true
		}
	}
	return false
}
