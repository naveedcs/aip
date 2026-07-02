package activation

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"syscall"
)

type ExitCodeError struct {
	Code int
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("process exited with code %d", e.Code)
}

func Run(plan Plan) error {
	cmd := exec.Command(plan.Command, plan.Args...)
	cmd.Dir = plan.Dir
	cmd.Env = envList(plan.Env)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return mapExitCode(cmd.Run())
}

func RunRaw(command string, args []string) error {
	cmd := exec.Command(command, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return mapExitCode(cmd.Run())
}

func FormatPlan(w io.Writer, plan Plan) {
	fmt.Fprintf(w, "Command: %s", plan.Command)
	for _, arg := range plan.Args {
		fmt.Fprintf(w, " %s", arg)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Working directory: %s\n", plan.Dir)
	fmt.Fprintln(w, "Environment:")

	keys := make([]string, 0, len(plan.MaskedEnv))
	for key := range plan.MaskedEnv {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(w, "  %s=%s\n", key, plan.MaskedEnv[key])
	}
}

func envList(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+values[key])
	}
	return out
}

func mapExitCode(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return &ExitCodeError{Code: 128 + int(status.Signal())}
		}
		return &ExitCodeError{Code: exitErr.ExitCode()}
	}
	return err
}
