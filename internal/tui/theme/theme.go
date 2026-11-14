// Package theme holds every colour and border of the interface. No other
// package may contain a colour literal. The terminal's own background and
// foreground are never painted; each token adapts to a dark or light terminal.
package theme

import "github.com/charmbracelet/lipgloss"

// Colour tokens. Positive, Negative and Warning keep their meaning everywhere
// and are never used for decoration.
var (
	Accent   = lipgloss.AdaptiveColor{Dark: "#58a6ff", Light: "#0969da"}
	Muted    = lipgloss.AdaptiveColor{Dark: "#8b949e", Light: "#57606a"}
	Positive = lipgloss.AdaptiveColor{Dark: "#3fb950", Light: "#1a7f37"}
	Negative = lipgloss.AdaptiveColor{Dark: "#f85149", Light: "#cf222e"}
	Warning  = lipgloss.AdaptiveColor{Dark: "#d29922", Light: "#9a6700"}
)

// Styles. Border and colour live here; widths and heights are set by the layout.
var (
	PanelFocused   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Accent)
	PanelUnfocused = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Muted)

	Title       = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	Label       = lipgloss.NewStyle().Foreground(Muted)
	KeyHint     = lipgloss.NewStyle().Foreground(Muted)
	SelectedRow = lipgloss.NewStyle().Foreground(Accent).Bold(true)
	Error       = lipgloss.NewStyle().Foreground(Negative)

	// Amounts: income is Positive, expense is Negative. Totals are bold.
	AmountIncome  = lipgloss.NewStyle().Foreground(Positive)
	AmountExpense = lipgloss.NewStyle().Foreground(Negative)
	AmountTotal   = lipgloss.NewStyle().Bold(true)

	Overlay = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Accent).Padding(0, 1)
)
