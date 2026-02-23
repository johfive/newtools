package main

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type state int

const (
	stateLoading state = iota
	stateReady
	stateDetail
	stateError
)

type model struct {
	state          state
	count          int
	showAll        bool
	spinner        spinner.Model
	list           list.Model
	detailTool     Tool
	githubTools    []Tool
	brewTools      []Tool
	githubDone     bool
	brewDone       bool
	installedDone  bool
	installedNames map[string]bool
	githubErr      error
	brewErr        error
	err            error
	width          int
	height         int
	history        *ToolHistory
}

// Custom delegate for rendering list items
type toolDelegate struct{}

func (d toolDelegate) Height() int                             { return 3 }
func (d toolDelegate) Spacing() int                            { return 1 }
func (d toolDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d toolDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	tool, ok := item.(Tool)
	if !ok {
		return
	}

	cursor := "  "
	if index == m.Index() {
		cursor = "> "
	}

	// Line 1: name + badge + age badge
	nameStr := lipgloss.NewStyle().Bold(true).Render(tool.Name)
	if index == m.Index() {
		nameStr = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170")).Render(tool.Name)
	}
	badge := sourceBadge(tool.Source)

	ageBadge := ""
	if !tool.CreatedAt.IsZero() {
		ageMonths := int(time.Since(tool.CreatedAt).Hours() / (24 * 30))
		if ageMonths < 2 {
			ageBadge = " " + newBadge.Render("NEW")
		} else if ageMonths < 12 {
			ageBadge = " " + newBadge.Render(fmt.Sprintf("%dmo", ageMonths))
		}
	}

	line1 := fmt.Sprintf("%s%s %s%s", cursor, nameStr, badge, ageBadge)

	// Line 2: description
	desc := tool.Description
	maxDescWidth := m.Width() - 6
	if maxDescWidth > 0 && len(desc) > maxDescWidth {
		desc = desc[:maxDescWidth-3] + "..."
	}
	line2 := fmt.Sprintf("    %s", dimStyle.Render(desc))

	// Line 3: stats
	var stats []string
	if tool.Stars > 0 {
		stats = append(stats, starStyle.Render(fmt.Sprintf("★ %s", formatStars(tool.Stars))))
	}
	if tool.Installs > 0 {
		stats = append(stats, installStyle.Render(fmt.Sprintf("⬇ %s/mo", formatInstalls(tool.Installs))))
	}
	line3 := "    "
	for i, s := range stats {
		if i > 0 {
			line3 += dimStyle.Render("  •  ")
		}
		line3 += s
	}

	fmt.Fprintf(w, "%s\n%s\n%s", line1, line2, line3)
}

func newModel(count int, showAll bool) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))

	m := model{
		state:   stateLoading,
		count:   count,
		showAll: showAll,
		spinner: s,
	}

	// If showing all, skip installed detection and history filtering
	if showAll {
		m.installedDone = true
	} else {
		m.history = loadHistory()
	}

	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick,
		fetchGitHub(m.count),
		fetchBrew(m.count),
	}

	if !m.showAll {
		cmds = append(cmds, detectInstalled())
	}

	return tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.state {
		case stateDetail:
			switch msg.String() {
			case "q", "esc", "backspace":
				m.state = stateReady
				return m, nil
			case "o":
				// Open URL in browser
				if m.detailTool.URL != "" {
					return m, openURL(m.detailTool.URL)
				}
			}
			return m, nil

		case stateReady:
			switch msg.String() {
			case "q":
				return m, tea.Quit
			case "enter":
				if item, ok := m.list.SelectedItem().(Tool); ok {
					m.detailTool = item
					m.state = stateDetail
					return m, nil
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == stateReady {
			m.list.SetSize(msg.Width, msg.Height)
		}

	case spinner.TickMsg:
		if m.state == stateLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case githubResultMsg:
		m.githubDone = true
		if msg.err != nil {
			m.githubErr = msg.err
		} else {
			m.githubTools = msg.tools
		}
		return m.checkReady()

	case brewResultMsg:
		m.brewDone = true
		if msg.err != nil {
			m.brewErr = msg.err
		} else {
			m.brewTools = msg.tools
		}
		return m.checkReady()

	case installedResultMsg:
		m.installedDone = true
		m.installedNames = msg.names
		return m.checkReady()
	}

	if m.state == stateReady {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) checkReady() (tea.Model, tea.Cmd) {
	if !m.githubDone || !m.brewDone || !m.installedDone {
		return m, nil
	}

	// Both sources failed
	if m.githubErr != nil && m.brewErr != nil {
		m.state = stateError
		m.err = fmt.Errorf("GitHub: %v\nHomebrew: %v", m.githubErr, m.brewErr)
		return m, nil
	}

	// Merge available results (pass nil installed/history when showAll)
	var installed map[string]bool
	if !m.showAll {
		installed = m.installedNames
	}
	tools := mergeTools(m.githubTools, m.brewTools, m.count, installed, m.history)

	if len(tools) == 0 {
		m.state = stateError
		m.err = fmt.Errorf("no tools found")
		return m, nil
	}

	// Convert to list items
	items := make([]list.Item, len(tools))
	for i, t := range tools {
		items[i] = t
	}

	// Create list
	delegate := toolDelegate{}
	l := list.New(items, delegate, m.width, m.height)
	l.Title = "🔧 Trending CLI Tools"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	// Show warning if one source failed
	if m.githubErr != nil {
		l.Title = "🔧 Trending CLI Tools (Homebrew only)"
	} else if m.brewErr != nil {
		l.Title = "🔧 Trending CLI Tools (GitHub only)"
	}

	m.list = l
	m.state = stateReady

	// Record displayed tools in history
	if m.history != nil {
		names := make([]string, len(tools))
		for i, t := range tools {
			names[i] = t.Name
		}
		m.history.RecordSeen(names)
		m.history.saveHistory()
	}

	return m, nil
}

func (m model) View() string {
	switch m.state {
	case stateLoading:
		return fmt.Sprintf("\n  %s Discovering trending CLI tools...\n", m.spinner.View())
	case stateError:
		return errStyle.Render(fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err))
	case stateDetail:
		return m.viewDetail()
	case stateReady:
		return m.list.View()
	}
	return ""
}

func (m model) viewDetail() string {
	t := m.detailTool
	var b strings.Builder

	b.WriteString(detailTitle.Render(fmt.Sprintf("🔧 %s", t.Name)))
	b.WriteString("\n\n")

	row := func(label, value string) {
		b.WriteString("  ")
		b.WriteString(detailLabel.Render(label))
		b.WriteString(detailValue.Render(value))
		b.WriteString("\n")
	}

	row("Description", t.Description)
	row("Source", t.Source)

	if t.Stars > 0 {
		row("Stars", formatStars(t.Stars))
	}
	if t.Installs > 0 {
		row("Installs/mo", formatInstalls(t.Installs))
	}
	if !t.CreatedAt.IsZero() {
		ageMonths := int(time.Since(t.CreatedAt).Hours() / (24 * 30))
		ageStr := t.CreatedAt.Format("Jan 2006")
		if ageMonths < 2 {
			ageStr += "  (brand new!)"
		} else if ageMonths < 12 {
			ageStr += fmt.Sprintf("  (%d months ago)", ageMonths)
		} else {
			years := ageMonths / 12
			ageStr += fmt.Sprintf("  (~%d years ago)", years)
		}
		row("Created", ageStr)
	}
	if t.URL != "" {
		b.WriteString("  ")
		b.WriteString(detailLabel.Render("URL"))
		b.WriteString(detailURL.Render(t.URL))
		b.WriteString("\n")
	}

	b.WriteString(detailHint.Render("o open in browser  •  esc back  •  q quit"))

	return b.String()
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		exec.Command("open", url).Run()
		return nil
	}
}
