package tui

import "github.com/charmbracelet/lipgloss"

// Colors - only include those that are actually used
var (
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	errorColor     = lipgloss.Color("#EF4444") // Red
)

// Styles
var (
	// App frame
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1)

	// List items
	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB"))

	dimStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(1, 0, 0, 0)

	// Badges
	successBadge = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	errorBadge = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Diff view styles
	addedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor) // Green for added lines

	deletedStyle = lipgloss.NewStyle().
			Foreground(errorColor) // Red for deleted lines
)
