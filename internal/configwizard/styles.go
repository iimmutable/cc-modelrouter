package configwizard

import (
	"strings"

	"github.com/mattn/go-runewidth"
	lipgloss "github.com/charmbracelet/lipgloss"
)

// Wizard color palette - Light theme (transparent backgrounds)
var (
	// Background colors (kept for selected/interactive elements only)
	BaseBackground   = lipgloss.Color("#f8f8f2") // Cream (used sparingly)
	PanelBackground  = lipgloss.Color("#ffffff") // White (used sparingly)
	AltRowBackground = lipgloss.Color("#eff0eb") // Light gray (alternate rows)

	// Text colors
	PrimaryText   = lipgloss.Color("#282a36") // Dark
	SecondaryText = lipgloss.Color("#6272a4") // Medium gray
	HeaderText    = lipgloss.Color("#bd93f9") // Purple

	// Accent colors
	SelectionAccent = lipgloss.Color("#8be9fd") // Cyan
	BorderColor     = lipgloss.Color("#44475a") // Gray

	// Status colors
	SuccessColor   = lipgloss.Color("#2e7d32") // Green
	SuccessBorder  = lipgloss.Color("#1b5e20") // Darker green for primary button border
	ErrorColor     = lipgloss.Color("#c62828") // Red
	ErrorBorder    = lipgloss.Color("#8e0000") // Darker red for danger button border
	WarningColor   = lipgloss.Color("#e65100") // Orange
	InfoColor      = lipgloss.Color("#1565c0") // Blue
)

// Base styles (no backgrounds on non-interactive elements — terminal native bg shows through)
var (
	// Main container
	MainContainerStyle = lipgloss.NewStyle().
				Foreground(PrimaryText).
				Width(64).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(BorderColor).
				Padding(1, 2)

	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(HeaderText).
			Width(60).
			Align(lipgloss.Center)

	// Section header
	SectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(HeaderText).
				Padding(0, 1)

	// Menu item - unselected
	MenuItemStyle = lipgloss.NewStyle().
			Foreground(PrimaryText).
			Padding(0, 2)

	// Menu item - selected (keeps background for visual distinction)
	MenuItemSelectedStyle = lipgloss.NewStyle().
				Foreground(PrimaryText).
				Background(SelectionAccent).
				Bold(true).
				Padding(0, 2)

	// Menu item - dimmed (for info display)
	MenuItemDimmedStyle = lipgloss.NewStyle().
				Foreground(SecondaryText).
				Padding(0, 2)

	// Menu item description (used when item is selected)
	MenuItemDescriptionStyle = lipgloss.NewStyle().
					Foreground(PrimaryText).
					Background(SelectionAccent).
					Padding(0, 2)

	// Input field
	InputFieldStyle = lipgloss.NewStyle().
			Foreground(PrimaryText).
			Border(lipgloss.NormalBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)

	// Input field focused
	InputFieldFocusedStyle = lipgloss.NewStyle().
				Foreground(PrimaryText).
				Border(lipgloss.NormalBorder()).
				BorderForeground(SelectionAccent).
				Padding(0, 1)

	// Button - unselected
	ButtonStyle = lipgloss.NewStyle().
			Foreground(PrimaryText).
			Background(PanelBackground).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#bbbbbb")).
			Padding(0, 2)

	// Button - selected/active (keeps background)
	ButtonActiveStyle = lipgloss.NewStyle().
				Foreground(PrimaryText).
				Background(SelectionAccent).
				Bold(true).
				Border(lipgloss.NormalBorder()).
				BorderForeground(SelectionAccent).
				Padding(0, 2)

	// Primary button (keeps background)
	ButtonPrimaryStyle = lipgloss.NewStyle().
				Foreground(BaseBackground).
				Background(SuccessColor).
				Bold(true).
				Border(lipgloss.NormalBorder()).
				BorderForeground(SuccessBorder).
				Padding(0, 2)

	// Danger button (keeps background)
	ButtonDangerStyle = lipgloss.NewStyle().
				Foreground(BaseBackground).
				Background(ErrorColor).
				Bold(true).
				Border(lipgloss.NormalBorder()).
				BorderForeground(ErrorBorder).
				Padding(0, 2)

	// Checkbox - unchecked
	CheckboxUncheckedStyle = lipgloss.NewStyle().
				Foreground(SecondaryText).
				SetString("[ ]")

	// Checkbox - checked
	CheckboxCheckedStyle = lipgloss.NewStyle().
				Foreground(SuccessColor).
				SetString("[✓]")

	// Checkbox - unchecked + focused (cyan highlight)
	CheckboxUncheckedFocusedStyle = lipgloss.NewStyle().
					Foreground(SelectionAccent).
					Bold(true).
					SetString("[ ]")

	// Checkbox - checked + focused (cyan highlight)
	CheckboxCheckedFocusedStyle = lipgloss.NewStyle().
					Foreground(SelectionAccent).
					Bold(true).
					SetString("[✓]")

	// Input field - disabled (dimmed, no border)
	InputFieldDisabledStyle = lipgloss.NewStyle().
					Foreground(SecondaryText).
					Padding(0, 1)

	// Focused row highlight background
	FocusedRowStyle = lipgloss.NewStyle().
				Background(AltRowBackground).
				Padding(0, 1)

	// Status indicators
	StatusOKStyle      = lipgloss.NewStyle().Foreground(SuccessColor)
	StatusErrorStyle   = lipgloss.NewStyle().Foreground(ErrorColor)
	StatusPendingStyle = lipgloss.NewStyle().Foreground(WarningColor).SetString("⟳")

	// List item - unselected
	ListItemStyle = lipgloss.NewStyle().
			Foreground(PrimaryText).
			Padding(0, 1)

	// List item - selected (keeps background)
	ListItemSelectedStyle = lipgloss.NewStyle().
				Foreground(PrimaryText).
				Background(SelectionAccent).
				Padding(0, 1)

	// List item - invalid (provider/model not found)
	ListItemInvalidStyle = lipgloss.NewStyle().
				Foreground(ErrorColor).
				Padding(0, 1)

	// List item - invalid + selected
	ListItemInvalidSelectedStyle = lipgloss.NewStyle().
					Foreground(ErrorColor).
					Background(SelectionAccent).
					Padding(0, 1)

	// Table header
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(HeaderText).
				Border(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(BorderColor)

	// Table row
	TableRowStyle = lipgloss.NewStyle().
			Foreground(PrimaryText)

	// Table row - alternate (keeps background for visual distinction)
	TableRowAltStyle = lipgloss.NewStyle().
				Foreground(PrimaryText).
				Background(AltRowBackground)

	// Modal/Overlay
	ModalStyle = lipgloss.NewStyle().
			Foreground(PrimaryText).
			Background(PanelBackground).
			Width(50).
			Height(9).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(HeaderText).
			Padding(1, 2).
			Align(lipgloss.Center)

	// Dropdown floating panel (keeps background for overlay distinction)
	DropdownStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(SelectionAccent).
			Background(PanelBackground).
			Padding(0, 1).
			Width(42)

	// Error message
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Padding(0, 1)

	// Warning message (inline, no padding)
	WarningStyle = lipgloss.NewStyle().
			Foreground(ErrorColor)

	// Help text
	HelpTextStyle = lipgloss.NewStyle().
			Foreground(SecondaryText).
			Padding(0, 1)

	// Key hint
	KeyHintStyle = lipgloss.NewStyle().
			Foreground(SelectionAccent).
			SetString("[" + "key" + "]")
)

// Helper functions

// truncate truncates a string to fit within maxWidth.
func truncate(s string, maxWidth int) string {
	return runewidth.Truncate(s, maxWidth, "...")
}

// padRight pads a string to the right to fill the specified width.
func padRight(s string, width int) string {
	currentWidth := runewidth.StringWidth(s)
	if currentWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-currentWidth)
}

// GetScreenTitle returns the title for a screen.
func GetScreenTitle(screen Screen) string {
	switch screen {
	case ScreenMainMenu:
		return "Configuration Wizard"
	case ScreenProviders:
		return "Providers"
	case ScreenAddProvider1:
		return "Add Provider (1/2)"
	case ScreenAddProvider2:
		return "Environment Setup (2/2)"
	case ScreenRoutes:
		return "Routes"
	case ScreenEditRoute:
		return "Add/Edit Route"
	case ScreenServer:
		return "Proxy Settings"
	case ScreenLogging:
		return "Logging Settings"
	case ScreenViewConfig:
		return "Current Configuration"
	case ScreenTestConnection:
		return "Test Connection"
	default:
		return "Configuration Wizard"
	}
}

// GetBackLabel returns the back label for a screen.
func GetBackLabel(screen Screen) string {
	switch screen {
	case ScreenAddProvider1:
		return "Cancel"
	case ScreenAddProvider2:
		return "← Back"
	default:
		return "← Back"
	}
}