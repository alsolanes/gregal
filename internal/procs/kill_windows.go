//go:build windows

package procs

import (
	"fmt"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsProcessGroup struct {
	mu       sync.Mutex
	job      windows.Handle
	assigned bool
}

var ntResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

func newProcessGroup(c *exec.Cmd) (processGroup, error) {
	g := &windowsProcessGroup{}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create process job: %w", err)
	}
	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("configure process job: %w", err)
	}
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	g.job = job
	return g, nil
}

func (g *windowsProcessGroup) start(c *exec.Cmd) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.job == 0 {
		return nil
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME,
		false, uint32(c.Process.Pid))
	if err != nil {
		return fmt.Errorf("open suspended process for job assignment: %w", err)
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(g.job, process); err != nil {
		return fmt.Errorf("assign process to job: %w", err)
	}
	g.assigned = true
	return resumeWindowsProcess(process)
}

func resumeWindowsProcess(process windows.Handle) error {
	if err := ntResumeProcess.Find(); err != nil {
		return err
	}
	status, _, _ := ntResumeProcess.Call(uintptr(process))
	if int32(status) < 0 {
		return fmt.Errorf("NtResumeProcess failed with status %#x", uint32(status))
	}
	return nil
}

func (g *windowsProcessGroup) kill(c *exec.Cmd) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.assigned && g.job != 0 {
		if err := windows.TerminateJobObject(g.job, 1); err == nil {
			return
		}
	}
	if c.Process != nil {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(c.Process.Pid)).Run()
		_ = c.Process.Kill()
	}
}

func (g *windowsProcessGroup) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.job != 0 {
		_ = windows.CloseHandle(g.job)
		g.job = 0
	}
}
