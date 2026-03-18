// Package ipc provides Unix socket IPC between the daemon and CLI.
package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
)

// Message is the IPC request/response envelope.
type Message struct {
	Command string            `json:"command"`
	Args    map[string]string `json:"args,omitempty"`
	Data    json.RawMessage   `json:"data,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// Handler processes an IPC command and returns a response.
type Handler func(msg Message) Message

// Server listens on a Unix socket.
type Server struct {
	socketPath string
	handlers   map[string]Handler
	listener   net.Listener
}

// NewServer creates an IPC server.
func NewServer(socketPath string) *Server {
	return &Server{
		socketPath: socketPath,
		handlers:   make(map[string]Handler),
	}
}

// Register adds a command handler.
func (s *Server) Register(command string, h Handler) {
	s.handlers[command] = h
}

// Start begins listening and accepting connections.
func (s *Server) Start() error {
	// Remove stale socket
	_ = os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen unix %s: %w", s.socketPath, err)
	}
	s.listener = ln

	go s.accept()
	return nil
}

// Stop closes the listener.
func (s *Server) Stop() {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	_ = os.Remove(s.socketPath)
}

func (s *Server) accept() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "use of closed") {
				return
			}
			continue
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()

	var req Message
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		resp := Message{Error: "invalid JSON: " + err.Error()}
		_ = json.NewEncoder(conn).Encode(resp)
		return
	}

	h, ok := s.handlers[req.Command]
	if !ok {
		resp := Message{Error: "unknown command: " + req.Command}
		_ = json.NewEncoder(conn).Encode(resp)
		return
	}

	resp := h(req)
	_ = json.NewEncoder(conn).Encode(resp)
}
