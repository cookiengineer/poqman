package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

type AgentRequest struct {
	ID      int      `json:"id"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	CWD     string   `json:"cwd,omitempty"`
	Signal  string   `json:"signal,omitempty"`
}

type AgentResponse struct {
	ID     int    `json:"id"`
	OK     bool   `json:"ok,omitempty"`
	Exit   int    `json:"exit,omitempty"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	Error  string `json:"error,omitempty"`
}

type AgentClient struct {
	conn  net.Conn
	scan  *bufio.Scanner
	idSeq int
	mu    sync.Mutex
}

func AgentConnect(socketPath string) (*AgentClient, error) {
	conn, err := net.DialTimeout("unix", socketPath, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to agent socket: %w", err)
	}

	return &AgentClient{
		conn: conn,
		scan: bufio.NewScanner(conn),
	}, nil
}

func (c *AgentClient) Execute(args []string, env []string, cwd string) (int, string, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.idSeq++
	req := AgentRequest{
		ID:      c.idSeq,
		Command: "execute",
		Args:    args,
		Env:     env,
		CWD:     cwd,
	}

	resp, err := c.sendAndReceive(&req)
	if err != nil {
		return -1, "", "", err
	}

	if resp.Error != "" {
		return -1, "", "", fmt.Errorf("agent error: %s", resp.Error)
	}

	return resp.Exit, resp.Stdout, resp.Stderr, nil
}

func (c *AgentClient) Signal(sig string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.idSeq++
	req := AgentRequest{
		ID:      c.idSeq,
		Command: "signal",
		Signal:  sig,
	}

	resp, err := c.sendAndReceive(&req)
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return fmt.Errorf("agent error: %s", resp.Error)
	}
	if !resp.OK {
		return fmt.Errorf("signal failed")
	}

	return nil
}

func (c *AgentClient) Ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.idSeq++
	req := AgentRequest{
		ID:      c.idSeq,
		Command: "ping",
	}

	resp, err := c.sendAndReceive(&req)
	if err != nil {
		return err
	}

	if resp.Error != "" {
		return fmt.Errorf("agent error: %s", resp.Error)
	}
	if !resp.OK {
		return fmt.Errorf("ping failed")
	}

	return nil
}

func (c *AgentClient) sendAndReceive(req *AgentRequest) (*AgentResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if !c.scan.Scan() {
		return nil, fmt.Errorf("no response: %v", c.scan.Err())
	}

	var resp AgentResponse
	if err := json.Unmarshal(c.scan.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &resp, nil
}

func (c *AgentClient) Close() error {
	return c.conn.Close()
}
