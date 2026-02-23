package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Homebrew packages that are libraries/runtimes, not CLI tools
var brewExclusions = map[string]bool{
	"openssl":       true, "openssl@3":     true, "openssl@1.1":   true,
	"python":        true, "python@3.13":   true, "python@3.12":   true,
	"python@3.11":   true, "python@3.10":   true,
	"node":          true, "node@22":       true, "node@20":       true,
	"node@18":       true, "node@16":       true,
	"icu4c":         true, "icu4c@76":      true,
	"libnghttp2":    true, "ca-certificates": true,
	"readline":      true, "sqlite":        true,
	"xz":           true, "zstd":          true,
	"gettext":       true, "glib":          true,
	"libtiff":       true, "jpeg":          true,
	"libpng":        true, "freetype":      true,
	"fontconfig":    true, "pcre2":         true,
	"gmp":          true, "libyaml":       true,
	"brotli":        true, "lz4":           true,
	"cmake":         true, "pkg-config":    true,
	"autoconf":      true, "automake":      true,
	"libtool":       true, "protobuf":      true,
	"boost":         true, "libssh2":       true,
	"nghttp2":       true, "c-ares":        true,
	"libuv":         true, "libevent":      true,
	"libffi":        true, "ncurses":       true,
	"berkeley-db":   true, "gdbm":          true,
	"mpdecimal":     true, "tcl-tk":        true,
	"ruby":          true, "go":            true,
	"rust":          true, "perl":          true,
	"lua":           true, "openjdk":       true,
	"openjdk@17":    true, "openjdk@11":    true,
	"php":           true, "php@8.2":       true,
	"composer":      true, "maven":         true,
	"gradle":        true, "ant":           true,
	"unzip":         true, "zip":           true,
	"gzip":          true, "p7zip":         true,
	"bzip2":         true, "pigz":          true,
	"libressl":      true, "gnutls":        true,
	"libxml2":       true, "libxslt":       true,
	"jpeg-xl":       true, "webp":          true,
	"little-cms2":   true, "ghostscript":   true,
	"poppler":       true, "harfbuzz":      true,
	"pango":         true, "cairo":         true,
	"pixman":        true, "gobject-introspection": true,
	"bdw-gc":        true, "guile":         true,
	"mpfr":          true, "libmpc":        true,
	"isl":           true, "gcc":           true,
	"llvm":          true, "utf8proc":      true,
	"pcre":          true, "oniguruma":     true,
	"jansson":       true, "json-c":        true,
	"krb5":          true, "libzip":        true,
	"docker-compose": true, "docker-buildx": true,
}

// queryGitHub runs a single GitHub search API call.
func queryGitHub(query, sortBy string, perPage int) ([]GitHubRepo, error) {
	u := fmt.Sprintf(
		"https://api.github.com/search/repositories?q=%s&sort=%s&order=desc&per_page=%d",
		url.QueryEscape(query), sortBy, perPage,
	)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "newtools-cli")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(body))
	}

	var result GitHubSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("GitHub decode: %w", err)
	}
	return result.Items, nil
}

func fetchGitHub(count int) tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		risingDate := now.AddDate(0, -18, 0).Format("2006-01-02")
		activeDate := now.AddDate(0, -3, 0).Format("2006-01-02")

		perPage := count * 3
		if perPage > 50 {
			perPage = 50
		}

		type queryResult struct {
			repos []GitHubRepo
			err   error
		}

		risingCh := make(chan queryResult, 1)
		activeCh := make(chan queryResult, 1)

		// Rising Stars: new repos gaining traction fast
		go func() {
			repos, err := queryGitHub(
				fmt.Sprintf("topic:cli stars:>50 created:>%s", risingDate),
				"stars", perPage,
			)
			risingCh <- queryResult{repos, err}
		}()

		// Active Established: mature repos with recent activity
		go func() {
			repos, err := queryGitHub(
				fmt.Sprintf("topic:cli stars:>500 pushed:>%s", activeDate),
				"updated", perPage,
			)
			activeCh <- queryResult{repos, err}
		}()

		rising := <-risingCh
		active := <-activeCh

		// If both fail, return an error
		if rising.err != nil && active.err != nil {
			return githubResultMsg{err: fmt.Errorf("rising: %v; active: %v", rising.err, active.err)}
		}

		// Deduplicate by full_name
		seen := make(map[string]bool)
		var allRepos []GitHubRepo

		// Rising stars first (higher priority)
		if rising.err == nil {
			for _, r := range rising.repos {
				key := strings.ToLower(r.FullName)
				if !seen[key] {
					seen[key] = true
					allRepos = append(allRepos, r)
				}
			}
		}
		if active.err == nil {
			for _, r := range active.repos {
				key := strings.ToLower(r.FullName)
				if !seen[key] {
					seen[key] = true
					allRepos = append(allRepos, r)
				}
			}
		}

		tools := make([]Tool, 0, len(allRepos))
		for _, repo := range allRepos {
			desc := repo.Description
			if len(desc) > 120 {
				desc = desc[:117] + "..."
			}
			tools = append(tools, Tool{
				Name:        repo.Name,
				Description: desc,
				Stars:       repo.StargazersCount,
				Source:      "GitHub",
				Score:       trendingScore(repo.StargazersCount, repo.CreatedAt),
				URL:         repo.HTMLURL,
				CreatedAt:   repo.CreatedAt,
			})
		}

		return githubResultMsg{tools: tools}
	}
}

// trendingScore favors repos that accumulate stars quickly relative to their age.
func trendingScore(stars int, createdAt time.Time) float64 {
	age := time.Since(createdAt)
	ageMonths := age.Hours() / (24 * 30)
	if ageMonths < 1 {
		ageMonths = 1
	}

	starsPerMonth := float64(stars) / ageMonths

	var multiplier float64
	switch {
	case ageMonths < 6:
		multiplier = 3.0
	case ageMonths < 12:
		multiplier = 2.5
	case ageMonths < 24:
		multiplier = 1.5
	default:
		multiplier = 1.0
	}

	return starsPerMonth * multiplier
}

func fetchBrew(count int) tea.Cmd {
	return func() tea.Msg {
		analyticsURL := "https://formulae.brew.sh/api/analytics/install-on-request/30d.json"

		req, err := http.NewRequest("GET", analyticsURL, nil)
		if err != nil {
			return brewResultMsg{err: err}
		}
		req.Header.Set("User-Agent", "newtools-cli")

		resp, err := httpClient.Do(req)
		if err != nil {
			return brewResultMsg{err: fmt.Errorf("Homebrew API: %w", err)}
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return brewResultMsg{err: fmt.Errorf("Homebrew API %d", resp.StatusCode)}
		}

		var result BrewAnalyticsResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return brewResultMsg{err: fmt.Errorf("Homebrew decode: %w", err)}
		}

		// Filter and collect top candidates
		type candidate struct {
			name     string
			installs int
		}
		var candidates []candidate
		for _, item := range result.Items {
			lower := strings.ToLower(item.Formula)
			if brewExclusions[lower] {
				continue
			}
			// Parse comma-formatted count string (e.g. "271,312")
			countStr := strings.ReplaceAll(item.Count, ",", "")
			installs, err := strconv.Atoi(countStr)
			if err != nil {
				continue
			}
			candidates = append(candidates, candidate{name: item.Formula, installs: installs})
		}

		// Already sorted by rank from API, but sort explicitly for safety
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].installs > candidates[j].installs
		})

		enrichCount := count * 3
		if enrichCount > len(candidates) {
			enrichCount = len(candidates)
		}
		candidates = candidates[:enrichCount]

		// Enrich with descriptions and homepages using a worker pool
		type enriched struct {
			desc     string
			homepage string
		}
		results := make([]enriched, enrichCount)
		var wg sync.WaitGroup
		sem := make(chan struct{}, 5) // 5 concurrent workers

		for i, c := range candidates {
			wg.Add(1)
			go func(idx int, name string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				desc, homepage := fetchFormulaInfo(name)
				results[idx] = enriched{desc: desc, homepage: homepage}
			}(i, c.name)
		}
		wg.Wait()

		tools := make([]Tool, 0, enrichCount)
		for i, c := range candidates {
			desc := results[i].desc
			if desc == "" {
				desc = "A Homebrew formula"
			}
			homepageURL := results[i].homepage
			if homepageURL == "" {
				homepageURL = fmt.Sprintf("https://formulae.brew.sh/formula/%s", url.PathEscape(c.name))
			}
			tools = append(tools, Tool{
				Name:        c.name,
				Description: desc,
				Installs:    c.installs,
				Source:      "Homebrew",
				Score:       float64(c.installs) / 50000,
				URL:         homepageURL,
			})
		}

		return brewResultMsg{tools: tools}
	}
}

func fetchFormulaInfo(name string) (desc, homepage string) {
	formulaURL := fmt.Sprintf("https://formulae.brew.sh/api/formula/%s.json", url.PathEscape(name))

	req, err := http.NewRequest("GET", formulaURL, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", "newtools-cli")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", ""
	}

	var info BrewFormulaInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", ""
	}

	desc = info.Desc
	if len(desc) > 120 {
		desc = desc[:117] + "..."
	}
	return desc, info.Homepage
}

func mergeTools(github, brew []Tool, count int, installed map[string]bool, history *ToolHistory) []Tool {
	index := make(map[string]*Tool)

	// Add GitHub tools first
	for _, t := range github {
		key := strings.ToLower(t.Name)
		tc := t
		index[key] = &tc
	}

	// Merge Homebrew tools
	for _, t := range brew {
		key := strings.ToLower(t.Name)
		if existing, ok := index[key]; ok {
			// Tool exists in both sources
			existing.Source = "Both"
			existing.Installs = t.Installs
			existing.Score = trendingScore(existing.Stars, existing.CreatedAt) + float64(t.Installs)/50000
			if existing.Description == "" {
				existing.Description = t.Description
			}
		} else {
			tc := t
			index[key] = &tc
		}
	}

	// Filter out installed tools
	if installed != nil {
		for key := range index {
			if installed[key] {
				delete(index, key)
			}
		}
	}

	// Filter out tools hidden by seen-history
	if history != nil {
		for key := range index {
			if history.IsHidden(key) {
				delete(index, key)
			}
		}
	}

	// Collect and sort by score
	tools := make([]Tool, 0, len(index))
	for _, t := range index {
		tools = append(tools, *t)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Score > tools[j].Score
	})

	if len(tools) > count {
		tools = tools[:count]
	}
	return tools
}
