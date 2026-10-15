//go:build linux || darwin

package llm

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func isolateCodexProcess(cmd *exec.Cmd) error {
	// The npm launcher spawns a native child; killing only the launcher leaks requests.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return stopCodexProcess(cmd) }
	cmd.WaitDelay = time.Second
	return nil
}
func stopCodexProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if err == syscall.ESRCH {
		return os.ErrProcessDone
	}
	return err
}
