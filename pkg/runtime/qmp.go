package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type QMPClient struct {
	conn net.Conn
	scan *bufio.Scanner
}

type QMPMessage struct {
	Event     string         `json:"event,omitempty"`
	Return    json.RawMessage `json:"return,omitempty"`
	Error     *QMPError      `json:"error,omitempty"`
	Timestamp json.RawMessage `json:"timestamp,omitempty"`
	ID        int            `json:"id,omitempty"`
}

type QMPError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

func QMPConnect(socketPath string) (*QMPClient, error) {
	conn, err := net.DialTimeout("unix", socketPath, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to QMP socket: %w", err)
	}

	client := &QMPClient{
		conn: conn,
		scan: bufio.NewScanner(conn),
	}
	client.scan.Buffer(make([]byte, 4096), 1024*1024)

	if _, err := client.readGreeting(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("read QMP greeting: %w", err)
	}

	if err := client.Execute("qmp_capabilities", nil); err != nil {
		conn.Close()
		return nil, fmt.Errorf("QMP capabilities: %w", err)
	}

	return client, nil
}

func (c *QMPClient) readGreeting() (*QMPMessage, error) {
	if !c.scan.Scan() {
		return nil, fmt.Errorf("no QMP greeting: %v", c.scan.Err())
	}
	var msg QMPMessage
	if err := json.Unmarshal(c.scan.Bytes(), &msg); err != nil {
		return nil, fmt.Errorf("parse QMP greeting: %w", err)
	}
	return &msg, nil
}

func (c *QMPClient) Execute(command string, args map[string]any) error {
	req := map[string]any{
		"execute": command,
		"id":      time.Now().UnixNano(),
	}
	if args != nil {
		req["arguments"] = args
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal QMP command: %w", err)
	}

	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("send QMP command: %w", err)
	}

	if !c.scan.Scan() {
		return fmt.Errorf("no QMP response: %v", c.scan.Err())
	}

	var msg QMPMessage
	if err := json.Unmarshal(c.scan.Bytes(), &msg); err != nil {
		return fmt.Errorf("parse QMP response: %w", err)
	}

	if msg.Error != nil {
		return fmt.Errorf("QMP error: %s: %s", msg.Error.Class, msg.Error.Desc)
	}

	return nil
}

func (c *QMPClient) ExecuteAndWait(command string, args map[string]any) (*QMPMessage, error) {
	if err := c.Execute(command, args); err != nil {
		return nil, err
	}

	return c.readResponse()
}

func (c *QMPClient) readResponse() (*QMPMessage, error) {
	if !c.scan.Scan() {
		return nil, fmt.Errorf("no QMP event: %v", c.scan.Err())
	}

	var msg QMPMessage
	if err := json.Unmarshal(c.scan.Bytes(), &msg); err != nil {
		return nil, fmt.Errorf("parse QMP event: %w", err)
	}

	return &msg, nil
}

func (c *QMPClient) WaitForEvent(eventName string, timeout time.Duration) (*QMPMessage, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		c.conn.SetReadDeadline(deadline)
		msg, err := c.readResponse()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return nil, fmt.Errorf("timed out waiting for QMP event %q", eventName)
			}
			return nil, err
		}
		if msg.Event == eventName {
			return msg, nil
		}
	}

	return nil, fmt.Errorf("timed out waiting for QMP event %q", eventName)
}

func (c *QMPClient) PowerDown() error {
	return c.Execute("system_powerdown", nil)
}

func (c *QMPClient) Quit() error {
	return c.Execute("quit", nil)
}

func (c *QMPClient) QueryStatus() (string, error) {
	msg, err := c.ExecuteAndWait("query-status", nil)
	if err != nil {
		return "", err
	}
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(msg.Return, &status); err != nil {
		return "", fmt.Errorf("parse status: %w", err)
	}
	return status.Status, nil
}

func (c *QMPClient) Close() error {
	return c.conn.Close()
}
