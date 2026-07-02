//go:build !windows

package mcp

import (
	"io"
	"os/exec"
	"syscall"
	"time"
)

func probeSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func cleanupProcessGroup(cmd *exec.Cmd, stdin io.Closer, stdout io.Closer) {
	_ = stdin.Close()
	_ = stdout.Close()
	if cmd.Process == nil {
		return
	}
	pgid := cmd.Process.Pid
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	grace := time.NewTimer(200 * time.Millisecond)
	defer grace.Stop()

	select {
	case <-done:
		if processGroupExists(pgid) {
			<-grace.C
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
	case <-grace.C:
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		<-done
	}
}

func processGroupExists(pgid int) bool {
	err := syscall.Kill(-pgid, 0)
	return err == nil || err == syscall.EPERM
}
