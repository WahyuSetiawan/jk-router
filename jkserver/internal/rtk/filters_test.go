package rtk_test

import (
	"encoding/json"
	"strings"
	"testing"

	"jkrouter/jkserver/internal/rtk"
)

func TestCavemanFilter(t *testing.T) {
	base := `{"messages":[{"role":"user","content":"Please respond as concisely as possible. Tell me about Go."}]}`
	reg := rtk.NewRegistry()
	reg.Enable("caveman")
	out, applied := reg.Apply([]byte(base))
	if len(applied) == 0 {
		t.Fatal("expected caveman to apply")
	}
	var req struct {
		Messages []struct{ Content string `json:"content"` } `json:"messages"`
	}
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Messages[0].Content == "" {
		t.Fatal("caveman stripped too much — content is empty")
	}
	if strings.Contains(req.Messages[0].Content, "Please respond as concisely") {
		t.Error("caveman should have stripped the prefix")
	}
}

func TestPonytailFilter(t *testing.T) {
	base := `{"messages":[{"role":"user","content":"// comment\n\nTell me about Go.\n/* block */\n\n\n\nFinal."}]}`
	reg := rtk.NewRegistry()
	reg.Enable("ponytail")
	out, applied := reg.Apply([]byte(base))
	if len(applied) == 0 {
		t.Fatal("expected ponytail to apply")
	}
	var req struct {
		Messages []struct{ Content string `json:"content"` } `json:"messages"`
	}
	json.Unmarshal(out, &req)
	c := req.Messages[0].Content
	if strings.Contains(c, "// comment") {
		t.Error("ponytail should strip // comments")
	}
	if strings.Contains(c, "block") {
		t.Error("ponytail should strip /* */ comments")
	}
	if strings.Contains(c, "\n\n\n") {
		t.Error("ponytail should collapse excessive blank lines")
	}
}

func TestHeadroomFilter(t *testing.T) {
	base := `{"messages":[{"role":"user","content":"hello world"},{"role":"system","content":"You are helpful."}]}`
	reg := rtk.NewRegistry()
	reg.Enable("headroom")
	out, _ := reg.Apply([]byte(base))
	var req struct{ Messages []struct{ Role string `json:"role"`; Content string `json:"content"` } `json:"messages"` }
	json.Unmarshal(out, &req)
	if req.Messages[1].Content != "You are helpful." {
		t.Error("system message should not be truncated")
	}
	_ = req.Messages[0].Content
}

func TestSystemInjectFilter(t *testing.T) {
	base := `{"messages":[{"role":"user","content":"hello"}]}`
	reg := rtk.NewRegistry()
	reg.Enable("system-inject")
	out, applied := reg.Apply([]byte(base))
	if len(applied) == 0 {
		t.Fatal("expected system-inject to apply")
	}
	var req struct{ Messages []struct{ Role string `json:"role"` } `json:"messages"` }
	json.Unmarshal(out, &req)
	if req.Messages[0].Role != "system" {
		t.Error("expected system message to be prepended")
	}
}

func TestNoOpWhenDisabled(t *testing.T) {
	base := `{"messages":[{"role":"user","content":"Please be concise. hello"}]}`
	reg := rtk.NewRegistry()
	// No filters enabled
	out, applied := reg.Apply([]byte(base))
	if len(applied) != 0 {
		t.Errorf("expected no filters applied, got %v", applied)
	}
	if string(out) != string(base) {
		t.Error("body should be unchanged when no filters enabled")
	}
}
