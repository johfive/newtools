package main

import (
	"fmt"
	"time"
)

// Tool represents a CLI tool discovered from GitHub and/or Homebrew.
type Tool struct {
	Name        string
	Description string
	Stars       int
	Installs    int
	Source      string // "GitHub", "Homebrew", or "Both"
	Score       float64
	URL         string
	CreatedAt   time.Time
}

func (t Tool) Title() string       { return t.Name }
func (t Tool) FilterValue() string { return t.Name + " " + t.Description }
func (t Tool) Description_() string {
	return t.Description
}

// GitHub API response types
type GitHubSearchResponse struct {
	Items []GitHubRepo `json:"items"`
}

type GitHubRepo struct {
	Name            string      `json:"name"`
	FullName        string      `json:"full_name"`
	Description     string      `json:"description"`
	StargazersCount int         `json:"stargazers_count"`
	HTMLURL         string      `json:"html_url"`
	Owner           GitHubOwner `json:"owner"`
	CreatedAt       time.Time   `json:"created_at"`
}

type GitHubOwner struct {
	Login string `json:"login"`
}

// Homebrew API response types
type BrewAnalyticsResponse struct {
	Items map[string]int `json:"formulae"`
}

type BrewFormulaInfo struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// Tea messages
type githubResultMsg struct {
	tools []Tool
	err   error
}

type brewResultMsg struct {
	tools []Tool
	err   error
}

type installedResultMsg struct {
	names map[string]bool
}

// Format helpers
func formatStars(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func formatInstalls(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
