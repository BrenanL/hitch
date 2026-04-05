package proxy

import (
	"encoding/json"
	"fmt"
	"os"
)

// RequestAnalysis is the full content breakdown of a logged API request.
type RequestAnalysis struct {
	Model       string             `json:"model"`
	System      SystemAnalysis     `json:"system"`
	Messages    MessageAnalysis    `json:"messages"`
	Tools       ToolDefAnalysis    `json:"tools"`
	ToolUses    ToolUseAnalysis    `json:"tool_uses"`
	ToolResults ToolResultAnalysis `json:"tool_results"`
	FileReads   []string           `json:"file_reads"`
	Composition ContentComposition `json:"composition"`
}

// SystemAnalysis describes the system prompt content.
type SystemAnalysis struct {
	BlockCount     int            `json:"block_count"`
	TotalSizeBytes int            `json:"total_size_bytes"`
	Types          map[string]int `json:"types"`
}

// MessageAnalysis describes the messages array.
type MessageAnalysis struct {
	Total             int            `json:"total"`
	ByRole            map[string]int `json:"by_role"`
	ConversationTurns int            `json:"conversation_turns"`
	TotalSizeBytes    int            `json:"total_size_bytes"`
}

// ToolDefAnalysis describes tool definitions.
type ToolDefAnalysis struct {
	Count          int      `json:"count"`
	Names          []string `json:"names"`
	TotalSizeBytes int      `json:"total_size_bytes"`
}

// ToolUseAnalysis describes tool_use blocks in assistant messages.
type ToolUseAnalysis struct {
	Count  int            `json:"count"`
	ByTool map[string]int `json:"by_tool"`
}

// ToolResultAnalysis describes tool_result blocks in user messages.
type ToolResultAnalysis struct {
	Count          int `json:"count"`
	TotalSizeBytes int `json:"total_size_bytes"`
	AvgSizeBytes   int `json:"avg_size_bytes"`
}

// ContentComposition is a percentage breakdown of request content by category.
type ContentComposition struct {
	SystemPercent       float64 `json:"system_percent"`
	ConversationPercent float64 `json:"conversation_percent"`
	ToolResultPercent   float64 `json:"tool_result_percent"`
	ToolDefPercent      float64 `json:"tool_def_percent"`
}

// AnalyzeRequestBody reads a .req.json transaction log file and produces a content breakdown.
func AnalyzeRequestBody(path string) (*RequestAnalysis, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading request log: %w", err)
	}

	// Unwrap the txlog envelope: {method, url, headers, body}
	var envelope struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing request log envelope: %w", err)
	}
	if len(envelope.Body) == 0 {
		return nil, fmt.Errorf("request log has no body")
	}

	// Parse the API request body
	var req struct {
		Model    string            `json:"model"`
		System   json.RawMessage   `json:"system"`
		Messages []json.RawMessage `json:"messages"`
		Tools    []json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(envelope.Body, &req); err != nil {
		return nil, fmt.Errorf("parsing API request body: %w", err)
	}

	a := &RequestAnalysis{
		Model: req.Model,
	}

	a.System = analyzeSystem(req.System)
	a.Messages, a.ToolUses, a.ToolResults, a.FileReads = analyzeMessages(req.Messages)
	a.Tools = analyzeToolDefs(req.Tools)
	a.Composition = computeComposition(a)

	return a, nil
}

func analyzeSystem(raw json.RawMessage) SystemAnalysis {
	sa := SystemAnalysis{Types: map[string]int{}}
	if len(raw) == 0 {
		return sa
	}

	// Try as string (older format)
	var s string
	if json.Unmarshal(raw, &s) == nil {
		sa.BlockCount = 1
		sa.TotalSizeBytes = len(s)
		sa.Types["text"] = 1
		return sa
	}

	// Try as array of blocks
	var blocks []json.RawMessage
	if json.Unmarshal(raw, &blocks) != nil {
		return sa
	}

	sa.BlockCount = len(blocks)
	for _, block := range blocks {
		sa.TotalSizeBytes += len(block)
		var b struct {
			Type         string          `json:"type"`
			CacheControl json.RawMessage `json:"cache_control,omitempty"`
		}
		if json.Unmarshal(block, &b) == nil {
			typ := b.Type
			if typ == "" {
				typ = "unknown"
			}
			if len(b.CacheControl) > 0 {
				typ += "+cache_control"
			}
			sa.Types[typ]++
		}
	}
	return sa
}

func analyzeMessages(messages []json.RawMessage) (MessageAnalysis, ToolUseAnalysis, ToolResultAnalysis, []string) {
	ma := MessageAnalysis{ByRole: map[string]int{}}
	tu := ToolUseAnalysis{ByTool: map[string]int{}}
	tr := ToolResultAnalysis{}
	var fileReads []string

	// Map tool_use_id -> tool name for correlating results
	toolNames := map[string]string{}

	for _, msgRaw := range messages {
		ma.TotalSizeBytes += len(msgRaw)

		var msg struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(msgRaw, &msg) != nil {
			continue
		}
		ma.Total++
		ma.ByRole[msg.Role]++

		// Content can be string or array of blocks
		var contentStr string
		if json.Unmarshal(msg.Content, &contentStr) == nil {
			continue // simple string content, nothing to parse
		}

		var blocks []json.RawMessage
		if json.Unmarshal(msg.Content, &blocks) != nil {
			continue
		}

		for _, blockRaw := range blocks {
			var block struct {
				Type       string          `json:"type"`
				Name       string          `json:"name,omitempty"`
				ID         string          `json:"id,omitempty"`
				ToolUseID  string          `json:"tool_use_id,omitempty"`
				Input      json.RawMessage `json:"input,omitempty"`
				Content    json.RawMessage `json:"content,omitempty"`
			}
			if json.Unmarshal(blockRaw, &block) != nil {
				continue
			}

			switch block.Type {
			case "tool_use":
				tu.Count++
				tu.ByTool[block.Name]++
				if block.ID != "" {
					toolNames[block.ID] = block.Name
				}

				// Detect file reads
				if block.Name == "Read" || block.Name == "read" {
					var input struct {
						FilePath string `json:"file_path"`
					}
					if json.Unmarshal(block.Input, &input) == nil && input.FilePath != "" {
						fileReads = append(fileReads, input.FilePath)
					}
				}

			case "tool_result":
				tr.Count++
				tr.TotalSizeBytes += len(block.Content)
			}
		}
	}

	if tr.Count > 0 {
		tr.AvgSizeBytes = tr.TotalSizeBytes / tr.Count
	}

	userCount := ma.ByRole["user"]
	assistantCount := ma.ByRole["assistant"]
	if userCount < assistantCount {
		ma.ConversationTurns = userCount
	} else {
		ma.ConversationTurns = assistantCount
	}

	return ma, tu, tr, fileReads
}

func analyzeToolDefs(tools []json.RawMessage) ToolDefAnalysis {
	td := ToolDefAnalysis{}
	td.Count = len(tools)
	for _, toolRaw := range tools {
		td.TotalSizeBytes += len(toolRaw)
		var t struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(toolRaw, &t) == nil && t.Name != "" {
			td.Names = append(td.Names, t.Name)
		}
	}
	return td
}

func computeComposition(a *RequestAnalysis) ContentComposition {
	systemBytes := float64(a.System.TotalSizeBytes)
	toolResultBytes := float64(a.ToolResults.TotalSizeBytes)
	toolDefBytes := float64(a.Tools.TotalSizeBytes)
	messageBytes := float64(a.Messages.TotalSizeBytes)

	// Conversation = messages minus tool results (tool results are within messages)
	conversationBytes := messageBytes - toolResultBytes
	if conversationBytes < 0 {
		conversationBytes = 0
	}

	total := systemBytes + messageBytes + toolDefBytes
	if total == 0 {
		return ContentComposition{}
	}

	return ContentComposition{
		SystemPercent:       systemBytes / total * 100,
		ConversationPercent: conversationBytes / total * 100,
		ToolResultPercent:   toolResultBytes / total * 100,
		ToolDefPercent:      toolDefBytes / total * 100,
	}
}
