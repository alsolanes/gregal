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
		`id="planBtn"`,
		`function doPlan`,
		`function planCard`,
		`◇ PLA`,
		`const MODES = ['code', 'chat', 'goal']`,
		`function goalCard`,
		`function runGoal`,
		`function loadGoals`,
		`.goal-row button.primary`,
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
