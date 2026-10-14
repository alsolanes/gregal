package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// A server that is not listening must fail after one short retry (a local
// server may be starting) with an actionable hint, not after six retries.
func TestConnexioRefusadaNoEsReintenta(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	retries := 0
	ctx := WithRetryHook(context.Background(), func(int, int, time.Duration, error) { retries++ })
	t0 := time.Now()
	_, err = New().Chat(ctx, "http://"+addr+"/v1", "", "m", []Message{{Role: "user", Content: "x"}}, 0, 10)
	if err == nil {
		t.Fatal("expected an error")
	}
	if retries != 1 {
		t.Fatalf("connection refused retried %d times, want 1", retries)
	}
	if !IsPermanentTransport(err) || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("err=%v", err)
	}
	if !IsRetryable(err) {
		t.Fatal("a fallback provider must still be tried")
	}
	if IsServerError(err) {
		t.Fatal("a transport error is not a server error")
	}
	if time.Since(t0) > 5*time.Second {
		t.Fatalf("took %v", time.Since(t0))
	}
}

func TestPistesTransportPermanent(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&net.DNSError{Err: "no such host", Name: "x.invalid", IsNotFound: true}, "host not found"},
		{fmt.Errorf("dial tcp: lookup x.invalid: no such host"), "host not found"},
		{fmt.Errorf("dial tcp 127.0.0.1:9: connectex: No connection could be made because the target machine actively refused it."), "connection refused"},
		{fmt.Errorf("tls: failed to verify certificate: x509: certificate signed by unknown authority"), "TLS certificate"},
		{errors.New("read: connection reset by peer"), ""},
		{errors.New("unexpected EOF"), ""},
	} {
		got := permanentTransportHint(tc.err)
		if tc.want == "" && got != "" || tc.want != "" && !strings.Contains(got, tc.want) {
			t.Errorf("hint(%v)=%q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestIsServerError(t *testing.T) {
	if !IsServerError(errors.New("provider x: HTTP 502: bad gateway")) {
		t.Fatal("502 is a server error")
	}
	for _, e := range []error{
		errors.New("provider x: HTTP 429: slow down"),
		errors.New("provider x: HTTP 400: bad"),
		errors.New("provider x: dial tcp: i/o timeout"),
		nil,
	} {
		if IsServerError(e) {
			t.Errorf("IsServerError(%v) = true", e)
		}
	}
}

// DNS failures are configuration errors: no retry at all.
func TestDNSNoEsReintenta(t *testing.T) {
	if n := refusedRetries(&net.DNSError{Err: "no such host", Name: "x.invalid", IsNotFound: true}); n != 0 {
		t.Fatalf("DNS retries = %d", n)
	}
	if n := refusedRetries(errors.New("dial tcp 127.0.0.1:9: connect: connection refused")); n != 1 {
		t.Fatalf("refused retries = %d", n)
	}
}
