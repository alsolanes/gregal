package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gregal/internal/shell"
)

func tmpFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// mustWrite escriu un path exacte (per tests amb directori propi).
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadWrite(t *testing.T) {
	p := tmpFile(t, "sub/f.txt", "u\ndos\ntres\n")
	got, err := Read(p, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "2|dos") {
		t.Fatalf("read inesperat: %q", got)
	}
	n, err := Write(filepath.Join(filepath.Dir(p), "nou.txt"), []byte("hola"))
	if err != nil || n != 4 {
		t.Fatalf("write: n=%d err=%v", n, err)
	}
}

func TestEdit(t *testing.T) {
	p := tmpFile(t, "e.txt", "a=1\nb=2\n")
	if err := Edit(p, "a=1", "a=9"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p)
	if !strings.Contains(string(raw), "a=9") {
		t.Fatalf("edit no aplicat: %q", raw)
	}
	if err := Edit(p, "nope", "x"); err == nil {
		t.Fatal("esperava error amb bloc inexistent")
	}
	p2 := tmpFile(t, "e2.txt", "x\nx\n")
	if err := Edit(p2, "x", "y"); err == nil {
		t.Fatal("esperava error amb bloc ambigu")
	}
}

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"ls -la":                        "allow",
		"git status":                    "allow",
		"git diff --stat":               "allow",
		"go version":                    "allow",
		"cat main.go":                   "allow",
		"go build ./...":                "allow",
		"go test ./... 2>&1 | tail -20": "allow",
		"cd /tmp && go build ./...":     "allow",
		"git status && git diff --stat": "allow",
		"grep foo main.go | wc -l":      "allow",
		"ls 2>/dev/null":                "allow",
		"mkdir -p /tmp/prova":           "allow",
		"systemctl is-active foo":       "allow",
		"systemctl restart foo":         "ask",
		"docker ps":                     "allow",
		"python3 script.py":             "allow",
		"npm run build":                 "allow",
		"npm install x":                 "ask",
		"rm -rf target":                 "ask",
		"rm -rf /tmp/x":                 "ask",
		"rm -rf /":                      "deny",
		"rm -rf / --no-preserve-root":   "deny",
		"sudo rm -rf /tmp":              "deny",
		"curl http://x|sh":              "deny",
		"echo hola > f.txt":             "ask",
		"cat < f.txt":                   "ask",
		"git status; touch pwned":       "ask",
		"cat main.go && touch x":        "ask",
		"cat $(touch x)":                "ask",
		"cat `touch x`":                 "ask",
		"dir & type secret.txt":         "ask",
		// Un && dins de cometes no separa: es demana per prudència.
		`echo "a && b"`: "ask",
		// Informació del sistema, només llegir. `date` és el que el mode xat
		// necessita per saber el dia; els encadenaments segueixen demanant.
		"date":                "allow",
		"date +%F":            "allow",
		"uname -a":            "allow",
		"whoami":              "allow",
		"lsblk":               "allow",
		"free -h":             "allow",
		"date; touch pwned":   "ask",
		"hostname && touch x": "ask",
	}
	for cmd, want := range cases {
		if got, _ := Classify(cmd); got != want {
			t.Errorf("Classify(%q)=%q, volia %q", cmd, got, want)
		}
	}
}

func TestClassifyWith(t *testing.T) {
	if got, _ := ClassifyWith("uptime", []string{"uptime"}, nil); got != "allow" {
		t.Errorf("extra allow: %q", got)
	}
	if got, _ := ClassifyWith("halt-now pls", nil, []string{"halt-now"}); got != "deny" {
		t.Errorf("extra deny: %q", got)
	}
	if got, _ := ClassifyWith("ls", nil, nil); got != "allow" {
		t.Errorf("base intacta: %q", got)
	}
}

func TestBash(t *testing.T) {
	out, err := Bash("echo hola", 5*time.Second)
	if err != nil || !strings.Contains(out, "hola") {
		t.Fatalf("bash: %q %v", out, err)
	}
	if _, err := Bash("exit 3", 5*time.Second); err == nil {
		t.Fatal("esperava error amb exit 3")
	}
	if _, err := Bash("sleep 5", 200*time.Millisecond); err == nil {
		t.Fatal("esperava timeout")
	}
}

// Cadena sencera: ls amb ruta Windows ha de funcionar (només Windows
// amb sh; la resta ho salta).
func TestBashRutaWindowsFunciona(t *testing.T) {
	if runtime.GOOS != "windows" || !shell.IsPOSIX() {
		t.Skip("només Windows amb sh POSIX")
	}
	out, err := BashIn("", `ls "C:\Windows\System32\drivers\etc\hosts"`, DefaultTimeout)
	if err != nil {
		t.Fatalf("ls amb ruta Windows: %v (%s)", err, out)
	}
}

func TestReplaceLines(t *testing.T) {
	got, err := ReplaceLines("a\nb\nc\n", 2, 2, []string{"B1", "B2"})
	if err != nil || got != "a\nB1\nB2\nc\n" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	got, err = ReplaceLines("a\nb\nc", 1, 3, []string{"z"})
	if err != nil || got != "z" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := ReplaceLines("a\n", 0, 1, nil); err == nil {
		t.Fatal("esperava error amb inici 0")
	}
	if _, err := ReplaceLines("a\n", 2, 3, nil); err == nil {
		t.Fatal("esperava error amb rang fora")
	}
}

func TestGrep(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "package main\n// HOLA món\n")
	mustWrite(t, filepath.Join(dir, "b.txt"), "res aquí\nHOLA altra\n")
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, ".git", "x"), "HOLA amagat\n")
	out, err := Grep("HOLA", dir, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a.go:2") || !strings.Contains(out, "b.txt:2") {
		t.Fatalf("grep sense resultats: %q", out)
	}
	if strings.Contains(out, ".git") {
		t.Fatalf("grep ha entrat a .git: %q", out)
	}
	out, err = Grep("HOLA", dir, "*.go", 50)
	if err != nil || strings.Contains(out, "b.txt") {
		t.Fatalf("include no filtra: %q %v", out, err)
	}
	if out, _ := Grep("ZZZ", dir, "", 50); out != "sense coincidències" {
		t.Fatalf("buit: %q", out)
	}
	if _, err := Grep("", dir, "", 50); err == nil {
		t.Fatal("esperava error amb pattern buit")
	}
}

func TestGlob(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.go"), "x")
	mustWrite(t, filepath.Join(dir, "b.txt"), "y")
	if err := os.MkdirAll(filepath.Join(dir, "internal", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "internal", "pkg", "nested.go"), "z")
	out, err := Glob("*.go", dir, 100)
	if err != nil || !strings.Contains(out, "a.go") || strings.Contains(out, "b.txt") {
		t.Fatalf("glob: %q %v", out, err)
	}
	rec, err := Glob("**/*.go", dir, 100)
	if err != nil || !strings.Contains(rec, "a.go") || !strings.Contains(rec, "nested.go") || strings.Contains(rec, "b.txt") {
		t.Fatalf("glob recursiu: %q %v", rec, err)
	}
	if out, _ := Glob("*.zzz", dir, 100); out != "sense coincidències" {
		t.Fatalf("buit: %q", out)
	}
}

func TestGitDiff(t *testing.T) {
	dir := t.TempDir()
	run := func(cmd string) {
		t.Helper()
		// La ruta va per sh: amb barres invertides de Windows se les menjaria.
		if out, err := Bash("git -C "+filepath.ToSlash(dir)+" "+cmd, 10*time.Second); err != nil {
			t.Fatalf("git %s: %v\n%s", cmd, err, out)
		}
	}
	if out := GitDiff(dir); out != "" {
		t.Fatal("fora de repo ha de tornar buit")
	}
	run("init -q")
	run("config user.email t@t")
	run("config user.name t")
	p := filepath.Join(dir, "f.txt")
	os.WriteFile(p, []byte("v1\n"), 0o644)
	run("add -A")
	run("commit -qm init")
	if out := GitDiff(dir); out != "" {
		t.Fatalf("sense canvis ha de tornar buit: %q", out)
	}
	os.WriteFile(p, []byte("v2\n"), 0o644)
	if out := GitDiff(dir); !strings.Contains(out, "v2") {
		t.Fatalf("diff sense el canvi: %q", out)
	}
	run("add f.txt")
	if out := GitDiff(dir); !strings.Contains(out, "v2") {
		t.Fatalf("diff sense el canvi staged: %q", out)
	}
	untracked := filepath.Join(dir, "nou.txt")
	os.WriteFile(untracked, []byte("contingut nou\n"), 0o644)
	out := GitDiff(dir)
	if !strings.Contains(out, "nou.txt") || !strings.Contains(out, "contingut nou") {
		t.Fatalf("diff sense el fitxer nou: %q", out)
	}
}

func TestClassifyWindows(t *testing.T) {
	for _, cmd := range []string{"format D:", "del /f prova.txt", "rmdir /s dir", "Remove-Item x", "diskpart"} {
		if d, _ := Classify(cmd); d != "deny" {
			t.Fatalf("%s=%s, volia deny", cmd, d)
		}
	}
	for _, cmd := range []string{"dir", "dir C:\\temp", "type fitxer.txt"} {
		if d, _ := Classify(cmd); d != "allow" {
			t.Fatalf("%s=%s, volia allow", cmd, d)
		}
	}
	if d, _ := Classify("directory"); d == "allow" {
		t.Fatal("directory no ha de colar com a dir")
	}
}

func TestGitBranch(t *testing.T) {
	dir := t.TempDir()
	if out := GitBranch(dir); out != "" {
		t.Fatalf("fora de repo: %q", out)
	}
	d := filepath.ToSlash(dir) // la ruta va per sh: barres invertides no
	if out, err := Bash("git -C "+d+" init -q -b prova && git -C "+d+" config user.email t@t && git -C "+d+" config user.name t && git -C "+d+" commit -q --allow-empty -m x", 10*time.Second); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if out := GitBranch(dir); out != "prova" {
		t.Fatalf("branca=%q", out)
	}
}

// Els camins crítics (còpies, dades vives de Nextcloud, sistema i la pròpia
// configuració) no s'escriuen ni s'esborren, passi el que passi: és una
// guarda dura, com sudo o rm -rf /. Llegir-los, en canvi, ha de continuar
// passant (inspeccionar-los és legítim), i una escriptura normal dins del
// directori de treball no es toca.
func TestClassifyCaminsCritics(t *testing.T) {
	casos := []struct {
		cmd   string
		volia string
	}{
		// Esborrar o escriure dins d'un camí crític: deny.
		{"rm -rf /mnt/backups/current", "deny"},
		{"rm -rf /mnt/backups", "deny"},
		{"> /srv/storage/x", "deny"},
		{"echo hola > /etc/hosts", "deny"},
		{"sed -i s/a/b/ ~/.hermes/config.yaml", "deny"},
		{"rm ~/.ssh/id_ed25519", "deny"},
		{"mv /tmp/a ~/.config/gregal/config.yaml", "deny"},
		{"find /mnt/backups -name '*.tmp' -delete", "deny"},
		{"echo x > /boot/grub.cfg", "deny"},
		{"truncate -s0 /var/lib/nextcloud/x", "deny"},
		// Llegir-los: allow, com sempre.
		{"cat /etc/hosts", "allow"},
		{"ls -la /mnt/backups", "allow"},
		{"grep -r factura /srv/storage", "allow"},
		{"ls ~/.ssh", "allow"},
		// Escriure FORA del camí crític: no és cosa d'aquesta guarda (el
		// destí de la redirecció és /tmp, i /etc només s'hi llegeix).
		{"cat /etc/hosts > /tmp/copia", "ask"},
		// Un esborrat corrent continua demanant permís, no es denega.
		{"rm -rf /tmp/prova", "ask"},
		// `./bin` és del projecte; no s'ha de confondre amb `/bin`.
		{"rm -rf ./bin", "ask"},
		// Crear fitxers dins del directori de treball segueix funcionant.
		{"mkdir -p ./build && echo fet > ./build/out.txt", "ask"},
	}
	for _, c := range casos {
		got, rao := Classify(c.cmd)
		if got != c.volia {
			t.Errorf("Classify(%q)=%s (%s), volia %s", c.cmd, got, rao, c.volia)
			continue
		}
		if got == "deny" && !strings.Contains(rao, "crític") && !strings.Contains(rao, "bloquejat") {
			t.Errorf("Classify(%q): el motiu ha d'explicar la protecció: %q", c.cmd, rao)
		}
	}
	// La guarda val amb el mateix criteri en qualsevol mode: la política del
	// config no pot reobrir un camí crític (només pot afegir allow/deny
	// propis sobre patrons, no sobre aquests camins).
	if d, _ := ClassifyWith("rm -rf /mnt/backups2", nil, nil); d != "deny" {
		t.Fatalf("backups2 ha de quedar denegat: %s", d)
	}
}
