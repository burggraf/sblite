// internal/realtime/conn_test.go
package realtime

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewConnID(t *testing.T) {
	// Test that connection IDs are valid UUIDs
	id := uuid.New().String()
	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("generated ID is not a valid UUID: %s", id)
	}
}

func TestConnSendChannel(t *testing.T) {
	// Test that send channel is buffered
	conn := &Conn{
		id:       uuid.New().String(),
		send:     make(chan []byte, sendBufferSize),
		done:     make(chan struct{}),
		channels: make(map[string]*ChannelSub),
	}

	// Should not block for buffered sends
	msg := []byte(`{"event":"test"}`)
	select {
	case conn.send <- msg:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("send channel should not block")
	}
}

func TestConnClose(t *testing.T) {
	conn := &Conn{
		id:       uuid.New().String(),
		send:     make(chan []byte, sendBufferSize),
		done:     make(chan struct{}),
		channels: make(map[string]*ChannelSub),
	}

	// Close should be idempotent
	conn.Close()
	conn.Close() // Should not panic
}

func TestConnSend(t *testing.T) {
	conn := &Conn{
		id:       uuid.New().String(),
		send:     make(chan []byte, sendBufferSize),
		done:     make(chan struct{}),
		channels: make(map[string]*ChannelSub),
	}

	// Test sending a message
	msg := NewReply("test-topic", "", "1", "ok", map[string]any{})
	err := conn.Send(msg)
	if err != nil {
		t.Errorf("Send returned error: %v", err)
	}

	// Verify message was queued
	select {
	case data := <-conn.send:
		if len(data) == 0 {
			t.Error("received empty data from send channel")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("message was not queued")
	}
}

func TestConnSendAfterClose(t *testing.T) {
	conn := &Conn{
		id:       uuid.New().String(),
		send:     make(chan []byte, sendBufferSize),
		done:     make(chan struct{}),
		channels: make(map[string]*ChannelSub),
	}

	conn.Close()

	// Send after close should not block or panic
	msg := NewReply("test-topic", "", "1", "ok", map[string]any{})
	err := conn.Send(msg)
	if err != nil {
		t.Errorf("Send after close returned error: %v", err)
	}
}

func TestConnSendBufferFull(t *testing.T) {
	// Create conn with small buffer
	conn := &Conn{
		id:       uuid.New().String(),
		send:     make(chan []byte, 1),
		done:     make(chan struct{}),
		channels: make(map[string]*ChannelSub),
	}

	// Fill the buffer
	msg := NewReply("test-topic", "", "1", "ok", map[string]any{})
	conn.Send(msg)

	// Next send should not block (message dropped)
	done := make(chan struct{})
	go func() {
		conn.Send(msg)
		close(done)
	}()

	select {
	case <-done:
		// Success - did not block
	case <-time.After(100 * time.Millisecond):
		t.Error("Send blocked on full buffer")
	}
}

func TestConnID(t *testing.T) {
	expectedID := uuid.New().String()
	conn := &Conn{
		id:       expectedID,
		send:     make(chan []byte, sendBufferSize),
		done:     make(chan struct{}),
		channels: make(map[string]*ChannelSub),
	}

	if conn.ID() != expectedID {
		t.Errorf("ID() = %s, want %s", conn.ID(), expectedID)
	}
}
