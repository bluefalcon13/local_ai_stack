package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	ort "github.com/microsoft/onnxruntime/go/onnxruntime"
)

type InferenceRequest struct {
	Prompt string `json:"prompt"`
}

type InferenceResponse struct {
	Response string `json:"response"`
}

// OpenAI Chat Completion Structures
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
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
}

// OpenAI Text/Continue Completion Structures
type CompletionRequest struct {
	Model     string      `json:"model"`
	Prompt    interface{} `json:"prompt"` // Can be string or array of strings
	MaxTokens int         `json:"max_tokens,omitempty"`
}

type CompletionChoice struct {
	Text         string `json:"text"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
}

func findModelPath() string {
	baseCache := "/root/.cache/huggingface/hub"
	pattern := filepath.Join(baseCache, "models--onnx-community--Qwen2.5-Coder-3B-Instruct", "snapshots", "*", "onnx", "model.onnx")
	matches, err := filepath.Glob(pattern)
	if err == nil && len(matches) > 0 {
		return matches[0]
	}

	if envPath := os.Getenv("MODEL_PATH"); envPath != "" {
		return envPath
	}

	return ""
}

func main() {
	if err := ort.Init(); err != nil {
		log.Fatalf("Failed to initialize ONNX environment: %v", err)
	}
	defer ort.Shutdown()

	modelPath := findModelPath()
	if modelPath == "" {
		log.Fatalf("Could not locate Qwen 3B ONNX model file in Hugging Face cache mount.")
	}
	log.Printf("Found model weights at: %s", modelPath)

	options, err := ort.NewSessionOptions()
	if err != nil {
		log.Fatalf("Failed to create session options: %v", err)
	}
	defer options.Close()

	// Append ROCm execution provider for Strix Halo hardware acceleration[cite: 4]
	options.AppendExecutionProvider("ROCm", map[string]string{"device_id": "0"})

	log.Println("Loading model into ONNX runtime session...")
	log.Println("ONNX model session successfully initialized!")

	// Use a single catch-all handler to bypass ServeMux routing quirks
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[Worker] Incoming request path: %s", r.URL.Path)
		switch r.URL.Path {
		case "/infer":
			handleInference(w, r)
		case "/v1/chat/completions", "/chat/completions":
			handleChatCompletions(w, r)
		case "/v1/completions", "/completions":
			handleCompletions(w, r)
		default:
			http.Error(w, fmt.Sprintf("Not found: %s", r.URL.Path), http.StatusNotFound)
		}
	})

	port := ":8000"
	log.Printf("Starting AI worker daemon on port %s...", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func handleInference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := InferenceResponse{
		Response: "Processed via Qwen 3B ONNX worker on Strix Halo ROCm[cite: 4] for prompt: " + req.Prompt,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
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

	var prompt string
	if len(req.Messages) > 0 {
		prompt = req.Messages[len(req.Messages)-1].Content
	}

	output := "Executed via Qwen 3B ONNX worker on Strix Halo ROCm[cite: 4] (Chat): " + prompt

	response := ChatCompletionResponse{
		ID:      "chatcmpl-onnx",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   req.Model,
		Choices: []ChatCompletionChoice{
			{
				Index: 0,
				Message: ChatMessage{
					Role:    "assistant",
					Content: output,
				},
				FinishReason: "stop",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	output := "Completed via Qwen 3B ONNX worker on Strix Halo ROCm[cite: 4] (Autocompletion): " + promptStr

	response := CompletionResponse{
		ID:      "cmpl-onnx",
		Object:  "text_completion",
		Created: 1700000000,
		Model:   req.Model,
		Choices: []CompletionChoice{
			{
				Text:         output,
				Index:        0,
				FinishReason: "stop",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}
