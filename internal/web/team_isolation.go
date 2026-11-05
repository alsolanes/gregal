package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gregal/internal/agent"
	"gregal/internal/team"
)

type crewFileState struct {
	path   string
	base   string
	exists bool
}

type crewSeatWorkspace struct {
	dir       string
	files     []crewFileState
	integrate bool
	ready     bool
}

// CrewWorkspaces contains isolated private copies for a validated crew flow.
// The object is per-run and must be closed by its owner.
type CrewWorkspaces struct {
	root  string
	dir   string
	seats map[string]*crewSeatWorkspace
}

var crewProjectLocks sync.Map // canonical project path -> *sync.Mutex

func assignmentPath(root, raw string) (string, error) {
	if raw == "" || filepath.IsAbs(raw) || filepath.VolumeName(raw) != "" || strings.Contains(raw, "\\") {
		return "", fmt.Errorf("assigned path %q must be a normalized relative path", raw)
	}
	clean := filepath.Clean(raw)
	if clean == "." || !filepath.IsLocal(clean) || filepath.ToSlash(clean) != raw {
		return "", fmt.Errorf("assigned path %q must be a normalized relative path", raw)
	}
	cur := root
	parts := strings.Split(clean, string(filepath.Separator))
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("assigned path %q traverses a symlink", raw)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("assigned path %q has a non-directory parent", raw)
		}
		if i == len(parts)-1 && !info.Mode().IsRegular() && !info.IsDir() {
			return "", fmt.Errorf("assigned target %q is not a regular file", raw)
		}
		if i == len(parts)-1 && info.IsDir() {
			return "", fmt.Errorf("assigned target %q is a directory", raw)
		}
	}
	return clean, nil
}

// PrepareCrewAssignments validates ownership before any worker starts and
// creates one private project copy per assigned builder.
func PrepareCrewAssignments(projectRoot string, spec *team.FlowSpec) (*CrewWorkspaces, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("invalid crew project root")
	}
	w := &CrewWorkspaces{root: root, seats: map[string]*crewSeatWorkspace{}}
	if spec == nil {
		return w, nil
	}
	owners := map[string]string{}
	hasCrew := false
	for _, node := range spec.Nodes {
		hasCrew = hasCrew || node.CrewAssigned
	}
	if !hasCrew {
		return w, nil
	}
	for _, node := range spec.Nodes {
		if node.Role == "coordinator" {
			continue
		}
		seat := &crewSeatWorkspace{}
		seat.integrate = node.CrewAssigned
		w.seats[node.ID] = seat
		if !node.CrewAssigned {
			continue
		}
		if node.Role != "builder" {
			return nil, fmt.Errorf("crew assignment marker on non-builder node %q", node.ID)
		}
		if len(node.Files) == 0 {
			return nil, fmt.Errorf("assigned crew builder %q has no assigned files", node.ID)
		}
		for _, raw := range node.Files {
			rel, e := assignmentPath(root, raw)
			if e != nil {
				return nil, e
			}
			for owned, owner := range owners {
				if owned == rel || strings.HasPrefix(owned, rel+string(filepath.Separator)) || strings.HasPrefix(rel, owned+string(filepath.Separator)) {
					return nil, fmt.Errorf("assigned paths %q and %q overlap between %s and %s", owned, rel, owner, node.ID)
				}
			}
			if strings.HasPrefix(rel, ".git"+string(filepath.Separator)) || rel == ".git" || strings.HasPrefix(rel, ".gregal"+string(filepath.Separator)) || rel == ".gregal" || strings.HasPrefix(rel, ".hermes"+string(filepath.Separator)) || rel == ".hermes" {
				return nil, fmt.Errorf("assigned path %q is reserved", rel)
			}
			owners[rel] = node.ID
			base, exists, e := fileDigest(filepath.Join(root, rel))
			if e != nil {
				return nil, e
			}
			seat.files = append(seat.files, crewFileState{path: rel, base: base, exists: exists})
		}
	}
	if len(w.seats) == 0 {
		return w, nil
	}
	tmp, err := os.MkdirTemp("", "gregal-crew-")
	if err != nil {
		return nil, err
	}
	w.dir = tmp
	for _, seat := range w.seats {
		copyDir, e := os.MkdirTemp(tmp, "seat-")
		if e != nil {
			w.Close()
			return nil, e
		}
		seat.dir = copyDir
		if seat.integrate {
			if e = copyProject(root, copyDir); e != nil {
				w.Close()
				return nil, e
			}
			seat.ready = true
		}
	}
	return w, nil
}

func fileDigest(path string) (string, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), true, nil
}

func copyProject(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		base := d.Name()
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() && (base == ".git" || base == ".gregal" || base == ".hermes" || base == ".codex") {
			return filepath.SkipDir
		}
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if e = os.MkdirAll(filepath.Dir(to), 0o755); e != nil {
			return e
		}
		in, e := os.Open(path)
		if e != nil {
			return e
		}
		defer in.Close()
		out, e := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm()&0o777)
		if e != nil {
			return e
		}
		_, e = io.Copy(out, in)
		ce := out.Close()
		if e != nil {
			return e
		}
		return ce
	})
}

func (w *CrewWorkspaces) Enabled() bool { return w != nil && len(w.seats) > 0 }

func (w *CrewWorkspaces) Workspace(nodeID string) (string, bool, error) {
	if w == nil {
		return "", false, nil
	}
	s, ok := w.seats[nodeID]
	if !ok {
		return "", false, nil
	}
	if !s.ready {
		if err := copyProject(w.root, s.dir); err != nil {
			return "", false, err
		}
		s.ready = true
	}
	return s.dir, true, nil
}

// Integrate copies back only assigned regular files. It checks user denies
// and the original baseline while holding a project-scoped lock.
func (w *CrewWorkspaces) Integrate(nodeID string, denied func(string) bool, beforeCommit func(rel, digest string) error, committed func(rel string) error) error {
	if w == nil {
		return errors.New("crew workspace missing")
	}
	s, ok := w.seats[nodeID]
	if !ok {
		return fmt.Errorf("no crew workspace for node %q", nodeID)
	}
	if !s.integrate {
		return fmt.Errorf("node %q does not own assigned files", nodeID)
	}
	lockAny, _ := crewProjectLocks.LoadOrStore(w.root, &sync.Mutex{})
	lock := lockAny.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()
	original, err := os.OpenRoot(w.root)
	if err != nil {
		return err
	}
	defer original.Close()
	private, err := os.OpenRoot(s.dir)
	if err != nil {
		return err
	}
	defer private.Close()
	type stagedFile struct {
		state  *crewFileState
		data   []byte
		digest string
		mode   os.FileMode
	}
	var changed []stagedFile
	for i := range s.files {
		f := &s.files[i]
		if denied != nil && denied(f.path) {
			return fmt.Errorf("user permissions deny integration of %s", f.path)
		}
		if err := rejectRootSymlinks(original, f.path); err != nil {
			return err
		}
		cur, exists, err := rootDigest(original, f.path)
		if err != nil {
			return err
		}
		if exists != f.exists || cur != f.base {
			return fmt.Errorf("assigned file %s changed in the project while the crew was working", f.path)
		}
		if err := rejectRootSymlinks(private, f.path); err != nil {
			return err
		}
		info, err := private.Lstat(f.path)
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("assigned file %s was not created in the private workspace", f.path)
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("assigned target %s is no longer a regular file", f.path)
		}
	}
	for i := range s.files {
		f := &s.files[i]
		stagedInfo, err := private.Stat(f.path)
		if err != nil {
			return err
		}
		data, err := private.ReadFile(f.path)
		if err != nil {
			return err
		}
		stagedHash := sha256.Sum256(data)
		if f.exists && hex.EncodeToString(stagedHash[:]) == f.base {
			continue
		}
		changed = append(changed, stagedFile{state: f, data: data, digest: hex.EncodeToString(stagedHash[:]), mode: stagedInfo.Mode().Perm() & 0o777})
	}
	// Persist every intent before the first project rename. If state storage
	// fails, no assigned file is integrated.
	for _, file := range changed {
		if beforeCommit != nil {
			if err := beforeCommit(file.state.path, file.digest); err != nil {
				return err
			}
		}
	}
	for _, file := range changed {
		f := file.state
		if denied != nil && denied(f.path) {
			return fmt.Errorf("user permissions deny integration of %s", f.path)
		}
		if err := rejectRootSymlinks(original, f.path); err != nil {
			return err
		}
		cur, exists, err := rootDigest(original, f.path)
		if err != nil {
			return err
		}
		if exists != f.exists || cur != f.base {
			return fmt.Errorf("assigned file %s changed in the project while the crew was working", f.path)
		}
		if err := original.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
			return err
		}
		temp := filepath.Join(filepath.Dir(f.path), fmt.Sprintf(".crew-integrate-%d", time.Now().UnixNano()))
		tmp, err := original.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, file.mode)
		if err != nil {
			return err
		}
		if _, err = tmp.Write(file.data); err == nil {
			err = tmp.Sync()
		}
		ce := tmp.Close()
		if err != nil {
			_ = original.Remove(temp)
			return err
		}
		if ce != nil {
			_ = original.Remove(temp)
			return ce
		}
		if err := original.Rename(temp, f.path); err != nil {
			_ = original.Remove(temp)
			return err
		}
		if dir, e := original.Open(filepath.Dir(f.path)); e == nil {
			_ = dir.Sync()
			_ = dir.Close()
		}
		f.base, f.exists, err = rootDigest(original, f.path)
		if err != nil {
			return err
		}
		if committed != nil {
			if err := committed(f.path); err != nil {
				return err
			}
		}
	}
	return nil
}

func rejectRootSymlinks(root *os.Root, rel string) error {
	cur := ""
	for _, part := range strings.Split(filepath.Clean(rel), string(filepath.Separator)) {
		if cur == "" {
			cur = part
		} else {
			cur = filepath.Join(cur, part)
		}
		st, err := root.Lstat(cur)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("assigned path %q traverses a symlink", rel)
		}
	}
	return nil
}

func rootDigest(root *os.Root, rel string) (string, bool, error) {
	b, err := root.ReadFile(rel)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), true, nil
}

func (w *CrewWorkspaces) AssignedPaths(nodeID string) []string {
	if w == nil {
		return nil
	}
	s := w.seats[nodeID]
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.files))
	for _, f := range s.files {
		out = append(out, f.path)
	}
	return out
}

func (w *CrewWorkspaces) Close() error {
	if w == nil || w.dir == "" {
		return nil
	}
	err := os.RemoveAll(w.dir)
	w.dir = ""
	return err
}

type crewToolBoundary struct {
	workspace string
	readOnly  bool
	allowed   []string
}

func (b crewToolBoundary) execute(ctx context.Context, name, args string) (string, []string, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	if name == "bash" || name == "bash_background" {
		if name == "bash_background" {
			return "", nil, errors.New("background process management is unavailable in isolated crew seats")
		}
		var a struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return "", nil, err
		}
		return b.shell(ctx, name, a.Command)
	}
	allowed := map[string]bool{"read": true, "grep": true, "glob": true, "read_image": true, "write": true, "edit": true, "patch": true}
	if !allowed[name] {
		return "", nil, fmt.Errorf("tool %s is not available in an isolated crew seat", name)
	}
	if b.readOnly && (name == "write" || name == "edit" || name == "patch" || name == "bash_kill") {
		return "", nil, errors.New("read-only session denies this tool")
	}
	var target struct {
		Path    string   `json:"path"`
		Pattern string   `json:"pattern"`
		Paths   []string `json:"paths"`
		Dir     string   `json:"dir"`
	}
	if err := json.Unmarshal([]byte(args), &target); err != nil {
		return "", nil, err
	}
	paths := target.Paths
	if target.Dir != "" {
		paths = append(paths, target.Dir)
	}
	if target.Path != "" {
		paths = append(paths, target.Path)
	}
	if (target.Pattern == "" || name == "glob") && len(paths) == 0 {
		paths = []string{"."}
	}
	for _, p := range paths {
		if p == "" || filepath.IsAbs(p) || filepath.VolumeName(p) != "" || !filepath.IsLocal(filepath.Clean(p)) || strings.Contains(p, "\\") || filepath.ToSlash(filepath.Clean(p)) != p {
			return "", nil, fmt.Errorf("path %q is outside the private crew workspace", p)
		}
		if err := rejectSymlinkPath(b.workspace, p); err != nil {
			return "", nil, err
		}
	}
	if name == "glob" && target.Pattern != "" {
		pat := filepath.Clean(target.Pattern)
		if filepath.IsAbs(target.Pattern) || filepath.VolumeName(target.Pattern) != "" || strings.Contains(target.Pattern, "\\") || !filepath.IsLocal(pat) || pat == ".." || strings.HasPrefix(pat, ".."+string(filepath.Separator)) || containsDotDotComponent(target.Pattern) {
			return "", nil, fmt.Errorf("glob pattern %q is outside the private workspace", target.Pattern)
		}
		for _, part := range strings.Split(pat, string(filepath.Separator)) {
			if strings.ContainsAny(part, "*?[]") {
				continue
			}
			if err := rejectSymlinkPath(b.workspace, filepath.Join(target.Dir, part)); err != nil {
				return "", nil, err
			}
		}
	}
	if name == "write" || name == "edit" || name == "patch" {
		if len(b.allowed) == 0 || len(paths) != 1 || !teamFileAssignmentAllows(filepath.ToSlash(filepath.Clean(paths[0])), b.allowed) {
			return "", nil, errors.New("write denied: target is not an assigned file")
		}
	}
	return agent.ExecCrewTool(ctx, b.workspace, name, args)
}

func rejectSymlinkPath(root, rel string) error {
	clean := filepath.Clean(rel)
	if clean == "." {
		return nil
	}
	cur := root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path %q traverses a symlink", rel)
		}
	}
	return nil
}

func containsDotDotComponent(p string) bool {
	for _, part := range strings.Split(filepath.ToSlash(p), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func (b crewToolBoundary) shell(ctx context.Context, name, command string) (string, []string, error) {
	if command == "" {
		return "", nil, errors.New("empty shell command")
	}
	// Sense bubblewrap: a la còpia privada, sense aïllament (team_shell_copy.go).
	if !crewShellConfined() {
		if b.readOnly {
			return "", nil, errors.New("read-only session denies the shell")
		}
		return b.shellInCopy(ctx, command)
	}
	bwrap, err := exec.LookPath("bwrap")
	if err != nil {
		return "", nil, errors.New("crew shell isolation unavailable: bwrap not found")
	}
	bindMode := "--bind"
	if b.readOnly {
		bindMode = "--ro-bind"
	}
	args := []string{"--die-with-parent", "--new-session", "--unshare-net", "--unshare-pid", "--unshare-ipc", "--unshare-uts", "--tmpfs", "/"}
	// Construct a small runtime root. The host project and home directory are
	// never mounted; only system executables/libraries and selected Go caches
	// needed by local checks are visible read-only.
	for _, dir := range []string{"/usr", "/bin", "/sbin", "/lib", "/lib64"} {
		if st, e := os.Stat(dir); e == nil && st.IsDir() {
			args = append(args, "--dir", dir, "--ro-bind-try", dir, dir)
		}
	}
	args = append(args, "--dir", "/etc")
	for _, file := range []string{"/etc/ld.so.cache", "/etc/passwd", "/etc/group", "/etc/nsswitch.conf"} {
		if st, e := os.Stat(file); e == nil && st.Mode().IsRegular() {
			args = append(args, "--ro-bind-try", file, file)
		}
	}
	for _, dir := range []string{"/run", "/proc", "/dev", "/tmp", "/tmp/home", "/tmp/cache", "/tmp/work", "/opt", "/opt/go", "/opt/gomodcache"} {
		args = append(args, "--dir", dir)
	}
	args = append(args, "--proc", "/proc", "--dev", "/dev", bindMode, b.workspace, "/tmp/work")
	goBin, _ := exec.LookPath("go")
	goRoot, modCache := "", ""
	if goBin != "" {
		if out, e := exec.Command(goBin, "env", "GOROOT").Output(); e == nil {
			goRoot = strings.TrimSpace(string(out))
		}
		if out, e := exec.Command(goBin, "env", "GOMODCACHE").Output(); e == nil {
			modCache = strings.TrimSpace(string(out))
		}
	}
	if goRoot != "" {
		goRoot, _ = filepath.EvalSymlinks(goRoot)
		args = append(args, "--ro-bind-try", goRoot, "/opt/go")
	}
	if modCache != "" {
		if st, e := os.Stat(modCache); e == nil && st.IsDir() {
			args = append(args, "--ro-bind-try", modCache, "/opt/gomodcache")
		}
	}
	args = append(args, "--clearenv", "--setenv", "PATH", "/opt/go/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin", "--setenv", "GOROOT", "/opt/go", "--setenv", "HOME", "/tmp/home", "--setenv", "TMPDIR", "/tmp", "--setenv", "GOCACHE", "/tmp/cache", "--setenv", "GOMODCACHE", "/opt/gomodcache", "--setenv", "GOPATH", "/tmp/gopath", "--setenv", "GOTOOLCHAIN", "local", "--setenv", "GOPROXY", "off", "--setenv", "PYTHONDONTWRITEBYTECODE", "1", "--chdir", "/tmp/work", "--", "/bin/sh", "-c", command)
	if name == "bash_background" {
		return "", nil, errors.New("confined background process management is unavailable in this crew seat")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bwrap, args...)
	cmd.Dir = b.workspace
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin"}
	var outBuf limitedBuffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	err = cmd.Run()
	if err != nil {
		return outBuf.String(), nil, fmt.Errorf("confined shell failed: %w", err)
	}
	return outBuf.String(), nil, nil
}

type limitedBuffer struct{ b strings.Builder }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	room := 1<<20 - b.b.Len()
	if room > 0 {
		if n > room {
			_, _ = b.b.Write(p[:room])
		} else {
			_, _ = b.b.Write(p)
		}
	}
	return n, nil
}
func (b *limitedBuffer) String() string { return b.b.String() }

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func shellArgs(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		q[i] = shellQuote(a)
	}
	return strings.Join(q, " ")
}
