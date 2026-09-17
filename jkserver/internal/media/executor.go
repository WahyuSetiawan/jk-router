package media

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Executor handles one media operation against an upstream provider.
type Executor struct {
	client   *http.Client
	baseURL  string
	path     string
	authHdr  string
	authPfx  string
	oper     string // "tts", "stt", "image", "video"
}

// NewTTSExecutor creates an executor for text-to-speech.
func NewTTSExecutor(r *Registry, apiKey string) *Executor {
	return &Executor{
		client:    &http.Client{Timeout: 60 * time.Second},
		baseURL:   r.TTSBaseURL,
		path:      r.TTSPath,
		authHdr:   r.TTSAuthHeader,
		authPfx:   r.TTSAuthPrefix,
		oper:      "tts",
	}
}

// NewSTTExecutor creates an executor for speech-to-text.
func NewSTTExecutor(r *Registry, apiKey string) *Executor {
	return &Executor{
		client:    &http.Client{Timeout: 60 * time.Second},
		baseURL:   r.STTBaseURL,
		path:      r.STTPath,
		authHdr:   r.STTAuthHeader,
		authPfx:   r.STTAuthPrefix,
		oper:      "stt",
	}
}

// NewImageExecutor creates an executor for image generation.
func NewImageExecutor(r *Registry, apiKey string) *Executor {
	return &Executor{
		client:    &http.Client{Timeout: 120 * time.Second},
		baseURL:   r.ImageBaseURL,
		path:      r.ImagePath,
		authHdr:   r.ImageAuthHeader,
		authPfx:   r.ImageAuthPrefix,
		oper:      "image",
	}
}

// NewVideoExecutor creates an executor for video generation.
func NewVideoExecutor(r *Registry, apiKey string) *Executor {
	return &Executor{
		client:    &http.Client{Timeout: 120 * time.Second},
		baseURL:   r.VideoBaseURL,
		path:      r.VideoPath,
		authHdr:   r.VideoAuthHeader,
		authPfx:   r.VideoAuthPrefix,
		oper:      "video",
	}
}

// ExecuteTextToSpeech sends a TTS request. Returns status code and error.
// Caller writes bw.Bytes() to the real response writer on success.
func (e *Executor) ExecuteTextToSpeech(body []byte, bw *bufWriter) (int, error) {
	req, err := http.NewRequest("POST", e.baseURL+e.path, bytes.NewReader(body))
	if err != nil {
		return 502, err
	}
	setAuth(req, e.authHdr, e.authPfx, "")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/*")
	return e.do(req, bw)
}

// ExecuteSpeechToText reads a multipart audio file from req and sends it upstream.
// Returns status code and error. Caller writes bw.Bytes() to the real response on success.
func (e *Executor) ExecuteSpeechToText(req *http.Request, bw *bufWriter) (int, error) {
	var buf bytes.Buffer
	wr := multipart.NewWriter(&buf)
	for k, vv := range req.MultipartForm.Value {
		for _, v := range vv {
			wr.WriteField(k, v)
		}
	}
	for k, fh := range req.MultipartForm.File {
		for _, f := range fh {
			fw, _ := wr.CreateFormFile(k, f.Filename)
			r, _ := f.Open()
			io.Copy(fw, r)
		}
	}
	wr.Close()

	httpReq, err := http.NewRequest("POST", e.baseURL+e.path, &buf)
	if err != nil {
		return 502, err
	}
	setAuth(httpReq, e.authHdr, e.authPfx, "")
	httpReq.Header.Set("Content-Type", wr.FormDataContentType())
	return e.do(httpReq, bw)
}

// ExecuteImageGeneration sends an image generation request. Returns status and error.
func (e *Executor) ExecuteImageGeneration(body []byte, bw *bufWriter) (int, error) {
	req, err := http.NewRequest("POST", e.baseURL+e.path, bytes.NewReader(body))
	if err != nil {
		return 502, err
	}
	setAuth(req, e.authHdr, e.authPfx, "")
	req.Header.Set("Content-Type", "application/json")
	return e.do(req, bw)
}

// ExecuteVideoGeneration sends a video generation request. Returns status and error.
func (e *Executor) ExecuteVideoGeneration(body []byte, bw *bufWriter) (int, error) {
	req, err := http.NewRequest("POST", e.baseURL+e.path, bytes.NewReader(body))
	if err != nil {
		return 502, err
	}
	setAuth(req, e.authHdr, e.authPfx, "")
	req.Header.Set("Content-Type", "application/json")
	return e.do(req, bw)
}

func (e *Executor) do(req *http.Request, bw *bufWriter) (int, error) {
	resp, err := e.client.Do(req)
	if err != nil {
		writeError(bw, fmt.Sprintf("%s upstream error: %v", e.oper, err), 502)
		return 502, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))

	if resp.StatusCode >= 400 {
		ct := resp.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/json") {
			bw.Header().Set("Content-Type", "application/json")
			bw.WriteHeader(resp.StatusCode)
			bw.Write(body)
			return resp.StatusCode, nil
		}
		writeError(bw, fmt.Sprintf("%s upstream error (status %d)", e.oper, resp.StatusCode), resp.StatusCode)
		return resp.StatusCode, fmt.Errorf("upstream %d", resp.StatusCode)
	}

	for k, vv := range resp.Header {
		for _, v := range vv {
			bw.Header().Set(k, v)
		}
	}
	bw.WriteHeader(resp.StatusCode)
	bw.Write(body)
	return resp.StatusCode, nil
}

func setAuth(req *http.Request, header, prefix, token string) {
	h := header
	if h == "" {
		h = "Authorization"
	}
	p := prefix
	if p == "" {
		p = "Bearer"
	}
	if token != "" {
		req.Header.Set(h, strings.TrimSpace(p)+" "+token)
	}
}

func writeError(bw *bufWriter, msg string, status int) {
	bw.Header().Set("Content-Type", "application/json")
	bw.WriteHeader(status)
	json.NewEncoder(bw).Encode(map[string]interface{}{
		"error": map[string]string{
			"type":    "upstream_error",
			"message": msg,
		},
	})
}
