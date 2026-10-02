package web

import "testing"

func TestListenRequiresAuthenticationOffLoopback(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:8097", "127.0.0.2:8097", "[::1]:8097", "localhost:8097"} {
		if err := validateListenAuth(addr, false); err != nil {
			t.Errorf("local address %q: %v", addr, err)
		}
	}
	for _, addr := range []string{":8097", "0.0.0.0:8097", "[::]:8097", "192.0.2.10:8097", "server.example.org:8097"} {
		if validateListenAuth(addr, false) == nil {
			t.Errorf("unauthenticated remote address %q accepted", addr)
		}
		if err := validateListenAuth(addr, true); err != nil {
			t.Errorf("authenticated address %q: %v", addr, err)
		}
	}
	if validateListenAuth("invalid-address", true) == nil {
		t.Fatal("invalid listen address accepted")
	}
}
