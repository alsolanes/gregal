package llm

import (
	"net/http"
	"testing"
	"time"
)

func TestBackoffRetryAfter(t *testing.T) {
	for _, code := range []int{429, 503} {
		for _, value := range []string{" 60 ", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)} {
			resp := &http.Response{StatusCode: code, Header: make(http.Header)}
			resp.Header.Set("Retry-After", value)
			if got := backoffFor(1, resp); got != 30*time.Second {
				t.Fatalf("code=%d value=%q got=%v", code, value, got)
			}
		}
	}
	resp := &http.Response{StatusCode: 429, Header: make(http.Header)}
	resp.Header.Set("Retry-After", time.Now().Add(10*time.Second).UTC().Format(http.TimeFormat))
	if got := backoffFor(1, resp); got < 8*time.Second || got > 10*time.Second {
		t.Fatalf("date backoff=%v", got)
	}
}

func TestBackoffWithoutJitter(t *testing.T) {
	previous := RetryJitter
	RetryJitter = 0
	t.Cleanup(func() { RetryJitter = previous })
	if got := backoffFor(1, nil); got != RetryBase {
		t.Fatalf("backoff=%v want=%v", got, RetryBase)
	}
}
