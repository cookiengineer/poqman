package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Process struct {
	cmd    *exec.Cmd
	PID    int
	PIDFile string
}

func StartProcess(binary string, args []string, pidFile string) (*Process, error) {
	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start QEMU: %w", err)
	}

	proc := &Process{
		cmd:     cmd,
		PID:     cmd.Process.Pid,
		PIDFile: pidFile,
	}

	return proc, nil
}

func StartProcessDetached(binary string, args []string, pidFile string, consoleLog string) (*Process, error) {
	logFile, err := os.OpenFile(consoleLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open console log: %w", err)
	}

	cmd := exec.Command(binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("start QEMU: %w", err)
	}

	proc := &Process{
		cmd:     cmd,
		PID:     cmd.Process.Pid,
		PIDFile: pidFile,
	}

	go func() {
		cmd.Wait()
		logFile.Close()
	}()

	return proc, nil
}

func (p *Process) Wait() error {
	if p.cmd == nil {
		return fmt.Errorf("process not started")
	}
	return p.cmd.Wait()
}

func (p *Process) WaitWithTimeout(timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		p.Kill()
		return fmt.Errorf("process did not exit within %s, killed", timeout)
	}
}

func (p *Process) Signal(sig os.Signal) error {
	if p.cmd == nil || p.cmd.Process == nil {
		return fmt.Errorf("process not started")
	}
	return p.cmd.Process.Signal(sig)
}

func (p *Process) Kill() error {
	if p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	pgid, err := syscall.Getpgid(p.PID)
	if err == nil {
		syscall.Kill(-pgid, syscall.SIGKILL)
	}

	return p.cmd.Process.Kill()
}

func (p *Process) IsRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	process, err := os.FindProcess(p.PID)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func ReadPIDFromFile(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, fmt.Errorf("read pidfile: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid: %w", err)
	}
	return pid, nil
}
