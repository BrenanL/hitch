package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BrenanL/hitch/internal/tokens"
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

// ---------------------------------------------------------------------------
// RequestBreakdown types and parser (SPEC-03 sections 2-5)
// ---------------------------------------------------------------------------

// RequestBreakdown is the full parsed breakdown of a single request body.
type RequestBreakdown struct {
	RequestID string
	Model     string
	MaxTokens int

	SystemComponents     []ComponentBreakdown
	Tools                ComponentBreakdown
	ThinkingBudgetTokens int
	Messages             []MessageBreakdown
	TotalEstimatedTokens int
	LargestComponents    []ComponentBreakdown
	FileReads            []FileReadInfo
	ToolCalls            []ToolCallInfo

	ActualInputTokens   int
	CacheReadTokens     int
	CacheCreationTokens int
	UncachedTokens      int
	CacheReadPct        float64
}

// ComponentBreakdown represents a single named segment of the context window.
type ComponentBreakdown struct {
	Label           string
	Category        string
	CharCount       int
	EstimatedTokens int
	Percentage      float64
	CacheStatus     string
	MessageIndex    int
}

// MessageBreakdown summarises one message in the conversation array.
type MessageBreakdown struct {
	Role            string
	Index           int
	Parts           []ComponentBreakdown
	EstimatedTokens int
}

// FileReadInfo records a file that was read and returned as a tool_result.
type FileReadInfo struct {
	FilePath        string
	CharCount       int
	EstimatedTokens int
	MessageIndex    int
	ToolUseID       string
}

// ToolCallInfo records one tool_use block.
type ToolCallInfo struct {
	ToolName    string
	ToolUseID   string
	InputJSON   string
	ResultChars int
}

// ---------------------------------------------------------------------------
// Internal JSON types for parsing
// ---------------------------------------------------------------------------

type rawRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    json.RawMessage `json:"system"`
	Messages  []rawMessage    `json:"messages"`
	Tools     json.RawMessage `json:"tools"`
	Thinking  *rawThinking    `json:"thinking"`
	Stream    bool            `json:"stream"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type rawContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Name      string          `json:"name,omitempty"`
	ID        string          `json:"id,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	CacheControl *struct {
		Type string `json:"type"`
	} `json:"cache_control,omitempty"`
}

type rawThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

// ---------------------------------------------------------------------------
// ParseRequestFile — public entry point
// ---------------------------------------------------------------------------

// ParseRequestFile reads a *.req.json file and returns a structured breakdown.
func ParseRequestFile(
	reqLogPath string,
	actualInputTokens, cacheReadTokens, cacheCreationTokens int,
) (*RequestBreakdown, error) {
	data, err := os.ReadFile(reqLogPath)
	if err != nil {
		return nil, fmt.Errorf("reading request log: %w", err)
	}

	var envelope struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing request log envelope: %w", err)
	}
	if len(envelope.Body) == 0 {
		return nil, fmt.Errorf("request log has no body")
	}

	bd, err := parseMessageBody(envelope.Body)
	if err != nil {
		return nil, err
	}

	bd.ActualInputTokens = actualInputTokens
	bd.CacheReadTokens = cacheReadTokens
	bd.CacheCreationTokens = cacheCreationTokens
	bd.UncachedTokens = actualInputTokens - cacheReadTokens - cacheCreationTokens
	if actualInputTokens > 0 {
		bd.CacheReadPct = float64(cacheReadTokens) / float64(actualInputTokens) * 100
	}

	return bd, nil
}

// parseMessageBody does the core parsing on the decoded body bytes.
func parseMessageBody(body []byte) (*RequestBreakdown, error) {
	var req rawRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parsing API request body: %w", err)
	}

	bd := &RequestBreakdown{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
	}

	if req.Thinking != nil {
		bd.ThinkingBudgetTokens = req.Thinking.BudgetTokens
	}

	bd.SystemComponents = parseSystemComponents(req.System)
	bd.Tools = parseToolsComponent(req.Tools)

	tuIndex := buildToolUseIndex(req.Messages)
	bd.Messages, bd.FileReads, bd.ToolCalls = parseMessages(req.Messages, tuIndex)

	// Compute total estimated tokens
	total := 0
	for _, c := range bd.SystemComponents {
		total += c.EstimatedTokens
	}
	total += bd.Tools.EstimatedTokens
	for _, m := range bd.Messages {
		total += m.EstimatedTokens
	}
	bd.TotalEstimatedTokens = total

	// Collect all components for percentage computation and top-N
	var all []ComponentBreakdown
	all = append(all, bd.SystemComponents...)
	if bd.Tools.EstimatedTokens > 0 {
		all = append(all, bd.Tools)
	}
	for _, m := range bd.Messages {
		all = append(all, m.Parts...)
	}
	all = computePercentages(all, total)

	// Write percentages back
	sysIdx := 0
	for i := range bd.SystemComponents {
		if sysIdx < len(all) {
			bd.SystemComponents[i].Percentage = all[sysIdx].Percentage
			sysIdx++
		}
	}
	if bd.Tools.EstimatedTokens > 0 && sysIdx < len(all) {
		bd.Tools.Percentage = all[sysIdx].Percentage
		sysIdx++
	}
	for mi := range bd.Messages {
		for pi := range bd.Messages[mi].Parts {
			if sysIdx < len(all) {
				bd.Messages[mi].Parts[pi].Percentage = all[sysIdx].Percentage
				sysIdx++
			}
		}
	}

	bd.LargestComponents = topN(all, 5)

	return bd, nil
}

func parseSystemComponents(raw json.RawMessage) []ComponentBreakdown {
	if len(raw) == 0 {
		return nil
	}

	var s string
	if json.Unmarshal(raw, &s) == nil {
		label, category := classifySystemBlock(rawContentBlock{Type: "text", Text: s}, 0)
		return []ComponentBreakdown{
			{
				Label:           label,
				Category:        category,
				CharCount:       len(s),
				EstimatedTokens: estimateTokens(len(s)),
				CacheStatus:     "none",
				MessageIndex:    -1,
			},
		}
	}

	var blocks []rawContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}

	var out []ComponentBreakdown
	for i, block := range blocks {
		label, category := classifySystemBlock(block, i)
		cacheStatus := "none"
		if block.CacheControl != nil {
			cacheStatus = "cache_candidate"
		}
		out = append(out, ComponentBreakdown{
			Label:           label,
			Category:        category,
			CharCount:       len(block.Text),
			EstimatedTokens: estimateTokens(len(block.Text)),
			CacheStatus:     cacheStatus,
			MessageIndex:    -1,
		})
	}
	return out
}

func parseToolsComponent(raw json.RawMessage) ComponentBreakdown {
	if len(raw) == 0 {
		return ComponentBreakdown{MessageIndex: -1}
	}

	var tools []json.RawMessage
	if err := json.Unmarshal(raw, &tools); err != nil {
		return ComponentBreakdown{MessageIndex: -1}
	}

	n := len(tools)
	if n == 0 {
		return ComponentBreakdown{MessageIndex: -1}
	}

	marshaled, _ := json.Marshal(tools)
	charCount := len(marshaled)

	return ComponentBreakdown{
		Label:           fmt.Sprintf("Tool definitions (%d tools)", n),
		Category:        "tool_defs",
		CharCount:       charCount,
		EstimatedTokens: estimateTokens(charCount),
		CacheStatus:     "none",
		MessageIndex:    -1,
	}
}

func parseMessages(
	messages []rawMessage,
	tuIndex map[string]rawContentBlock,
) ([]MessageBreakdown, []FileReadInfo, []ToolCallInfo) {
	var msgBreakdowns []MessageBreakdown
	var fileReads []FileReadInfo
	var toolCalls []ToolCallInfo

	resultChars := map[string]int{}

	for msgIdx, msg := range messages {
		mb := MessageBreakdown{
			Role:  msg.Role,
			Index: msgIdx,
		}

		blocks := extractMessageContent(msg.Content)
		for _, block := range blocks {
			var comp ComponentBreakdown
			comp.MessageIndex = msgIdx
			comp.CacheStatus = "none"
			if block.CacheControl != nil {
				comp.CacheStatus = "cache_candidate"
			}

			switch block.Type {
			case "tool_use":
				inputStr := ""
				if len(block.Input) > 0 {
					inputStr = string(block.Input)
				}
				truncInput := inputStr
				if len(truncInput) > 200 {
					truncInput = truncInput[:200]
				}
				toolCalls = append(toolCalls, ToolCallInfo{
					ToolName:  block.Name,
					ToolUseID: block.ID,
					InputJSON: truncInput,
				})
				comp.Label = "Tool call: " + block.Name
				comp.Category = "tool_call"
				comp.CharCount = len(block.Name) + len(inputStr)
				comp.EstimatedTokens = estimateTokens(comp.CharCount)

			case "tool_result":
				resultText := extractToolResultText(block.Content)
				chars := len(resultText)
				if block.ToolUseID != "" {
					resultChars[block.ToolUseID] = chars
				}
				comp.Label = labelToolResult(block.ToolUseID, tuIndex)
				comp.Category = "tool_result"
				comp.CharCount = chars
				comp.EstimatedTokens = estimateTokens(chars)

				if tu, ok := tuIndex[block.ToolUseID]; ok && tu.Name == "Read" {
					var inp struct {
						FilePath string `json:"file_path"`
					}
					if len(tu.Input) > 0 {
						_ = json.Unmarshal(tu.Input, &inp)
					}
					if inp.FilePath != "" {
						fileReads = append(fileReads, FileReadInfo{
							FilePath:        inp.FilePath,
							CharCount:       chars,
							EstimatedTokens: estimateTokens(chars),
							MessageIndex:    msgIdx,
							ToolUseID:       block.ToolUseID,
						})
					}
				}

			case "text":
				comp.CharCount = len(block.Text)
				comp.EstimatedTokens = estimateTokens(comp.CharCount)
				if msg.Role == "assistant" {
					comp.Label = "Assistant response"
					comp.Category = "assistant_text"
				} else {
					comp.Label = "User message"
					comp.Category = "user_text"
				}

			default:
				comp.Label = "Unknown block: " + block.Type
				comp.Category = "unknown"
			}

			mb.Parts = append(mb.Parts, comp)
			mb.EstimatedTokens += comp.EstimatedTokens
		}

		msgBreakdowns = append(msgBreakdowns, mb)
	}

	for i, tc := range toolCalls {
		if rc, ok := resultChars[tc.ToolUseID]; ok {
			toolCalls[i].ResultChars = rc
		}
	}

	return msgBreakdowns, fileReads, toolCalls
}

func labelToolResult(toolUseID string, tuIndex map[string]rawContentBlock) string {
	tu, ok := tuIndex[toolUseID]
	if !ok {
		return "Tool result (unmatched)"
	}

	switch tu.Name {
	case "Read":
		var inp struct {
			FilePath string `json:"file_path"`
		}
		if len(tu.Input) > 0 {
			_ = json.Unmarshal(tu.Input, &inp)
		}
		if inp.FilePath != "" {
			return "File: " + inp.FilePath
		}
		return "Tool result: Read"
	case "Bash":
		return "Bash output"
	case "Glob":
		return "Glob result"
	case "Grep":
		return "Grep result"
	case "Write", "Edit", "NotebookEdit":
		var inp struct {
			FilePath string `json:"file_path"`
		}
		if len(tu.Input) > 0 {
			_ = json.Unmarshal(tu.Input, &inp)
		}
		if inp.FilePath != "" {
			return "Edit result: " + inp.FilePath
		}
		return "Edit result"
	default:
		return "Tool result: " + tu.Name
	}
}

// classifySystemBlock returns the label and category for one system content block.
func classifySystemBlock(block rawContentBlock, index int) (label, category string) {
	text := block.Text

	if index == 0 {
		return "System Prompt (base)", "system"
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Contents of") && strings.Contains(line, "CLAUDE.md") {
			if strings.Contains(line, "~/.claude/") || strings.Contains(line, "/.claude/CLAUDE.md") {
				return "CLAUDE.md (user)", "system"
			}
			return "CLAUDE.md (project)", "system"
		}
		if strings.HasPrefix(strings.TrimSpace(line), "CLAUDE.md") {
			return "CLAUDE.md (project)", "system"
		}
	}

	if strings.HasPrefix(strings.TrimSpace(text), "---") &&
		(strings.Contains(text, "# Memory") || strings.Contains(text, "MEMORY.md")) {
		return "Auto-memory", "memory"
	}
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[Use ") {
			return "Auto-memory", "memory"
		}
	}

	if strings.Contains(text, "## Rules") || strings.Contains(text, "## Constraints") ||
		strings.Contains(text, ".claude/rules/") {
		return "Rules", "system"
	}

	if strings.Contains(text, "additionalContext") || strings.Contains(text, "hook output") {
		return "Hook context", "system"
	}

	if strings.Contains(text, "## Output Style") || strings.Contains(text, "outputStyle") {
		return "Output style", "system"
	}

	hint := text
	if len(hint) > 60 {
		hint = hint[:60]
	}
	return fmt.Sprintf("System (unknown): %q", hint), "system"
}

// buildToolUseIndex scans all messages and builds a map from tool_use_id to rawContentBlock.
func buildToolUseIndex(messages []rawMessage) map[string]rawContentBlock {
	idx := map[string]rawContentBlock{}
	for _, msg := range messages {
		blocks := extractMessageContent(msg.Content)
		for _, block := range blocks {
			if block.Type == "tool_use" && block.ID != "" {
				idx[block.ID] = block
			}
		}
	}
	return idx
}

// extractToolResultText extracts the string content from a tool_result block's Content field.
func extractToolResultText(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}

	var s string
	if json.Unmarshal(content, &s) == nil {
		return s
	}

	var blocks []rawContentBlock
	if json.Unmarshal(content, &blocks) == nil {
		var sb strings.Builder
		for _, b := range blocks {
			sb.WriteString(b.Text)
		}
		return sb.String()
	}

	return string(content)
}

// extractMessageContent parses a content field that may be a string or array of blocks.
func extractMessageContent(content json.RawMessage) []rawContentBlock {
	if len(content) == 0 {
		return nil
	}

	var s string
	if json.Unmarshal(content, &s) == nil {
		return []rawContentBlock{{Type: "text", Text: s}}
	}

	var blocks []rawContentBlock
	if err := json.Unmarshal(content, &blocks); err != nil {
		return nil
	}
	return blocks
}

// estimateTokens returns a rough token count from a character count using tokens.Estimate.
func estimateTokens(charCount int) int {
	return tokens.Estimate(charCount)
}

// computePercentages sets the Percentage field on all components given the total.
func computePercentages(components []ComponentBreakdown, total int) []ComponentBreakdown {
	if total == 0 {
		return components
	}
	for i := range components {
		components[i].Percentage = float64(components[i].EstimatedTokens) / float64(total) * 100
	}
	return components
}

// topN returns the N largest ComponentBreakdown entries by EstimatedTokens.
func topN(components []ComponentBreakdown, n int) []ComponentBreakdown {
	sorted := make([]ComponentBreakdown, len(components))
	copy(sorted, components)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].EstimatedTokens > sorted[j].EstimatedTokens
	})
	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

// GenerateWhySummary generates a natural-language explanation from a breakdown.
func GenerateWhySummary(bd *RequestBreakdown, cost float64) string {
	if bd == nil {
		return ""
	}

	totalK := bd.TotalEstimatedTokens / 1000

	catTotals := map[string]int{}
	for _, c := range bd.SystemComponents {
		catTotals[c.Category] += c.EstimatedTokens
	}
	catTotals["tool_defs"] += bd.Tools.EstimatedTokens
	for _, m := range bd.Messages {
		for _, p := range m.Parts {
			catTotals[p.Category] += p.EstimatedTokens
		}
	}

	dominant := ""
	dominantTokens := 0
	for cat, tok := range catTotals {
		if tok > dominantTokens {
			dominantTokens = tok
			dominant = cat
		}
	}

	dominantPct := 0.0
	if bd.TotalEstimatedTokens > 0 {
		dominantPct = float64(dominantTokens) / float64(bd.TotalEstimatedTokens) * 100
	}

	dominantLabel := categoryLabel(dominant)
	dominantK := dominantTokens / 1000

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Request %s (%.0fK tok, $%.2f): %s dominated at %.1f%% (%dK tok estimated).",
		bd.RequestID, float64(totalK), cost, dominantLabel, dominantPct, dominantK))

	var largeReads []FileReadInfo
	for _, fr := range bd.FileReads {
		if fr.EstimatedTokens > 5000 {
			largeReads = append(largeReads, fr)
		}
	}
	if len(largeReads) > 0 {
		sort.Slice(largeReads, func(i, j int) bool {
			return largeReads[i].EstimatedTokens > largeReads[j].EstimatedTokens
		})
		top := largeReads
		if len(top) > 3 {
			top = top[:3]
		}
		var parts []string
		for _, fr := range top {
			parts = append(parts, fmt.Sprintf("%s (%dK)", fr.FilePath, fr.EstimatedTokens/1000))
		}
		sb.WriteString(fmt.Sprintf("\n%d large file reads consumed most of that: %s.",
			len(largeReads), strings.Join(parts, ", ")))
	}

	convTokens := catTotals["user_text"] + catTotals["assistant_text"]
	if convTokens > 30000 {
		sb.WriteString(fmt.Sprintf("\nConversation history is long at %d tok — consider /compact to reduce future costs.", convTokens))
	}

	if bd.CacheReadTokens > 0 {
		healthLabel := "low"
		if bd.CacheReadPct > 60 {
			healthLabel = "healthy"
		}
		sb.WriteString(fmt.Sprintf("\nCache hit rate was %s at %.1f%% (%dK read tokens).",
			healthLabel, bd.CacheReadPct, bd.CacheReadTokens/1000))
	}

	if bd.ThinkingBudgetTokens > 0 {
		sb.WriteString(fmt.Sprintf("\nThinking budget was %d tok — accounts for estimated %d tok in output.",
			bd.ThinkingBudgetTokens, bd.ThinkingBudgetTokens))
	}

	return sb.String()
}

func categoryLabel(cat string) string {
	switch cat {
	case "system":
		return "System prompt"
	case "memory":
		return "Auto-memory"
	case "tool_defs":
		return "Tool definitions"
	case "user_text":
		return "User messages"
	case "assistant_text":
		return "Assistant responses"
	case "tool_call":
		return "Tool calls"
	case "tool_result":
		return "Tool results"
	default:
		return cat
	}
}
