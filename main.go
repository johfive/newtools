package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

const version = "1.2.0"

func main() {
	count := 5
	showAll := false

	// Check for bare -N flag (e.g., -20) and -v/-version before flag parsing
	bareNumRe := regexp.MustCompile(`^-(\d+)$`)
	var remaining []string
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--version" || arg == "-version" {
			fmt.Printf("newtools v%s\n", version)
			os.Exit(0)
		}
		if m := bareNumRe.FindStringSubmatch(arg); m != nil {
			n, err := strconv.Atoi(m[1])
			if err == nil && n > 0 {
				count = n
			}
		} else {
			remaining = append(remaining, arg)
		}
	}

	// Parse standard flags from remaining args
	fs := flag.NewFlagSet("newtools", flag.ExitOnError)
	nFlag := fs.Int("n", 0, "number of tools to display")
	aFlag := fs.Bool("a", false, "show all tools including already installed ones")
	allFlag := fs.Bool("all", false, "show all tools including already installed ones")
	fs.Parse(remaining)

	if *nFlag > 0 {
		count = *nFlag
	}
	if *aFlag || *allFlag {
		showAll = true
	}

	p := tea.NewProgram(newModel(count, showAll), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
