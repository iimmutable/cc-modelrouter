package monitor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/iimmutable/cc-modelrouter/internal/usage"
	"github.com/mattn/go-runewidth"
	lipgloss "github.com/charmbracelet/lipgloss"
)

// Adaptive layout styles - Catppuccin Mocha Theme
var (
	// Column width ratios for consistent table alignment
	requestsColRatio = 0.24 // Requests: 24%
	tokensColRatio   = 0.24 // Tokens: 24% (same width as Requests)
	fbacksColRatio   = 0.14 // Fbacks: 14% (smaller than Requests)

	// Box styles with dynamic width
	DynamicBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(BorderColor)

	// Table cell styles (using light theme palette)
	TableCellStyle = lipgloss.NewStyle().
			Foreground(PrimaryText) // #282a36 - dark text

	TableCellAltStyle = lipgloss.NewStyle().
				Foreground(PrimaryText).
				Background(AltRowBackground) // #eff0eb - light gray

	TableHeaderCellStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(HeaderText) // #bd93f9 - purple
)

// View renders the complete UI
func (m *MonitorModel) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Validate minimum width before any rendering
	if m.WindowSize.Width == 0 {
		return "Initializing..."
	}
	if m.WindowSize.Width < 76 {
		return "Terminal too narrow. Please resize to at least 76 columns."
	}

	// Ensure layout is calculated
	if m.LeftPanelWidth == 0 {
		m.calculateLayoutWidths()
	}

	var sections []string

	// 1. Header Bar - track its height
	header := m.renderHeader()
	sections = append(sections, header)
	headerHeight := strings.Count(header, "\n") + 1 + 2 // +2 for header borders (top + bottom)

	// 2. Content Area — render and measure height for flexible console log sizing
	content := m.renderContent()
	sections = append(sections, content)
	contentHeight := strings.Count(content, "\n") + 1 + 2 // +2 for content borders (top + bottom)

	// 3. Console Log (conditional) - pass total height (header + content)
	if m.ConsoleLogEnabled && m.SelectedInstance != "" {
		sections = append(sections, m.renderConsoleLog(headerHeight+contentHeight))
	}

	// 4. Status Bar
	sections = append(sections, m.renderStatusBar())

	return strings.Join(sections, "\n")
}

// renderHeader renders the header with date tabs and summary
func (m *MonitorModel) renderHeader() string {
	// Use lipgloss to render tabs with proper width calculation
	tabNames := []string{"TODAY", "WEEK", "MONTH", "YTD", "TTM"}

	// Calculate tab width dynamically based on available space
	availableWidth := m.WindowSize.Width
	tabContentWidth := (availableWidth - 20) / 5 // 5 tabs, more conservative spacing
	if tabContentWidth < 6 {
		tabContentWidth = 6
	}

	// Build tabs using lipgloss for proper width handling
	var tabStrings []string
	for i, name := range tabNames {
		style := InactiveTabStyle
		if DateRange(i) == m.SelectedDateRange {
			style = ActiveTabStyle
		}
		// Use lipgloss.Place to center the tab name within the allocated width
		tabStrings = append(tabStrings, style.Width(tabContentWidth).Render(name))
	}

	// Build the tab bar with lipgloss borders
	tabBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		tabStrings...,
	)

	// Summary stats
	summary := ""
	if m.Stats != nil {
		reqStyle := m.flashStyle("summary:requests", MutedStyle)
		tokStyle := m.flashStyle("summary:tokens", MutedStyle)
		fbStyle := m.flashStyle("summary:fallbacks", MutedStyle)
		summary = fmt.Sprintf("Requests: %s   Tokens: %s   Fallbacks: %s",
			reqStyle.Render(formatNumber(m.Stats.Summary.TotalRequests)),
			tokStyle.Render(formatTokens(m.Stats.Summary.TotalTokens)),
			fbStyle.Render(fmt.Sprintf("%d", m.Stats.Summary.TotalFallbacks)))
	}

	// Build header with proper borders using lipgloss
	headerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(MutedColor).
		Width(m.WindowSize.Width - 2)

	content := tabBar
	if summary != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			tabBar,
			MutedStyle.Padding(0, 2).Render(summary),
		)
	}

	return headerStyle.Render(content)
}

// renderContent renders the main content area (tables + instance list)
func (m *MonitorModel) renderContent() string {
	// Use pre-calculated widths from the model
	leftWidth := m.LeftPanelWidth
	rightWidth := m.RightPanelWidth

	// Left panel: stats tables in separate bordered boxes
	routeTable := m.renderRouteTable(leftWidth - 2)
	modelTable := m.renderModelTable(leftWidth - 2)

	// Right panel: instance list
	rightPanel := m.renderInstanceList(rightWidth - 2)

	// Use lipgloss for proper border handling with dynamic widths
	leftBoxStyle := DynamicBoxStyle.Width(leftWidth)
	rightBoxStyle := DynamicBoxStyle.Width(rightWidth)

	routeBox := leftBoxStyle.Render(routeTable)
	modelBox := leftBoxStyle.Render(modelTable)
	leftBorder := routeBox + "\n" + modelBox
	rightBorder := rightBoxStyle.Render(rightPanel)

	// Use lipgloss JoinHorizontal for proper layout
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftBorder,
		rightBorder,
	)

	return content + "\n"
}

// renderRouteTable renders the BY ROUTE section
func (m *MonitorModel) renderRouteTable(width int) string {
	// Defensive: ensure minimum width
	if width < 18 {
		width = 18
	}

	// Calculate column widths using consistent ratios
	reqColWidth := int(float64(width) * requestsColRatio) // 24%
	if reqColWidth < 8 {
		reqColWidth = 8
	}
	tokenColWidth := int(float64(width) * tokensColRatio) // 24%
	if tokenColWidth < 8 {
		tokenColWidth = 8
	}
	fallbackColWidth := int(float64(width) * fbacksColRatio) // 14%
	if fallbackColWidth < 6 {
		fallbackColWidth = 6
	}
	routeColWidth := width - reqColWidth - tokenColWidth - fallbackColWidth // 38%
	if routeColWidth < 10 {
		routeColWidth = 10
	}

	title := TableHeaderCellStyle.Render(fmt.Sprintf(" BY ROUTE "))

	if m.Stats == nil || len(m.Stats.ByRoute) == 0 {
		return title + "\n" + MutedStyle.Render(" No usage data for this period ")
	}

	var rows []string

	// Header row - empty first column (Route/Model omitted for cleaner look)
	headerRow := strings.Repeat(" ", routeColWidth) +
		TableHeaderCellStyle.Width(fallbackColWidth).Align(lipgloss.Right).Render("Fbacks") +
		TableHeaderCellStyle.Width(reqColWidth).Align(lipgloss.Right).Render("Requests") +
		TableHeaderCellStyle.Width(tokenColWidth).Align(lipgloss.Right).Render("Tokens")
	rows = append(rows, headerRow)

	// Separator
	sep := strings.Repeat("─", width)
	rows = append(rows, sep)

	// Sort routes alphabetically
	routes := make([]*usage.RouteStats, 0, len(m.Stats.ByRoute))
	for _, stats := range m.Stats.ByRoute {
		routes = append(routes, stats)
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Route < routes[j].Route
	})

	for i, route := range routes {
		// Use uncolored style for data cells - apply color per-cell
		cellStyle := TableCellStyle
		if i%2 == 1 {
			cellStyle = TableCellAltStyle
		}

		routeCell := cellStyle.Width(routeColWidth).Render(truncate(route.Route, routeColWidth))
		fallbackCell := m.flashStyle("route:"+route.Route+":fallbacks", cellStyle).Width(fallbackColWidth).Align(lipgloss.Right).Render(fmt.Sprintf("%d", route.Fallbacks))
		reqCell := m.flashStyle("route:"+route.Route+":requests", cellStyle).Width(reqColWidth).Align(lipgloss.Right).Render(formatNumber(route.Requests))
		tokenCell := m.flashStyle("route:"+route.Route+":tokens", cellStyle).Width(tokenColWidth).Align(lipgloss.Right).Render(formatTokens(route.Tokens))

		rows = append(rows, routeCell+fallbackCell+reqCell+tokenCell)
	}

	return title + "\n" + strings.Join(rows, "\n")
}

// renderModelTable renders the BY MODEL section
func (m *MonitorModel) renderModelTable(width int) string {
	// Defensive: ensure minimum width
	if width < 17 {
		width = 17
	}

	// Calculate column widths using consistent ratios
	reqColWidth := int(float64(width) * requestsColRatio) // 24%
	if reqColWidth < 8 {
		reqColWidth = 8
	}
	tokenColWidth := int(float64(width) * tokensColRatio) // 24%
	if tokenColWidth < 8 {
		tokenColWidth = 8
	}
	modelColWidth := width - reqColWidth - tokenColWidth // 52%
	if modelColWidth < 12 {
		modelColWidth = 12
	}

	title := TableHeaderCellStyle.Render(fmt.Sprintf(" BY MODEL "))

	if m.Stats == nil || len(m.Stats.ByModel) == 0 {
		return title + "\n" + MutedStyle.Render(" No model data ")
	}

	var rows []string

	// Header row - empty first column (Model omitted for cleaner look)
	headerRow := strings.Repeat(" ", modelColWidth) +
		TableHeaderCellStyle.Width(reqColWidth).Align(lipgloss.Right).Render("Requests") +
		TableHeaderCellStyle.Width(tokenColWidth).Align(lipgloss.Right).Render("Tokens")
	rows = append(rows, headerRow)

	// Separator
	sep := strings.Repeat("─", width)
	rows = append(rows, sep)

	// Sort models by token count descending
	models := make([]*usage.ModelStats, 0, len(m.Stats.ByModel))
	for _, stats := range m.Stats.ByModel {
		models = append(models, stats)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Tokens > models[j].Tokens
	})

	for i, model := range models {
		cellStyle := TableCellStyle
		if i%2 == 1 {
			cellStyle = TableCellAltStyle
		}

		modelCell := cellStyle.Width(modelColWidth).Render(truncate(model.Model, modelColWidth))
		reqCell := m.flashStyle("model:"+model.Model+":requests", cellStyle).Width(reqColWidth).Align(lipgloss.Right).Render(formatNumber(model.Requests))
		tokenCell := m.flashStyle("model:"+model.Model+":tokens", cellStyle).Width(tokenColWidth).Align(lipgloss.Right).Render(formatTokens(model.Tokens))

		rows = append(rows, modelCell+reqCell+tokenCell)
	}

	return title + "\n" + strings.Join(rows, "\n")
}

// renderInstanceList renders the INSTANCES panel
func (m *MonitorModel) renderInstanceList(width int) string {
	// Defensive: ensure minimum width
	if width < 1 {
		width = 1
	}

	title := TableHeaderCellStyle.Width(width).Render(" INSTANCES ")

	var items []string

	// "** ALL **" option
	allContent := "** ALL **"
	if m.SelectedInstance == "" {
		allContent = SelectedInstanceStyle.Width(width).Render(allContent)
	} else {
		allContent = UnselectedInstanceStyle.Width(width).Render(allContent)
	}
	items = append(items, allContent)

	// Individual instances
	for i, inst := range m.Instances {
		// Check if this instance is selected
		isSelected := false
		if m.InstanceCursor == i+1 {
			isSelected = true
		}

		indicator := "○"
		style := UnselectedInstanceStyle
		if inst.IsRunning {
			indicator = "●"
		}
		if isSelected {
			style = SelectedInstanceStyle
		}

		// Build the content with indicator and use lipgloss.Place for proper width filling
		content := indicator + " " + inst.ID
		line := lipgloss.Place(width, 1, lipgloss.Left, ' ', style.Render(content))
		items = append(items, line)

		// Limit visible items
		if i > 15 {
			items = append(items, MutedStyle.Width(width).Render("... (more)"))
			break
		}
	}

	if len(m.Instances) == 0 {
		items = append(items, MutedStyle.Width(width).Render("(no instances)"))
	}

	return title + "\n" + strings.Join(items, "\n")
}

// renderConsoleLog renders the console log pane
// usedHeight is the number of lines used by header + content area, so the
// console log can size itself to fit the remaining terminal height.
func (m *MonitorModel) renderConsoleLog(usedHeight int) string {
	enabled := "[✔]"
	if m.ConsoleLogPaused {
		enabled = "[⏸]"
	}

	// Ensure minimum width
	consoleWidth := m.WindowSize.Width
	if consoleWidth < 35 {
		consoleWidth = 35
	}

	// Use lipgloss for title
	boxStyle := DynamicBoxStyle.Width(consoleWidth - 2)
	title := TableHeaderCellStyle.Render(" CONSOLE LOG ") + " " +
		MutedStyle.Render(enabled+" enabled(c)")

	// Get log lines
	lines := m.LogBuffer.GetFilteredLines(m.LogLevelFilters)

	// Calculate maxLines from remaining terminal height.
	// Fixed overhead: status bar(1) + log footer/filters(1) + console box borders(3) = 5
	const fixedOverhead = 5
	maxLines := m.WindowSize.Height - usedHeight - fixedOverhead
	if maxLines < 3 {
		maxLines = 3
	}
	if maxLines > 15 {
		maxLines = 15
	}

	startIdx := len(lines) - maxLines
	if startIdx < 0 {
		startIdx = 0
	}

	var contentLines []string
	for _, line := range lines[startIdx:] {
		style := LogLevelStyles[line.Level]
		truncated := runewidth.Truncate(strings.TrimSuffix(line.Raw, "\n"), consoleWidth-4, "")
		contentLines = append(contentLines, style.Render(truncated))
	}

	// Pad with empty lines if needed
	emptyLineStyle := lipgloss.NewStyle().Width(consoleWidth - 4)
	for i := len(contentLines); i < maxLines; i++ {
		contentLines = append(contentLines, emptyLineStyle.Render(""))
	}

	content := strings.Join(contentLines, "\n")
	footer := m.renderLogLevelFilters()

	return boxStyle.Render(title+"\n"+content) + "\n" + footer
}

// renderLogLevelFilters renders the log level filter checkboxes
func (m *MonitorModel) renderLogLevelFilters() string {
	levels := []struct {
		key   string
		level LogLevel
		label string
	}{
		{"1", LogLevelVerbs, "VERBS"},
		{"2", LogLevelTrace, "TRACE"},
		{"3", LogLevelDebug, "DEBUG"},
		{"4", LogLevelInfo, "INFO"},
		{"5", LogLevelWarn, "WARN"},
		{"6", LogLevelError, "ERROR"},
		{"7", LogLevelFatal, "FATAL"},
	}

	var filters []string
	for _, l := range levels {
		checkbox := "[ ]"
		if m.LogLevelFilters&LogLevelSet(l.level) != 0 {
			checkbox = "[✔]"
		}

		filter := fmt.Sprintf("%s %s", checkbox, l.label)
		style := LogLevelStyles[l.level]
		filters = append(filters, style.Render(filter))
	}

	// Use lipgloss for proper layout
	separatorStyle := lipgloss.NewStyle().Foreground(MutedColor)
	footerStyle := DynamicBoxStyle.Width(m.WindowSize.Width - 2)

	content := separatorStyle.Render(strings.Join(filters, "──"))

	return footerStyle.Render(content)
}

// renderStatusBar renders the status bar with shortcuts
func (m *MonitorModel) renderStatusBar() string {
	// Keyboard shortcuts
	shortcuts := "q:quit"
	if m.SelectedInstance != "" {
		shortcuts += " | c:console"
	}
	if m.ConsoleLogEnabled {
		shortcuts += " | space:pause | 1-7:filters"
	}
	shortcuts += " | ←→:date | ↑↓:instance | r:refresh"

	// Calculate padding using visual width
	shortcutsWidth := runewidth.StringWidth(shortcuts)
	padding := m.WindowSize.Width - shortcutsWidth - 3
	if padding < 0 {
		padding = 0
	}

	// Use lipgloss for status bar
	statusBarStyle := lipgloss.NewStyle().
		Width(m.WindowSize.Width - 2).
		Background(StatusBarBackground).
		Foreground(SecondaryText)

	shortcutStyle := lipgloss.NewStyle().
		Width(m.WindowSize.Width - 3).
		Render(shortcuts + strings.Repeat(" ", padding))

	return statusBarStyle.Render(shortcutStyle)
}

// Helper functions
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 10000 {
		return fmt.Sprintf("%.2fK", float64(n)/1000)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.2fM", float64(n)/1000000)
}

func formatTokens(n int) string {
	switch {
	case n < 1_000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	}
}

func truncate(s string, maxLen int) string {
	return runewidth.Truncate(s, maxLen, "...")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}