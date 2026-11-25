package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ArtifactExport struct {
	TaskID    string  `json:"task_id"`
	TaskTitle string  `json:"task_title"`
	Path      string  `json:"path"`
	Content   string  `json:"content"`
	Score     float64 `json:"score"`
}

type PlanStatusResponse struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func main() {
	defaultURL := os.Getenv("ORCHESTRATOR_URL")
	if defaultURL == "" {
		defaultURL = "http://localhost:8080"
	}

	urlFlag := flag.String("url", defaultURL, "Orchestrator URL (e.g. http://192.168.50.4:8080)")
	planFlag := flag.String("plan", "latest", "Plan ID to sync (or 'latest'/'active')")
	destFlag := flag.String("dest", ".", "Destination root directory to sync files into")
	watchFlag := flag.Bool("watch", false, "Continuously watch and sync artifacts until plan completes")
	intervalFlag := flag.Duration("interval", 3*time.Second, "Polling interval when -watch is enabled")
	flag.Parse()

	baseURL := strings.TrimRight(*urlFlag, "/")
	fmt.Printf("🚀 Strix Artifact Syncer\n")
	fmt.Printf("   Orchestrator: %s\n", baseURL)
	fmt.Printf("   Plan Target : %s\n", *planFlag)
	fmt.Printf("   Destination : %s\n", *destFlag)
	fmt.Printf("   Watch Mode  : %t\n\n", *watchFlag)

	syncedHashes := make(map[string]int)

	for {
		syncedCount, planDone, err := syncOnce(baseURL, *planFlag, *destFlag, syncedHashes)
		if err != nil {
			fmt.Printf("⚠️ [%s] Sync error: %v\n", time.Now().Format("15:04:05"), err)
		} else if syncedCount > 0 {
			fmt.Printf("✨ [%s] Synced %d new/updated artifact(s)\n", time.Now().Format("15:04:05"), syncedCount)
		}

		if !*watchFlag || planDone {
			if planDone {
				fmt.Printf("\n🎉 [%s] Plan execution finished! All artifacts synced to %s\n", time.Now().Format("15:04:05"), *destFlag)
			}
			break
		}

		time.Sleep(*intervalFlag)
	}
}

func syncOnce(baseURL, planID, destRoot string, seen map[string]int) (int, bool, error) {
	// 1. Fetch artifacts
	artURL := fmt.Sprintf("%s/api/v1/plans/%s/artifacts", baseURL, planID)
	resp, err := http.Get(artURL)
	if err != nil {
		return 0, false, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return 0, false, fmt.Errorf("plan %s not found (yet)", planID)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, false, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var artifacts []ArtifactExport
	if err := json.NewDecoder(resp.Body).Decode(&artifacts); err != nil {
		return 0, false, fmt.Errorf("failed to decode artifacts JSON: %w", err)
	}

	newSyncs := 0
	for _, art := range artifacts {
		key := fmt.Sprintf("%s:%s", art.TaskID, art.Path)
		contentLen := len(art.Content)
		if prevLen, exists := seen[key]; exists && prevLen == contentLen {
			continue
		}

		// Normalize OS path separators (converts forward slashes to \ on Windows)
		cleanRel := filepath.FromSlash(strings.TrimPrefix(art.Path, "/"))
		targetPath := filepath.Join(destRoot, cleanRel)

		// Create parent directory
		parentDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return newSyncs, false, fmt.Errorf("failed creating dir %s: %w", parentDir, err)
		}

		// Write raw UTF-8 content directly
		if err := os.WriteFile(targetPath, []byte(art.Content), 0644); err != nil {
			return newSyncs, false, fmt.Errorf("failed writing %s: %w", targetPath, err)
		}

		seen[key] = contentLen
		newSyncs++
		fmt.Printf("   ✅ [%s] %s (Score: %.1f) -> %s (%d bytes)\n", art.TaskID, art.TaskTitle, art.Score, cleanRel, contentLen)
	}

	// 2. Check plan status
	statusURL := fmt.Sprintf("%s/api/v1/plans/%s", baseURL, planID)
	sResp, err := http.Get(statusURL)
	planDone := false
	if err == nil {
		defer sResp.Body.Close()
		var plan PlanStatusResponse
		if json.NewDecoder(sResp.Body).Decode(&plan) == nil {
			if plan.Status == "Completed" || plan.Status == "Failed" {
				planDone = true
			}
		}
	}

	return newSyncs, planDone, nil
}
