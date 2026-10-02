package config

import "testing"

func routerCfg() *Config {
	return &Config{
		Router: RouterCfg{Mode: "auto"},
		Roles: map[string]Role{
			"chat": {Provider: "p", Model: "barat"},
			"code": {Provider: "p", Model: "fort"},
		},
	}
}

func TestRoutePregunta(t *testing.T) {
	c := routerCfg()
	r, why := c.Route("què fa aquesta funció?", false, "code")
	if r != "chat" || why == "" {
		t.Fatalf("r=%q why=%q", r, why)
	}
}

func TestRouteEdicioNoBaixa(t *testing.T) {
	c := routerCfg()
	r, _ := c.Route("arregla el bug de login?", false, "code")
	if r != "code" {
		t.Fatalf("una edició no baixa a chat: %q", r)
	}
}

func TestRouteOffRespecta(t *testing.T) {
	c := routerCfg()
	c.Router.Mode = "off"
	r, why := c.Route("què és això?", false, "code")
	if r != "code" || why != "" {
		t.Fatalf("off ha de respectar: r=%q why=%q", r, why)
	}
}

func TestRouteStrong(t *testing.T) {
	c := routerCfg()
	c.Router.StrongRole = "fort"
	c.Roles["fort"] = Role{Provider: "p", Model: "molt-fort"}
	r, why := c.Route("refactoritza tot el projecte a moduls", false, "code")
	if r != "fort" || why == "" {
		t.Fatalf("r=%q why=%q", r, why)
	}
}

func TestRouteSenseVision(t *testing.T) {
	c := routerCfg()
	r, _ := c.Route("descriu", true, "code")
	if r != "code" {
		t.Fatalf("sense rol vision es respecta: %q", r)
	}
	c.Roles["vision"] = Role{Provider: "p", Model: "veu"}
	r, _ = c.Route("descriu", true, "code")
	if r != "vision" {
		t.Fatalf("amb rol vision: %q", r)
	}
}

func TestEscalate(t *testing.T) {
	c := routerCfg()
	c.Router.EscalateAfter = 2
	c.Router.StrongRole = "fort"
	c.Roles["fort"] = Role{Provider: "p", Model: "molt-fort"}
	if _, ok := c.EscalateTarget(1, false, "code"); ok {
		t.Fatal("amb 1 no s'escala (límit 2)")
	}
	if _, ok := c.EscalateTarget(2, true, "code"); ok {
		t.Fatal("el pin manual guanya")
	}
	tgt, ok := c.EscalateTarget(2, false, "code")
	if !ok || tgt != "fort" {
		t.Fatalf("tgt=%q ok=%v", tgt, ok)
	}
	if _, ok := c.EscalateTarget(5, false, "fort"); ok {
		t.Fatal("si ja hi som no s'escala")
	}
	c.Router.EscalateAfter = 0
	if _, ok := c.EscalateTarget(9, false, "code"); ok {
		t.Fatal("apagat = mai")
	}
}

func TestValidateRouterBudget(t *testing.T) {
	c := routerCfg()
	c.Router.EscalateAfter = -1
	if err := c.Validate(); err == nil {
		t.Fatal("escalate_after negatiu ha de fallar")
	}
	c.Router.EscalateAfter = 0
	c.Budget.SessionUSD = -1
	if err := c.Validate(); err == nil {
		t.Fatal("budget negatiu ha de fallar")
	}
	c.Budget.SessionUSD = 0
	c.Router.StrongRole = "inexistent"
	if err := c.Validate(); err == nil {
		t.Fatal("strong_role inexistent ha de fallar")
	}
}

func TestRouteComptar(t *testing.T) {
	c := routerCfg()
	r, why := c.Route("quants fitxers .py hi ha aquí?", false, "code")
	if r != "chat" || why == "" {
		t.Fatalf("comptar no edita: r=%q why=%q", r, why)
	}
}
