//go:build windows

package pty

import (
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

type conPTY struct {
	hpc  windows.Handle
	proc windows.Handle
	pid  int
	in   *os.File // escrivim aquí: arriba a la consola com a tecles
	out  *os.File // llegim d'aquí: el que pinta la consola, en VT

	closeOnce sync.Once
	waitOnce  sync.Once
	exit      int
	waitErr   error
}

// Start engega spec dins d'una pseudoconsola (Windows 10 1809 o posterior).
func Start(spec Spec) (PTY, error) {
	cols, rows := clamp(spec.Cols, spec.Rows)
	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		return nil, err
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		windows.CloseHandle(inR)
		windows.CloseHandle(inW)
		return nil, err
	}
	var hpc windows.Handle
	if err := windows.CreatePseudoConsole(windows.Coord{X: int16(cols), Y: int16(rows)}, inR, outW, 0, &hpc); err != nil {
		for _, h := range []windows.Handle{inR, inW, outR, outW} {
			windows.CloseHandle(h)
		}
		return nil, err
	}
	// La consola ja té els seus extrems; els nostres sobren.
	windows.CloseHandle(inR)
	windows.CloseHandle(outW)
	fail := func(err error) (PTY, error) {
		windows.ClosePseudoConsole(hpc)
		windows.CloseHandle(inW)
		windows.CloseHandle(outR)
		return nil, err
	}

	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return fail(err)
	}
	defer attrs.Delete()
	// El valor de l'atribut és el HPCON mateix, no un punter a ell.
	if err := attrs.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, *(*unsafe.Pointer)(unsafe.Pointer(&hpc)), unsafe.Sizeof(hpc)); err != nil {
		return fail(err)
	}
	si := windows.StartupInfoEx{}
	si.Cb = uint32(unsafe.Sizeof(si))
	si.ProcThreadAttributeList = attrs.List()
	// Amb la sortida del pare redirigida (el servidor sota l'app d'escriptori,
	// els tests), el fill l'heretaria i escriuria fora de la consola. Amb
	// handles nuls explícits, fa servir els de la pseudoconsola.
	si.Flags |= windows.STARTF_USESTDHANDLES

	argv := append([]string{spec.Path}, spec.Args...)
	cmdLine, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(argv))
	if err != nil {
		return fail(err)
	}
	app, err := windows.UTF16PtrFromString(spec.Path)
	if err != nil {
		return fail(err)
	}
	var dir *uint16
	if spec.Dir != "" {
		if dir, err = windows.UTF16PtrFromString(spec.Dir); err != nil {
			return fail(err)
		}
	}
	env := envBlock(spec.Env)
	var envPtr *uint16
	if len(env) > 0 {
		envPtr = &env[0]
	}
	var pi windows.ProcessInformation
	// Suspès fins que és dins del job: res del que engegui se n'escapa.
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT | windows.CREATE_SUSPENDED)
	if err := windows.CreateProcess(app, cmdLine, nil, nil, false, flags, envPtr, dir, &si.StartupInfo, &pi); err != nil {
		return fail(err)
	}
	if job := killOnCloseJob(); job != 0 {
		_ = windows.AssignProcessToJobObject(job, pi.Process)
	}
	_, _ = windows.ResumeThread(pi.Thread)
	windows.CloseHandle(pi.Thread)
	return &conPTY{
		hpc:  hpc,
		proc: pi.Process,
		pid:  int(pi.ProcessId),
		in:   os.NewFile(uintptr(inW), "conpty-in"),
		out:  os.NewFile(uintptr(outR), "conpty-out"),
	}, nil
}

var (
	jobOnce sync.Once
	job     windows.Handle
)

// killOnCloseJob és un job amb KILL_ON_JOB_CLOSE que només té aquest
// procés: si el servidor mor de cop (sense passar per Kill), Windows tanca
// el handle i s'emporta els TUIs i el que hagin engegat, en comptes de
// deixar-los orfes. 0 si no es pot crear.
func killOnCloseJob() windows.Handle {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			windows.CloseHandle(h)
			return
		}
		job = h
	})
	return job
}

// envBlock fa el bloc d'entorn UTF-16 que demana CreateProcess: ordenat,
// KEY=valor separats per zeros i un zero doble al final.
func envBlock(env []string) []uint16 {
	if len(env) == 0 {
		return nil
	}
	sorted := append([]string(nil), env...)
	sort.Slice(sorted, func(i, j int) bool { return strings.ToUpper(sorted[i]) < strings.ToUpper(sorted[j]) })
	var out []uint16
	for _, kv := range sorted {
		if strings.IndexByte(kv, 0) >= 0 {
			continue
		}
		out = append(out, utf16.Encode([]rune(kv))...)
		out = append(out, 0)
	}
	return append(out, 0)
}

func (p *conPTY) Read(b []byte) (int, error)  { return p.out.Read(b) }
func (p *conPTY) Write(b []byte) (int, error) { return p.in.Write(b) }
func (p *conPTY) Pid() int                    { return p.pid }

func (p *conPTY) Resize(cols, rows int) error {
	cols, rows = clamp(cols, rows)
	return windows.ResizePseudoConsole(p.hpc, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (p *conPTY) Kill() error {
	err := windows.TerminateProcess(p.proc, 1)
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		// Ja ha acabat.
		return nil
	}
	return err
}

func (p *conPTY) Wait() (int, error) {
	p.waitOnce.Do(func() {
		if _, err := windows.WaitForSingleObject(p.proc, windows.INFINITE); err != nil {
			p.waitErr = err
			return
		}
		var code uint32
		if err := windows.GetExitCodeProcess(p.proc, &code); err != nil {
			p.waitErr = err
			return
		}
		p.exit = int(code)
	})
	return p.exit, p.waitErr
}

// Close tanca la pseudoconsola. ClosePseudoConsole espera que algú buidi
// la sortida: qui llegeix ha de continuar llegint fins a l'EOF, per això
// es fa en segon pla i no bloqueja qui crida.
func (p *conPTY) Close() error {
	p.closeOnce.Do(func() {
		_ = p.in.Close()
		go func() {
			windows.ClosePseudoConsole(p.hpc)
			_ = p.out.Close()
			windows.CloseHandle(p.proc)
		}()
	})
	return nil
}
