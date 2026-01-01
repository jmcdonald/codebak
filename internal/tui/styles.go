package tui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	warningColor   = lipgloss.Color("#F59E0B") // Yellow
	errorColor     = lipgloss.Color("#EF4444") // Red
	bgColor        = lipgloss.Color("#1F2937") // Dark gray
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

	// Status bar
	statusStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
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

	warningBadge = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	errorBadge = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Table headers
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#9CA3AF")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(mutedColor)

	// Column styles for alignment
	colProject = lipgloss.NewStyle().Width(30)
	colVersion = lipgloss.NewStyle().Width(20)
	colSize    = lipgloss.NewStyle().Width(12).Align(lipgloss.Right)
	colFiles   = lipgloss.NewStyle().Width(8).Align(lipgloss.Right)
	colGit     = lipgloss.NewStyle().Width(10)
	colDate    = lipgloss.NewStyle().Width(15)
)
