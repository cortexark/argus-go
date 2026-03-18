package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client sends commands to the daemon over a Unix socket.
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates an IPC client.
func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath, timeout: 5 * time.Second}
}

// Send sends a command and returns the response.
func (c *Client) Send(command string, args map[string]string) (Message, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return Message{}, fmt.Errorf("daemon not running (connect: %w)", err)
	}
	defer conn.Close()

	req := Message{Command: command, Args: args}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Message{}, fmt.Errorf("send: %w", err)
	}

	var resp Message
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Message{}, fmt.Errorf("recv: %w", err)
	}
	if resp.Error != "" {
		return resp, fmt.Errorf("daemon error: %s", resp.Error)
	}
	return resp, nil
}

// IsRunning returns true if the daemon socket is accessible.
func (c *Client) IsRunning() bool {
	conn, err := net.DialTimeout("unix", c.socketPath, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
