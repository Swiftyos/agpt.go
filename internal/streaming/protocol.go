package streaming

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// AI SDK Data Stream Protocol implementation
// See: https://ai-sdk.dev/docs/ai-sdk-ui/stream-protocol

// Stream part types as per AI SDK specification
const (
	PartTypeText         = "0" // Text part
	PartTypeData         = "2" // Data array
	PartTypeError        = "3" // Error
	PartTypeAnnotation   = "8" // Message annotation
	PartTypeFinishReason = "d" // Finish reason with usage
	PartTypeStart        = "f" // Start with message ID
)

// StreamWriter handles writing AI SDK compatible stream responses
type StreamWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewStreamWriter creates a new stream writer with proper headers
func NewStreamWriter(w http.ResponseWriter) (*StreamWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	// Set headers for SSE-like streaming
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Vercel-AI-Data-Stream", "v1")
	w.WriteHeader(http.StatusOK)

	return &StreamWriter{w: w, flusher: flusher}, nil
}

// WriteStart writes the message start part with message ID
func (sw *StreamWriter) WriteStart(messageID string) error {
	data := map[string]interface{}{
		"messageId": messageID,
	}
	return sw.writePart(PartTypeStart, data)
}

// WriteText writes a text chunk
func (sw *StreamWriter) WriteText(text string) error {
	return sw.writeRaw(fmt.Sprintf("%s:%s\n", PartTypeText, text))
}

// WriteData writes arbitrary data
func (sw *StreamWriter) WriteData(data []interface{}) error {
	return sw.writePart(PartTypeData, data)
}

// WriteError writes an error message
func (sw *StreamWriter) WriteError(message string) error {
	return sw.writeRaw(fmt.Sprintf("%s:%s\n", PartTypeError, message))
}

// WriteAnnotation writes a message annotation
func (sw *StreamWriter) WriteAnnotation(annotation interface{}) error {
	return sw.writePart(PartTypeAnnotation, []interface{}{annotation})
}

// FinishReason represents the completion reason and usage stats
type FinishReason struct {
	FinishReason string `json:"finishReason"`
	Usage        *Usage `json:"usage,omitempty"`
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// WriteFinish writes the finish message with reason and usage
func (sw *StreamWriter) WriteFinish(reason string, usage *Usage) error {
	data := FinishReason{
		FinishReason: reason,
		Usage:        usage,
	}
	return sw.writePart(PartTypeFinishReason, data)
}

// Close flushes any remaining data
func (sw *StreamWriter) Close() {
	sw.flusher.Flush()
}

func (sw *StreamWriter) writePart(partType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return sw.writeRaw(fmt.Sprintf("%s:%s\n", partType, string(jsonData)))
}

func (sw *StreamWriter) writeRaw(data string) error {
	_, err := io.WriteString(sw.w, data)
	if err != nil {
		return err
	}
	sw.flusher.Flush()
	return nil
}

// DataStreamResponse is a helper for non-streaming responses
// that still follow the AI SDK format
type DataStreamResponse struct {
	Messages []Message `json:"messages,omitempty"`
	Text     string    `json:"text,omitempty"`
}

type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt,omitempty"`
}
