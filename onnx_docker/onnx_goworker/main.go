package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ----------------------------------------------------------------------------
// Core Types
// ----------------------------------------------------------------------------

type InferenceJob struct {
	Ctx         context.Context
	Prompt      string
	MaxTokens   int
	Temperature float64
	StreamCb    func(chunk string) bool // non-nil for streaming or early stop/cancel abort
	ReplyCh     chan InferenceResult
}

type InferenceResult struct {
	Output     string
	Stats      GenStats
	StatusCode int
	Err        error
}

type WorkerExecuteRequest struct {
	TaskID      string  `json:"task_id,omitempty"`
	TaskType    string  `json:"task_type,omitempty"`
	Instruction string  `json:"instruction,omitempty"`
	Context     string  `json:"context,omitempty"`
	Prompt      string  `json:"prompt,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

type WorkerExecuteResponse struct {
	TaskID           string  `json:"task_id,omitempty"`
	Output           string  `json:"output"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	DurationSeconds  float64 `json:"duration_seconds"`
	TokensPerSecond  float64 `json:"tokens_per_second"`
}

type CompletionRequest struct {
	Model       string      `json:"model"`
	Prompt      interface{} `json:"prompt"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Temperature float64     `json:"temperature,omitempty"`
	Stream      bool        `json:"stream,omitempty"`
	Stop        any         `json:"stop,omitempty"`
}

type CompletionChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type UsageInfo struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	TokensPerSecond  float64 `json:"tokens_per_second"`
	DurationSeconds  float64 `json:"duration_seconds"`
}

type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   UsageInfo          `json:"usage"`
}

type CompletionChunkChoice struct {
	Text         string  `json:"text"`
	Index        int     `json:"index"`
	FinishReason *string `json:"finish_reason"`
}

type CompletionChunk struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []CompletionChunkChoice `json:"choices"`
}

type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type ModelListResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// ----------------------------------------------------------------------------
// Global Queue & State
// ----------------------------------------------------------------------------

var (
	genaiModel     *GenAIModel
	inferenceQueue = make(chan InferenceJob, 64)
	maxNPUContext  = 4096
)

func findModelDir() string {
	modelPath := os.Getenv("MODEL_PATH")
	if modelPath != "" {
		if fi, err := os.Stat(modelPath); err == nil {
			if fi.IsDir() {
				return modelPath
			}
			return filepath.Dir(modelPath)
		}
	}

	candidates := []string{
		"/root/models/qwen-1.5b-npu",
		"/workspace/models/qwen-1.5b-npu",
		"/root/models/qwen-7b-npu",
		"/workspace/models/qwen-7b-npu",
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}

	baseCache := os.Getenv("HF_HOME")
	if baseCache == "" {
		baseCache = "/root/.cache/huggingface/hub"
	}
	matches, _ := filepath.Glob(filepath.Join(baseCache, "models--*--*", "snapshots", "*"))
	if len(matches) > 0 {
		return matches[0]
	}

	return ""
}

func configuredModelName() string {
	if m := strings.TrimSpace(os.Getenv("WORKER_MODEL")); m != "" {
		return m
	}
	return "qwen2.5-coder-1.5b-npu"
}

// ----------------------------------------------------------------------------
// Single Dedicated OS-Pinned NPU Worker
// ----------------------------------------------------------------------------

func startSerialNPUWorker() {
	go func() {
		// Pin this goroutine to a single OS thread to maintain XRT / Ryzen AI driver context
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		log.Println("[NPU Worker] Warming up XDNA 2 firmware and prefill/decode subgraphs...")
		_, _, err := genaiModel.GenerateStream("<|im_start|>user\nwarmup<|im_end|>\n<|im_start|>assistant\n", 16, 0.0, nil)
		if err != nil {
			log.Printf("[NPU Worker] Warmup returned error (non-fatal): %v", err)
		} else {
			log.Println("[NPU Worker] Warmup complete. Serial worker queue running at peak efficiency.")
		}

		for job := range inferenceQueue {
			if job.Ctx != nil && job.Ctx.Err() != nil {
				job.ReplyCh <- InferenceResult{StatusCode: http.StatusRequestTimeout, Err: job.Ctx.Err()}
				continue
			}
			if job.StreamCb != nil {
				out, stats, genErr := genaiModel.GenerateStream(job.Prompt, job.MaxTokens, job.Temperature, job.StreamCb)
				status := http.StatusOK
				if genErr != nil {
					status = http.StatusInternalServerError
				}
				job.ReplyCh <- InferenceResult{Output: out, Stats: stats, StatusCode: status, Err: genErr}
			} else {
				out, stats, genErr := genaiModel.Generate(job.Prompt, job.MaxTokens, job.Temperature)
				status := http.StatusOK
				if genErr != nil {
					status = http.StatusInternalServerError
				}
				job.ReplyCh <- InferenceResult{Output: out, Stats: stats, StatusCode: status, Err: genErr}
			}
		}
	}()
}

// ----------------------------------------------------------------------------
// Main Lifecycle
// ----------------------------------------------------------------------------

func main() {
	if envMax := os.Getenv("MAX_CONTEXT_LENGTH"); envMax != "" {
		var val int
		if _, err := fmt.Sscanf(envMax, "%d", &val); err == nil && val > 0 {
			maxNPUContext = val
		}
	}

	libPath := os.Getenv("GENAI_LIB_PATH")
	if libPath == "" {
		libPath = "/opt/amd/ryzenai/1.9.0/onnxruntime/lib/libonnxruntime-genai.so"
	}
	if _, err := os.Stat(libPath); err != nil {
		libPath = "libonnxruntime-genai.so"
	}

	log.Printf("Initializing ONNX Runtime GenAI from %s...", libPath)
	if err := InitGenAIRuntime(libPath); err != nil {
		log.Fatalf("FATAL: Failed to initialize GenAI runtime: %v", err)
	}

	modelDir := findModelDir()
	if modelDir == "" {
		log.Fatalf("FATAL: Could not locate GenAI model directory. Failing fast.")
	}
	log.Printf("Loading model directory: %s", modelDir)

	var err error
	genaiModel, err = LoadGenAIModel(modelDir)
	if err != nil {
		log.Fatalf("FATAL: Failed to load GenAI model from %s: %v", modelDir, err)
	}
	defer genaiModel.Close()

	log.Printf("AMD Ryzen AI GenAI model initialized (Max Context: %d tokens)!", maxNPUContext)

	// Start serial OS-pinned NPU worker loop
	startSerialNPUWorker()

	// Start 60-second periodic background telemetry logger
	startPeriodicTelemetryLogger(60 * time.Second)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[NPU Worker] %s %s", r.Method, r.URL.Path)
		switch r.URL.Path {
		case "/healthz", "/health":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":       "ok",
				"device":       "AMD XDNA 2 NPU (Ryzen AI)",
				"queue_length": len(inferenceQueue),
				"max_context":  maxNPUContext,
			})
		// Direct High-Throughput ADK Agent Endpoints (No OpenAI Ceremony)
		case "/execute", "/v1/worker/execute", "/agent/run":
			handleExecute(w, r)
		case "/infer":
			handleExecute(w, r)
		case "/v1/chat/completions", "/chat/completions":
			handleChatCompletions(w, r)
		// Lightweight FIM Autocomplete Endpoint for VS Code Continue
		case "/v1/completions", "/completions", "/autocomplete":
			handleCompletions(w, r)
		case "/v1/models", "/models":
			handleModels(w, r)
		case "/stats", "/v1/stats":
			handleStats(w, r)
		case "/metrics":
			handlePrometheusMetrics(w, r)
		default:
			http.Error(w, fmt.Sprintf("Not found: %s", r.URL.Path), http.StatusNotFound)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8000"
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	log.Printf("Starting NPU Agent worker daemon on port %s...", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// ----------------------------------------------------------------------------
// Direct ADK Agent Execution Handler
// ----------------------------------------------------------------------------

func handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed. POST required.", http.StatusMethodNotAllowed)
		return
	}

	var req WorkerExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	prompt := req.Prompt
	if prompt == "" {
		var sb strings.Builder
		sb.WriteString("<|im_start|>system\nYou are a specialized code generation worker running on the AMD Ryzen AI NPU. Produce only clean, production-ready, idiomatic code satisfying the exact acceptance criteria.<|im_end|>\n")
		if req.Context != "" {
			sb.WriteString(fmt.Sprintf("<|im_start|>user\nContext & Dependencies:\n%s\n\nTask:\n%s<|im_end|>\n", req.Context, req.Instruction))
		} else {
			sb.WriteString(fmt.Sprintf("<|im_start|>user\n%s<|im_end|>\n", req.Instruction))
		}
		sb.WriteString("<|im_start|>assistant\n")
		prompt = sb.String()
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	// 1. Fast-Fail Guardrail: Check Token Count Against NPU Context Ceiling
	pTokens, err := genaiModel.CountTokens(prompt)
	if err != nil {
		http.Error(w, "Token count failure: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if pTokens+maxTokens > maxNPUContext {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge) // HTTP 413
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":         "context_length_exceeded",
			"code":          http.StatusRequestEntityTooLarge,
			"prompt_tokens": pTokens,
			"max_tokens":    maxTokens,
			"max_context":   maxNPUContext,
			"message":       fmt.Sprintf("Prompt (%d tokens) plus max_tokens (%d) exceeds NPU limit of %d. Task must be split by Project Manager.", pTokens, maxTokens, maxNPUContext),
		})
		return
	}

	// 2. Dispatch to Serial Queue
	replyCh := make(chan InferenceResult, 1)
	job := InferenceJob{
		Ctx:         r.Context(),
		Prompt:      prompt,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		ReplyCh:     replyCh,
	}

	select {
	case inferenceQueue <- job:
	case <-r.Context().Done():
		http.Error(w, "Request canceled before enqueue", http.StatusRequestTimeout)
		return
	}

	// 3. Await Serial Execution
	select {
	case res := <-replyCh:
		if res.Err != nil {
			http.Error(w, res.Err.Error(), res.StatusCode)
			return
		}
		recordTelemetry(res.Stats)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Tokens-Per-Second", fmt.Sprintf("%.2f", res.Stats.TokensPerSecond))
		_ = json.NewEncoder(w).Encode(WorkerExecuteResponse{
			TaskID:           req.TaskID,
			Output:           res.Output,
			PromptTokens:     res.Stats.PromptTokens,
			CompletionTokens: res.Stats.CompletionTokens,
			TotalTokens:      res.Stats.TotalTokens,
			DurationSeconds:  res.Stats.Duration.Seconds(),
			TokensPerSecond:  res.Stats.TokensPerSecond,
		})

	case <-r.Context().Done():
		http.Error(w, "Client disconnected during inference", http.StatusRequestTimeout)
	}
}

// ----------------------------------------------------------------------------
// OpenAI-Compatible Chat Completions (Routed through Serial NPU Queue)
// ----------------------------------------------------------------------------

type ChatMessage struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   UsageInfo              `json:"usage"`
}

func handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var sb strings.Builder
	hasSystem := false
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			hasSystem = true
			break
		}
	}
	if !hasSystem {
		sb.WriteString("<|im_start|>system\nYou are Qwen, a specialized coding and forensic extraction worker running on the AMD Ryzen AI NPU.<|im_end|>\n")
	}
	for _, msg := range req.Messages {
		role := msg.Role
		if role == "" {
			role = "user"
		}
		sb.WriteString(fmt.Sprintf("<|im_start|>%s\n%s<|im_end|>\n", role, msg.Content))
	}
	sb.WriteString("<|im_start|>assistant\n")
	prompt := sb.String()

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	// 1. Guardrail against context overflow
	pTokens, err := genaiModel.CountTokens(prompt)
	if err == nil && pTokens+maxTokens > maxNPUContext {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusRequestEntityTooLarge) // HTTP 413
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("Prompt (%d tokens) plus max_tokens (%d) exceeds NPU limit of %d", pTokens, maxTokens, maxNPUContext),
				"type":    "context_length_exceeded",
				"code":    413,
			},
		})
		return
	}

	// 2. Queue into serial worker
	replyCh := make(chan InferenceResult, 1)
	job := InferenceJob{
		Ctx:         r.Context(),
		Prompt:      prompt,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		ReplyCh:     replyCh,
	}

	select {
	case inferenceQueue <- job:
	case <-r.Context().Done():
		http.Error(w, "Request canceled", http.StatusRequestTimeout)
		return
	}

	select {
	case res := <-replyCh:
		if res.Err != nil {
			http.Error(w, res.Err.Error(), res.StatusCode)
			return
		}
		recordTelemetry(res.Stats)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ChatCompletionResponse{
			ID:      fmt.Sprintf("chatcmpl-%d", time.Now().Unix()),
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []ChatCompletionChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: res.Output,
					},
					FinishReason: "stop",
				},
			},
			Usage: UsageInfo{
				PromptTokens:     res.Stats.PromptTokens,
				CompletionTokens: res.Stats.CompletionTokens,
				TotalTokens:      res.Stats.TotalTokens,
				TokensPerSecond:  res.Stats.TokensPerSecond,
				DurationSeconds:  res.Stats.Duration.Seconds(),
			},
		})
	case <-r.Context().Done():
		http.Error(w, "Client disconnected", http.StatusRequestTimeout)
	}
}

// ----------------------------------------------------------------------------
// Tab-Autocomplete FIM Endpoint (Preserved for VS Code Continue)
// ----------------------------------------------------------------------------

var qwenFIMSafetyStops = []string{
	"<|endoftext|>",
	"<|im_end|>",
	"<|fim_pad|>",
	"<|file_sep|>",
}

func parseStopSequences(raw any) []string {
	seen := make(map[string]struct{})
	var stops []string
	add := func(s string) {
		if s == "" {
			return
		}
		if _, exists := seen[s]; exists {
			return
		}
		seen[s] = struct{}{}
		stops = append(stops, s)
	}

	switch v := raw.(type) {
	case string:
		add(v)
	case []string:
		for _, s := range v {
			add(s)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	}

	for _, s := range qwenFIMSafetyStops {
		add(s)
	}
	return stops
}

type stopFilter struct {
	pending    string
	stops      []string
	maxStopLen int
	stopped    bool
}

func newStopFilter(stops []string) *stopFilter {
	maxLen := 0
	cleaned := make([]string, 0, len(stops))
	for _, s := range stops {
		if s == "" {
			continue
		}
		cleaned = append(cleaned, s)
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}
	return &stopFilter{
		stops:      cleaned,
		maxStopLen: maxLen,
	}
}

func (f *stopFilter) Push(chunk string) (string, bool) {
	if f.stopped {
		return "", true
	}
	if chunk == "" && f.pending == "" {
		return "", false
	}
	f.pending += chunk

	earliestIdx := -1
	for _, s := range f.stops {
		if idx := strings.Index(f.pending, s); idx != -1 {
			if earliestIdx == -1 || idx < earliestIdx {
				earliestIdx = idx
			}
		}
	}
	if earliestIdx != -1 {
		emit := f.pending[:earliestIdx]
		f.pending = ""
		f.stopped = true
		return emit, true
	}

	if f.maxStopLen <= 1 || len(f.pending) == 0 {
		emit := f.pending
		f.pending = ""
		return emit, false
	}

	startScan := len(f.pending) - (f.maxStopLen - 1)
	if startScan < 0 {
		startScan = 0
	}
	holdStart := len(f.pending)
	for i := startScan; i < len(f.pending); i++ {
		suffix := f.pending[i:]
		matched := false
		for _, s := range f.stops {
			if len(s) > len(suffix) && strings.HasPrefix(s, suffix) {
				matched = true
				break
			}
		}
		if matched {
			holdStart = i
			break
		}
	}

	emit := f.pending[:holdStart]
	f.pending = f.pending[holdStart:]
	return emit, false
}

func (f *stopFilter) Flush() string {
	if f.stopped || len(f.pending) == 0 {
		f.pending = ""
		return ""
	}
	out := f.pending
	f.pending = ""
	return out
}

func handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Context().Err() != nil {
		return
	}

	var req CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var promptStr string
	switch p := req.Prompt.(type) {
	case string:
		promptStr = p
	case []interface{}:
		if len(p) > 0 {
			if s, ok := p[0].(string); ok {
				promptStr = s
			}
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 128
	}

	if r.Context().Err() != nil {
		return
	}

	// Context Check (clamp maxTokens when prompt fits under maxNPUContext)
	pTokens, err := genaiModel.CountTokens(promptStr)
	if err == nil {
		if pTokens >= maxNPUContext {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "context_length_exceeded",
				"code":  413,
			})
			return
		}
		if pTokens+maxTokens > maxNPUContext {
			maxTokens = maxNPUContext - pTokens
		}
	}

	chunkID := fmt.Sprintf("cmpl-%d", time.Now().Unix())
	created := time.Now().Unix()
	modelName := req.Model
	if modelName == "" {
		modelName = configuredModelName()
	}
	filter := newStopFilter(parseStopSequences(req.Stop))

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		flusher, hasFlusher := w.(http.Flusher)
		writeChunk := func(text string, finishReason *string) {
			chunk := CompletionChunk{
				ID:      chunkID,
				Object:  "text_completion",
				Created: created,
				Model:   modelName,
				Choices: []CompletionChunkChoice{
					{
						Text:         text,
						Index:        0,
						FinishReason: finishReason,
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			if hasFlusher {
				flusher.Flush()
			}
		}

		replyCh := make(chan InferenceResult, 1)
		streamCb := func(chunkStr string) bool {
			if r.Context().Err() != nil {
				return false
			}
			emit, hitStop := filter.Push(chunkStr)
			if emit != "" {
				writeChunk(emit, nil)
			}
			return !hitStop
		}

		select {
		case inferenceQueue <- InferenceJob{
			Ctx:         r.Context(),
			Prompt:      promptStr,
			MaxTokens:   maxTokens,
			Temperature: req.Temperature,
			StreamCb:    streamCb,
			ReplyCh:     replyCh,
		}:
		case <-r.Context().Done():
			return
		}

		res := <-replyCh
		if r.Context().Err() != nil {
			return
		}
		if res.Err != nil {
			log.Printf("[NPU Worker] Streaming completion error: %v", res.Err)
			return
		}
		recordTelemetry(res.Stats)

		if tail := filter.Flush(); tail != "" {
			writeChunk(tail, nil)
		}

		stopReason := "stop"
		writeChunk("", &stopReason)
		fmt.Fprintf(w, "data: [DONE]\n\n")
		if hasFlusher {
			flusher.Flush()
		}
		return
	}

	// Non-streaming completion (also uses StreamCb to stop early on stop sequences/client disconnect)
	var outBuf strings.Builder
	replyCh := make(chan InferenceResult, 1)
	streamCb := func(chunkStr string) bool {
		if r.Context().Err() != nil {
			return false
		}
		emit, hitStop := filter.Push(chunkStr)
		if emit != "" {
			outBuf.WriteString(emit)
		}
		return !hitStop
	}

	select {
	case inferenceQueue <- InferenceJob{
		Ctx:         r.Context(),
		Prompt:      promptStr,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		StreamCb:    streamCb,
		ReplyCh:     replyCh,
	}:
	case <-r.Context().Done():
		return
	}

	res := <-replyCh
	if r.Context().Err() != nil {
		return
	}
	if res.Err != nil {
		http.Error(w, fmt.Sprintf("Generation error: %v", res.Err), res.StatusCode)
		return
	}
	recordTelemetry(res.Stats)

	if tail := filter.Flush(); tail != "" {
		outBuf.WriteString(tail)
	}

	w.Header().Set("X-Tokens-Per-Second", fmt.Sprintf("%.2f", res.Stats.TokensPerSecond))
	w.Header().Set("X-Completion-Tokens", fmt.Sprintf("%d", res.Stats.CompletionTokens))
	w.Header().Set("X-Duration-Seconds", fmt.Sprintf("%.3f", res.Stats.Duration.Seconds()))

	response := CompletionResponse{
		ID:      chunkID,
		Object:  "text_completion",
		Created: created,
		Model:   modelName,
		Choices: []CompletionChoice{
			{
				Text:         outBuf.String(),
				Index:        0,
				FinishReason: "stop",
			},
		},
		Usage: UsageInfo{
			PromptTokens:     res.Stats.PromptTokens,
			CompletionTokens: res.Stats.CompletionTokens,
			TotalTokens:      res.Stats.TotalTokens,
			TokensPerSecond:  res.Stats.TokensPerSecond,
			DurationSeconds:  res.Stats.Duration.Seconds(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// ----------------------------------------------------------------------------
// Telemetry & Metrics
// ----------------------------------------------------------------------------

func handleModels(w http.ResponseWriter, r *http.Request) {
	candidates := []string{
		"qwen2.5-coder-1.5b",
		"qwen2.5-coder-1.5b-npu",
		"Qwen2.5-Coder-1.5B-NPU",
		"amd/Qwen2.5-Coder-1.5B-Instruct_rai_1.9.0_npu",
		configuredModelName(),
	}
	seen := make(map[string]struct{}, len(candidates))
	models := make([]ModelInfo, 0, len(candidates))
	for _, id := range candidates {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, ModelInfo{
			ID:      id,
			Object:  "model",
			Created: 1700000000,
			OwnedBy: "amd-npu",
		})
	}
	resp := ModelListResponse{
		Object: "list",
		Data:   models,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

type TelemetrySample struct {
	Timestamp time.Time
	Tokens    int
	Duration  float64
	TPS       float64
}

type TelemetryState struct {
	mu            sync.RWMutex
	Samples       []TelemetrySample
	Window        time.Duration
	LastActive    time.Time
	LastLoggedReq int64
	TotalRequests int64
	PeakTPS       float64
	LastTPS       float64
}

var telemetry = TelemetryState{
	Window: 5 * time.Minute,
}

func pruneSamplesLocked(now time.Time) {
	cutoff := now.Add(-telemetry.Window)
	valid := 0
	for _, s := range telemetry.Samples {
		if s.Timestamp.After(cutoff) {
			telemetry.Samples[valid] = s
			valid++
		}
	}
	telemetry.Samples = telemetry.Samples[:valid]
}

func recordTelemetry(stats GenStats) {
	if stats.CompletionTokens <= 0 || stats.Duration <= 0 {
		return
	}
	tps := stats.TokensPerSecond
	now := time.Now()

	telemetry.mu.Lock()
	defer telemetry.mu.Unlock()

	telemetry.TotalRequests++
	telemetry.LastTPS = tps
	telemetry.LastActive = now
	if tps > telemetry.PeakTPS {
		telemetry.PeakTPS = tps
	}

	telemetry.Samples = append(telemetry.Samples, TelemetrySample{
		Timestamp: now,
		Tokens:    stats.CompletionTokens,
		Duration:  stats.Duration.Seconds(),
		TPS:       tps,
	})
	pruneSamplesLocked(now)
}

func startPeriodicTelemetryLogger(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			telemetry.mu.Lock()
			now := time.Now()
			pruneSamplesLocked(now)

			if telemetry.TotalRequests == telemetry.LastLoggedReq {
				telemetry.mu.Unlock()
				continue
			}
			telemetry.LastLoggedReq = telemetry.TotalRequests

			recentTokens := 0
			recentSecs := 0.0
			for _, s := range telemetry.Samples {
				recentTokens += s.Tokens
				recentSecs += s.Duration
			}
			avgTPS := 0.0
			if recentSecs > 0 {
				avgTPS = float64(recentTokens) / recentSecs
			}
			last := telemetry.LastTPS
			peak := telemetry.PeakTPS
			windowReqs := len(telemetry.Samples)
			totalReqs := telemetry.TotalRequests
			telemetry.mu.Unlock()

			log.Printf("[NPU Telemetry] Total Reqs: %d (Window: %d) | Window Tokens: %d | Rolling Avg: %.1f tok/s | Last: %.1f tok/s | Peak: %.1f tok/s | Queue: %d",
				totalReqs, windowReqs, recentTokens, avgTPS, last, peak, len(inferenceQueue))
		}
	}()
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	telemetry.mu.Lock()
	now := time.Now()
	pruneSamplesLocked(now)

	recentTokens := 0
	recentSecs := 0.0
	for _, s := range telemetry.Samples {
		recentTokens += s.Tokens
		recentSecs += s.Duration
	}
	avg := 0.0
	if recentSecs > 0 {
		avg = float64(recentTokens) / recentSecs
	}
	resp := map[string]interface{}{
		"device":                     "AMD NPU (XDNA 2 / Ryzen AI)",
		"model":                      configuredModelName(),
		"rolling_tokens_generated":   recentTokens,
		"rolling_average_tps":        avg,
		"last_tokens_per_second":     telemetry.LastTPS,
		"peak_tokens_per_second":     telemetry.PeakTPS,
		"total_requests":             telemetry.TotalRequests,
		"queue_depth":                len(inferenceQueue),
		"max_context":                maxNPUContext,
	}
	telemetry.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	telemetry.mu.Lock()
	now := time.Now()
	pruneSamplesLocked(now)

	recentTokens := 0
	recentSecs := 0.0
	for _, s := range telemetry.Samples {
		recentTokens += s.Tokens
		recentSecs += s.Duration
	}
	avg := 0.0
	if recentSecs > 0 {
		avg = float64(recentTokens) / recentSecs
	}
	last := telemetry.LastTPS
	peak := telemetry.PeakTPS
	totalReqs := telemetry.TotalRequests
	qDepth := len(inferenceQueue)
	telemetry.mu.Unlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("npu_tokens_generated_rolling %d\n", recentTokens))
	sb.WriteString(fmt.Sprintf("npu_tokens_per_second %.2f\n", last))
	sb.WriteString(fmt.Sprintf("npu_peak_tokens_per_second %.2f\n", peak))
	sb.WriteString(fmt.Sprintf("npu_average_tokens_per_second %.2f\n", avg))
	sb.WriteString(fmt.Sprintf("npu_requests_total %d\n", totalReqs))
	sb.WriteString(fmt.Sprintf("npu_queue_depth %d\n", qDepth))

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Write([]byte(sb.String()))
}
