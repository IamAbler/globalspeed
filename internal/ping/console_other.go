//go:build !windows

package ping

import "os/exec"

func hideConsole(cmd *exec.Cmd) {}
