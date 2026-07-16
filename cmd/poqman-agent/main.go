package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type AgentRequest struct {
	ID      int               `json:"id"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     []string          `json:"env,omitempty"`
	CWD     string            `json:"cwd,omitempty"`
	Signal  string            `json:"signal,omitempty"`
}

type AgentResponse struct {
	ID     int    `json:"id"`
	OK     bool   `json:"ok,omitempty"`
	Exit   int    `json:"exit,omitempty"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	Error  string `json:"error,omitempty"`
}

var (
	activeProcess  *os.Process
	activeProcessMu sync.Mutex
	stdoutBuf      []byte
	stderrBuf      []byte
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		var req AgentRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(0, fmt.Sprintf("parse request: %v", err))
			continue
		}

		switch req.Command {
		case "execute":
			handleExecute(&req)
		case "signal":
			handleSignal(&req)
		case "ping":
			handlePing(&req)
		default:
			sendError(req.ID, fmt.Sprintf("unknown command: %s", req.Command))
		}
	}
}

func handleExecute(req *AgentRequest) {
	stdoutBuf = nil
	stderrBuf = nil

	cmd := exec.Command(req.Args[0], req.Args[1:]...)

	cmd.Env = append(os.Environ(), req.Env...)
	if req.CWD != "" {
		cmd.Dir = req.CWD
	}

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		sendError(req.ID, fmt.Sprintf("start command: %v", err))
		return
	}

	activeProcessMu.Lock()
	activeProcess = cmd.Process
	activeProcessMu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		stdoutBuf, _ = io.ReadAll(stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		stderrBuf, _ = io.ReadAll(stderrPipe)
	}()

	wg.Wait()

	err := cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	activeProcessMu.Lock()
	activeProcess = nil
	activeProcessMu.Unlock()

	resp := AgentResponse{
		ID:     req.ID,
		Exit:   exitCode,
		Stdout: string(stdoutBuf),
		Stderr: string(stderrBuf),
	}
	sendResponse(&resp)
}

func handleSignal(req *AgentRequest) {
	activeProcessMu.Lock()
	proc := activeProcess
	activeProcessMu.Unlock()

	if proc == nil {
		sendError(req.ID, "no active process")
		return
	}

	sig := parseSignal(req.Signal)
	if err := proc.Signal(sig); err != nil {
		sendError(req.ID, fmt.Sprintf("send signal: %v", err))
		return
	}

	sendResponse(&AgentResponse{ID: req.ID, OK: true})
}

func handlePing(req *AgentRequest) {
	sendResponse(&AgentResponse{ID: req.ID, OK: true})
}

func sendResponse(resp *AgentResponse) {
	data, _ := json.Marshal(resp)
	os.Stdout.Write(append(data, '\n'))
}

func sendError(id int, message string) {
	resp := AgentResponse{ID: id, Error: message}
	data, _ := json.Marshal(resp)
	os.Stdout.Write(append(data, '\n'))
}

func parseSignal(name string) os.Signal {
	switch name {
	case "SIGTERM":
		return syscall.SIGTERM
	case "SIGINT":
		return syscall.SIGINT
	case "SIGKILL":
		return syscall.SIGKILL
	case "SIGHUP":
		return syscall.SIGHUP
	default:
		return syscall.SIGTERM
	}
}
