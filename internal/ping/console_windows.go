//go:build windows

package ping

import (
	"os/exec"
	"syscall"
)

func hideConsole(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }
