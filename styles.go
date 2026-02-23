package main

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			PaddingLeft(2)

	// Source badges
	githubBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("236")).
			Padding(0, 1).
			Bold(true)

	brewBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("208")).
			Padding(0, 1).
			Bold(true)

	bothBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("35")).
			Padding(0, 1).
			Bold(true)

	starStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))

	installStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("81"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	errStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			Padding(1, 2)

	newBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("42")).
			Padding(0, 1).
			Bold(true)

	// Detail view styles
	detailTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("170")).
			PaddingLeft(2).
			PaddingTop(1)

	detailLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("252")).
			Width(14)

	detailValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	detailURL = lipgloss.NewStyle().
			Foreground(lipgloss.Color("81")).
			Underline(true)

	detailHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(2).
			PaddingTop(1)
)

func sourceBadge(source string) string {
	switch source {
	case "GitHub":
		return githubBadge.Render("GitHub")
	case "Homebrew":
		return brewBadge.Render("Brew")
	case "Both":
		return bothBadge.Render("Both")
	default:
		return source
	}
}
