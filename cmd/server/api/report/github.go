// Package report implements the in-game reporter: player-submitted bug reports
// and test-server access requests. Each submission is appended to a local JSONL
// log (the growing log the maintainer owns) and, when configured, mirrored as a
// comment on a pinned collective GitHub issue so the maintainer is notified by
// email. Players never touch GitHub directly — no GitHub account required.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"pubkey-quest/cmd/server/utils"
)

// githubClient is a small shared HTTP client for the GitHub REST API.
var githubClient = &http.Client{Timeout: 10 * time.Second}

// postIssueComment posts body as a comment on the given issue of the configured
// repo. It is best-effort: if the reporter is not configured (no token, repo, or
// issue number) it is a no-op, and any error is logged and returned so callers
// can decide whether to surface it. The local JSONL log is the source of truth,
// so a GitHub failure never fails the player's submission.
func postIssueComment(issueNumber int, body string) error {
	cfg := utils.AppConfig.Report

	if cfg.GitHubToken == "" || cfg.GitHubRepo == "" || issueNumber == 0 {
		log.Printf("📂 Reporter: GitHub post skipped (not configured) — logged locally only")
		return nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", cfg.GitHubRepo, issueNumber)

	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return fmt.Errorf("marshal comment: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.GitHubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := githubClient.Do(req)
	if err != nil {
		return fmt.Errorf("post to GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GitHub returned %d: %s", resp.StatusCode, string(msg))
	}

	log.Printf("✅ Reporter: posted comment to issue #%d", issueNumber)
	return nil
}
