package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gregal/internal/advise"
	"gregal/internal/session"
)

// runAdvise llista vies de millora detectades (dependències, toolchain,
// higiene del repo, backend empaquetat). És només lectura i mai falla dur:
// sense xarxa o sense eines, ho diu i continua. Codi de sortida sempre 0
// (és un assessorament, no una comprovació).
func runAdvise(args []string) {
	fs := flag.NewFlagSet("advise", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "sortida JSON del informe")
	offline := fs.Bool("offline", false, "omet els detectors que necessiten xarxa")
	root := fs.String("arrel", "", "arrel del repo a analitzar (defecte: directori actual o el del binari)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	dir := *root
	if dir == "" {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil && repoDir != "" {
			if _, err2 := os.Stat(filepath.Join(repoDir, "go.mod")); err2 == nil {
				dir = repoDir
			}
		}
	}
	rep := advise.Run(context.Background(), dir, !*offline)

	// Persistència tolerant: si no es pot desar, s'avisa i es continua
	// amb l'informe en memòria.
	store := advise.Open(filepath.Join(filepath.Dir(session.DefaultDir()), "propostes.json"))
	if _, err := store.Upsert(rep.Proposals); err != nil {
		fmt.Fprintln(os.Stderr, "advise: no s'ha pogut desar ("+err.Error()+")")
	}

	if *jsonOut {
		raw, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(raw))
		return
	}
	if len(rep.Proposals) == 0 {
		fmt.Println("Tot al dia: cap millora detectada.")
	} else {
		fmt.Printf("%d propostes:\n", len(rep.Proposals))
		for _, p := range rep.Proposals {
			fmt.Printf("\n[%s] %s\n  %s\n  font: %s · esforç: %s · caduca: %s\n",
				p.Severity, p.Title, p.Detail, p.Source, p.Effort, p.ExpiresAt.Format("02/01/2006"))
		}
	}
	fmt.Println("\nDetectors:")
	for _, o := range rep.Outcomes {
		extra := ""
		if o.Reason != "" {
			extra = " (" + o.Reason + ")"
		}
		fmt.Printf("  %-13s %s%s\n", o.Name, o.Status, extra)
	}
}
