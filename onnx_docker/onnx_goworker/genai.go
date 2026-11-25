package main

/*
#cgo LDFLAGS: -ldl
#include "genai.h"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
	"unsafe"
)

type GenAIModel struct {
	model     *C.OgaModel
	tokenizer *C.OgaTokenizer
	mu        sync.Mutex
}

func InitGenAIRuntime(libPath string) error {
	cLib := C.CString(libPath)
	defer C.free(unsafe.Pointer(cLib))
	if ret := C.InitGenAI(cLib); ret != 0 {
		return fmt.Errorf("failed to init GenAI (code %d): %s", ret, C.GoString(C.GetGenAILastError()))
	}
	return nil
}

func LoadGenAIModel(modelDir string) (*GenAIModel, error) {
	cDir := C.CString(modelDir)
	defer C.free(unsafe.Pointer(cDir))

	var model *C.OgaModel
	res := C.GenAI_CreateModel(cDir, &model)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		return nil, errors.New(errStr)
	}

	var tokenizer *C.OgaTokenizer
	res = C.GenAI_CreateTokenizer(model, &tokenizer)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		C.GenAI_DestroyModel(model)
		return nil, errors.New(errStr)
	}

	return &GenAIModel{
		model:     model,
		tokenizer: tokenizer,
	}, nil
}

func (m *GenAIModel) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tokenizer != nil {
		C.GenAI_DestroyTokenizer(m.tokenizer)
		m.tokenizer = nil
	}
	if m.model != nil {
		C.GenAI_DestroyModel(m.model)
		m.model = nil
	}
}

// GenStats records throughput and timing metrics for a generation run.
type GenStats struct {
	PromptTokens     int           `json:"prompt_tokens"`
	CompletionTokens int           `json:"completion_tokens"`
	TotalTokens      int           `json:"total_tokens"`
	Duration         time.Duration `json:"duration"`
	TokensPerSecond  float64       `json:"tokens_per_second"`
}

func (m *GenAIModel) Generate(prompt string, maxTokens int, temperature float64) (string, GenStats, error) {
	return m.GenerateStream(prompt, maxTokens, temperature, nil)
}

func (m *GenAIModel) CountTokens(prompt string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.tokenizer == nil {
		return 0, errors.New("tokenizer not initialized")
	}

	cPrompt := C.CString(prompt)
	defer C.free(unsafe.Pointer(cPrompt))

	var sequences *C.OgaSequences
	res := C.GenAI_CreateSequences(&sequences)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		return 0, errors.New(errStr)
	}
	defer C.GenAI_DestroySequences(sequences)

	res = C.GenAI_TokenizerEncode(m.tokenizer, cPrompt, sequences)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		return 0, errors.New(errStr)
	}

	count := int(C.GenAI_SequencesGetSequenceCount(sequences, 0))
	return count, nil
}

func (m *GenAIModel) GenerateStream(prompt string, maxTokens int, temperature float64, tokenCb func(string) bool) (string, GenStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.model == nil || m.tokenizer == nil {
		return "", GenStats{}, errors.New("model or tokenizer is not initialized")
	}

	cPrompt := C.CString(prompt)
	defer C.free(unsafe.Pointer(cPrompt))

	// Encode prompt
	var sequences *C.OgaSequences
	res := C.GenAI_CreateSequences(&sequences)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		return "", GenStats{}, errors.New(errStr)
	}
	defer C.GenAI_DestroySequences(sequences)

	res = C.GenAI_TokenizerEncode(m.tokenizer, cPrompt, sequences)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		return "", GenStats{}, errors.New(errStr)
	}

	debugTokens := os.Getenv("DEBUG_TOKENS") == "1"
	promptTokens := int(C.GenAI_SequencesGetSequenceCount(sequences, 0))
	tokenData := C.GenAI_SequencesGetSequenceData(sequences, 0)
	if debugTokens && tokenData != nil && promptTokens > 0 {
		firstTokens := unsafe.Slice((*int32)(unsafe.Pointer(tokenData)), promptTokens)
		if len(firstTokens) > 20 {
			log.Printf("[GenAI] Prompt has %d tokens. First 20: %v", promptTokens, firstTokens[:20])
		} else {
			log.Printf("[GenAI] Prompt has %d tokens: %v", promptTokens, firstTokens)
		}
	}
	if maxTokens <= 0 {
		maxTokens = 512
	}
	totalMaxLen := promptTokens + maxTokens
	maxSeqLen := 4096
	if envMax := os.Getenv("MAX_CONTEXT_LENGTH"); envMax != "" {
		var val int
		if _, err := fmt.Sscanf(envMax, "%d", &val); err == nil && val > 0 {
			maxSeqLen = val
		}
	}
	if promptTokens >= maxSeqLen {
		return "", GenStats{}, fmt.Errorf("prompt length (%d tokens) exceeds NPU maximum context window (%d)", promptTokens, maxSeqLen)
	}
	if totalMaxLen > maxSeqLen {
		totalMaxLen = maxSeqLen
	}

	// Generator params
	var params *C.OgaGeneratorParams
	res = C.GenAI_CreateGeneratorParams(m.model, &params)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		return "", GenStats{}, errors.New(errStr)
	}
	defer C.GenAI_DestroyGeneratorParams(params)

	cMaxLen := C.CString("max_length")
	C.GenAI_GeneratorParamsSetSearchNumber(params, cMaxLen, C.double(totalMaxLen))
	C.free(unsafe.Pointer(cMaxLen))

	if temperature > 0.0 {
		cDoSample := C.CString("do_sample")
		C.GenAI_GeneratorParamsSetSearchBool(params, cDoSample, C.bool(true))
		C.free(unsafe.Pointer(cDoSample))

		cTemp := C.CString("temperature")
		C.GenAI_GeneratorParamsSetSearchNumber(params, cTemp, C.double(temperature))
		C.free(unsafe.Pointer(cTemp))
	}

	// Create generator
	var generator *C.OgaGenerator
	res = C.GenAI_CreateGenerator(m.model, params, &generator)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		return "", GenStats{}, errors.New(errStr)
	}
	defer C.GenAI_DestroyGenerator(generator)

	res = C.GenAI_Generator_AppendTokenSequences(generator, sequences)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		return "", GenStats{}, errors.New(errStr)
	}

	// Tokenizer stream
	var stream *C.OgaTokenizerStream
	res = C.GenAI_CreateTokenizerStream(m.tokenizer, &stream)
	if res != nil {
		errStr := C.GoString(C.GenAI_ResultGetError(res))
		C.GenAI_DestroyResult(res)
		return "", GenStats{}, errors.New(errStr)
	}
	defer C.GenAI_DestroyTokenizerStream(stream)

	startTime := time.Now()
	tokenCount := 0

	var sb strings.Builder
	for !bool(C.GenAI_Generator_IsDone(generator)) {
		res = C.GenAI_Generator_GenerateNextToken(generator)
		if res != nil {
			errStr := C.GoString(C.GenAI_ResultGetError(res))
			C.GenAI_DestroyResult(res)
			return sb.String(), GenStats{}, errors.New(errStr)
		}

		var nextTokens *C.int32_t
		var count C.size_t
		res = C.GenAI_Generator_GetNextTokens(generator, &nextTokens, &count)
		if res != nil {
			errStr := C.GoString(C.GenAI_ResultGetError(res))
			C.GenAI_DestroyResult(res)
			return sb.String(), GenStats{}, errors.New(errStr)
		}

		if count > 0 {
			tokenSlice := unsafe.Slice((*int32)(unsafe.Pointer(nextTokens)), int(count))
			latestToken := tokenSlice[len(tokenSlice)-1]
			tokenCount++

			var cChunk *C.char
			res = C.GenAI_TokenizerStreamDecode(stream, C.int32_t(latestToken), &cChunk)
			if res == nil && cChunk != nil {
				chunkStr := C.GoString(cChunk)
				if debugTokens {
					log.Printf("[GenAI] Emitted token %d: %q", latestToken, chunkStr)
				}
				sb.WriteString(chunkStr)
				if tokenCb != nil {
					if !tokenCb(chunkStr) {
						break
					}
				}
			}
			if res != nil {
				C.GenAI_DestroyResult(res)
			}
		}
	}

	elapsed := time.Since(startTime)
	tokSec := 0.0
	if elapsed.Seconds() > 0 {
		tokSec = float64(tokenCount) / elapsed.Seconds()
	}
	stats := GenStats{
		PromptTokens:     promptTokens,
		CompletionTokens: tokenCount,
		TotalTokens:      promptTokens + tokenCount,
		Duration:         elapsed,
		TokensPerSecond:  tokSec,
	}
	log.Printf("[GenAI] Generation finished: %d tokens generated in %s (%.1f tok/s, prompt=%d)", tokenCount, elapsed.Round(time.Millisecond), tokSec, promptTokens)

	return sb.String(), stats, nil
}

