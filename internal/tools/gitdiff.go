package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// G3 — diffs estructurats. GitDiff dona un text pensat per al revisor;
// aquí el diff surt parsejat (fitxers → hunks → línies) perquè la UI el
// pugui pintar i, sobretot, perquè es pugui descartar un hunk concret.

// DiffLine és una línia del diff: kind "ctx", "add" o "del".
type DiffLine struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	Old  int    `json:"old,omitempty"`
	New  int    `json:"new,omitempty"`
}

// DiffHunk és un bloc @@ amb el seu pegat aïllat (aplicable amb git apply).
type DiffHunk struct {
	Index   int        `json:"index"`
	Header  string     `json:"header"`
	Lines   []DiffLine `json:"lines"`
	Added   int        `json:"added"`
	Removed int        `json:"removed"`
	Patch   string     `json:"patch"`
}

// DiffFile agrupa els hunks d'un fitxer.
type DiffFile struct {
	Path      string     `json:"path"`
	OldPath   string     `json:"old_path,omitempty"`
	Status    string     `json:"status"` // modified | added | deleted | renamed
	Binary    bool       `json:"binary"`
	Untracked bool       `json:"untracked"`
	Added     int        `json:"added"`
	Removed   int        `json:"removed"`
	Hunks     []DiffHunk `json:"hunks"`
}

const diffMaxLinesPerHunk = 2000

// GitDiffStructured retorna els canvis del working tree respecte a HEAD,
// incloent-hi els fitxers nous (untracked) com a altes senceres.
// Sense repo git retorna una llista buida i error nil: la UI ho ensenya
// com "sense canvis" en comptes de petar.
func GitDiffStructured(dir string) ([]DiffFile, error) {
	if _, err := gitOut(dir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, nil
	}
	raw, err := gitOut(dir, "diff", "HEAD", "--no-color", "--unified=3")
	if err != nil {
		// Repo sense HEAD encara (cap commit): staged + working.
		staged, _ := gitOut(dir, "diff", "--cached", "--no-color", "--unified=3")
		work, _ := gitOut(dir, "diff", "--no-color", "--unified=3")
		raw = staged + work
	}
	files := parseUnifiedDiff(raw)
	names, _ := gitOut(dir, "ls-files", "--others", "--exclude-standard", "-z")
	for _, rel := range strings.Split(names, "\x00") {
		if rel == "" {
			continue
		}
		files = append(files, untrackedDiff(dir, rel))
	}
	return files, nil
}

// untrackedDiff presenta un fitxer nou com un hunk d'altes.
func untrackedDiff(dir, rel string) DiffFile {
	f := DiffFile{Path: rel, Status: "added", Untracked: true}
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		return f
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		f.Binary = true
		return f
	}
	body := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(body) > diffMaxLinesPerHunk {
		body = body[:diffMaxLinesPerHunk]
	}
	h := DiffHunk{Index: 0, Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(body))}
	for i, l := range body {
		h.Lines = append(h.Lines, DiffLine{Kind: "add", Text: l, New: i + 1})
	}
	h.Added = len(body)
	f.Hunks = []DiffHunk{h}
	f.Added = len(body)
	return f
}

// parseUnifiedDiff converteix la sortida de git diff en fitxers i hunks.
func parseUnifiedDiff(raw string) []DiffFile {
	var out []DiffFile
	var cur *DiffFile
	var hunk *DiffHunk
	var oldLn, newLn int
	flush := func() {
		if cur != nil && hunk != nil {
			hunk.Patch = buildHunkPatch(*cur, *hunk)
			cur.Hunks = append(cur.Hunks, *hunk)
			hunk = nil
		}
	}
	closeFile := func() {
		flush()
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			closeFile()
			a, b := splitDiffGit(line)
			cur = &DiffFile{Path: b, OldPath: a, Status: "modified"}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "new file mode"):
			cur.Status = "added"
		case strings.HasPrefix(line, "deleted file mode"):
			cur.Status = "deleted"
		case strings.HasPrefix(line, "rename to "):
			cur.Status = "renamed"
			cur.Path = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "Binary files "):
			cur.Binary = true
		case strings.HasPrefix(line, "@@"):
			flush()
			o, n, header := parseHunkHeader(line)
			oldLn, newLn = o, n
			hunk = &DiffHunk{Index: len(cur.Hunks), Header: header}
		case hunk == nil:
			continue
		case strings.HasPrefix(line, "+"):
			if len(hunk.Lines) < diffMaxLinesPerHunk {
				hunk.Lines = append(hunk.Lines, DiffLine{Kind: "add", Text: line[1:], New: newLn})
			}
			hunk.Added++
			cur.Added++
			newLn++
		case strings.HasPrefix(line, "-"):
			if len(hunk.Lines) < diffMaxLinesPerHunk {
				hunk.Lines = append(hunk.Lines, DiffLine{Kind: "del", Text: line[1:], Old: oldLn})
			}
			hunk.Removed++
			cur.Removed++
			oldLn++
		case strings.HasPrefix(line, " "):
			if len(hunk.Lines) < diffMaxLinesPerHunk {
				hunk.Lines = append(hunk.Lines, DiffLine{Kind: "ctx", Text: line[1:], Old: oldLn, New: newLn})
			}
			oldLn++
			newLn++
		case line == `\ No newline at end of file`:
			// res a mostrar, però compta per al pegat.
		}
	}
	closeFile()
	return out
}

// buildHunkPatch refà el pegat d'un sol hunk perquè git apply el pugui
// aplicar (o revertir) tot sol.
func buildHunkPatch(f DiffFile, h DiffHunk) string {
	old, new := f.OldPath, f.Path
	if old == "" {
		old = f.Path
	}
	var b strings.Builder
	fmt.Fprintf(&b, "diff --git a/%s b/%s\n", old, new)
	if f.Status == "added" {
		fmt.Fprintf(&b, "--- /dev/null\n+++ b/%s\n", new)
	} else if f.Status == "deleted" {
		fmt.Fprintf(&b, "--- a/%s\n+++ /dev/null\n", old)
	} else {
		fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", old, new)
	}
	b.WriteString(h.Header + "\n")
	for _, l := range h.Lines {
		switch l.Kind {
		case "add":
			b.WriteString("+" + l.Text + "\n")
		case "del":
			b.WriteString("-" + l.Text + "\n")
		default:
			b.WriteString(" " + l.Text + "\n")
		}
	}
	return b.String()
}

func splitDiffGit(line string) (string, string) {
	rest := strings.TrimPrefix(line, "diff --git ")
	parts := strings.Fields(rest)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.TrimPrefix(parts[0], "a/"), strings.TrimPrefix(parts[1], "b/")
}

// parseHunkHeader llegeix "@@ -a,b +c,d @@ cua".
func parseHunkHeader(line string) (int, int, string) {
	head := line
	if i := strings.Index(line[2:], "@@"); i >= 0 {
		head = line[:i+4]
	}
	oldStart, newStart := 1, 1
	fields := strings.Fields(strings.Trim(head, "@ "))
	for _, f := range fields {
		num := func(s string) int {
			if i := strings.Index(s, ","); i >= 0 {
				s = s[:i]
			}
			n, _ := strconv.Atoi(s)
			if n == 0 {
				n = 1
			}
			return n
		}
		if strings.HasPrefix(f, "-") {
			oldStart = num(f[1:])
		}
		if strings.HasPrefix(f, "+") {
			newStart = num(f[1:])
		}
	}
	return oldStart, newStart, line
}

// GitApplyReverse reverteix un pegat (un hunk solt inclòs) sobre el
// working tree. És el "descarta aquest canvi" de la pestanya Canvis.
func GitApplyReverse(dir, patch string) error {
	if strings.TrimSpace(patch) == "" {
		return fmt.Errorf("pegat buit")
	}
	args := []string{}
	if patchTargetIsLF(dir, patch) {
		// Amb core.autocrlf=true (el que ve de sèrie al Git de Windows) git
		// reescriu tot el fitxer amb CRLF encara que el hunk només en toqui
		// una línia: descartar un canvi et deixava el fitxer sencer modificat.
		// Si ara mateix és LF pur, apaguem el filtre i només canvien les
		// línies del pegat.
		args = append(args, "-c", "core.autocrlf=false", "-c", "core.eol=lf")
	}
	args = append(args, "apply", "-R", "--unidiff-zero", "--whitespace=nowarn", "-")
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(patch)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("no s'ha pogut descartar: %s", msg)
	}
	return nil
}

// patchTargetIsLF diu si el fitxer que toca el pegat existeix i avui no té cap
// CRLF. Només llavors val la pena desactivar el filtre de finals de línia:
// si el fitxer ja és CRLF, el pegat (que git diff emet normalitzat a LF) s'hi
// ha d'aplicar amb el filtre posat o no hi encaixaria cap línia de context.
func patchTargetIsLF(dir, patch string) bool {
	rel := ""
	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			rel = strings.TrimSuffix(strings.TrimPrefix(line, "+++ b/"), "\r")
			break
		}
	}
	if rel == "" || rel == "/dev/null" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		return false
	}
	return !bytes.Contains(raw, []byte("\r\n"))
}

// GitStatusShort retorna `git status --short` (buit si no hi ha repo).
// La UI l'usa per posar la marca d'estat a cada fitxer de l'arbre.
func GitStatusShort(dir string) string {
	out, err := gitOut(dir, "status", "--short")
	if err != nil {
		return ""
	}
	return out
}
