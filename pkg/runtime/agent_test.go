package runtime

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestAgentRequest_Marshal(t *testing.T) {
	req := AgentRequest{
		ID:      1,
		Command: "execute",
		Args:    []string{"cat", "/etc/hosts"},
		Env:     []string{"PATH=/usr/bin"},
		CWD:     "/",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed AgentRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.ID != 1 {
		t.Errorf("expected ID 1, got %d", parsed.ID)
	}
	if parsed.Command != "execute" {
		t.Errorf("expected command 'execute', got %s", parsed.Command)
	}
	if len(parsed.Args) != 2 || parsed.Args[0] != "cat" {
		t.Errorf("unexpected args: %v", parsed.Args)
	}
	if parsed.CWD != "/" {
		t.Errorf("expected CWD '/', got %s", parsed.CWD)
	}
}

func TestAgentResponse_Marshal(t *testing.T) {
	resp := AgentResponse{
		ID:     1,
		Exit:   0,
		Stdout: "hello\n",
		Stderr: "",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed AgentResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.Exit != 0 {
		t.Errorf("expected exit 0, got %d", parsed.Exit)
	}
	if parsed.Stdout != "hello\n" {
		t.Errorf("expected stdout 'hello\\n', got %q", parsed.Stdout)
	}
}

func TestAgentResponse_Error(t *testing.T) {
	resp := AgentResponse{
		ID:    2,
		Error: "command not found",
	}

	data, _ := json.Marshal(resp)
	var parsed AgentResponse
	json.Unmarshal(data, &parsed)

	if parsed.Error != "command not found" {
		t.Errorf("expected error, got %s", parsed.Error)
	}
}

func TestAgentRequest_Signal(t *testing.T) {
	req := AgentRequest{
		ID:      3,
		Command: "signal",
		Signal:  "SIGTERM",
	}

	data, _ := json.Marshal(req)
	var parsed AgentRequest
	json.Unmarshal(data, &parsed)

	if parsed.Signal != "SIGTERM" {
		t.Errorf("expected SIGTERM, got %s", parsed.Signal)
	}
}

func TestAgentRequest_Ping(t *testing.T) {
	req := AgentRequest{
		ID:      4,
		Command: "ping",
	}

	data, _ := json.Marshal(req)
	var parsed AgentRequest
	json.Unmarshal(data, &parsed)

	if parsed.Command != "ping" {
		t.Errorf("expected 'ping', got %s", parsed.Command)
	}
}

func TestAgentClient_Execute(t *testing.T) {
	server, client := agentPipe()

	go func() {
		scanner := bufio.NewScanner(server)
		for scanner.Scan() {
			var req AgentRequest
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}

			resp := AgentResponse{
				ID:     req.ID,
				OK:     true,
				Exit:   0,
				Stdout: "output from " + req.Args[0],
			}
			data, _ := json.Marshal(resp)
			server.Write(append(data, '\n'))
		}
	}()

	ac := &AgentClient{
		conn: client,
		scan: bufio.NewScanner(client),
	}

	exitCode, stdout, stderr, err := ac.Execute([]string{"echo", "hello"}, nil, "/tmp")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit 0, got %d", exitCode)
	}
	if stdout != "output from echo" {
		t.Errorf("expected 'output from echo', got %q", stdout)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}
}

func TestAgentClient_Ping(t *testing.T) {
	server, client := agentPipe()

	go func() {
		scanner := bufio.NewScanner(server)
		for scanner.Scan() {
			var req AgentRequest
			json.Unmarshal(scanner.Bytes(), &req)
			resp := AgentResponse{ID: req.ID, OK: true}
			data, _ := json.Marshal(resp)
			server.Write(append(data, '\n'))
		}
	}()

	ac := &AgentClient{
		conn: client,
		scan: bufio.NewScanner(client),
	}

	if err := ac.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestAgentClient_Signal(t *testing.T) {
	server, client := agentPipe()

	go func() {
		scanner := bufio.NewScanner(server)
		for scanner.Scan() {
			var req AgentRequest
			json.Unmarshal(scanner.Bytes(), &req)
			ok := false
			if req.Command == "signal" && req.Signal == "SIGTERM" {
				ok = true
			}
			resp := AgentResponse{ID: req.ID, OK: ok}
			data, _ := json.Marshal(resp)
			server.Write(append(data, '\n'))
		}
	}()

	ac := &AgentClient{
		conn: client,
		scan: bufio.NewScanner(client),
	}

	if err := ac.Signal("SIGTERM"); err != nil {
		t.Fatalf("Signal: %v", err)
	}
}

func TestAgentClient_ExecuteError(t *testing.T) {
	server, client := agentPipe()

	go func() {
		scanner := bufio.NewScanner(server)
		for scanner.Scan() {
			var req AgentRequest
			json.Unmarshal(scanner.Bytes(), &req)
			resp := AgentResponse{ID: req.ID, Error: "file not found"}
			data, _ := json.Marshal(resp)
			server.Write(append(data, '\n'))
		}
	}()

	ac := &AgentClient{
		conn: client,
		scan: bufio.NewScanner(client),
	}

	_, _, _, err := ac.Execute([]string{"nonexistent"}, nil, "")
	if err == nil {
		t.Error("expected error for agent error response")
	}
}

func TestAgentClient_Close(t *testing.T) {
	server, client := agentPipe()
	server.Close()

	ac := &AgentClient{
		conn: client,
		scan: bufio.NewScanner(client),
	}

	err := ac.Close()
	if err != nil {
		t.Errorf("close should not error on already-closed pipe: %v", err)
	}
}

func agentPipe() (net.Conn, net.Conn) {
	server, client := net.Pipe()
	return server, client
}

func TestAgentConnect_UnixSocket(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := tmpDir + "/agent.sock"

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scanner := bufio.NewScanner(conn)
		if scanner.Scan() {
			var req AgentRequest
			json.Unmarshal(scanner.Bytes(), &req)
			resp := AgentResponse{ID: req.ID, OK: true}
			data, _ := json.Marshal(resp)
			conn.Write(append(data, '\n'))
		}
	}()

	time.Sleep(10 * time.Millisecond)

	client, err := AgentConnect(socketPath)
	if err != nil {
		ln.Close()
		t.Fatalf("AgentConnect: %v", err)
	}

	if err := client.Ping(); err != nil {
		client.Close()
		ln.Close()
		t.Fatalf("Ping: %v", err)
	}

	client.Close()
	ln.Close()
	<-done
}

func TestAgentConnect_NoSocket(t *testing.T) {
	_, err := AgentConnect("/nonexistent/path/agent.sock")
	if err == nil {
		t.Error("expected error for nonexistent socket")
	}
}
