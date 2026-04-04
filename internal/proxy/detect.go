package proxy

import (
	"bytes"
	"encoding/json"
)

var microcompactMarker = []byte("[Old tool result content cleared]")

// detectMicrocompact counts silent context stripping events in a request body.
// Claude Code replaces tool results with this marker mid-session, invisibly
// degrading context quality (Bug 4 from ArkNill's cc-relay analysis).
func detectMicrocompact(body []byte) int {
	return bytes.Count(body, microcompactMarker)
}

// detectBudgetTruncation scans tool results for signs of the ~200K character budget cap.
// When exceeded, older tool results get truncated to 1-41 characters (Bug 5).
// Returns the count of likely-truncated results and total tool result size in bytes.
func detectBudgetTruncation(body []byte) (truncatedCount int, totalSize int) {
	var req struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if json.Unmarshal(body, &req) != nil {
		return 0, 0
	}

	for _, msgRaw := range req.Messages {
		var msg struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(msgRaw, &msg) != nil {
			continue
		}
		if msg.Role != "user" {
			continue
		}

		// Content is an array of content blocks
		var blocks []struct {
			Type    string          `json:"type"`
			Content json.RawMessage `json:"content,omitempty"`
		}
		if json.Unmarshal(msg.Content, &blocks) != nil {
			continue
		}

		for _, block := range blocks {
			if block.Type != "tool_result" || len(block.Content) == 0 {
				continue
			}

			// Try to unmarshal as a JSON string (most common case)
			var s string
			if json.Unmarshal(block.Content, &s) == nil {
				size := len(s)
				totalSize += size
				if size >= 1 && size <= 41 {
					truncatedCount++
				}
				continue
			}

			// Content might be an array of sub-blocks (text, image, etc.)
			var subBlocks []struct {
				Type string `json:"type"`
				Text string `json:"text,omitempty"`
			}
			if json.Unmarshal(block.Content, &subBlocks) == nil {
				for _, sb := range subBlocks {
					size := len(sb.Text)
					totalSize += size
					if sb.Type == "text" && size >= 1 && size <= 41 {
						truncatedCount++
					}
				}
				continue
			}

			// Fallback: measure raw JSON size
			totalSize += len(block.Content)
		}
	}

	return truncatedCount, totalSize
}
