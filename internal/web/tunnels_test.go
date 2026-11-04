package web

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// L'ordre `tunnel route dns` ha de rebre l'identificador del túnel demanat: amb
// el nom, cloudflared apuntava el CNAME a un altre túnel i l'amfitrió donava 404
// encara que el connector fos viu.
func TestCloudflaredRouteResolElNomAlIdentificador(t *testing.T) {
	args := filepath.Join(t.TempDir(), "args.txt")
	t.Setenv(fakeCloudflaredEnv, args)
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
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

// fakeCloudflaredEnv fa que el binari de test faci de cloudflared (vegeu
// TestMain): un script de shell no s'executa a Windows.
const fakeCloudflaredEnv = "GREGAL_TEST_FAKE_CLOUDFLARED"

// actAsFakeCloudflared respon `tunnel list` amb dos túnels i desa els arguments de
// qualsevol altra ordre al fitxer de l'entorn. Torna false si no li toca.
func actAsFakeCloudflared() bool {
	argsFile := os.Getenv(fakeCloudflaredEnv)
	if argsFile == "" {
		return false
	}
	if len(os.Args) > 2 && os.Args[1] == "tunnel" && os.Args[2] == "list" {
		fmt.Print(`[{"id":"67cae018-df7a-4215-95e6-67431d3490ab","name":"main"},` +
			`{"id":"a7558e19-1e93-4a79-bc64-fc564c6ca527","name":"vents"}]`)
		return true
	}
	if err := os.WriteFile(argsFile, []byte(strings.Join(os.Args[1:], " ")+"\n"), 0o644); err != nil {
		os.Exit(1)
	}
	return true
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
