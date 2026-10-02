package tools

import (
	"context"
	"strings"
	"testing"
)

func TestGitHubIssueViewConstrueixArgsSegurs(t *testing.T) {
	old := GHRunner
	defer func() { GHRunner = old }()
	var got []string
	GHRunner = func(ctx context.Context, args ...string) (string, error) {
		got = append([]string{}, args...)
		return `{"number":42,"title":"bug"}`, nil
	}
	out, err := GitHub("gh_issue", `{"action":"view","number":42,"repo":"acme/gregal"}`)
	if err != nil || !strings.Contains(out, `"number":42`) {
		t.Fatalf("out=%q err=%v", out, err)
	}
	want := []string{"issue", "view", "42", "--json", "number,title,state,body,author,url,comments,labels,assignees", "--repo", "acme/gregal"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("args=%v, volia %v", got, want)
	}
}

func TestGitHubValidaAccionsIDades(t *testing.T) {
	old := GHRunner
	defer func() { GHRunner = old }()
	GHRunner = func(context.Context, ...string) (string, error) { return "ok", nil }
	for _, tc := range []struct {
		tool, args string
	}{
		{"gh_issue", `{"action":"view"}`},
		{"gh_pr", `{"action":"diff"}`},
		{"gh_pr", `{"action":"wat","number":1}`},
		{"gh_issue", `{"action":"list","repo":"acme/not valid"}`},
	} {
		if _, err := GitHub(tc.tool, tc.args); err == nil {
			t.Fatalf("hauria de fallar: %s %s", tc.tool, tc.args)
		}
	}
}
