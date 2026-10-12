package selfupdate

import (
	"context"
	"strings"
	"testing"
)

func TestRestartRefusesCallingServiceBeforeSystemctl(t *testing.T) {
	// The executable cannot be found: a correct guard must refuse first,
	// without even trying to execute service-control from its own cgroup.
	t.Setenv("PATH", t.TempDir())
	err := reiniciaEn(context.Background(), "gregal-telegram", "0::/user.slice/user@1000.service/app.slice/gregal-telegram.service\n")
	if err == nil || !strings.Contains(err.Error(), "pròpia") {
		t.Fatalf("self restart not refused before subprocess: %v", err)
	}
}
