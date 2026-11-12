// Gregal: l'agent de codi, els vents i el servidor web. La CLI sencera viu a
// internal/cli; aquest main només hi posa la versió. Gregal Jocs (els jocs)
// és un altre binari, jocs/cmd/gregal-jocs, i aquest no l'importa.
package main

import "gregal/internal/cli"

// version es fixa en compilar: go build -ldflags "-X main.version=vX.Y.Z".
var version = "v1.72.4"

// repoDir l'omple la compilacio (-ldflags "-X main.repoDir=..."): aixi el
// binari recorda d'on va sortir i `gregal update` el sap trobar encara que
// es cridi des d'un altre directori.
var repoDir string

func main() {
	cli.Main(cli.Build{Version: version, RepoDir: repoDir})
}
