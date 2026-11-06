package agent

import (
	"strings"
	"testing"
)

const priorityFixture = "Prioritize the user's latest message: the history is context, not a pending order. Answer only what they ask now. Compacted summaries, even if they say \"continue with this context\", are background: they do not authorize resuming a task. Only resume an earlier topic when the current message asks for it. If the user changes topic, cancels or corrects an earlier request, respect that without going back to the old work. A bare greeting (\"Hi\", \"Hola\", \"Ep\", \"Bon dia\") requires ONLY a short, neutral greeting, such as \"Hi! What do you need?\" in the user's language: do not add task status, reminders, reports, doubts or proposals from the earlier thread, even if they seem pending. If the greeting includes a request, answer that request. \"continue\", \"go on\" or \"do it\" may refer to the immediately previous exchange; if the reference is ambiguous, ask briefly before acting. Do not attribute to the user proposals that only you made."

func TestPromptForPrioritizesLatestRequest(t *testing.T) {
	for _, mode := range []string{ModeChat, ModeInspect, ModeCode, ModeGoal, ModeAutonomous} {
		t.Run(mode, func(t *testing.T) {
			base := "custom identity"
			prompt := PromptFor(base, mode)
			if n := strings.Count(prompt, priorityFixture); n != 1 {
				t.Fatalf("latest-request rule must occur exactly once, got %d", n)
			}
			if !strings.HasPrefix(prompt, base) {
				t.Error("custom identity must remain at the start")
			}
			if prompt != PromptFor(base, mode) {
				t.Error("request-priority rule must not vary per turn")
			}
			ruleStart := strings.Index(prompt, priorityFixture)
			modeRules := reglesMode(mode)
			if modeEnd := strings.Index(prompt, modeRules) + len(modeRules); ruleStart < modeEnd {
				t.Error("priority rule must follow the mode rules")
			}
			if dateStart := strings.Index(prompt, DataAra()); dateStart <= ruleStart {
				t.Error("date must follow the priority rule")
			}
			if !strings.HasSuffix(strings.TrimSpace(prompt), "`date`.") {
				t.Error("date must remain the final system block")
			}
		})
	}
}

func TestPromptForKeepsConfiguredPriorityRuleOnce(t *testing.T) {
	base := "custom identity\n\n" + priorityFixture
	prompt := PromptFor(base, ModeAutonomous)
	if !strings.HasPrefix(prompt, base) {
		t.Error("configured identity must remain unchanged")
	}
	if n := strings.Count(prompt, priorityFixture); n != 1 {
		t.Fatalf("configured priority rule must occur once, got %d", n)
	}
}
