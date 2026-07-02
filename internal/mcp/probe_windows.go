//go:build windows

package mcp

import (
	"io"
	"os/exec"
	"syscall"
)

func probeSysProcAttr() *syscall.SysProcAttr {
	return nil
}

func cleanupProcessGroup(cmd *exec.Cmd, stdin io.Closer, stdout io.Closer) {
	_ = stdin.Close()
	_ = stdout.Close()
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}
