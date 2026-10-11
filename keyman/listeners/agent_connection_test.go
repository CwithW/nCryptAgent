package listeners

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh/agent"
)

type trackedAgentConn struct {
	io.ReadWriteCloser
	closes int32
}

func (c *trackedAgentConn) Close() error {
	atomic.AddInt32(&c.closes, 1)
	return c.ReadWriteCloser.Close()
}

func TestServeAgentConnectionClosesAfterClientDisconnect(t *testing.T) {
	for i := 0; i < 64; i++ {
		server, client := net.Pipe()
		conn := &trackedAgentConn{ReadWriteCloser: server}
		done := make(chan error, 1)
		go func() { done <- serveAgentConnection(agent.NewKeyring(), conn) }()
		client.SetDeadline(time.Now().Add(2 * time.Second))
		keys, err := agent.NewClient(client).List()
		client.Close()
		if err != nil || len(keys) != 0 {
			t.Fatalf("request %d: keys=%d, error=%v", i, len(keys), err)
		}
		select {
		case err := <-done:
			if !errors.Is(err, io.EOF) {
				t.Fatalf("request %d: disconnect error=%v", i, err)
			}
		case <-time.After(2 * time.Second):
			server.Close()
			t.Fatalf("request %d: handler did not exit", i)
		}
		if n := atomic.LoadInt32(&conn.closes); n != 1 {
			t.Fatalf("request %d: server connection closed %d times, want 1", i, n)
		}
	}
}

type brokenAgentConn struct {
	*bytes.Reader
	writeErr error
	closes   int
}

func (c *brokenAgentConn) Write(p []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(p), nil
}

func (c *brokenAgentConn) Close() error {
	c.closes++
	return nil
}

func TestServeAgentConnectionClosesOnProtocolError(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"zero length", []byte{0, 0, 0, 0}},
		{"oversized request", []byte{255, 255, 255, 255}},
		{"truncated request", []byte{0, 0, 0, 2, 11}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := &brokenAgentConn{Reader: bytes.NewReader(tc.data)}
			if err := serveAgentConnection(agent.NewKeyring(), conn); err == nil {
				t.Fatal("malformed request unexpectedly succeeded")
			}
			if conn.closes != 1 {
				t.Fatalf("connection closed %d times, want 1", conn.closes)
			}
		})
	}
}

func TestServeAgentConnectionClosesOnWriteError(t *testing.T) {
	want := errors.New("client stopped reading")
	conn := &brokenAgentConn{
		Reader:   bytes.NewReader([]byte{0, 0, 0, 1, 11}),
		writeErr: want,
	}
	if err := serveAgentConnection(agent.NewKeyring(), conn); !errors.Is(err, want) {
		t.Fatalf("error=%v, want %v", err, want)
	}
	if conn.closes != 1 {
		t.Fatalf("connection closed %d times, want 1", conn.closes)
	}
}
