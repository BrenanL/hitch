package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SSE data types for parsing Anthropic API streaming responses.

type sseEvent struct {
	Type    string          `json:"type"`
	Message *sseMessage     `json:"message,omitempty"`
	Delta   json.RawMessage `json:"delta,omitempty"`
	Usage   *sseUsage       `json:"usage,omitempty"`
}

type sseMessage struct {
	ID    string    `json:"id"`
	Model string    `json:"model"`
	Usage *sseUsage `json:"usage,omitempty"`
}

type sseUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
}

type sseDelta struct {
	StopReason string `json:"stop_reason,omitempty"`
}

// handleStreaming passes through SSE events while extracting metadata and logging to disk.
func (s *Server) handleStreaming(w http.ResponseWriter, resp *http.Response, rec *RequestLog, respLog *ResponseLog) {
	copyResponseHeaders(w, resp)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, resp.Body)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := scanner.Text()

		// Forward immediately — zero added delay
		fmt.Fprintf(w, "%s\n", line)
		flusher.Flush()

		// Log the raw line to disk
		respLog.WriteLine(line)

		// Parse data lines for logging metadata
		if strings.HasPrefix(line, "data: ") {
			parseSSEData(strings.TrimPrefix(line, "data: "), rec)
		}
	}

	if err := scanner.Err(); err != nil {
		rec.Error = fmt.Sprintf("stream error: %v", err)
	}
}

// handleNonStreaming forwards a non-streaming response and logs it.
func (s *Server) handleNonStreaming(w http.ResponseWriter, resp *http.Response, rec *RequestLog, respLog *ResponseLog) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read upstream response", http.StatusBadGateway)
		rec.Error = fmt.Sprintf("read error: %v", err)
		return
	}

	copyResponseHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	w.Write(body)

	// Log the full body to disk
	respLog.WriteBody(body)

	if resp.StatusCode < 400 {
		parseNonStreamingResponse(body, rec)
	} else {
		rec.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
}

// parseSSEData extracts token counts, model, and stop reason from a single SSE data line.
func parseSSEData(jsonStr string, rec *RequestLog) {
	var evt sseEvent
	if err := json.Unmarshal([]byte(jsonStr), &evt); err != nil {
		return
	}

	switch evt.Type {
	case "message_start":
		if evt.Message != nil {
			rec.RequestID = evt.Message.ID
			if evt.Message.Model != "" {
				rec.Model = evt.Message.Model
			}
			if evt.Message.Usage != nil {
				rec.InputTokens = evt.Message.Usage.InputTokens
				rec.CacheReadTokens = evt.Message.Usage.CacheReadTokens
				rec.CacheCreationTokens = evt.Message.Usage.CacheCreationTokens
			}
		}

	case "message_delta":
		if evt.Usage != nil {
			rec.OutputTokens = evt.Usage.OutputTokens
		}
		if len(evt.Delta) > 0 {
			var delta sseDelta
			if json.Unmarshal(evt.Delta, &delta) == nil && delta.StopReason != "" {
				rec.StopReason = delta.StopReason
			}
		}
	}
}

// parseNonStreamingResponse extracts metadata from a complete API response.
func parseNonStreamingResponse(body []byte, rec *RequestLog) {
	var resp struct {
		ID         string   `json:"id"`
		Model      string   `json:"model"`
		StopReason string   `json:"stop_reason"`
		Usage      sseUsage `json:"usage"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return
	}
	rec.RequestID = resp.ID
	if resp.Model != "" {
		rec.Model = resp.Model
	}
	rec.StopReason = resp.StopReason
	rec.InputTokens = resp.Usage.InputTokens
	rec.OutputTokens = resp.Usage.OutputTokens
	rec.CacheReadTokens = resp.Usage.CacheReadTokens
	rec.CacheCreationTokens = resp.Usage.CacheCreationTokens
}
