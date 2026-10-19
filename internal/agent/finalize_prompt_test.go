package agent

import (
	"strings"
	"testing"
)

func TestFinalPromptOnlySuggestsUsefulReviewableSkills(t *testing.T) {
	for _, phrase := range []string{
		"successfully completed, non-autonomous task",
		"Suggest at most one skill",
		"enabled skill index",
		"sensitive information",
		"Usually make no suggestion",
		"compact draft",
		"Do not create or save it until the user explicitly agrees",
	} {
		if !strings.Contains(FinalPrompt, phrase) {
			t.Errorf("final prompt must include %q", phrase)
		}
	}
	if strings.Contains(FinalPromptLlarg(20, 4), `"Skill suggestion"`) {
		t.Fatal("failed/long final should not surface skill suggestions")
	}
}
