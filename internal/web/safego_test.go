package web

import "testing"

// Un pànic en una goroutine de fons no pot tombar el procés.
func TestRecoverGoSwallowsPanic(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer recoverGo("test")
		panic("boom")
	}()
	<-done
}
