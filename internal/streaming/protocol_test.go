package streaming

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockFlusher is a ResponseWriter that also implements http.Flusher
type mockFlusher struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (m *mockFlusher) Flush() {
	m.flushed = true
}

func TestNewStreamWriter(t *testing.T) {
	t.Run("success with flusher", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}

		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}
		if sw == nil {
			t.Fatal("NewStreamWriter() returned nil")
		}

		// Check headers
		if got := rec.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
			t.Errorf("Content-Type = %q, want %q", got, "text/plain; charset=utf-8")
		}
		if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
		}
		if got := rec.Header().Get("Connection"); got != "keep-alive" {
			t.Errorf("Connection = %q, want %q", got, "keep-alive")
		}
		if got := rec.Header().Get("X-Vercel-AI-Data-Stream"); got != "v1" {
			t.Errorf("X-Vercel-AI-Data-Stream = %q, want %q", got, "v1")
		}
	})

	t.Run("failure without flusher", func(t *testing.T) {
		// Use a type that truly doesn't implement Flusher
		w := &nonFlusher{
			headers: make(http.Header),
		}

		sw, err := NewStreamWriter(w)
		if err == nil {
			t.Error("NewStreamWriter() expected error for non-flusher")
		}
		if sw != nil {
			t.Error("NewStreamWriter() expected nil for non-flusher")
		}
	})
}

// nonFlusher is a ResponseWriter that does NOT implement http.Flusher
type nonFlusher struct {
	headers    http.Header
	statusCode int
	body       []byte
}

func (n *nonFlusher) Write(b []byte) (int, error) {
	n.body = append(n.body, b...)
	return len(b), nil
}

func (n *nonFlusher) Header() http.Header {
	return n.headers
}

func (n *nonFlusher) WriteHeader(code int) {
	n.statusCode = code
}

func TestStreamWriterWriteStart(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	err := sw.WriteStart("msg-123")
	if err != nil {
		t.Fatalf("WriteStart() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `f:{"messageId":"msg-123"}`) {
		t.Errorf("WriteStart() body = %q, want to contain messageId", body)
	}
}

func TestStreamWriterWriteText(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	err := sw.WriteText("Hello, world!")
	if err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "0:Hello, world!") {
		t.Errorf("WriteText() body = %q, want to contain text", body)
	}
}

func TestStreamWriterWriteData(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	data := []interface{}{map[string]string{"key": "value"}}
	err := sw.WriteData(data)
	if err != nil {
		t.Fatalf("WriteData() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `2:[{"key":"value"}]`) {
		t.Errorf("WriteData() body = %q, want to contain data", body)
	}
}

func TestStreamWriterWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	err := sw.WriteError("Something went wrong")
	if err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "3:Something went wrong") {
		t.Errorf("WriteError() body = %q, want to contain error", body)
	}
}

func TestStreamWriterWriteAnnotation(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	annotation := map[string]string{"userMessageId": "user-1", "messageId": "msg-1"}
	err := sw.WriteAnnotation(annotation)
	if err != nil {
		t.Fatalf("WriteAnnotation() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "8:") {
		t.Errorf("WriteAnnotation() body = %q, want to contain annotation prefix", body)
	}
	if !strings.Contains(body, "userMessageId") {
		t.Errorf("WriteAnnotation() body = %q, want to contain annotation data", body)
	}
}

func TestStreamWriterWriteFinish(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	usage := &Usage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
	}
	err := sw.WriteFinish("stop", usage)
	if err != nil {
		t.Fatalf("WriteFinish() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "d:") {
		t.Errorf("WriteFinish() body = %q, want to contain finish prefix", body)
	}
	if !strings.Contains(body, `"finishReason":"stop"`) {
		t.Errorf("WriteFinish() body = %q, want to contain finishReason", body)
	}
	if !strings.Contains(body, `"promptTokens":10`) {
		t.Errorf("WriteFinish() body = %q, want to contain usage", body)
	}
}

func TestStreamWriterWriteFinishWithoutUsage(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	err := sw.WriteFinish("stop", nil)
	if err != nil {
		t.Fatalf("WriteFinish() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `"finishReason":"stop"`) {
		t.Errorf("WriteFinish() body = %q, want to contain finishReason", body)
	}
}

func TestStreamWriterClose(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	sw.Close()
	if !w.flushed {
		t.Error("Close() should flush the writer")
	}
}

func TestPartTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"PartTypeText", PartTypeText, "0"},
		{"PartTypeData", PartTypeData, "2"},
		{"PartTypeError", PartTypeError, "3"},
		{"PartTypeAnnotation", PartTypeAnnotation, "8"},
		{"PartTypeFinishReason", PartTypeFinishReason, "d"},
		{"PartTypeStart", PartTypeStart, "f"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestCompleteStreamFlow(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	// Simulate a complete streaming flow
	sw.WriteStart("msg-123")
	sw.WriteText("Hello")
	sw.WriteText(" World")
	sw.WriteText("!")
	sw.WriteFinish("stop", &Usage{
		PromptTokens:     5,
		CompletionTokens: 3,
		TotalTokens:      8,
	})
	sw.WriteAnnotation(map[string]string{
		"userMessageId": "user-msg-1",
		"messageId":     "msg-123",
	})
	sw.Close()

	body := rec.Body.String()

	// Verify all parts are present
	expectedParts := []string{
		`f:{"messageId":"msg-123"}`,
		"0:Hello",
		"0: World",
		"0:!",
		`"finishReason":"stop"`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(body, part) {
			t.Errorf("body missing expected part: %q", part)
		}
	}
}
