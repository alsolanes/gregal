package serviceguard

import (
	"os"
	"strings"
	"testing"
)

func TestCheckDiscoversOwnRuntimeService(t *testing.T) {
	raw, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		t.Skip("no Linux cgroup membership")
	}
	var unit string
	for _, line := range strings.Split(string(raw), "\n") {
		for _, component := range strings.Split(line, "/") {
			if strings.HasSuffix(component, ".service") && !strings.HasPrefix(component, "user@") {
				unit = component
			}
		}
	}
	if unit == "" {
		t.Skip("test process is not in a service")
	}
	if err := Check("systemctl --user stop " + unit); err == nil {
		t.Fatalf("runtime self-stop not guarded: %s", unit)
	}
}

func TestCheckRestartOnlyDeniesActualOwnRuntimeUnit(t *testing.T) {
	raw, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		t.Skip("no Linux cgroup membership")
	}
	own := scopeFromCgroup(string(raw)).ownUnit
	if OwnUnit() != own {
		t.Fatalf("OwnUnit = %q; process membership says %q", OwnUnit(), own)
	}
	if own == "" {
		t.Skip("test process is not in a service")
	}
	for _, unit := range []string{own, strings.TrimSuffix(own, ".service")} {
		if err := CheckRestart(unit); err == nil {
			t.Errorf("restart of runtime own unit %q not rejected", unit)
		}
	}
	for _, unit := range []string{"gregal-external-updater-test.service", "unrelated.service", own + "-other"} {
		if unit == own {
			continue
		}
		if err := CheckRestart(unit); err != nil {
			t.Errorf("external restart %q rejected: %v", unit, err)
		}
	}
}

func TestCheckProtectsRuntimeOwnUnitOutsideGregalNamespace(t *testing.T) {
	s := scope{ownUnit: "assistant-live@aleix.service"}
	for _, command := range []string{
		`systemctl --user stop assistant-live@aleix.service`,
		`systemctl restart assistant-live@aleix`,
		`systemctl kill 'assistant-live@*'`,
		`sh -c 'systemctl disable assistant-live@aleix'`,
	} {
		if err := s.check(command, 0); err == nil {
			t.Errorf("runtime own-unit command not denied: %s", command)
		}
	}
	for _, command := range []string{
		`systemctl stop assistant-live@other.service`,
		`systemctl show assistant-live@aleix.service`,
	} {
		if err := s.check(command, 0); err != nil {
			t.Errorf("unrelated/read-only runtime unit action rejected: %v", err)
		}
	}
}

func TestCheckEnvSplitStringPreservesTrailingArguments(t *testing.T) {
	for _, command := range []string{
		`env -S systemctl --user stop gregal-telegram.service`,
		`env --split-string=systemctl --user restart gregal-telegram.service`,
		`env -S "systemctl --user" stop gregal-telegram.service`,
	} {
		if err := Check(command); err == nil {
			t.Errorf("trailing self-stop arguments dropped: %s", command)
		}
	}
}

func TestCheckStaticWrapperAndQuotedBypasses(t *testing.T) {
	for _, command := range []string{
		`$'systemctl' --user stop gregal-telegram`,
		`systemctl --user stop $'gregal-\\x74elegram.service'`,
		`bash -c $'systemctl --user stop gregal-telegram'`,
		`env -S 'systemctl --user stop gregal-telegram'`,
		`env --split-string='systemctl --user stop gregal-telegram'`,
		`sudo -u aleix systemctl --user stop gregal-telegram`,
		`systemd-run --user --unit=maintenance systemctl --user stop gregal-telegram`,
	} {
		if err := Check(command); err == nil {
			t.Errorf("static wrapper bypass was not rejected: %s", command)
		}
	}
}

func TestCheckShellForms(t *testing.T) {
	for _, command := range []string{
		`systemctl --user stop gregal-telegram.service`,
		`systemctl stop --user gregal-telegram`,
		`systemctl --no-block restart --user nginx gregal-telegram.service`,
		`systemctl disable --now gregal-web.service`,
		`systemctl kill -s TERM --kill-whom main gregal-telegram`,
		`systemctl --user --job-mode replace-irreversibly stop gregal-telegram`,
		`systemctl --user stop 'gregal-telegram.service'`,
		`'systemctl' --user st"op" gre\gal-telegram`,
		`cd /tmp && systemctl --user stop gregal-telegram; systemctl --user start gregal-telegram`,
		"echo before\nsystemctl --user restart gregal-telegram",
		`systemctl status nginx | systemctl --user stop gregal-telegram`,
		`(systemctl --user stop gregal-telegram) &`,
		`echo $(systemctl --user stop gregal-telegram)`,
		"echo `systemctl --user stop gregal-telegram`",
		`if true; then systemctl --user stop gregal-telegram; fi`,
		`/usr/bin/systemctl --user stop gregal-telegram`,
		`env X=1 /usr/bin/systemctl --user stop gregal-telegram`,
		`command -- systemctl --user stop gregal-telegram`,
		`exec systemctl --user stop gregal-telegram`,
		`timeout 3s systemctl --user stop gregal-telegram`,
		`sh -c 'systemctl --user stop gregal-telegram'`,
		`bash -lc 'systemctl --user restart gregal-telegram.service'`,
		`eval 'systemctl --user stop gregal-telegram'`,
		`systemctl --user stop 'gregal-*'`,
		`systemctl --user stop '*.service'`,
		`systemctl --user stop '*'`,
		`unit=gregal-telegram; systemctl --user stop "$unit"`,
		`systemctl --user try-restart gregal-telegram`,
		`systemctl --user reload-or-restart gregal-telegram`,
		`systemctl --user mask --now gregal-telegram`,
	} {
		t.Run(command, func(t *testing.T) {
			if err := Check(command); err == nil {
				t.Fatalf("self-stop command was not rejected: %s", command)
			}
		})
	}
}

func TestCheckLeavesReadsAndUnrelatedActionsUntouched(t *testing.T) {
	for _, command := range []string{
		`systemctl --user status gregal-telegram.service`,
		`systemctl show --user gregal-telegram -p MainPID`,
		`systemctl --user is-active gregal-telegram`,
		`systemctl --user start gregal-telegram`,
		`systemctl --user enable --now gregal-telegram`,
		`systemctl --user stop nginx.service`,
		`systemctl --user restart gregalish.service`,
		`systemctl stop --signal TERM unrelated.service`,
		`systemctl --host stop status gregal-telegram`,
		`systemctl --user show --property stop gregal-telegram`,
		`systemctl --user stop nginx; systemctl --user status gregal-telegram`,
		`systemctl --user stop nginx # gregal-telegram`,
		`systemctl --user stop nginx; echo gregal-telegram`,
		`echo 'systemctl --user stop gregal-telegram'`,
		`printf '%s' 'systemctl --user stop gregal-telegram'`,
		`grep 'systemctl stop gregal-telegram' notes.md`,
		`systemctl --user stop gregal-telegram.timer`,
		`systemctl --user stop 'nginx*'`,
	} {
		t.Run(command, func(t *testing.T) {
			if err := Check(command); err != nil {
				t.Fatalf("unrelated/read-only command rejected: %v", err)
			}
		})
	}
}
