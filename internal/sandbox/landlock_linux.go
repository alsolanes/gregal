//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// landlockABI torna la versió d'ABI de Landlock del kernel (0 = no n'hi ha).
func landlockABI() int {
	v, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	if errno != 0 {
		return 0
	}
	return int(v)
}

// Drets de fitxer per ABI (vegeu include/uapi/linux/landlock.h).
const (
	llExecute    = 1 << 0
	llWriteFile  = 1 << 1
	llReadFile   = 1 << 2
	llReadDir    = 1 << 3
	llRemoveDir  = 1 << 4
	llRemoveFile = 1 << 5
	llMakeChar   = 1 << 6
	llMakeDir    = 1 << 7
	llMakeReg    = 1 << 8
	llMakeSock   = 1 << 9
	llMakeFifo   = 1 << 10
	llMakeBlock  = 1 << 11
	llMakeSym    = 1 << 12
	llRefer      = 1 << 13 // ABI 2
	llTruncate   = 1 << 14 // ABI 3
	llIoctlDev   = 1 << 15 // ABI 5
)

func fsHandled(abi int) uint64 {
	v := uint64(llExecute | llWriteFile | llReadFile | llReadDir | llRemoveDir | llRemoveFile |
		llMakeChar | llMakeDir | llMakeReg | llMakeSock | llMakeFifo | llMakeBlock | llMakeSym)
	if abi >= 2 {
		v |= llRefer
	}
	if abi >= 3 {
		v |= llTruncate
	}
	if abi >= 5 {
		v |= llIoctlDev
	}
	return v
}

// RunHelper aplica Landlock i fa exec de l'ordre. No torna si va bé; si no,
// escriu l'error i torna el codi de sortida.
func RunHelper(args []string) int {
	spec, ok := ParseHelperArgs(args)
	if !ok {
		fmt.Fprintln(os.Stderr, "sandbox helper: bad arguments")
		return 2
	}
	if err := aplicaLandlock(spec); err != nil {
		fmt.Fprintln(os.Stderr, "sandbox helper:", err)
		return 126
	}
	bin, err := exec.LookPath(spec.Argv[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "sandbox helper:", err)
		return 127
	}
	err = syscall.Exec(bin, spec.Argv, os.Environ())
	fmt.Fprintln(os.Stderr, "sandbox helper: exec:", err)
	return 126
}

func aplicaLandlock(spec HelperSpec) error {
	abi := landlockABI()
	if abi < 1 {
		return fmt.Errorf("landlock unavailable")
	}
	// La restricció és del fil: el mateix fil ha de restringir-se i fer
	// l'exec.
	runtime.LockOSThread()
	handled := fsHandled(abi)
	attr := unix.LandlockRulesetAttr{Handled_access_fs: handled}
	size := unsafe.Sizeof(attr.Handled_access_fs)
	if abi >= 4 {
		size = unsafe.Sizeof(attr.Handled_access_fs) + unsafe.Sizeof(attr.Handled_access_net)
		if spec.NetworkOff {
			attr.Handled_access_net = unix.LANDLOCK_ACCESS_NET_BIND_TCP | unix.LANDLOCK_ACCESS_NET_CONNECT_TCP
		}
	} else if spec.NetworkOff {
		return fmt.Errorf("landlock ABI %d cannot block the network", abi)
	}
	fd, _, errno := unix.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(&attr)), size, 0)
	if errno != 0 {
		return fmt.Errorf("landlock_create_ruleset: %v", errno)
	}
	ruleset := int(fd)
	defer unix.Close(ruleset)
	lectura := uint64(llExecute | llReadFile | llReadDir)
	if err := afegeixRegla(ruleset, "/", lectura); err != nil {
		return err
	}
	// /dev: /dev/null, /dev/tty... s'han de poder escriure.
	_ = afegeixRegla(ruleset, "/dev", lectura|llWriteFile|(handled&llIoctlDev))
	for _, p := range spec.Writable {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := afegeixRegla(ruleset, p, handled); err != nil {
			return err
		}
	}
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("no_new_privs: %v", err)
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, uintptr(ruleset), 0, 0); errno != 0 {
		return fmt.Errorf("landlock_restrict_self: %v", errno)
	}
	return nil
}

func afegeixRegla(ruleset int, path string, access uint64) error {
	fd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open %s: %v", path, err)
	}
	defer unix.Close(fd)
	// Un fitxer (no directori) només admet drets de fitxer.
	var st unix.Stat_t
	if unix.Fstat(fd, &st) == nil && st.Mode&unix.S_IFMT != unix.S_IFDIR {
		access &= llExecute | llWriteFile | llReadFile | llTruncate | llIoctlDev
	}
	rule := unix.LandlockPathBeneathAttr{Allowed_access: access, Parent_fd: int32(fd)}
	if _, _, errno := unix.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, uintptr(ruleset), unix.LANDLOCK_RULE_PATH_BENEATH, uintptr(unsafe.Pointer(&rule)), 0, 0, 0); errno != 0 {
		return fmt.Errorf("landlock_add_rule %s: %v", path, errno)
	}
	return nil
}
