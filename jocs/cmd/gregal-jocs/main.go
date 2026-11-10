// Gregal Jocs: el mateix binari que Gregal (CLI, servidor, vents) amb els
// jocs (l'Arena i la lliga dels vents) registrats, i --open obre /jocs/.
// L'app d'escriptori Gregal Jocs l'engega com a backend.
//
//	go build -o gregal-jocs ./jocs/cmd/gregal-jocs
package main

import (
	"gregal/internal/cli"
	_ "gregal/jocs/server" // registra el producte Gregal Jocs (init)
)

// version es fixa en compilar: go build -ldflags "-X main.version=vX.Y.Z".
// scripts/release.cjs la puja alhora que la del main.go de l'arrel.
var version = "v1.68.0"

// repoDir l'omple la compilació (-ldflags "-X main.repoDir=..."), com a
// Gregal.
var repoDir string

func main() {
	cli.Main(cli.Build{
		Version: version, RepoDir: repoDir,
		Package:  "./jocs/cmd/gregal-jocs",
		OpenPath: "/jocs/",
		// La seva identitat de servei (service-jocs.json) i el seu port: corre
		// al costat del servidor de Gregal sobre el mateix perfil.
		Product:     "jocs",
		DefaultPort: "8197",
	})
}
