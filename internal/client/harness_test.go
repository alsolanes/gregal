package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScopedSubmissionAndInteractions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Gregal-Session") != "employee-session" || r.Header.Get("Authorization") != "Bearer employee-token" {
			t.Errorf("request lost identity/session scope: %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v2/runs" {
			w.WriteHeader(http.StatusAccepted)
		}
		_, _ = w.Write([]byte(`{"run":{"id":7},"cursor":12}`))
	}))
	defer server.Close()
	base := New(server.URL, "employee-token", "")
	scoped := base.ForSession("employee-session")
	if base.SessionID != "" {
		t.Fatal("scoping mutated original client")
	}
	submitted, err := scoped.Submit(context.Background(), "", "Analyze", "", "turn-1", "")
	if err != nil || submitted.Cursor != 12 || submitted.Run.ID != 7 {
		t.Fatalf("submission: %+v %v", submitted, err)
	}
	if _, err := scoped.Run(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if err := scoped.CancelRun(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if err := scoped.Approve(context.Background(), "permission", false, false); err != nil {
		t.Fatal(err)
	}
	if err := scoped.AnswerQuestion(context.Background(), "question", "Approved data"); err != nil {
		t.Fatal(err)
	}
}
