package translator

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// RegisterOpenAIClaudeTranslators registers openai↔anthropic pair translators.
func RegisterOpenAIClaudeTranslators(reg *Registry) {
	openaiToClaude := &Translator{
		Request:  openaiToClaudeRequest,
		Response: openaiToClaudeResponse,
		Stream:   passthroughStream,
	}
	claudeToOpenai := &Translator{
		Request:  claudeToOpenAIRequest,
		Response: claudeToOpenAIResponse,
		Stream:   passthroughStream,
	}
	reg.Register(PairFor(FormatOpenAI, FormatAnthropic), openaiToClaude)
	reg.Register(PairFor(FormatAnthropic, FormatOpenAI), claudeToOpenai)
}

// ─────────────────── openai → anthropic ──────────────────────────────────────

func openaiToClaudeRequest(reqBody []byte, _, _ Format) ([]byte, error) {
	var req struct {
		Model       string      `json:"model"`
		Messages    []json.RawMessage `json:"messages"`
		MaxTokens   int         `json:"max_tokens"`
		Temperature float64    `json:"temperature"`
		Stream      bool        `json:"stream"`
		System      interface{} `json:"system"`
	}
	if err := json.Unmarshal(reqBody, &req); err != nil {
		return nil, err
	}
	out := map[string]interface{}{
		"model":    req.Model,
		"messages": openaiMessagesToClaude(req.Messages),
	}
	if req.MaxTokens > 0 {
		out["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		out["temperature"] = req.Temperature
	}
	if req.Stream {
		out["stream"] = true
	}
	if req.System != nil {
		out["system"] = req.System
	}
	return json.Marshal(out)
}

func openaiMessagesToClaude(msgs []json.RawMessage) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(msgs))
	for _, raw := range msgs {
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		role, _ := m["role"].(string)
		content := m["content"]
		switch role {
		case "system":
			continue
		case "assistant":
			out = append(out, openaiAssistantToClaude(m))
		case "tool":
			out = append(out, map[string]interface{}{
				"role":    "user",
				"content": openaiContentToClaude(content),
			})
		default:
			out = append(out, map[string]interface{}{
				"role":    role,
				"content": openaiContentToClaude(content),
			})
		}
	}
	return out
}

func openaiContentToClaude(content interface{}) interface{} {
	if content == nil || content == "" {
		return []map[string]interface{}{{"type": "text", "text": ""}}
	}
	switch v := content.(type) {
	case string:
		return []map[string]interface{}{{"type": "text", "text": v}}
	case []interface{}:
		var out []interface{}
		for _, item := range v {
			block, ok := item.(map[string]interface{})
			if !ok {
				out = append(out, item)
				continue
			}
			switch block["type"] {
			case "image_url":
				imgURL, _ := block["image_url"].(map[string]interface{})
				urlStr, _ := imgURL["url"].(string)
				mediaType, data := parseDataURL(urlStr)
				out = append(out, map[string]interface{}{
					"type": "image",
					"source": map[string]interface{}{
						"type":      "base64",
						"media_type": mediaType,
						"data":      data,
					},
				})
			default:
				out = append(out, block)
			}
		}
		return out
	default:
		return []map[string]interface{}{{"type": "text", "text": fmt.Sprint(v)}}
	}
}

func parseDataURL(dataURL string) (mediaType, data string) {
	const prefix = "data:"
	if !strings.HasPrefix(dataURL, prefix) {
		return "application/octet-stream", dataURL
	}
	rest := dataURL[len(prefix):]
	sep := strings.Index(rest, ";base64,")
	if sep < 0 {
		return "", rest
	}
	return rest[:sep], rest[sep+8:]
}

func openaiAssistantToClaude(m map[string]interface{}) map[string]interface{} {
	msg := map[string]interface{}{"role": "assistant"}
	content := m["content"]
	if content != nil && content != "" {
		msg["content"] = openaiContentToClaude(content)
	}
	if tcRaw, ok := m["tool_calls"]; ok && tcRaw != nil {
		tcBytes, _ := json.Marshal(tcRaw)
		msg["content"] = appendContentBlocks(msg["content"], tcBytes)
	}
	return msg
}

func appendContentBlocks(existing interface{}, rawJSON []byte) []byte {
	var blocks []map[string]interface{}
	existingJSON, _ := json.Marshal(existing)
	json.Unmarshal(existingJSON, &blocks)
	var newBlocks []map[string]interface{}
	json.Unmarshal(rawJSON, &newBlocks)
	blocks = append(blocks, newBlocks...)
	out, _ := json.Marshal(blocks)
	return out
}

func openaiToClaudeResponse(body []byte, _, _ Format, _ map[string][]string) ([]byte, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Role      string            `json:"role"`
				Content   json.RawMessage   `json:"content"`
				ToolCalls []json.RawMessage `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return body, nil
	}
	if len(resp.Choices) == 0 {
		return body, nil
	}
	ch := resp.Choices[0]
	var content interface{}
	if len(ch.Message.Content) > 0 {
		json.Unmarshal(ch.Message.Content, &content)
	} else {
		content = ""
	}
	out := map[string]interface{}{
		"id":          "msg_<id>",
		"type":        "message",
		"role":        "assistant",
		"content":     content,
		"stop_reason": ch.FinishReason,
	}
	return json.Marshal(out)
}

// ─────────────────── anthropic → openai ──────────────────────────────────────

func claudeToOpenAIRequest(reqBody []byte, _, _ Format) ([]byte, error) {
	var req struct {
		Model       string      `json:"model"`
		Messages    []json.RawMessage `json:"messages"`
		MaxTokens   int         `json:"max_tokens"`
		Temperature float64    `json:"temperature"`
		Stream      bool        `json:"stream"`
		System      interface{} `json:"system"`
	}
	if err := json.Unmarshal(reqBody, &req); err != nil {
		return nil, err
	}
	out := map[string]interface{}{
		"model":    req.Model,
		"messages": claudeMessagesToOpenAI(req.Messages),
		"stream":   req.Stream,
	}
	if req.MaxTokens > 0 {
		out["max_tokens"] = req.MaxTokens
	}
	if req.Temperature > 0 {
		out["temperature"] = req.Temperature
	}
	if req.System != nil {
		out["system"] = req.System
	}
	return json.Marshal(out)
}

func claudeMessagesToOpenAI(msgs []json.RawMessage) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(msgs))
	for _, raw := range msgs {
		var m map[string]interface{}
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		role, _ := m["role"].(string)
		contentRaw := m["content"]
		switch role {
		case "assistant":
			out = append(out, claudeAssistantToOpenAI(contentRaw))
		case "user":
			// Check if this user message contains only tool_result blocks → convert to tool message.
			if blocks, ok := contentRaw.([]interface{}); ok && isPureToolResult(blocks) {
				for _, b := range blocks {
					if bm, ok := b.(map[string]interface{}); ok && bm["type"] == "tool_result" {
						out = append(out, map[string]interface{}{
							"role":        "tool",
							"tool_call_id": bm["tool_use_id"],
							"content":     bm["content"],
						})
					}
				}
			} else {
				out = append(out, map[string]interface{}{
					"role":    "user",
					"content": claudeContentToOpenAI(contentRaw),
				})
			}
		default:
			out = append(out, map[string]interface{}{
				"role":    role,
				"content": claudeContentToOpenAI(contentRaw),
			})
		}
	}
	return out
}

func isPureToolResult(blocks []interface{}) bool {
	if len(blocks) == 0 {
		return false
	}
	for _, b := range blocks {
		bm, ok := b.(map[string]interface{})
		if !ok || bm["type"] != "tool_result" {
			return false
		}
	}
	return true
}

func claudeAssistantToOpenAI(contentRaw interface{}) map[string]interface{} {
	msg := map[string]interface{}{"role": "assistant"}
	contentBlocks, ok := contentRaw.([]interface{})
	if !ok {
		msg["content"] = contentRaw
		return msg
	}
	var textParts []string
	var toolCalls []map[string]interface{}
	for _, block := range contentBlocks {
		b, _ := block.(map[string]interface{})
		switch b["type"] {
		case "text":
			if t, ok := b["text"].(string); ok {
				textParts = append(textParts, t)
			}
		case "tool_use":
			toolCalls = append(toolCalls, map[string]interface{}{
				"id":   b["id"],
				"type": "function",
				"function": map[string]interface{}{
					"name":      b["name"],
					"arguments": b["input"],
				},
			})
		}
	}
	if len(textParts) > 0 {
		msg["content"] = strings.Join(textParts, "\n")
	}
	if len(toolCalls) > 0 {
		msg["tool_calls"] = toolCalls
	}
	return msg
}

func claudeContentToOpenAI(contentRaw interface{}) interface{} {
	switch v := contentRaw.(type) {
	case string:
		return v
	case []interface{}:
		var textParts []string
		var outBlocks []interface{}
		for _, block := range v {
			b, ok := block.(map[string]interface{})
			if !ok {
				outBlocks = append(outBlocks, block)
				continue
			}
			switch b["type"] {
			case "text":
				if t, ok := b["text"].(string); ok {
					textParts = append(textParts, t)
				}
			case "image":
				src, _ := b["source"].(map[string]interface{})
				outBlocks = append(outBlocks, map[string]interface{}{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": fmt.Sprintf("data:%s;base64,%s", src["media_type"], src["data"]),
					},
				})
			default:
				outBlocks = append(outBlocks, b)
			}
		}
		if len(textParts) > 0 && len(outBlocks) == 0 {
			return strings.Join(textParts, "\n")
		}
		if len(outBlocks) > 0 {
			return outBlocks
		}
		return strings.Join(textParts, "\n")
	default:
		return v
	}
}

func claudeToOpenAIResponse(body []byte, _, _ Format, _ map[string][]string) ([]byte, error) {
	var resp struct {
		Type       string                   `json:"type"`
		ID         string                   `json:"id"`
		Role       string                   `json:"role"`
		Content    []map[string]interface{} `json:"content"`
		StopReason string                   `json:"stop_reason"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return body, nil
	}
	if resp.Type != "message" {
		return body, nil
	}
	var content interface{}
	for _, block := range resp.Content {
		if block["type"] == "text" {
			if t, ok := block["text"].(string); ok {
				if content == nil {
					content = t
				} else if s, ok := content.(string); ok {
					content = s + "\n" + t
				}
			}
		}
	}
	if content == nil {
		content = ""
	}
	out := map[string]interface{}{
		"id":      resp.ID,
		"object":  "chat.completion",
		"choices": []map[string]interface{}{{
			"index":         0,
			"message":       map[string]interface{}{"role": resp.Role, "content": content},
			"finish_reason": resp.StopReason,
		}},
	}
	return json.Marshal(out)
}

func passthroughStream(src io.Reader, dst io.Writer, _, _ Format) error {
	buf := make([]byte, 4096)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			dst.Write(buf[:n])
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
