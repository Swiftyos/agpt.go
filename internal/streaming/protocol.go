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
// See: https://ai-sdk.dev/docs/ai-sdk-ui/stream-protocol
const (
	PartTypeText             = "0" // Text delta
	PartTypeFunctionCall     = "1" // Function call (legacy)
	PartTypeData             = "2" // Data array
	PartTypeError            = "3" // Error message
	PartTypeAssistantMsg     = "4" // Assistant message
	PartTypeAssistantCtrl    = "5" // Assistant control data
	PartTypeDataMessage      = "6" // Structured data message
	PartTypeToolCallDelta    = "7" // Tool call streaming delta (legacy)
	PartTypeAnnotation       = "8" // Message annotation
	PartTypeToolCall         = "9" // Tool call invocation
	PartTypeToolResult       = "a" // Tool call result
	PartTypeToolCallStart    = "b" // Tool call streaming start
	PartTypeToolCallArgDelta = "c" // Tool call argument delta
	PartTypeFinishMessage    = "d" // Finish message (final)
	PartTypeFinishStep       = "e" // Finish step (per LLM call)
	PartTypeStart            = "f" // Message start with ID
)

// StepType represents the type of step in multi-step flows
type StepType string

const (
	StepTypeInitial    StepType = "initial"
	StepTypeContinue   StepType = "continue"
	StepTypeToolResult StepType = "tool-result"
)

// FinishReasonType represents why the LLM stopped generating
type FinishReasonType string

const (
	FinishReasonStop          FinishReasonType = "stop"
	FinishReasonLength        FinishReasonType = "length"
	FinishReasonContentFilter FinishReasonType = "content-filter"
	FinishReasonToolCalls     FinishReasonType = "tool-calls"
	FinishReasonError         FinishReasonType = "error"
	FinishReasonOther         FinishReasonType = "other"
	FinishReasonUnknown       FinishReasonType = "unknown"
)

// ErrEmptyMessageID is returned when messageID is empty
var ErrEmptyMessageID = fmt.Errorf("messageID cannot be empty")

// ErrEmptyToolCallID is returned when toolCallID is empty
var ErrEmptyToolCallID = fmt.Errorf("toolCallID cannot be empty")

// ErrEmptyToolName is returned when toolName is empty
var ErrEmptyToolName = fmt.Errorf("toolName cannot be empty")

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

// StartData represents the message start payload (type "f")
type StartData struct {
	MessageID string `json:"messageId"`
}

// WriteStart writes the message start part with message ID
func (sw *StreamWriter) WriteStart(messageID string) error {
	if messageID == "" {
		return ErrEmptyMessageID
	}
	data := StartData{MessageID: messageID}
	return sw.writePart(PartTypeStart, data)
}

// WriteText writes a text chunk (JSON encoded to handle special characters)
func (sw *StreamWriter) WriteText(text string) error {
	// JSON encode to properly escape newlines and special characters
	return sw.writePart(PartTypeText, text)
}

// WriteData writes arbitrary data
func (sw *StreamWriter) WriteData(data []interface{}) error {
	return sw.writePart(PartTypeData, data)
}

// WriteError writes an error message (JSON encoded)
func (sw *StreamWriter) WriteError(message string) error {
	return sw.writePart(PartTypeError, message)
}

// WriteAnnotation writes a message annotation
func (sw *StreamWriter) WriteAnnotation(annotation interface{}) error {
	return sw.writePart(PartTypeAnnotation, []interface{}{annotation})
}

// Usage represents token usage statistics
type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// FinishStepData represents the finish step payload (type "e")
type FinishStepData struct {
	FinishReason FinishReasonType `json:"finishReason"`
	Usage        *Usage           `json:"usage,omitempty"`
	IsContinued  bool             `json:"isContinued,omitempty"`
}

// FinishMessageData represents the finish message payload (type "d")
type FinishMessageData struct {
	FinishReason FinishReasonType `json:"finishReason"`
	Usage        *Usage           `json:"usage,omitempty"`
}

// ToolCall represents a tool call invocation (type "9")
type ToolCall struct {
	ToolCallID string      `json:"toolCallId"`
	ToolName   string      `json:"toolName"`
	Args       interface{} `json:"args"`
}

// ToolResult represents a tool call result (type "a")
type ToolResult struct {
	ToolCallID string      `json:"toolCallId"`
	Result     interface{} `json:"result"`
}

// ToolCallStart represents the start of streaming tool call (type "b")
type ToolCallStart struct {
	ToolCallID string `json:"toolCallId"`
	ToolName   string `json:"toolName"`
}

// ToolCallArgDelta represents incremental tool call arguments (type "c")
type ToolCallArgDelta struct {
	ToolCallID    string `json:"toolCallId"`
	ArgsTextDelta string `json:"argsTextDelta"`
}

// WriteToolCall writes a tool call invocation (type "9")
func (sw *StreamWriter) WriteToolCall(toolCallID, toolName string, args interface{}) error {
	if toolCallID == "" {
		return ErrEmptyToolCallID
	}
	if toolName == "" {
		return ErrEmptyToolName
	}
	data := ToolCall{
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Args:       args,
	}
	return sw.writePart(PartTypeToolCall, data)
}

// WriteToolResult writes a tool call result (type "a")
func (sw *StreamWriter) WriteToolResult(toolCallID string, result interface{}) error {
	if toolCallID == "" {
		return ErrEmptyToolCallID
	}
	data := ToolResult{
		ToolCallID: toolCallID,
		Result:     result,
	}
	return sw.writePart(PartTypeToolResult, data)
}

// WriteToolCallStart writes the start of a streaming tool call (type "b")
func (sw *StreamWriter) WriteToolCallStart(toolCallID, toolName string) error {
	if toolCallID == "" {
		return ErrEmptyToolCallID
	}
	if toolName == "" {
		return ErrEmptyToolName
	}
	data := ToolCallStart{
		ToolCallID: toolCallID,
		ToolName:   toolName,
	}
	return sw.writePart(PartTypeToolCallStart, data)
}

// WriteToolCallArgDelta writes incremental tool call arguments (type "c")
func (sw *StreamWriter) WriteToolCallArgDelta(toolCallID, argsDelta string) error {
	if toolCallID == "" {
		return ErrEmptyToolCallID
	}
	data := ToolCallArgDelta{
		ToolCallID:    toolCallID,
		ArgsTextDelta: argsDelta,
	}
	return sw.writePart(PartTypeToolCallArgDelta, data)
}

// WriteFinishStep writes a finish step part (type "e") - used per LLM call
func (sw *StreamWriter) WriteFinishStep(reason FinishReasonType, usage *Usage, isContinued bool) error {
	data := FinishStepData{
		FinishReason: reason,
		Usage:        usage,
		IsContinued:  isContinued,
	}
	return sw.writePart(PartTypeFinishStep, data)
}

// WriteFinishMessage writes the final finish message (type "d")
func (sw *StreamWriter) WriteFinishMessage(reason FinishReasonType, usage *Usage) error {
	data := FinishMessageData{
		FinishReason: reason,
		Usage:        usage,
	}
	return sw.writePart(PartTypeFinishMessage, data)
}

// WriteFinish writes the finish message with reason and usage (alias for WriteFinishMessage)
// Deprecated: Use WriteFinishMessage or WriteFinishStep for proper protocol compliance
func (sw *StreamWriter) WriteFinish(reason string, usage *Usage) error {
	data := FinishMessageData{
		FinishReason: FinishReasonType(reason),
		Usage:        usage,
	}
	return sw.writePart(PartTypeFinishMessage, data)
}

// Close flushes any remaining data
func (sw *StreamWriter) Close() {
	sw.flusher.Flush()
}

func (sw *StreamWriter) writePart(partType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	return sw.writeRaw(fmt.Sprintf("%s:%s\n", partType, string(jsonData)))
}

func (sw *StreamWriter) writeRaw(data string) error {
	n, err := io.WriteString(sw.w, data)
	if err != nil {
		return fmt.Errorf("write failed after %d bytes: %w", n, err)
	}
	if n != len(data) {
		return fmt.Errorf("partial write: wrote %d of %d bytes", n, len(data))
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

// Message represents a chat message
type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt,omitempty"`
}
