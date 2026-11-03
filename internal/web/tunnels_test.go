package web

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// L'ordre `tunnel route dns` ha de rebre l'identificador del túnel demanat: amb
// el nom, cloudflared apuntava el CNAME a un altre túnel i l'amfitrió donava 404
// encara que el connector fos viu.
func TestCloudflaredRouteResolElNomAlIdentificador(t *testing.T) {
	dir := t.TempDir()
	args := filepath.Join(dir, "args.txt")
	bin := filepath.Join(dir, "cloudflared")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = tunnel ] && [ \"$2\" = list ]; then\n" +
		"  printf '[{\"id\":\"67cae018-df7a-4215-95e6-67431d3490ab\",\"name\":\"main\"}," +
		"{\"id\":\"a7558e19-1e93-4a79-bc64-fc564c6ca527\",\"name\":\"vents\"}]'\n" +
		"  exit 0\n" +
		"fi\n" +
		"printf '%s\\n' \"$*\" > " + args + "\n" +
		"exit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("no s'ha pogut escriure el cloudflared fals: %v", err)
	}
	if _, err := cloudflaredRoute(bin, "vents", "prova.example.com"); err != nil {
		t.Fatalf("la ruta hauria de funcionar amb el cloudflared fals: %v", err)
	}
	got, err := os.ReadFile(args)
	if err != nil {
		t.Fatalf("el cloudflared fals no s'ha executat: %v", err)
	}
	line := strings.TrimSpace(string(got))
	if !strings.Contains(line, "a7558e19-1e93-4a79-bc64-fc564c6ca527") {
		t.Fatalf("la ruta ha d'anar al túnel demanat (vents): %q", line)
	}
	if strings.Contains(line, "67cae018") {
		t.Fatalf("la ruta ha apuntat el CNAME a un altre túnel: %q", line)
	}
	if !strings.Contains(line, "prova.example.com") {
		t.Fatalf("l'amfitrió s'ha perdut: %q", line)
	}
}

func TestResolveTunnelIDDeixaEstarElsIdentificadors(t *testing.T) {
	id := "a7558e19-1e93-4a79-bc64-fc564c6ca527"
	if got := resolveTunnelID(context.Background(), "/no/existeix", id); got != id {
		t.Fatalf("un identificador no s'ha de tocar: %q", got)
	}
	if got := resolveTunnelID(context.Background(), "/no/existeix", "vents"); got != "vents" {
		t.Fatalf("si no hi ha cloudflared s'ha de tornar el nom original: %q", got)
	}
	if got := resolveTunnelID(context.Background(), "/no/existeix", ""); got != "" {
		t.Fatalf("buit s'ha de quedar buit: %q", got)
	}
}
