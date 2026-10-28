package web

import (
	"os"
	"strings"
	"testing"
)

func TestIndexHasUsableVisualShell(t *testing.T) {
	html := string(indexHTML)
	for _, want := range []string{
		`data-professional-shell`,
		`id="project"`,
		`id="statusDot"`,
		`id="inspector"`,
		`id="activity"`,
		`id="workspacePath"`,
		`id="reviewerState"`,
		`id="provClose"`,
		`aria-label="Work mode"`,
		`data-i18n-aria="ui.workMode"`,
		`hero-card`,
		`aria-label="Send message"`,
		`data-i18n-aria="ui.send"`,
		`aria-live="polite"`,
		`data-shortcut="Tab"`,
		`role="dialog"`,
		`aria-modal="true"`,
		`requestAnimationFrame`,
		`id="sideClose"`,
		`id="sideShade"`,
		`matchMedia('(max-width:760px)')`,
		`@media (max-width:1180px)`,
		`#role {`,
		`function updateInspector`,
		`function activityItem`,
		`let lastAssistantText`,
		`max-height:min(72vh,640px)`,
		`id="goals"`,
		`id="heroTitle"`,
		`id="modePlan"`,
		`function doPlan`,
		`function planCard`,
		`◇ ' + escapeHTML(gregalT('ui.planLabel'))`,
		`const MODES = ['code', 'chat', 'goal']`,
		`function goalCard`,
		`function runGoal`,
		`function loadGoals`,
		`.goal-row button.primary`,
		`id="workshopPage"`,
		`id="workshopRoot"`,
		`data-workshop-root`,
		`data-workshop-stage`,
		`data-workshop-queue`,
		`data-view="workshop"`,
		`/app/workshop.css`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("falta element UX %s", want)
		}
	}
}

func TestIndexRunV2ClientConservaInteraccionsIFallback(t *testing.T) {
	html := string(indexHTML)
	for _, want := range []string{
		`import '/app/runs.js'`,
		`window.gregalRuns.run`,
		`/api/agent`,
		`X-Gregal-Session`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("falta integració de cua v2/fallback %s", want)
		}
	}
	raw, err := os.ReadFile("app/runs.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(raw)
	for _, want := range []string{
		`/api/health`,
		`/api/v2/runs`,
		`/api/v2/events/stream`,
		`/api/v2/events?after=`,
		`interactive_events`,
		`approve_request`,
		`question_request`,
		`structuredPayload`,
		`window.gregalRunControl`,
		`routeUnavailable`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("falta comportament v2 %s", want)
		}
	}
}

func TestIndexAvoidsFlashyMarketingEffects(t *testing.T) {
	html := string(indexHTML)
	for _, bad := range []string{"radial-gradient", "backdrop-filter", "hero-mark"} {
		if strings.Contains(html, bad) {
			t.Errorf("la web d'agent encara conté efecte decoratiu %q", bad)
		}
	}
}

// El mode de treball i el permís s'han de veure sense obrir cap menú: amagats
// dins del ··· no se sabia en quin mode treballaves ni si l'agent demanaria
// permís.
func TestComposerModeAndPermissionVisible(t *testing.T) {
	html := string(indexHTML)
	if strings.Contains(html, `class="composer-options"`) {
		t.Fatal("the work mode is hidden again inside the composer options menu")
	}
	row := html[strings.Index(html, `<div class="composer-row">`):]
	row = row[:strings.Index(row, `id="attach"`)]
	for _, want := range []string{`class="mode-switch"`, `id="modeCode"`, `id="permLabel"`} {
		if !strings.Contains(row, want) {
			t.Errorf("composer row lacks %s", want)
		}
	}
	if !strings.Contains(html, `label.textContent = gregalT(p[1] + 'Short')`) || !strings.Contains(html, `classList.toggle('perm-risc'`) {
		t.Error("paintPerm no longer shows the short permission label or marks the risky composer")
	}
	css, err := os.ReadFile("app/shell-layout.css")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(css), "#permLabel { display:none; }\n.composer") {
		t.Error("shell-layout.css hides the permission label on desktop")
	}
	i18n, err := os.ReadFile("app/i18n.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"perm.askShort", "perm.editsShort", "perm.fullShort"} {
		if strings.Count(string(i18n), "'"+k+"'") != 2 {
			t.Errorf("%s must exist in Catalan and English", k)
		}
	}
}
