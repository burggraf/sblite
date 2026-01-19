package log

import (
	"bytes"
	"log/slog"
	"testing"
)

func TestBufferHandler_StoresLines(t *testing.T) {
	buf := NewRingBuffer(10)
	h := NewBufferHandler(nil, buf) // nil wrapped handler is valid

	logger := slog.New(h)
	logger.Info("test message", "key", "value")

	lines := buf.Lines(10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] == "" {
		t.Error("expected non-empty line")
	}
}

func TestRingBuffer_Capacity(t *testing.T) {
	buf := NewRingBuffer(3)

	buf.Add("line1")
	buf.Add("line2")
	buf.Add("line3")
	buf.Add("line4") // should evict line1

	lines := buf.Lines(10)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line2" {
		t.Errorf("expected oldest line to be 'line2', got %q", lines[0])
	}
	if lines[2] != "line4" {
		t.Errorf("expected newest line to be 'line4', got %q", lines[2])
	}
}

func TestRingBuffer_LinesLimit(t *testing.T) {
	buf := NewRingBuffer(10)
	for i := 0; i < 5; i++ {
		buf.Add("line")
	}

	lines := buf.Lines(3)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}

func TestRingBuffer_Total(t *testing.T) {
	buf := NewRingBuffer(3)
	buf.Add("line1")
	buf.Add("line2")

	if buf.Total() != 2 {
		t.Errorf("expected total 2, got %d", buf.Total())
	}
	if buf.Capacity() != 3 {
		t.Errorf("expected capacity 3, got %d", buf.Capacity())
	}
}

func TestBufferHandler_ForwardsToWrapped(t *testing.T) {
	buf := NewRingBuffer(10)
	var output bytes.Buffer
	wrapped := slog.NewTextHandler(&output, nil)
	h := NewBufferHandler(wrapped, buf)

	logger := slog.New(h)
	logger.Info("forwarded message")

	// Check buffer has the line
	lines := buf.Lines(10)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line in buffer, got %d", len(lines))
	}

	// Check wrapped handler received the line
	if output.Len() == 0 {
		t.Error("expected wrapped handler to receive log")
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	buf := NewRingBuffer(10)

	lines := buf.Lines(10)
	if len(lines) != 0 {
		t.Fatalf("expected 0 lines from empty buffer, got %d", len(lines))
	}
	if buf.Total() != 0 {
		t.Errorf("expected total 0, got %d", buf.Total())
	}
}

func TestRingBuffer_DefaultCapacity(t *testing.T) {
	buf := NewRingBuffer(0) // invalid capacity
	if buf.Capacity() != 500 {
		t.Errorf("expected default capacity 500, got %d", buf.Capacity())
	}

	buf2 := NewRingBuffer(-1) // negative capacity
	if buf2.Capacity() != 500 {
		t.Errorf("expected default capacity 500, got %d", buf2.Capacity())
	}
}
