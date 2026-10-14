package sandbox

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func fals(goos string, have map[string]bool, abi int, docker bool) probe {
	return probe{
		goos:        goos,
		look:        func(n string) bool { return have[n] },
		landlockABI: func() int { return abi },
		dockerOK:    func() bool { return docker },
		helperOK:    func() bool { return true },
	}
}

func TestResolOffNoSondejaRes(t *testing.T) {
	st := resol(Config{}, probe{goos: "linux"})
	if st.Mode != ModeOff || st.Active || st.Refuses || st.Backend != BackendNone || st.Warning != "" {
		t.Fatalf("off: %+v", st)
	}
}

func TestResolLinuxPrefereixBwrap(t *testing.T) {
	st := resol(Config{Mode: "workspace", Network: "off"}, fals("linux", map[string]bool{"bwrap": true}, 4, false))
	if !st.Active || st.Backend != BackendBwrap || !st.NetworkIsolated || st.Warning != "" {
		t.Fatalf("%+v", st)
	}
}

func TestResolLinuxSenseBwrapUsaLandlock(t *testing.T) {
	st := resol(Config{Mode: "workspace"}, fals("linux", nil, 3, false))
	if !st.Active || st.Backend != BackendLandlock {
		t.Fatalf("%+v", st)
	}
	// Xarxa off amb ABI 3: workspace avisa i continua; strict refusa.
	st = resol(Config{Mode: "workspace", Network: "off"}, fals("linux", nil, 3, false))
	if !st.Active || st.Refuses || !strings.Contains(st.Warning, "network stays ON") {
		t.Fatalf("workspace: %+v", st)
	}
	st = resol(Config{Mode: "strict", Network: "off"}, fals("linux", nil, 3, false))
	if st.Active || !st.Refuses {
		t.Fatalf("strict: %+v", st)
	}
}

func TestResolLinuxSenseRes(t *testing.T) {
	st := resol(Config{Mode: "strict"}, fals("linux", nil, 0, false))
	if st.Active || !st.Refuses || !strings.Contains(st.Warning, "refused") {
		t.Fatalf("%+v", st)
	}
}

func TestResolWindowsFallback(t *testing.T) {
	// auto a Windows: cap backend automàtic.
	st := resol(Config{Mode: "workspace"}, fals("windows", map[string]bool{"docker": true}, 0, true))
	if st.Active || st.Refuses || !strings.Contains(st.Warning, "UNSANDBOXED") || !strings.Contains(st.Warning, "backend: docker") {
		t.Fatalf("workspace auto: %+v", st)
	}
	st = resol(Config{Mode: "strict"}, fals("windows", nil, 0, false))
	if !st.Refuses || st.Active {
		t.Fatalf("strict auto: %+v", st)
	}
	// docker demanat però el dimoni no respon.
	st = resol(Config{Mode: "strict", Backend: "docker"}, fals("windows", map[string]bool{"docker": true}, 0, false))
	if !st.Refuses || !strings.Contains(st.Warning, "daemon") {
		t.Fatalf("strict docker down: %+v", st)
	}
	// docker disponible.
	st = resol(Config{Mode: "workspace", Backend: "docker", Network: "off"}, fals("windows", map[string]bool{"docker": true}, 0, true))
	if !st.Active || st.Backend != BackendDocker || !st.NetworkIsolated {
		t.Fatalf("docker ok: %+v", st)
	}
}

func TestResolMac(t *testing.T) {
	st := resol(Config{Mode: "workspace"}, fals("darwin", map[string]bool{"sandbox-exec": true}, 0, false))
	if !st.Active || st.Backend != BackendSandboxExec {
		t.Fatalf("%+v", st)
	}
}

func TestPlanificaRefusaIAvisa(t *testing.T) {
	st := Status{Mode: ModeStrict, Refuses: true, Warning: "no backend"}
	if _, err := planifica(Config{Mode: "strict"}, st, "", "echo hi", ""); err == nil {
		t.Fatal("strict sense backend ha de refusar")
	}
	st = Status{Mode: ModeWorkspace, Warning: "unsandboxed"}
	p, err := planifica(Config{Mode: "workspace"}, st, "", "echo hi", "unsandboxed")
	if err != nil || p.Warning != "unsandboxed" || p.Backend != BackendNone || p.Name == "" {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestAvisNomesUnCop(t *testing.T) {
	vell := provador
	defer func() { provador = vell; Configure(Config{}) }()
	provador = func() probe { return fals("windows", nil, 0, false) }
	Configure(Config{Mode: "workspace"})
	p1, err1 := Command("", "echo a")
	p2, err2 := Command("", "echo b")
	if err1 != nil || err2 != nil || p1.Warning == "" || p2.Warning != "" {
		t.Fatalf("p1=%q p2=%q %v %v", p1.Warning, p2.Warning, err1, err2)
	}
	Configure(Config{Mode: "strict"})
	if _, err := Command("", "echo c"); err == nil {
		t.Fatal("strict ha de refusar")
	}
}

func TestBwrapArgs(t *testing.T) {
	a := BwrapArgs(BwrapOpts{Writable: []string{"/ws", "/tmp", "/home/u/.cache"}, Dir: "/ws", NetworkOff: true, TempDir: "/tmp"}, "/bin/sh", []string{"-c", "ls"})
	j := strings.Join(a, " ")
	for _, w := range []string{"--ro-bind / /", "--bind-try /ws /ws", "--bind-try /tmp /tmp", "--unshare-net", "--chdir /ws", "-- /bin/sh -c ls", "--die-with-parent"} {
		if !strings.Contains(j, w) {
			t.Errorf("falta %q a %q", w, j)
		}
	}
	a = BwrapArgs(BwrapOpts{Writable: []string{"/ws", "/tmp"}, Dir: "/ws", Strict: true, TempDir: "/tmp"}, "sh", []string{"-c", "x"})
	j = strings.Join(a, " ")
	if !strings.Contains(j, "--tmpfs /tmp") || strings.Contains(j, "--bind-try /tmp") || strings.Contains(j, "--unshare-net") {
		t.Errorf("strict: %q", j)
	}
}

func TestBwrapReal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("només Linux")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		t.Skip("sense bwrap")
	}
	ws := t.TempDir()
	a := BwrapArgs(BwrapOpts{Writable: []string{ws}, Dir: ws}, "sh", []string{"-c", "touch ok && ! touch /etc/gregal-sbx-test 2>/dev/null"})
	if out, err := exec.Command("bwrap", a...).CombinedOutput(); err != nil {
		t.Skipf("bwrap no funciona en aquest entorn: %v %s", err, out)
	}
}

func TestLandlockHelperArgsRoundTrip(t *testing.T) {
	a := LandlockHelperArgs([]string{"/ws", "/tmp"}, true, "/bin/sh", []string{"-c", "echo -- hi"})
	if a[0] != HelperArg {
		t.Fatal(a)
	}
	s, ok := ParseHelperArgs(a[1:])
	if !ok || !s.NetworkOff || len(s.Writable) != 2 || strings.Join(s.Argv, " ") != "/bin/sh -c echo -- hi" {
		t.Fatalf("%+v %v", s, ok)
	}
	if _, ok := ParseHelperArgs([]string{"--rw"}); ok {
		t.Fatal("arguments trencats acceptats")
	}
}

func TestSeatbeltProfile(t *testing.T) {
	p := SeatbeltProfile([]string{"/Users/a b/ws", `/tmp/we"ird`}, true)
	for _, w := range []string{"(deny file-write*)", `(subpath "/Users/a b/ws")`, `(subpath "/tmp/we\"ird")`, "(deny network*)", "(remote unix-socket)"} {
		if !strings.Contains(p, w) {
			t.Errorf("falta %q:\n%s", w, p)
		}
	}
	if strings.Contains(SeatbeltProfile([]string{"/ws"}, false), "network") {
		t.Error("amb xarxa on no s'ha de tocar la xarxa")
	}
}

func TestDockerArgs(t *testing.T) {
	a := DockerArgs(DockerOpts{Name: "gregal-sbx-1", Image: "alpine:3.20", Workspace: `C:\dev\proj`, NetworkOff: true, Strict: true}, `ls C:\dev\proj\src && echo ok`)
	j := strings.Join(a, " ")
	for _, w := range []string{"run --rm -i", "--network none", "--name gregal-sbx-1", "--read-only", "--cap-drop ALL",
		`type=bind,src=C:\dev\proj,dst=/c/dev/proj`, "-w /c/dev/proj", "alpine:3.20 sh -c ls /c/dev/proj/src && echo ok"} {
		if !strings.Contains(j, w) {
			t.Errorf("falta %q a %q", w, j)
		}
	}
	a = DockerArgs(DockerOpts{Image: "img", Workspace: "/home/u/p", User: "1000:1000"}, "make")
	j = strings.Join(a, " ")
	if !strings.Contains(j, "--network bridge") || !strings.Contains(j, "--user 1000:1000") || !strings.Contains(j, "dst=/home/u/p") || strings.Contains(j, "--read-only") {
		t.Errorf("%q", j)
	}
}

func TestContainerPath(t *testing.T) {
	for in, want := range map[string]string{`C:\dev\x`: "/c/dev/x", "D:/a/b": "/d/a/b", "/home/u": "/home/u", `C:\`: "/c/"} {
		if got := ContainerPath(in); got != want {
			t.Errorf("%q → %q, volia %q", in, got, want)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := (Config{Mode: "workspace", Network: "off", Backend: "docker"}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []Config{{Mode: "jail"}, {Network: "maybe"}, {Backend: "firejail"}, {AllowWrite: []string{" "}}} {
		if c.Validate() == nil {
			t.Errorf("hauria de fallar: %+v", c)
		}
	}
}
