package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
)

// PRSetChildSubreaper is the value of PR_SET_CHILD_SUBREAPER in prctl(2)
const PRSetChildSubreaper = 36

// ExecCmd executes a command with args and returns its output as a string along
// with an error, if any
func ExecCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("`%v %v` failed: %v (%v)", name, strings.Join(args, " "), stderr.String(), err)
	}

	return stdout.String(), nil
}

// ExecCmdWithStdStreams execute a command with the specified standard streams.
func ExecCmdWithStdStreams(stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("`%v %v` failed: %v", name, strings.Join(args, " "), err)
	}

	return nil
}

// SetSubreaper sets the value i as the subreaper setting for the calling process
func SetSubreaper(i int) error {
	return Prctl(PRSetChildSubreaper, uintptr(i), 0, 0, 0)
}

// Prctl is a way to make the prctl linux syscall
func Prctl(option int, arg2, arg3, arg4, arg5 uintptr) (err error) {
	_, _, e1 := syscall.Syscall6(syscall.SYS_PRCTL, uintptr(option), arg2, arg3, arg4, arg5, 0)
	if e1 != 0 {
		err = e1
	}
	return
}

// StartReaper starts a goroutine to reap processes
func StartReaper() {
	logrus.Infof("Starting reaper")
	go func() {
		sigs := make(chan os.Signal, 10)
		signal.Notify(sigs, syscall.SIGCHLD)
		for {
			// Wait for a child to terminate
			sig := <-sigs
			for {
				// Reap processes
				var status syscall.WaitStatus
				cpid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)
				if err != nil {
					if err != syscall.ECHILD {
						logrus.Debugf("wait4 after %v: %v", sig, err)
					}
					break
				}
				if cpid < 1 {
					break
				}
				if status.Exited() {
					logrus.Debugf("Reaped process with pid %d, exited with status %d", cpid, status.ExitStatus())
				} else if status.Signaled() {
					logrus.Debugf("Reaped process with pid %d, exited on %s", cpid, status.Signal())
				} else {
					logrus.Debugf("Reaped process with pid %d", cpid)
				}
			}
		}
	}()
}

// StatusToExitCode converts wait status code to an exit code
func StatusToExitCode(status int) int {
	return ((status) & 0xff00) >> 8
}
