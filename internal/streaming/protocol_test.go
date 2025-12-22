package streaming

import (
	"errors"
	"fmt"
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

// failingWriter simulates a writer that fails after N bytes
type failingWriter struct {
	written   int
	failAfter int
	headers   http.Header
}

func (f *failingWriter) Write(p []byte) (n int, err error) {
	if f.written >= f.failAfter {
		return 0, fmt.Errorf("simulated write failure")
	}
	n = len(p)
	if f.written+n > f.failAfter {
		n = f.failAfter - f.written
		f.written += n
		return n, fmt.Errorf("simulated write failure after %d bytes", n)
	}
	f.written += n
	return n, nil
}

func (f *failingWriter) Header() http.Header {
	if f.headers == nil {
		f.headers = make(http.Header)
	}
	return f.headers
}

func (f *failingWriter) WriteHeader(code int) {}

func (f *failingWriter) Flush() {}

func TestStreamWriterWriteStart(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteStart("msg-123")
		if err != nil {
			t.Fatalf("WriteStart() error = %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, `f:{"messageId":"msg-123"}`) {
			t.Errorf("WriteStart() body = %q, want to contain messageId", body)
		}
	})

	t.Run("empty messageID returns error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteStart("")
		if err == nil {
			t.Error("WriteStart() expected error for empty messageID")
		}
		if !errors.Is(err, ErrEmptyMessageID) {
			t.Errorf("WriteStart() error = %v, want ErrEmptyMessageID", err)
		}
	})
}

func TestStreamWriterWriteText(t *testing.T) {
	t.Run("simple text", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteText("Hello, world!")
		if err != nil {
			t.Fatalf("WriteText() error = %v", err)
		}

		body := rec.Body.String()
		// Text is now JSON encoded
		if !strings.Contains(body, `0:"Hello, world!"`) {
			t.Errorf("WriteText() body = %q, want to contain JSON-encoded text", body)
		}
	})

	t.Run("text with newlines is properly escaped", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteText("Hello\nWorld")
		if err != nil {
			t.Fatalf("WriteText() error = %v", err)
		}

		body := rec.Body.String()
		// Newlines should be escaped in JSON
		if !strings.Contains(body, `0:"Hello\nWorld"`) {
			t.Errorf("WriteText() body = %q, want newlines escaped", body)
		}
		// Should be a single line in the stream
		lines := strings.Split(strings.TrimSpace(body), "\n")
		if len(lines) != 1 {
			t.Errorf("WriteText() produced %d lines, want 1", len(lines))
		}
	})

	t.Run("text with special characters", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteText("Hello \"world\" \t tab")
		if err != nil {
			t.Fatalf("WriteText() error = %v", err)
		}

		body := rec.Body.String()
		// Quotes and tabs should be escaped
		if !strings.Contains(body, `\"world\"`) {
			t.Errorf("WriteText() body = %q, want quotes escaped", body)
		}
	})
}

func TestStreamWriterWriteData(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	data := []interface{}{map[string]string{"key": "value"}}
	err = sw.WriteData(data)
	if err != nil {
		t.Fatalf("WriteData() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `2:[{"key":"value"}]`) {
		t.Errorf("WriteData() body = %q, want to contain data", body)
	}
}

func TestStreamWriterWriteError(t *testing.T) {
	t.Run("simple error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteError("Something went wrong")
		if err != nil {
			t.Fatalf("WriteError() error = %v", err)
		}

		body := rec.Body.String()
		// Error is now JSON encoded
		if !strings.Contains(body, `3:"Something went wrong"`) {
			t.Errorf("WriteError() body = %q, want to contain JSON-encoded error", body)
		}
	})

	t.Run("error with special characters", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteError("Error: \"invalid input\"\nDetails: none")
		if err != nil {
			t.Fatalf("WriteError() error = %v", err)
		}

		body := rec.Body.String()
		// Should be properly escaped
		lines := strings.Split(strings.TrimSpace(body), "\n")
		if len(lines) != 1 {
			t.Errorf("WriteError() produced %d lines, want 1", len(lines))
		}
	})
}

func TestStreamWriterWriteAnnotation(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	annotation := map[string]string{"userMessageId": "user-1", "messageId": "msg-1"}
	err = sw.WriteAnnotation(annotation)
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
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	usage := &Usage{
		PromptTokens:     10,
		CompletionTokens: 20,
		TotalTokens:      30,
	}
	err = sw.WriteFinish("stop", usage)
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
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	err = sw.WriteFinish("stop", nil)
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
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

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
		{"PartTypeFunctionCall", PartTypeFunctionCall, "1"},
		{"PartTypeData", PartTypeData, "2"},
		{"PartTypeError", PartTypeError, "3"},
		{"PartTypeAssistantMsg", PartTypeAssistantMsg, "4"},
		{"PartTypeAssistantCtrl", PartTypeAssistantCtrl, "5"},
		{"PartTypeDataMessage", PartTypeDataMessage, "6"},
		{"PartTypeToolCallDelta", PartTypeToolCallDelta, "7"},
		{"PartTypeAnnotation", PartTypeAnnotation, "8"},
		{"PartTypeToolCall", PartTypeToolCall, "9"},
		{"PartTypeToolResult", PartTypeToolResult, "a"},
		{"PartTypeToolCallStart", PartTypeToolCallStart, "b"},
		{"PartTypeToolCallArgDelta", PartTypeToolCallArgDelta, "c"},
		{"PartTypeFinishMessage", PartTypeFinishMessage, "d"},
		{"PartTypeFinishStep", PartTypeFinishStep, "e"},
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

func TestStreamWriterWriteToolCall(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		args := map[string]interface{}{
			"location": "San Francisco",
			"unit":     "celsius",
		}
		err = sw.WriteToolCall("call_123", "get_weather", args)
		if err != nil {
			t.Fatalf("WriteToolCall() error = %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "9:") {
			t.Errorf("WriteToolCall() body = %q, want to contain tool call prefix '9:'", body)
		}
		if !strings.Contains(body, `"toolCallId":"call_123"`) {
			t.Errorf("WriteToolCall() body = %q, want to contain toolCallId", body)
		}
		if !strings.Contains(body, `"toolName":"get_weather"`) {
			t.Errorf("WriteToolCall() body = %q, want to contain toolName", body)
		}
		if !strings.Contains(body, `"location":"San Francisco"`) {
			t.Errorf("WriteToolCall() body = %q, want to contain args", body)
		}
	})

	t.Run("empty toolCallID returns error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteToolCall("", "get_weather", nil)
		if err == nil {
			t.Error("WriteToolCall() expected error for empty toolCallID")
		}
		if !errors.Is(err, ErrEmptyToolCallID) {
			t.Errorf("WriteToolCall() error = %v, want ErrEmptyToolCallID", err)
		}
	})

	t.Run("empty toolName returns error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteToolCall("call_123", "", nil)
		if err == nil {
			t.Error("WriteToolCall() expected error for empty toolName")
		}
		if !errors.Is(err, ErrEmptyToolName) {
			t.Errorf("WriteToolCall() error = %v, want ErrEmptyToolName", err)
		}
	})
}

func TestStreamWriterWriteToolResult(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		result := map[string]interface{}{
			"temperature": 22,
			"condition":   "sunny",
		}
		err = sw.WriteToolResult("call_123", result)
		if err != nil {
			t.Fatalf("WriteToolResult() error = %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "a:") {
			t.Errorf("WriteToolResult() body = %q, want to contain tool result prefix 'a:'", body)
		}
		if !strings.Contains(body, `"toolCallId":"call_123"`) {
			t.Errorf("WriteToolResult() body = %q, want to contain toolCallId", body)
		}
		if !strings.Contains(body, `"temperature":22`) {
			t.Errorf("WriteToolResult() body = %q, want to contain result data", body)
		}
	})

	t.Run("empty toolCallID returns error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteToolResult("", nil)
		if err == nil {
			t.Error("WriteToolResult() expected error for empty toolCallID")
		}
		if !errors.Is(err, ErrEmptyToolCallID) {
			t.Errorf("WriteToolResult() error = %v, want ErrEmptyToolCallID", err)
		}
	})
}

func TestStreamWriterWriteToolCallStart(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteToolCallStart("call_456", "search_database")
		if err != nil {
			t.Fatalf("WriteToolCallStart() error = %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "b:") {
			t.Errorf("WriteToolCallStart() body = %q, want to contain prefix 'b:'", body)
		}
		if !strings.Contains(body, `"toolCallId":"call_456"`) {
			t.Errorf("WriteToolCallStart() body = %q, want to contain toolCallId", body)
		}
		if !strings.Contains(body, `"toolName":"search_database"`) {
			t.Errorf("WriteToolCallStart() body = %q, want to contain toolName", body)
		}
	})

	t.Run("empty toolCallID returns error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteToolCallStart("", "search_database")
		if !errors.Is(err, ErrEmptyToolCallID) {
			t.Errorf("WriteToolCallStart() error = %v, want ErrEmptyToolCallID", err)
		}
	})

	t.Run("empty toolName returns error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteToolCallStart("call_456", "")
		if !errors.Is(err, ErrEmptyToolName) {
			t.Errorf("WriteToolCallStart() error = %v, want ErrEmptyToolName", err)
		}
	})
}

func TestStreamWriterWriteToolCallArgDelta(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteToolCallArgDelta("call_456", `{"query": "test`)
		if err != nil {
			t.Fatalf("WriteToolCallArgDelta() error = %v", err)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "c:") {
			t.Errorf("WriteToolCallArgDelta() body = %q, want to contain prefix 'c:'", body)
		}
		if !strings.Contains(body, `"toolCallId":"call_456"`) {
			t.Errorf("WriteToolCallArgDelta() body = %q, want to contain toolCallId", body)
		}
		if !strings.Contains(body, `"argsTextDelta"`) {
			t.Errorf("WriteToolCallArgDelta() body = %q, want to contain argsTextDelta", body)
		}
	})

	t.Run("empty toolCallID returns error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteToolCallArgDelta("", `{"query": "test`)
		if !errors.Is(err, ErrEmptyToolCallID) {
			t.Errorf("WriteToolCallArgDelta() error = %v, want ErrEmptyToolCallID", err)
		}
	})

	t.Run("empty argsDelta is allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := &mockFlusher{ResponseRecorder: rec}
		sw, err := NewStreamWriter(w)
		if err != nil {
			t.Fatalf("NewStreamWriter() error = %v", err)
		}

		err = sw.WriteToolCallArgDelta("call_456", "")
		if err != nil {
			t.Errorf("WriteToolCallArgDelta() unexpected error = %v", err)
		}
	})
}

func TestStreamWriterWriteFinishStep(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	usage := &Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}
	err = sw.WriteFinishStep(FinishReasonToolCalls, usage, true)
	if err != nil {
		t.Fatalf("WriteFinishStep() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "e:") {
		t.Errorf("WriteFinishStep() body = %q, want to contain prefix 'e:'", body)
	}
	if !strings.Contains(body, `"finishReason":"tool-calls"`) {
		t.Errorf("WriteFinishStep() body = %q, want to contain finishReason", body)
	}
	if !strings.Contains(body, `"isContinued":true`) {
		t.Errorf("WriteFinishStep() body = %q, want to contain isContinued", body)
	}
	if !strings.Contains(body, `"promptTokens":100`) {
		t.Errorf("WriteFinishStep() body = %q, want to contain usage", body)
	}
}

func TestStreamWriterWriteFinishMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	usage := &Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}
	err = sw.WriteFinishMessage(FinishReasonStop, usage)
	if err != nil {
		t.Fatalf("WriteFinishMessage() error = %v", err)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "d:") {
		t.Errorf("WriteFinishMessage() body = %q, want to contain prefix 'd:'", body)
	}
	if !strings.Contains(body, `"finishReason":"stop"`) {
		t.Errorf("WriteFinishMessage() body = %q, want to contain finishReason", body)
	}
}

func TestFinishReasonTypes(t *testing.T) {
	tests := []struct {
		name     string
		got      FinishReasonType
		expected string
	}{
		{"FinishReasonStop", FinishReasonStop, "stop"},
		{"FinishReasonLength", FinishReasonLength, "length"},
		{"FinishReasonContentFilter", FinishReasonContentFilter, "content-filter"},
		{"FinishReasonToolCalls", FinishReasonToolCalls, "tool-calls"},
		{"FinishReasonError", FinishReasonError, "error"},
		{"FinishReasonOther", FinishReasonOther, "other"},
		{"FinishReasonUnknown", FinishReasonUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.got) != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestStepTypes(t *testing.T) {
	tests := []struct {
		name     string
		got      StepType
		expected string
	}{
		{"StepTypeInitial", StepTypeInitial, "initial"},
		{"StepTypeContinue", StepTypeContinue, "continue"},
		{"StepTypeToolResult", StepTypeToolResult, "tool-result"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.got) != tt.expected {
				t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.expected)
			}
		})
	}
}

func TestErrorVariables(t *testing.T) {
	if ErrEmptyMessageID == nil {
		t.Error("ErrEmptyMessageID should not be nil")
	}
	if ErrEmptyToolCallID == nil {
		t.Error("ErrEmptyToolCallID should not be nil")
	}
	if ErrEmptyToolName == nil {
		t.Error("ErrEmptyToolName should not be nil")
	}
}

func TestWriteRawPartialWrite(t *testing.T) {
	t.Run("write error", func(t *testing.T) {
		// Tests the err != nil path
		fw := &failingWriter{failAfter: 5}
		sw := &StreamWriter{w: fw, flusher: fw}

		err := sw.writeRaw("This is a long message that will fail")
		if err == nil {
			t.Error("writeRaw() expected error for write failure")
		}
		if !strings.Contains(err.Error(), "write failed") {
			t.Errorf("writeRaw() error = %v, want to contain 'write failed'", err)
		}
	})

	t.Run("partial write no error", func(t *testing.T) {
		// Tests the n != len(data) path where Write returns partial bytes with no error
		pw := &partialWriter{maxBytes: 5}
		sw := &StreamWriter{w: pw, flusher: pw}

		err := sw.writeRaw("This is a long message")
		if err == nil {
			t.Error("writeRaw() expected error for partial write")
		}
		if !strings.Contains(err.Error(), "partial write") {
			t.Errorf("writeRaw() error = %v, want to contain 'partial write'", err)
		}
	})
}

// partialWriter simulates a writer that only writes N bytes without returning an error
type partialWriter struct {
	maxBytes int
	headers  http.Header
}

func (p *partialWriter) Write(data []byte) (int, error) {
	if len(data) > p.maxBytes {
		return p.maxBytes, nil // Partial write, no error
	}
	return len(data), nil
}

func (p *partialWriter) Header() http.Header {
	if p.headers == nil {
		p.headers = make(http.Header)
	}
	return p.headers
}

func (p *partialWriter) WriteHeader(code int) {}
func (p *partialWriter) Flush()               {}

func TestCompleteStreamFlow(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	// Simulate a complete streaming flow
	_ = sw.WriteStart("msg-123")
	_ = sw.WriteText("Hello")
	_ = sw.WriteText(" World")
	_ = sw.WriteText("!")
	_ = sw.WriteFinishStep(FinishReasonStop, &Usage{
		PromptTokens:     5,
		CompletionTokens: 3,
		TotalTokens:      8,
	}, false)
	_ = sw.WriteFinishMessage(FinishReasonStop, &Usage{
		PromptTokens:     5,
		CompletionTokens: 3,
		TotalTokens:      8,
	})
	_ = sw.WriteAnnotation(map[string]string{
		"userMessageId": "user-msg-1",
		"messageId":     "msg-123",
	})
	sw.Close()

	body := rec.Body.String()

	// Verify all parts are present (text is now JSON encoded)
	expectedParts := []string{
		`f:{"messageId":"msg-123"}`,
		`0:"Hello"`,
		`0:" World"`,
		`0:"!"`,
		"e:", // finish step
		"d:", // finish message
		`"finishReason":"stop"`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(body, part) {
			t.Errorf("body missing expected part: %q\nbody: %s", part, body)
		}
	}
}

func TestCompleteToolCallStreamFlow(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	// Simulate a complete tool calling flow
	// Step 1: Initial message with tool call
	_ = sw.WriteStart("msg-456")
	_ = sw.WriteText("Let me check the weather for you.")

	// Stream tool call start
	_ = sw.WriteToolCallStart("call_abc123", "get_weather")

	// Stream tool call arguments incrementally
	_ = sw.WriteToolCallArgDelta("call_abc123", `{"location":`)
	_ = sw.WriteToolCallArgDelta("call_abc123", `"San Francisco"`)
	_ = sw.WriteToolCallArgDelta("call_abc123", `}`)

	// Write complete tool call
	_ = sw.WriteToolCall("call_abc123", "get_weather", map[string]interface{}{
		"location": "San Francisco",
	})

	// Finish step with tool_calls reason (isContinued = true)
	_ = sw.WriteFinishStep(FinishReasonToolCalls, &Usage{
		PromptTokens:     20,
		CompletionTokens: 10,
		TotalTokens:      30,
	}, true)

	// Step 2: Tool result comes back
	_ = sw.WriteToolResult("call_abc123", map[string]interface{}{
		"temperature": 22,
		"condition":   "sunny",
	})

	// Step 3: Continue with response after tool result
	_ = sw.WriteText("The weather in San Francisco is sunny with a temperature of 22°C.")

	// Finish step (no more tool calls)
	_ = sw.WriteFinishStep(FinishReasonStop, &Usage{
		PromptTokens:     50,
		CompletionTokens: 20,
		TotalTokens:      70,
	}, false)

	// Final finish message
	_ = sw.WriteFinishMessage(FinishReasonStop, &Usage{
		PromptTokens:     50,
		CompletionTokens: 20,
		TotalTokens:      70,
	})

	_ = sw.WriteAnnotation(map[string]string{
		"userMessageId": "user-msg-2",
		"messageId":     "msg-456",
	})
	sw.Close()

	body := rec.Body.String()

	// Verify all parts are present
	expectedParts := []struct {
		prefix string
		desc   string
	}{
		{`f:{"messageId":"msg-456"}`, "message start"},
		{`0:"Let me check the weather for you."`, "initial text"},
		{"b:", "tool call start"},
		{`"toolCallId":"call_abc123"`, "tool call ID"},
		{`"toolName":"get_weather"`, "tool name"},
		{"c:", "tool call arg delta"},
		{"9:", "complete tool call"},
		{`e:`, "finish step"},
		{`"finishReason":"tool-calls"`, "tool-calls finish reason"},
		{`"isContinued":true`, "isContinued flag"},
		{"a:", "tool result"},
		{`"temperature":22`, "tool result data"},
		{`0:"The weather in San Francisco`, "response text"},
		{`"finishReason":"stop"`, "stop finish reason"},
		{"d:", "finish message"},
		{"8:", "annotation"},
	}

	for _, exp := range expectedParts {
		if !strings.Contains(body, exp.prefix) {
			t.Errorf("body missing %s: %q\nbody: %s", exp.desc, exp.prefix, body)
		}
	}
}

func TestStartDataStruct(t *testing.T) {
	// Test that StartData struct is properly defined
	s := StartData{MessageID: "test-123"}
	if s.MessageID != "test-123" {
		t.Errorf("StartData.MessageID = %q, want %q", s.MessageID, "test-123")
	}
}

func BenchmarkWriteText(b *testing.B) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sw.WriteText("Hello, world!")
	}
}

func BenchmarkWriteToolCall(b *testing.B) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, _ := NewStreamWriter(w)

	args := map[string]interface{}{
		"location": "San Francisco",
		"unit":     "celsius",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sw.WriteToolCall("call_123", "get_weather", args)
	}
}

func TestWritePartJSONMarshalError(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &mockFlusher{ResponseRecorder: rec}
	sw, err := NewStreamWriter(w)
	if err != nil {
		t.Fatalf("NewStreamWriter() error = %v", err)
	}

	// Channels cannot be marshaled to JSON
	unmarshalable := make(chan int)

	// WriteData accepts []interface{}, so we can pass an unmarshalable value
	err = sw.WriteData([]interface{}{unmarshalable})
	if err == nil {
		t.Error("WriteData() expected error for unmarshalable data")
	}
	if !strings.Contains(err.Error(), "failed to marshal data") {
		t.Errorf("WriteData() error = %v, want to contain 'failed to marshal data'", err)
	}
}
