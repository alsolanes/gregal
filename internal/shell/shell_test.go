package shell

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// L'intèrpret triat ha d'existir i executar de debò una ordre: és el que
// va fallar al portable de Windows ("sh" a pèl, sense mirar si hi era).
func TestArgvExecutaAQualsevolSO(t *testing.T) {
	name, args := Argv("echo hola-des-de-la-shell")
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	if !strings.Contains(string(out), "hola-des-de-la-shell") {
		t.Fatalf("sortida inesperada de %s: %q", name, out)
	}
	if Name() != name {
		t.Fatalf("Name()=%q, Argv diu %q", Name(), name)
	}
}

// Amb un sh la sintaxi POSIX (&&) ha de funcionar; amb cmd, el /C.
func TestArgvCoherentAmbIsPOSIX(t *testing.T) {
	name, args := Argv("x")
	if IsPOSIX() && args[0] != "-c" {
		t.Fatalf("intèrpret POSIX %s amb args %v", name, args)
	}
	if !IsPOSIX() && args[0] != "/C" {
		t.Fatalf("intèrpret no POSIX %s amb args %v", name, args)
	}
}

// A Git Bash les rutes C:\... s'han de traduir a /c/... (si no, cada ls
// amb ruta absoluta falla); URL i la resta no es toquen.
func TestNormalitzaPathsWindows(t *testing.T) {
	if runtime.GOOS != "windows" || !IsPOSIX() {
		t.Skip("només Windows amb sh POSIX")
	}
	for _, c := range []struct{ in, want string }{
		{`ls "C:\Users\x\f.txt"`, `ls "/c/Users/x/f.txt"`},
		{`type C:/a/b.txt`, `type /c/a/b.txt`},
		{`ls C:\a\b.txt, echo fi`, `ls /c/a/b.txt, echo fi`},
		{`curl https://a.b/c/d`, `curl https://a.b/c/d`},
		{`echo "ratio 3:4"`, `echo "ratio 3:4"`},
		{`ls /c/ja/be`, `ls /c/ja/be`},
		{`echo C:`, `echo C:`},
	} {
		if got := NormalitzaPathsWindows(c.in); got != c.want {
			t.Errorf("%q → %q, volia %q", c.in, got, c.want)
		}
	}
}
