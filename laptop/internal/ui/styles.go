package ui

import "github.com/charmbracelet/lipgloss"

var (
	dash        = "—"
	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color("#33415c")).
		Foreground(lipgloss.Color("#ffffff")).
		Padding(0, 1)
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a93a3"))
	greenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#46c88c"))
	redStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ee6482"))
	activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f5d76e"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8a93a3"))
	qrStyle     = lipgloss.NewStyle().Padding(0, 1)
)
