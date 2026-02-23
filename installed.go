package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func detectInstalled() tea.Cmd {
	return func() tea.Msg {
		names := make(map[string]bool)

		// Try brew list --formula with a timeout
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "brew", "list", "--formula")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			for _, line := range strings.Split(out.String(), "\n") {
				name := strings.TrimSpace(line)
				if name != "" {
					names[strings.ToLower(name)] = true
				}
			}
		}

		// Scan common bin directories for executables
		home, _ := os.UserHomeDir()
		dirs := []string{
			"/opt/homebrew/bin",
			"/usr/local/bin",
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".cargo", "bin"),
		}

		for _, dir := range dirs {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				if info.Mode()&0111 != 0 {
					names[strings.ToLower(e.Name())] = true
				}
			}
		}

		return installedResultMsg{names: names}
	}
}
