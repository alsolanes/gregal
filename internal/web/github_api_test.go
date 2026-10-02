package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gregal/internal/tools"
)

func TestGitHubAPIReadOnlyRequest(t *testing.T) {
	old := tools.GHRunner
	defer func() { tools.GHRunner = old }()
	var got []string
	tools.GHRunner = func(ctx context.Context, args ...string) (string, error) {
		got = append([]string{}, args...)
		return "#42 arreglat", nil
	}
	s := goalTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/github", strings.NewReader(`{"tool":"gh_pr","action":"diff","number":42,"repo":"acme/gregal"}`))
	rec := httptest.NewRecorder()
	s.handleGitHub(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("github api: %d %s", rec.Code, rec.Body.String())
	}
	var out map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out["output"] != "#42 arreglat" {
		t.Fatalf("resposta: %+v err=%v", out, err)
	}
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "pr diff 42") || !strings.Contains(joined, "--repo acme/gregal") {
		t.Fatalf("arguments insegurs o incomplets: %v", got)
	}
}

func TestGitHubAPIRetjaToolArbitrary(t *testing.T) {
	s := goalTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/github", strings.NewReader(`{"tool":"bash","action":"list"}`))
	rec := httptest.NewRecorder()
	s.handleGitHub(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("tool arbitrària ha de fallar: %d %s", rec.Code, rec.Body.String())
	}
}
