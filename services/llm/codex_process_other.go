//go:build !linux && !darwin

package llm

import (
	"errors"
	"os/exec"
)

func isolateCodexProcess(*exec.Cmd) error {
	return errors.New("Codex Bot integration requires Linux or macOS")
}
func stopCodexProcess(cmd *exec.Cmd) error { return cmd.Process.Kill() }
