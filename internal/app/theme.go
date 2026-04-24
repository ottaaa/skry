package app

import "github.com/charmbracelet/lipgloss"

var (
	ColorFg       = lipgloss.Color("#c0caf5")
	ColorMuted    = lipgloss.Color("#565f89")
	ColorBorder   = lipgloss.Color("#3b4261")
	ColorAccent   = lipgloss.Color("#7aa2f7")
	ColorModified = lipgloss.Color("#e0af68")
	ColorAdded    = lipgloss.Color("#9ece6a")
	ColorDeleted  = lipgloss.Color("#f7768e")
	ColorRenamed  = lipgloss.Color("#bb9af7")
	ColorUntrack  = lipgloss.Color("#7dcfff")
	ColorAddBg    = lipgloss.Color("#1f3a2a")
	ColorDelBg    = lipgloss.Color("#3a1f26")
	ColorMatchBg  = lipgloss.Color("#3d3a1f")

	TitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	HintStyle     = lipgloss.NewStyle().Foreground(ColorMuted)
	HeaderStyle   = lipgloss.NewStyle().Foreground(ColorFg).Padding(0, 1)
	FooterStyle   = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	PaneStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder)
	ActivePane    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorAccent)
	SelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1a1b26")).Background(ColorAccent)
	ModalStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(ColorAccent).Padding(0, 1)
)
