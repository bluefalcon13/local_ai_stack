package main

import (
	"slices"
	"strings"
	"testing"
)

func TestParseStopSequences(t *testing.T) {
	// Single string input
	single := parseStopSequences("\n\n")
	if !slices.Contains(single, "\n\n") {
		t.Fatalf("expected single stop sequence to be preserved, got %v", single)
	}
	for _, required := range qwenFIMSafetyStops {
		if !slices.Contains(single, required) {
			t.Errorf("expected Qwen FIM safety stop %q in %v", required, single)
		}
	}

	// Array with duplicates, empty strings, and non-strings
	arr := parseStopSequences([]any{"", "\n", "\n\n", "\n", 42, "<|file_sep|>"})
	countNL := 0
	countFileSep := 0
	for _, s := range arr {
		if s == "" {
			t.Errorf("empty stop sequence should have been filtered: %v", arr)
		}
		if s == "\n" {
			countNL++
		}
		if s == "<|file_sep|>" {
			countFileSep++
		}
	}
	if countNL != 1 {
		t.Errorf("expected \\n once after dedup, got %d in %v", countNL, arr)
	}
	if countFileSep != 1 {
		t.Errorf("expected <|file_sep|> once after dedup, got %d in %v", countFileSep, arr)
	}
}

func TestStopFilter_MultiTokenAndHoldback(t *testing.T) {
	// Case 1: "\n\n" split across chunks ("\n" then "\nnext")
	f := newStopFilter(parseStopSequences([]any{"\n\n"}))
	emit, hit := f.Push("return x\n")
	if hit {
		t.Fatalf("unexpected stop hit on single newline")
	}
	if emit != "return x" {
		t.Fatalf("expected \"return x\" emitted with newline held back, got %q", emit)
	}
	emit, hit = f.Push("\nnextLine()")
	if !hit {
		t.Fatalf("expected stop hit on second newline")
	}
	if emit != "" {
		t.Fatalf("expected empty emit at stop boundary, got %q", emit)
	}
	if tail := f.Flush(); tail != "" {
		t.Fatalf("expected empty Flush() after stop, got %q", tail)
	}

	// Case 2: "<|file_sep|>" split across chunks ("foo<|file" then "_sep|>rest")
	f2 := newStopFilter(parseStopSequences(nil))
	emit, hit = f2.Push("foo<|file")
	if hit {
		t.Fatalf("unexpected stop hit on partial marker")
	}
	if emit != "foo" || strings.Contains(emit, "<|file") {
		t.Fatalf("expected \"foo\" without partial stop marker leak, got %q", emit)
	}
	emit, hit = f2.Push("_sep|>rest")
	if !hit {
		t.Fatalf("expected stop hit when marker completed")
	}
	if emit != "" {
		t.Fatalf("expected empty emit before marker, got %q", emit)
	}
	if tail := f2.Flush(); tail != "" {
		t.Fatalf("expected empty Flush() after marker stop, got %q", tail)
	}
}

func TestStopFilter_PartialFalseAlarmFlushes(t *testing.T) {
	f := newStopFilter(parseStopSequences([]any{"\n\n"}))

	// Chunk ends with a potential stop prefix "\n" -> held back
	emit, hit := f.Push("line1\n")
	if hit || emit != "line1" {
		t.Fatalf("got emit=%q hit=%v, want \"line1\" false", emit, hit)
	}

	// Next chunk breaks the match ("code") -> held "\n" should flush with "code"
	emit, hit = f.Push("code")
	if hit || emit != "\ncode" {
		t.Fatalf("got emit=%q hit=%v, want \"\\ncode\" false", emit, hit)
	}

	// Chunk ends with a partial stop prefix that is never completed before EOS
	emit, hit = f.Push("<|fim")
	if hit || emit != "" {
		t.Fatalf("expected partial suffix held back, got emit=%q hit=%v", emit, hit)
	}
	if tail := f.Flush(); tail != "<|fim" {
		t.Fatalf("expected Flush() to emit trailing held text \"<|fim\", got %q", tail)
	}
}
