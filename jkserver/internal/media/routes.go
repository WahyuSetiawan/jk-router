package media

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// Account is a media account loaded from the database.
type Account struct {
	ID         int64
	ProviderID string
	APIKey     string
	Active     bool
}

// IsActive reports whether this account can be used.
func (a *Account) IsActive() bool { return a.Active }

// Store loads media accounts from the database. Implementations populate the map.
type Store interface {
	LoadAccounts(fn func(*Account))
}

type bufWriter struct {
	bytes.Buffer
	header http.Header
	code   int
}

func newBufWriter() *bufWriter {
	return &bufWriter{header: make(http.Header), code: 200}
}

func (b *bufWriter) Header() http.Header      { return b.header }
func (b *bufWriter) WriteHeader(code int)     { b.code = code }

// Router wires media endpoints onto a chi.Mux.
// Endpoints: /v1/audio/speech, /v1/audio/transcriptions, /v1/images/generations, /v1/videos/*
func Router(d Store) chi.Router {
	r := chi.NewRouter()

	// In-memory account cache, refreshed on each request (low traffic service).
	var (
		cacheMu sync.RWMutex
		cache   map[int64]*Account
	)
	refreshCache := func() {
		if d == nil {
			cache = nil
			return
		}
		m := make(map[int64]*Account)
		d.LoadAccounts(func(a *Account) { m[a.ID] = a })
		cacheMu.Lock()
		cache = m
		cacheMu.Unlock()
	}

	// buildExecutorsForOp returns executors for the given operation filtered by provider_id=model if present.
	buildExecutorsForOp := func(model string, op OpType) []*Executor {
		cacheMu.RLock()
		defer cacheMu.RUnlock()
		var out []*Executor
		for _, a := range cache {
			if !a.Active {
				continue
			}
			reg := FindByID(a.ProviderID)
			if reg == nil {
				continue
			}
			var capCap Capability
			switch op {
			case OpTTS:
				capCap = CapTTS
			case OpSTT:
				capCap = CapSTT
			case OpImage:
				capCap = CapImage
			case OpVideo:
				capCap = CapVideo
			}
			if reg.Capabilities&capCap == 0 {
				continue
			}
			var exe *Executor
			switch op {
			case OpTTS:
				exe = NewTTSExecutor(reg, a.APIKey)
			case OpSTT:
				exe = NewSTTExecutor(reg, a.APIKey)
			case OpImage:
				exe = NewImageExecutor(reg, a.APIKey)
			case OpVideo:
				exe = NewVideoExecutor(reg, a.APIKey)
			}
			if exe != nil {
				out = append(out, exe)
			}
		}
		// If model specified, also try direct registry match (openai, elevenlabs, etc.)
		if model != "" {
			reg := FindByID(model)
			if reg != nil {
				var capCap Capability
				switch op {
				case OpTTS:
					capCap = CapTTS
				case OpSTT:
					capCap = CapSTT
				case OpImage:
					capCap = CapImage
				case OpVideo:
					capCap = CapVideo
				}
				if reg.Capabilities&capCap != 0 {
					found := false
					for _, a := range cache {
						if a.ProviderID == model && a.Active {
							found = true
							break
						}
					}
					if !found {
						var exe *Executor
						switch op {
						case OpTTS:
							exe = NewTTSExecutor(reg, "")
						case OpSTT:
							exe = NewSTTExecutor(reg, "")
						case OpImage:
							exe = NewImageExecutor(reg, "")
						case OpVideo:
							exe = NewVideoExecutor(reg, "")
						}
						if exe != nil {
							out = append([]*Executor{exe}, out...)
						}
					}
				}
			}
		}
		return out
	}

	// ── POST /v1/audio/speech ─────────────────────────────────────────────────
	r.Post("/audio/speech", func(w http.ResponseWriter, req *http.Request) {
		bearer := extractBearer(req)
		if bearer == "" {
			http.Error(w, `{"error":{"type":"invalid_request_error","message":"missing authorization"}}`, http.StatusUnauthorized)
			return
		}
		refreshCache()

		body, err := io.ReadAll(io.LimitReader(req.Body, 4*1024*1024))
		if err != nil {
			http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
			return
		}

		var reqBody struct {
			Model  string  `json:"model"`
			Voice  string  `json:"voice"`
			Input  string  `json:"input"`
			Format string  `json:"response_format"`
			Speed  float64 `json:"speed"`
		}
		if err := json.Unmarshal(body, &reqBody); err != nil || reqBody.Input == "" || reqBody.Model == "" {
			http.Error(w, `{"error":{"type":"invalid_request_error","message":"model and input are required"}}`, http.StatusBadRequest)
			return
		}

		executors := buildExecutorsForOp(reqBody.Model, OpTTS)
		if len(executors) == 0 {
			http.Error(w, fmt.Sprintf(`{"error":{"type":"invalid_request_error","message":"no active TTS accounts for model %q"}}`, reqBody.Model), http.StatusServiceUnavailable)
			return
		}

		start := time.Now()
		var lastErr error
		for i, exe := range executors {
			bw := newBufWriter()
			code, err := exe.ExecuteTextToSpeech(body, bw)
			if err == nil && code >= 200 && code < 300 {
				ct := bw.header.Get("Content-Type")
				if ct == "" {
					ct = "audio/mpeg"
				}
				w.Header().Set("Content-Type", ct)
				w.WriteHeader(bw.code)
				w.Write(bw.Bytes())
				log.Printf("[media/tts] model=%s elapsed=%dms", reqBody.Model, time.Since(start).Milliseconds())
				return
			}
			lastErr = err
			log.Printf("[media/tts] attempt=%d failed: %v", i, err)
		}
		http.Error(w, fmt.Sprintf(`{"error":{"type":"upstream_error","message":"all TTS providers failed: %v"}}`, lastErr), http.StatusBadGateway)
	})

	// ── POST /v1/audio/transcriptions ───────────────────────────────────────────
	r.Post("/audio/transcriptions", func(w http.ResponseWriter, req *http.Request) {
		bearer := extractBearer(req)
		if bearer == "" {
			http.Error(w, `{"error":{"type":"invalid_request_error","message":"missing authorization"}}`, http.StatusUnauthorized)
			return
		}
		if req.MultipartForm == nil {
			if err := req.ParseMultipartForm(50 << 20); err != nil {
				http.Error(w, `{"error":"multipart form required"}`, http.StatusBadRequest)
				return
			}
		}
		refreshCache()

		executors := buildExecutorsForOp("", OpSTT)
		if len(executors) == 0 {
			http.Error(w, `{"error":{"type":"invalid_request_error","message":"no active STT accounts"}}`, http.StatusServiceUnavailable)
			return
		}

		var lastErr error
		for i, exe := range executors {
			bw := newBufWriter()
			code, err := exe.ExecuteSpeechToText(req, bw)
			if err == nil && code >= 200 && code < 300 {
				for k, vv := range bw.header {
					for _, v := range vv {
						w.Header().Set(k, v)
					}
				}
				w.WriteHeader(bw.code)
				w.Write(bw.Bytes())
				return
			}
			lastErr = err
			log.Printf("[media/stt] attempt=%d failed: %v", i, err)
		}
		http.Error(w, fmt.Sprintf(`{"error":{"type":"upstream_error","message":"all STT providers failed: %v"}}`, lastErr), http.StatusBadGateway)
	})

	// ── POST /v1/images/generations ─────────────────────────────────────────────
	r.Post("/images/generations", func(w http.ResponseWriter, req *http.Request) {
		bearer := extractBearer(req)
		if bearer == "" {
			http.Error(w, `{"error":{"type":"invalid_request_error","message":"missing authorization"}}`, http.StatusUnauthorized)
			return
		}
		refreshCache()

		body, err := io.ReadAll(io.LimitReader(req.Body, 4*1024*1024))
		if err != nil {
			http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
			return
		}

		var reqBody struct {
			Model string `json:"model"`
		}
		json.Unmarshal(body, &reqBody)

		executors := buildExecutorsForOp(reqBody.Model, OpImage)
		if len(executors) == 0 {
			http.Error(w, `{"error":{"type":"invalid_request_error","message":"no active image accounts"}}`, http.StatusServiceUnavailable)
			return
		}

		var lastErr error
		for i, exe := range executors {
			bw := newBufWriter()
			code, err := exe.ExecuteImageGeneration(body, bw)
			if err == nil && code >= 200 && code < 300 {
				for k, vv := range bw.header {
					for _, v := range vv {
						w.Header().Set(k, v)
					}
				}
				w.WriteHeader(bw.code)
				w.Write(bw.Bytes())
				return
			}
			lastErr = err
			log.Printf("[media/image] attempt=%d failed: %v", i, err)
		}
		http.Error(w, fmt.Sprintf(`{"error":{"type":"upstream_error","message":"all image providers failed: %v"}}`, lastErr), http.StatusBadGateway)
	})

	// ── POST /v1/videos/generations ─────────────────────────────────────────────
	// ponytail: video providers use async APIs (poll job status). This is a thin
	// passthrough that returns whatever the provider returns — upgrade once a
	// concrete provider contract is chosen.
	r.Post("/videos/generations", func(w http.ResponseWriter, req *http.Request) {
		bearer := extractBearer(req)
		if bearer == "" {
			http.Error(w, `{"error":{"type":"invalid_request_error","message":"missing authorization"}}`, http.StatusUnauthorized)
			return
		}
		refreshCache()

		body, err := io.ReadAll(io.LimitReader(req.Body, 4*1024*1024))
		if err != nil {
			http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
			return
		}

		var reqBody struct {
			Model string `json:"model"`
		}
		json.Unmarshal(body, &reqBody)

		executors := buildExecutorsForOp(reqBody.Model, OpVideo)
		if len(executors) == 0 {
			http.Error(w, `{"error":{"type":"invalid_request_error","message":"no active video accounts"}}`, http.StatusServiceUnavailable)
			return
		}

		var lastErr error
		for i, exe := range executors {
			bw := newBufWriter()
			code, err := exe.ExecuteVideoGeneration(body, bw)
			if err == nil && code >= 200 && code < 300 {
				for k, vv := range bw.header {
					for _, v := range vv {
						w.Header().Set(k, v)
					}
				}
				w.WriteHeader(bw.code)
				w.Write(bw.Bytes())
				return
			}
			lastErr = err
			log.Printf("[media/video] attempt=%d failed: %v", i, err)
		}
		http.Error(w, fmt.Sprintf(`{"error":{"type":"upstream_error","message":"all video providers failed: %v"}}`, lastErr), http.StatusBadGateway)
	})

	return r
}

// OpType identifies the media operation.
type OpType int

const (
	OpTTS   OpType = 1
	OpSTT   OpType = 2
	OpImage OpType = 3
	OpVideo OpType = 4
)

func extractBearer(req *http.Request) string {
	auth := req.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return ""
}
