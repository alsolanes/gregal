package web

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"gregal/internal/jobs"
	"gregal/internal/llm"
)

// TestMain fa tres coses: escurça els reintents del client LLM (1 s, 2 s,
// 4 s… fins a 50 s; són per a proveïdors de debò que arrenquen sota demanda,
// i amb els servidors falsos dels tests farien esperar minuts), aparta les
// dades de debò amb GREGAL_DATA_DIR i fa que el directori d'usuari sigui un
// de temporal. Sense això les proves desaven converses, tokens i contrasenyes
// a ~/.local/share/gregal: milers de fitxers al cap del temps. Els jobs
// (jobs.New("") de New) i workspaces.json no miren GREGAL_DATA_DIR sinó el
// home, i els tests dels vents hi deixaven el job wind-xaloc i runs falsos.
// Al final comprova que el magatzem de jobs i workspaces.json de debò no
// s'han tocat.
func TestMain(m *testing.M) {
	llm.RetryBase, llm.RetryMax, llm.RetryJitter = 5*time.Millisecond, 20*time.Millisecond, time.Millisecond
	dir, err := os.MkdirTemp("", "gregal-web-test-")
	if err != nil {
		panic(err)
	}
	// Els camins de debò es calculen abans de canviar el home.
	realPaths := []string{jobs.DefaultDir(), workspacesPath()}
	before := snapshotFiles(realPaths)
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		panic(err)
	}
	os.Setenv("GREGAL_DATA_DIR", dir)
	// HOME sol no n'hi ha prou a Windows: os.UserHomeDir() hi llegeix USERPROFILE.
	os.Setenv("HOME", home)
	os.Setenv("USERPROFILE", home)
	code := m.Run()
	if diff := diffFiles(before, snapshotFiles(realPaths)); len(diff) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: els tests han tocat fitxers de debò (o un gregal en marxa hi ha escrit mentrestant):\n")
		for _, d := range diff {
			fmt.Fprintf(os.Stderr, "  %s\n", d)
		}
		if code == 0 {
			code = 1
		}
	}
	os.RemoveAll(dir)
	os.Exit(code)
}

// snapshotFiles recull mida i data de cada fitxer de sota els camins (fitxers
// o directoris); els que no existeixen no hi surten.
func snapshotFiles(paths []string) map[string]string {
	out := map[string]string{}
	for _, root := range paths {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err == nil {
				out[p] = fmt.Sprintf("%d bytes, %s", info.Size(), info.ModTime().Format(time.RFC3339Nano))
			}
			return nil
		})
	}
	return out
}

func diffFiles(before, after map[string]string) []string {
	var diff []string
	for p, a := range after {
		if b, ok := before[p]; !ok {
			diff = append(diff, "nou: "+p)
		} else if a != b {
			diff = append(diff, "canviat: "+p)
		}
	}
	for p := range before {
		if _, ok := after[p]; !ok {
			diff = append(diff, "esborrat: "+p)
		}
	}
	sort.Strings(diff)
	return diff
}
