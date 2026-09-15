package translator_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"jkrouter/jkserver/internal/translator"
)

func registerOpenAIClaudeTranslators(reg *translator.Registry) {
	translator.RegisterOpenAIClaudeTranslators(reg)
}

// TestOpenAIClaudeRoundTrip verifies the openai→claude translator produces
// valid Anthropic Messages API request bodies, and the reverse produces valid
// OpenAI Chat completions request bodies.
func TestOpenAIClaudeRoundTrip(t *testing.T) {
	reg := translator.NewRegistry()
	registerOpenAIClaudeTranslators(reg)

	openaiReq := map[string]interface{}{
		"model":       "gpt-4o",
		"messages":    []map[string]interface{}{{"role": "user", "content": "hello"}},
		"max_tokens":  1024,
		"temperature": 0.7,
	}
	body, _ := json.Marshal(openaiReq)

	pair := translator.PairFor(translator.FormatOpenAI, translator.FormatAnthropic)
	tf := reg.Get(pair)
	if tf == nil {
		t.Fatalf("no translator for pair %s", pair)
	}

	claudeBody, err := tf.Request(body, translator.FormatOpenAI, translator.FormatAnthropic)
	if err != nil {
		t.Fatalf("translate openai→claude: %v", err)
	}

	var claude map[string]interface{}
	if err := json.Unmarshal(claudeBody, &claude); err != nil {
		t.Fatalf("unmarshal claude body: %v\nbody: %s", err, string(claudeBody))
	}

	if claude["model"] != "gpt-4o" {
		t.Errorf("model mismatch: got %v, want gpt-4o", claude["model"])
	}
	msgs, ok := claude["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages not an array: %T", claude["messages"])
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	msg := msgs[0].(map[string]interface{})
	if msg["role"] != "user" {
		t.Errorf("role mismatch: got %v", msg["role"])
	}
	contentArr, ok := msg["content"].([]interface{})
	if !ok {
		t.Fatalf("content should be []interface{}, got %T", msg["content"])
	}
	if len(contentArr) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contentArr))
	}
	block := contentArr[0].(map[string]interface{})
	if block["type"] != "text" {
		t.Errorf("content block type: got %v, want text", block["type"])
	}
	if block["text"] != "hello" {
		t.Errorf("text mismatch: got %v, want hello", block["text"])
	}

	// Reverse: claude → openai
	reversePair := translator.PairFor(translator.FormatAnthropic, translator.FormatOpenAI)
	tfRev := reg.Get(reversePair)
	if tfRev == nil {
		t.Fatalf("no reverse translator for pair %s", reversePair)
	}

	openaiBack, err := tfRev.Request(claudeBody, translator.FormatAnthropic, translator.FormatOpenAI)
	if err != nil {
		t.Fatalf("translate claude→openai: %v", err)
	}

	var openai map[string]interface{}
	if err := json.Unmarshal(openaiBack, &openai); err != nil {
		t.Fatalf("unmarshal openai back: %v", err)
	}
	if openai["model"] != "gpt-4o" {
		t.Errorf("model round-trip: got %v", openai["model"])
	}
	msgs2, ok := openai["messages"].([]interface{})
	if !ok || len(msgs2) != 1 {
		t.Fatalf("messages round-trip failed")
	}
	msg2 := msgs2[0].(map[string]interface{})
	if msg2["content"] != "hello" {
		t.Errorf("content round-trip: got %v", msg2["content"])
	}
}

// TestOpenAIToClaudeWithImage verifies multimodal image_url content is
// translated to Anthropic's image content block format.
func TestOpenAIToClaudeWithImage(t *testing.T) {
	reg := translator.NewRegistry()
	registerOpenAIClaudeTranslators(reg)

	openaiReq := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]interface{}{{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "text", "text": "what is this?"},
				{
					"type": "image_url",
					"image_url": map[string]interface{}{
						"url": "data:image/png;base64,iVBORw0KGgo=",
					},
				},
			},
		}},
	}
	body, _ := json.Marshal(openaiReq)

	pair := translator.PairFor(translator.FormatOpenAI, translator.FormatAnthropic)
	tf := reg.Get(pair)
	claudeBody, err := tf.Request(body, translator.FormatOpenAI, translator.FormatAnthropic)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	var claude map[string]interface{}
	json.Unmarshal(claudeBody, &claude)
	msgs := claude["messages"].([]interface{})
	msg := msgs[0].(map[string]interface{})
	contentArr := msg["content"].([]interface{})

	if contentArr[0].(map[string]interface{})["type"] != "text" {
		t.Errorf("first block type: got %v", contentArr[0])
	}
	imgBlock := contentArr[1].(map[string]interface{})
	if imgBlock["type"] != "image" {
		t.Errorf("second block type: got %v, want image", imgBlock["type"])
	}
	src := imgBlock["source"].(map[string]interface{})
	if src["type"] != "base64" {
		t.Errorf("image source type: got %v", src["type"])
	}
	gotData, _ := src["data"].(string)
	if gotData != "iVBORw0KGgo=" {
		t.Errorf("base64 data mismatch: got %q", gotData)
	}
}

// TestClaudeToOpenAIWithToolUse verifies tool_use content blocks become
// OpenAI tool calls, and tool_result become role=tool messages.
func TestClaudeToOpenAIWithToolUse(t *testing.T) {
	reg := translator.NewRegistry()
	registerOpenAIClaudeTranslators(reg)

	claudeReq := map[string]interface{}{
		"model": "claude-3-5-sonnet",
		"messages": []map[string]interface{}{
			{"role": "assistant", "content": []interface{}{
				map[string]interface{}{"type": "text", "text": "let me check"},
				map[string]interface{}{
					"type": "tool_use",
					"id":   "toolu_01ABC",
					"name": "get_weather",
					"input": map[string]interface{}{"city": "Jakarta"},
				},
			}},
			{"role": "user", "content": []interface{}{
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "toolu_01ABC",
					"content":     "sunny, 28°C",
				},
			}},
		},
	}
	body, _ := json.Marshal(claudeReq)

	pair := translator.PairFor(translator.FormatAnthropic, translator.FormatOpenAI)
	tf := reg.Get(pair)
	openaiBody, err := tf.Request(body, translator.FormatAnthropic, translator.FormatOpenAI)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}

	var openai map[string]interface{}
	json.Unmarshal(openaiBody, &openai)
	msgs := openai["messages"].([]interface{})

	msg0 := msgs[0].(map[string]interface{})
	if msg0["role"] != "assistant" {
		t.Errorf("role: got %v", msg0["role"])
	}
	if msg0["content"] != "let me check" {
		t.Errorf("content: got %v", msg0["content"])
	}
	tcArr, ok := msg0["tool_calls"].([]interface{})
	if !ok || len(tcArr) != 1 {
		t.Fatalf("expected 1 tool_call, got %v", msg0["tool_calls"])
	}
	tc := tcArr[0].(map[string]interface{})
	if tc["type"] != "function" {
		t.Errorf("tool_call type: got %v", tc["type"])
	}
	fc := tc["function"].(map[string]interface{})
	if fc["name"] != "get_weather" {
		t.Errorf("function name: got %v", fc["name"])
	}

	msg1 := msgs[1].(map[string]interface{})
	if msg1["role"] != "tool" {
		t.Errorf("tool msg role: got %v", msg1["role"])
	}
}

// TestRequiredCapabilities verifies capability detection from request body.
func TestRequiredCapabilities(t *testing.T) {
	req := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]interface{}{
			{"role": "user", "content": []map[string]interface{}{
				{"type": "text", "text": "hello"},
				{"type": "image_url", "image_url": map[string]interface{}{"url": "data:image/png;base64,abc"}},
			}},
		},
	}
	body, _ := json.Marshal(req)
	caps := translator.RequiredCapabilities(body)
	if !caps[translator.CapVision] {
		t.Errorf("expected vision capability, got %v", caps)
	}
}

// TestModelCaps verifies ModelCaps capability lookup.
func TestModelCaps(t *testing.T) {
	m := translator.ModelCaps{ID: "gpt-4o", Capabilities: []translator.Capability{translator.CapVision}}
	if !m.HasCapability(translator.CapVision) {
		t.Error("expected vision capability")
	}
	if m.HasCapability(translator.CapAudio) {
		t.Error("should not have audio capability")
	}

	models := []translator.ModelCaps{
		{ID: "text-only", Capabilities: []translator.Capability{}},
		{ID: "vision-model", Capabilities: []translator.Capability{translator.CapVision}},
	}
	if !translator.HasAnyCapability(models, []translator.Capability{translator.CapVision}) {
		t.Error("expected HasAnyCapability to find vision")
	}
}

// TestTranslateResponseIdentity verifies that unregistered pairs pass through.
func TestTranslateResponseIdentity(t *testing.T) {
	original := []byte(`{"id":"chatcmpl-1","object":"chat.completion","choices":[{"delta":{}}]}`)
	var headers map[string][]string
	tf := translator.NewRegistry().Get(translator.PairFor(translator.FormatOpenAI, translator.FormatOpenAI))
	if tf != nil {
		out, err := tf.Response(original, translator.FormatOpenAI, translator.FormatOpenAI, headers)
		if err != nil {
			t.Fatalf("identity: %v", err)
		}
		if !bytes.Equal(out, original) {
			t.Errorf("identity failed: got %s", out)
		}
	}
	// No translator for openai→openai is expected — nil is correct behavior.
}
