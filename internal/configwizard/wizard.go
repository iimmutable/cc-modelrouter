package configwizard

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iimmutable/cc-modelrouter/internal/config"

	"github.com/mattn/go-runewidth"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
)

// WizardKeyMap defines the key bindings for the wizard.
type WizardKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Escape  key.Binding
	Delete  key.Binding
	Tab     key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() WizardKeyMap {
	return WizardKeyMap{
		Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "select")),
		Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("Esc", "back")),
		Delete: key.NewBinding(key.WithKeys("del", "d"), key.WithHelp("Del", "delete")),
		Tab:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "next")),
	}
}

// WizardModel is the main Bubble Tea model for the wizard.
type WizardModel struct {
	state    *WizardState
	keys     WizardKeyMap
	width    int
	height   int
	textInput textinput.Model
	focusedField int // 0 = first input, 1 = second, etc.
}

// blankLine renders a blank line spanning the full content width.
func (m *WizardModel) blankLine() string {
	return lipgloss.NewStyle().Width(m.contentWidth()).Render(" ")
}

// divider renders a horizontal rule using box-drawing characters.
func (m *WizardModel) divider() string {
	return lipgloss.NewStyle().
		Foreground(BorderColor).
		Width(m.contentWidth()).
		Render(strings.Repeat("─", m.contentWidth()))
}

// fullWidth pads any content to fill contentWidth.
func (m *WizardModel) fullWidth(content string) string {
	return lipgloss.PlaceHorizontal(m.contentWidth(), lipgloss.Left, content)
}

// inputFieldWidth returns the Width to pass to InputFieldStyle/InputFieldFocusedStyle
// so the total rendered width (content + border + padding) fits within contentWidth.
func (m *WizardModel) inputFieldWidth() int {
	return m.contentWidth() - InputFieldStyle.GetHorizontalFrameSize()
}

// buttonFieldWidth returns the Width to pass to ButtonStyle so it fits within contentWidth.
func (m *WizardModel) buttonFieldWidth() int {
	return m.contentWidth() - ButtonStyle.GetHorizontalFrameSize()
}

// contentWidth returns the available width inside the main container,
// accounting for border, padding, and margins.
func (m *WizardModel) contentWidth() int {
	w := (m.width - 2) - MainContainerStyle.GetHorizontalFrameSize()
	if w < 20 {
		return 20
	}
	return w
}

// NewWizardModel creates a new wizard model.
func NewWizardModel(cfg *config.Config, configPath string) *WizardModel {
	state := NewWizardState(cfg, configPath)

	return &WizardModel{
		state: state,
		keys:  DefaultKeyMap(),
	}
}

// Init initializes the wizard.
func (m *WizardModel) Init() tea.Cmd {
	return tea.EnableBracketedPaste
}

// Update handles messages.
func (m *WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case TestConnectionResultMsg:
		m.state.TestStatus = "done"
		if msg.Success {
			m.state.TestStatus = "success"
			m.state.TestLatency = msg.Latency.Seconds()
		} else {
			m.state.TestStatus = "error"
			m.state.TestError = msg.Error
		}
		return m, nil

	case portTestDoneMsg:
		m.state.PortStatus = msg.status
		m.state.PortTesting = false
		return m, nil
	}

	return m, cmd
}

type TestConnectionResultMsg struct {
	Success   bool
	Latency   time.Duration
	Error     string
}

type portTestDoneMsg struct {
	status string
}

// handleKeyPress handles keyboard input.
func (m *WizardModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.state.HasUnsavedChanges() {
			m.state.ShowConfirm = true
			m.state.ConfirmCursor = 0 // Default to Yes (user initiated quit)
			m.state.ConfirmMessage = "You have unsaved changes. Quit anyway?"
			m.state.ConfirmAction = func() bool {
				return true // Allow quit
			}
			return m, nil
		}
		return m, tea.Quit

	case tea.KeyEscape:
		return m.handleEscape()

	case tea.KeyUp, tea.KeyDown:
		return m.handleNavigation(msg)

	case tea.KeyEnter:
		return m.handleEnter()

	case tea.KeyLeft, tea.KeyRight:
		if m.state.ShowConfirm {
			if msg.Type == tea.KeyLeft {
				m.state.ConfirmCursor = 0 // Yes
			} else {
				m.state.ConfirmCursor = 1 // No
			}
			return m, nil
		}

	case tea.KeyTab:
		if max := m.getMaxFields(); max > 0 {
			m.focusedField = (m.focusedField + 1) % max
			// Hide dropdowns when tabbing away from their fields
			if m.state.CurrentScreen == ScreenAddProvider1 {
				if m.focusedField != 0 {
					m.state.ShowDropdown = false
					m.state.DropdownCursor = 0
				}
				if m.focusedField != 2 {
					m.state.ShowModelDropdown = false
					m.state.ModelDropdownCursor = 0
				}
			}
			if m.state.CurrentScreen == ScreenEditRoute {
				m.state.ShowRouteNameDropdown = false
				m.state.RouteNameDropdownCursor = 0
				m.state.ShowDropdown = false
				m.state.DropdownCursor = 0
				m.state.ShowModelDropdown = false
				m.state.ModelDropdownCursor = 0
			}
			if m.state.CurrentScreen == ScreenLogging {
				m.state.ShowLogLevelDropdown = false
				m.state.LogLevelDropdownCursor = 0
				m.state.ShowLogDestDropdown = false
				m.state.LogDestDropdownCursor = 0
			}
		}
		return m, nil
	}

	// Handle character keys based on screen
	switch m.state.CurrentScreen {
	case ScreenAddProvider1, ScreenAddProvider2:
		return m.handleFormInput(msg)
	case ScreenServer:
		return m.handleServerInput(msg)
	case ScreenLogging:
		return m.handleLoggingInput(msg)
	case ScreenEditRoute:
		if msg.String() == "a" && m.focusedField == 1 {
			// Insert new chain entry after the currently selected item
			if len(m.state.EditRouteChain) == 0 {
				m.state.EditRouteChain = append(m.state.EditRouteChain, config.RouteTarget{Provider: "", Model: ""})
				m.state.EditRouteChainCursor = 0
			} else {
				pos := m.state.EditRouteChainCursor + 1
				m.state.EditRouteChain = append(
					m.state.EditRouteChain[:pos],
					append([]config.RouteTarget{{Provider: "", Model: ""}}, m.state.EditRouteChain[pos:]...)...,
				)
				m.state.EditRouteChainCursor = pos
			}
			// Open provider dropdown for the new entry
			m.state.ShowDropdown = true
			m.state.DropdownCursor = 0
			m.state.ShowModelDropdown = false
			return m, nil
		}
		if (msg.String() == "backspace" || msg.String() == "delete" || msg.String() == "del") && m.focusedField == 1 {
			if len(m.state.EditRouteChain) > 0 {
				m.state.EditRouteChain = append(
					m.state.EditRouteChain[:m.state.EditRouteChainCursor],
					m.state.EditRouteChain[m.state.EditRouteChainCursor+1:]...,
				)
				if m.state.EditRouteChainCursor >= len(m.state.EditRouteChain) && len(m.state.EditRouteChain) > 0 {
					m.state.EditRouteChainCursor = len(m.state.EditRouteChain) - 1
				}
			}
			return m, nil
		}
		return m.handleRouteEditInput(msg)
	case ScreenProviders:
		if msg.String() == "a" {
			m.state.EditRouteName = ""
			m.state.NewProviderName = ""
			m.state.NewProviderBaseURL = ""
			m.state.NewProviderTransformer = ""
			m.state.NewProviderModels = ""
			m.state.NewProviderAPIKey = ""
			m.state.ProviderPreset = ""
			m.state.ShowDropdown = true
			m.state.DropdownCursor = 0
			m.state.ShowModelDropdown = false
			m.state.ModelDropdownCursor = 0
			m.state.AddToShellConfig = true
			m.state.SourceImmediately = true
			m.state.CurrentScreen = ScreenAddProvider1
			m.focusedField = 0
			return m, nil
		}
		if msg.String() == "backspace" || msg.String() == "delete" || msg.String() == "del" {
			return m.handleProvidersDelete()
		}
	case ScreenRoutes:
		if msg.String() == "a" {
			m.state.EditRouteName = ""
			m.state.EditRouteChain = nil
			m.state.EditRouteChainCursor = 0
			m.state.ShowDropdown = false
			m.state.DropdownCursor = 0
			m.state.ShowModelDropdown = false
			m.state.ModelDropdownCursor = 0
			m.state.ShowRouteNameDropdown = false
			m.state.RouteNameDropdownCursor = 0
			m.state.CurrentScreen = ScreenEditRoute
			return m, nil
		}
		if msg.String() == "backspace" || msg.String() == "delete" || msg.String() == "del" {
			return m.handleRoutesDelete()
		}
	}

	return m, nil
}

// handleEscape handles the escape key.
func (m *WizardModel) handleEscape() (tea.Model, tea.Cmd) {
	// If showing confirmation, dismiss it
	if m.state.ShowConfirm {
		m.state.ShowConfirm = false
		m.state.ConfirmMessage = ""
		m.state.ConfirmAction = nil
		m.state.ConfirmCursor = 0
		return m, nil
	}

	switch m.state.CurrentScreen {
	case ScreenMainMenu:
		if m.state.ProviderCursor != 5 {
			m.state.ProviderCursor = 5 // Jump to "Save & Exit"
		} else {
			m.state.ProviderCursor = 6 // Already on Save & Exit, go to "Quit without saving"
		}
		return m, nil

	case ScreenAddProvider1:
		if m.state.ShowDropdown {
			m.state.ShowDropdown = false
			m.state.DropdownCursor = 0
			return m, nil
		}
		if m.state.ShowModelDropdown {
			m.state.ShowModelDropdown = false
			m.state.ModelDropdownCursor = 0
			return m, nil
		}
		// If editing an existing provider, save form changes back to in-memory config
		if m.state.EditingProvider {
			providerName := strings.TrimSpace(m.state.NewProviderName)
			modelsStr := strings.TrimSpace(m.state.NewProviderModels)
			if providerName != "" && modelsStr != "" {
				models := strings.Split(modelsStr, "\n")
				// Filter empty model entries
				var validModels []string
				for _, mdl := range models {
					if strings.TrimSpace(mdl) != "" {
						validModels = append(validModels, strings.TrimSpace(mdl))
					}
				}
				if len(validModels) > 0 {
					apiKey := strings.TrimSpace(m.state.NewProviderAPIKey)
					envVarName := GenerateEnvVarName(providerName)
					m.state.Config.Providers[providerName] = config.ProviderConfig{
						APIKey:      "${" + envVarName + "}",
						BaseURL:     strings.TrimSpace(m.state.NewProviderBaseURL),
						Transformer: strings.TrimSpace(m.state.NewProviderTransformer),
						Models:      validModels,
					}
					m.state.HasChanges = true

					// Track resolved API key
					if apiKey != "" {
						if m.state.ResolvedAPIKeys == nil {
							m.state.ResolvedAPIKeys = make(map[string]string)
						}
						m.state.ResolvedAPIKeys[providerName] = apiKey
					}

					// Shell integration is deferred to "Save & Exit"
				}
			}
		}
		m.resetAddProviderState()
		m.state.CurrentScreen = ScreenProviders

	case ScreenAddProvider2:
		m.state.CurrentScreen = ScreenAddProvider1

	case ScreenLogging:
		if m.state.ShowLogLevelDropdown {
			m.state.ShowLogLevelDropdown = false
			m.state.LogLevelDropdownCursor = 0
			return m, nil
		}
		if m.state.ShowLogDestDropdown {
			m.state.ShowLogDestDropdown = false
			m.state.LogDestDropdownCursor = 0
			return m, nil
		}
		// Sync logging settings to in-memory config
		m.state.Config.Logging.Enabled = m.state.LoggingEnabled
		m.state.Config.Logging.Level = m.state.LoggingLevel
		m.state.Config.Logging.Destination = m.state.LoggingDestination
		m.state.Config.Logging.FilePath = m.state.LoggingFilePath
		m.state.HasChanges = true
		m.state.PortStatus = ""
		m.state.ProviderCursor = m.state.MainMenuCursor
		m.state.CurrentScreen = ScreenMainMenu

	case ScreenServer:
		// Sync server settings to in-memory config (same validation as handleServerSave)
		host := strings.TrimSpace(m.state.ServerHost)
		portStr := strings.TrimSpace(m.state.ServerPort)
		if host != "" {
			if port, err := strconv.Atoi(portStr); err == nil && port >= 1024 && port <= 65535 {
				m.state.Config.Server.Host = host
				m.state.Config.Server.Port = port
				m.state.HasChanges = true
			}
		}
		m.state.PortStatus = ""
		m.state.ProviderCursor = m.state.MainMenuCursor
		m.state.CurrentScreen = ScreenMainMenu

	case ScreenProviders, ScreenRoutes, ScreenViewConfig:
		m.state.PortStatus = ""
		m.state.ProviderCursor = m.state.MainMenuCursor
		m.state.CurrentScreen = ScreenMainMenu

	case ScreenEditRoute:
		if m.state.ShowRouteNameDropdown {
			m.state.ShowRouteNameDropdown = false
			m.state.RouteNameDropdownCursor = 0
			return m, nil
		}
		if m.state.ShowDropdown {
			// Cancelled before selecting a provider — remove the empty entry
			if m.state.EditRouteChainCursor < len(m.state.EditRouteChain) {
				cursor := m.state.EditRouteChainCursor
				if m.state.EditRouteChain[cursor].Provider == "" && m.state.EditRouteChain[cursor].Model == "" {
					m.state.EditRouteChain = append(
						m.state.EditRouteChain[:cursor],
						m.state.EditRouteChain[cursor+1:]...,
					)
					if m.state.EditRouteChainCursor >= len(m.state.EditRouteChain) && len(m.state.EditRouteChain) > 0 {
						m.state.EditRouteChainCursor = len(m.state.EditRouteChain) - 1
					}
				}
			}
			m.state.ShowDropdown = false
			m.state.DropdownCursor = 0
			return m, nil
		}
		if m.state.ShowModelDropdown {
			// Cancelled after selecting a provider but before selecting a model — remove partial entry
			if m.state.EditRouteChainCursor < len(m.state.EditRouteChain) {
				cursor := m.state.EditRouteChainCursor
				if m.state.EditRouteChain[cursor].Model == "" {
					m.state.EditRouteChain = append(
						m.state.EditRouteChain[:cursor],
						m.state.EditRouteChain[cursor+1:]...,
					)
					if m.state.EditRouteChainCursor >= len(m.state.EditRouteChain) && len(m.state.EditRouteChain) > 0 {
						m.state.EditRouteChainCursor = len(m.state.EditRouteChain) - 1
					}
				}
			}
			m.state.ShowModelDropdown = false
			m.state.ModelDropdownCursor = 0
			return m, nil
		}
		// Save draft chain back to config before navigating away
		routeName := strings.TrimSpace(m.state.EditRouteName)
		if routeName != "" {
			var chainParts []string
			for _, target := range m.state.EditRouteChain {
				if target.Provider != "" && target.Model != "" {
					chainParts = append(chainParts, fmt.Sprintf("%s:%s", target.Provider, target.Model))
				}
			}
			if len(chainParts) > 0 {
				m.state.Config.Router.Routes[routeName] = strings.Join(chainParts, ";")
				m.state.HasChanges = true
			} else {
				delete(m.state.Config.Router.Routes, routeName)
				m.state.HasChanges = true
			}
		}
		m.state.CurrentScreen = ScreenRoutes

	case ScreenTestConnection:
		m.state.CurrentScreen = ScreenProviders

	default:
		m.state.ProviderCursor = m.state.MainMenuCursor
		m.state.CurrentScreen = ScreenMainMenu
	}

	m.state.ErrorMessage = ""
	return m, nil
}

// handleNavigation handles up/down navigation.
func (m *WizardModel) handleNavigation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	isUp := msg.Type == tea.KeyUp || msg.String() == "k"

	// If dropdown is visible on Add Provider screen, navigate dropdown instead
	if m.state.CurrentScreen == ScreenAddProvider1 && m.state.ShowDropdown && m.focusedField == 0 {
		matches := m.getPresetMatches()
		if len(matches) > 0 {
			if isUp {
				m.state.DropdownCursor = (m.state.DropdownCursor - 1 + len(matches)) % len(matches)
			} else {
				m.state.DropdownCursor = (m.state.DropdownCursor + 1) % len(matches)
			}
		}
		return m, nil
	}

	// If model dropdown is visible on Add Provider screen, navigate model dropdown instead
	if m.state.CurrentScreen == ScreenAddProvider1 && m.state.ShowModelDropdown && m.focusedField == 2 {
		matches := m.getModelSuggestions()
		if len(matches) > 0 {
			if isUp {
				m.state.ModelDropdownCursor = (m.state.ModelDropdownCursor - 1 + len(matches)) % len(matches)
			} else {
				m.state.ModelDropdownCursor = (m.state.ModelDropdownCursor + 1) % len(matches)
			}
		}
		return m, nil
	}

	// If logging level dropdown is visible, navigate it instead
	if m.state.CurrentScreen == ScreenLogging && m.state.ShowLogLevelDropdown && m.focusedField == 1 {
		levels := LogLevelOptions
		if len(levels) > 0 {
			if isUp {
				m.state.LogLevelDropdownCursor = (m.state.LogLevelDropdownCursor - 1 + len(levels)) % len(levels)
			} else {
				m.state.LogLevelDropdownCursor = (m.state.LogLevelDropdownCursor + 1) % len(levels)
			}
		}
		return m, nil
	}

	// If logging destination dropdown is visible, navigate it instead
	if m.state.CurrentScreen == ScreenLogging && m.state.ShowLogDestDropdown && m.focusedField == 2 {
		dests := LogDestinationOptions
		if len(dests) > 0 {
			if isUp {
				m.state.LogDestDropdownCursor = (m.state.LogDestDropdownCursor - 1 + len(dests)) % len(dests)
			} else {
				m.state.LogDestDropdownCursor = (m.state.LogDestDropdownCursor + 1) % len(dests)
			}
		}
		return m, nil
	}

	// If route name dropdown is visible on Edit Route screen, navigate it instead
	if m.state.CurrentScreen == ScreenEditRoute && m.state.ShowRouteNameDropdown && m.focusedField == 0 {
		matches := m.getRouteNameDropdownList()
		if len(matches) > 0 {
			if isUp {
				m.state.RouteNameDropdownCursor = (m.state.RouteNameDropdownCursor - 1 + len(matches)) % len(matches)
			} else {
				m.state.RouteNameDropdownCursor = (m.state.RouteNameDropdownCursor + 1) % len(matches)
			}
		}
		return m, nil
	}

	// If dropdown is visible on Edit Route screen (chain list), navigate dropdown instead
	if m.state.CurrentScreen == ScreenEditRoute && m.focusedField == 1 {
		if m.state.ShowDropdown {
			matches := m.getChainProviderList()
			if len(matches) > 0 {
				if isUp {
					m.state.DropdownCursor = (m.state.DropdownCursor - 1 + len(matches)) % len(matches)
				} else {
					m.state.DropdownCursor = (m.state.DropdownCursor + 1) % len(matches)
				}
			}
			return m, nil
		}
		if m.state.ShowModelDropdown {
			matches := m.getChainModelList()
			if len(matches) > 0 {
				if isUp {
					m.state.ModelDropdownCursor = (m.state.ModelDropdownCursor - 1 + len(matches)) % len(matches)
				} else {
					m.state.ModelDropdownCursor = (m.state.ModelDropdownCursor + 1) % len(matches)
				}
			}
			return m, nil
		}
		// No dropdown open — navigate chain items
		chainLen := len(m.state.EditRouteChain)
		if chainLen > 0 {
			if isUp {
				m.state.EditRouteChainCursor = (m.state.EditRouteChainCursor - 1 + chainLen) % chainLen
			} else {
				m.state.EditRouteChainCursor = (m.state.EditRouteChainCursor + 1) % chainLen
			}
		}
		return m, nil
	}

	switch m.state.CurrentScreen {
	case ScreenMainMenu:
		if isUp && m.state.ProviderCursor > 0 {
			m.state.ProviderCursor--
		} else if !isUp && m.state.ProviderCursor < 6 {
			m.state.ProviderCursor++
		}

	case ScreenProviders:
		providerCount := len(m.state.Config.Providers)
		if providerCount > 0 {
			if isUp {
				m.state.ProviderCursor = (m.state.ProviderCursor - 1 + providerCount) % providerCount
			} else if !isUp {
				m.state.ProviderCursor = (m.state.ProviderCursor + 1) % providerCount
			}
		}

	case ScreenRoutes:
		routeCount := len(m.state.Config.Router.Routes)
		if routeCount > 0 {
			if isUp {
				m.state.RouteCursor = (m.state.RouteCursor - 1 + routeCount) % routeCount
			} else {
				m.state.RouteCursor = (m.state.RouteCursor + 1) % routeCount
			}
		}
	}

	return m, nil
}

// handleEnter handles the enter key.
func (m *WizardModel) handleEnter() (tea.Model, tea.Cmd) {
	// If showing confirmation, handle it
	if m.state.ShowConfirm {
		if m.state.ConfirmCursor == 0 && m.state.ConfirmAction != nil && m.state.ConfirmAction() {
			return m, tea.Quit
		}
		m.state.ShowConfirm = false
		m.state.ConfirmMessage = ""
		m.state.ConfirmAction = nil
		return m, nil
	}

	switch m.state.CurrentScreen {
	case ScreenMainMenu:
		return m.handleMainMenuEnter()

	case ScreenProviders:
		return m.handleProvidersEnter()

	case ScreenRoutes:
		return m.handleRoutesEnter()

	case ScreenServer:
		return m.handleServerSave()

	case ScreenLogging:
		// If log level dropdown is open, select the item
		if m.state.ShowLogLevelDropdown && m.focusedField == 1 {
			if m.state.LogLevelDropdownCursor < len(LogLevelOptions) {
				m.state.LoggingLevel = LogLevelOptions[m.state.LogLevelDropdownCursor]
			}
			m.state.ShowLogLevelDropdown = false
			m.state.LogLevelDropdownCursor = 0
			return m, nil
		}
		// If log destination dropdown is open, select the item
		if m.state.ShowLogDestDropdown && m.focusedField == 2 {
			if m.state.LogDestDropdownCursor < len(LogDestinationOptions) {
				m.state.LoggingDestination = LogDestinationOptions[m.state.LogDestDropdownCursor]
			}
			m.state.ShowLogDestDropdown = false
			m.state.LogDestDropdownCursor = 0
			return m, nil
		}
		// If focused on level field, open dropdown (only when logging is enabled)
		if m.focusedField == 1 {
			if !m.state.LoggingEnabled {
				return m, nil
			}
			m.state.ShowLogLevelDropdown = true
			// Set cursor to current level
			for i, l := range LogLevelOptions {
				if l == m.state.LoggingLevel {
					m.state.LogLevelDropdownCursor = i
					break
				}
			}
			return m, nil
		}
		// If focused on destination field, open dropdown (only when logging is enabled)
		if m.focusedField == 2 {
			if !m.state.LoggingEnabled {
				return m, nil
			}
			m.state.ShowLogDestDropdown = true
			// Set cursor to current destination
			for i, d := range LogDestinationOptions {
				if d == m.state.LoggingDestination {
					m.state.LogDestDropdownCursor = i
					break
				}
			}
			return m, nil
		}
		return m.handleLoggingSave()

	case ScreenViewConfig:
		// Export config to file
		m.exportConfig()

	case ScreenAddProvider1:
		// If dropdown is visible and focused on name field, select preset
		if m.state.ShowDropdown && m.focusedField == 0 {
			matches := m.getPresetMatches()
			if len(matches) > 0 && m.state.DropdownCursor < len(matches) {
				m.applyPreset(matches[m.state.DropdownCursor])
			} else {
				m.state.ShowDropdown = false
			}
			return m, nil
		}
		// If model dropdown is visible and focused on models field, select model
		if m.state.ShowModelDropdown && m.focusedField == 2 {
			matches := m.getModelSuggestions()
			if len(matches) > 0 && m.state.ModelDropdownCursor < len(matches) {
				m.insertModelFromDropdown(matches[m.state.ModelDropdownCursor])
			} else {
				m.state.ShowModelDropdown = false
			}
			return m, nil
		}
		// If focused on name field and dropdown is not showing, open it
		if m.focusedField == 0 && !m.state.ShowDropdown {
			m.state.ShowDropdown = true
			m.state.DropdownCursor = 0
			return m, nil
		}
		// If focused on models field and current line has content, add newline instead of navigating
		if m.focusedField == 2 {
			currentLine := m.state.NewProviderModels
			if idx := strings.LastIndex(currentLine, "\n"); idx >= 0 {
				currentLine = currentLine[idx+1:]
			}
			if currentLine != "" {
				m.state.NewProviderModels += "\n"
				m.state.ShowModelDropdown = true
				m.state.ModelDropdownCursor = 0
				return m, nil
			}
		}
		return m.handleAddProvider1Enter()

	case ScreenAddProvider2:
		return m.handleAddProvider2Enter()

	case ScreenEditRoute:
		return m.handleEditRouteEnter()
	}

	return m, nil
}

func (m *WizardModel) handleMainMenuEnter() (tea.Model, tea.Cmd) {
	m.state.MainMenuCursor = m.state.ProviderCursor
	switch m.state.ProviderCursor {
	case 0: // Providers
		m.state.ProviderCursor = 0
		m.state.CurrentScreen = ScreenProviders
	case 1: // Routes
		m.state.RouteCursor = 0
		m.state.CurrentScreen = ScreenRoutes
	case 2: // Server
		m.state.ServerHost = m.state.Config.Server.Host
		m.state.ServerPort = strconv.Itoa(m.state.Config.Server.Port)
		m.state.PortStatus = ""
		m.state.CurrentScreen = ScreenServer
		return m, m.checkPortAvailability()
	case 3: // Logging
		m.state.LoggingEnabled = m.state.Config.Logging.Enabled
		m.state.LoggingLevel = m.state.Config.Logging.Level
		if m.state.LoggingLevel == "" {
			m.state.LoggingLevel = "info"
		}
		m.state.LoggingDestination = m.state.Config.Logging.Destination
		if m.state.LoggingDestination == "" {
			m.state.LoggingDestination = "stdout"
		}
		m.state.LoggingFilePath = m.state.Config.Logging.FilePath
		m.state.CurrentScreen = ScreenLogging
	case 4: // View Config
		m.state.CurrentScreen = ScreenViewConfig
	case 5: // Save & Exit
		if m.state.HasUnsavedChanges() {
			if err := m.saveConfig(); err != nil {
				m.state.ErrorMessage = fmt.Sprintf("Failed to save: %v", err)
				return m, nil
			}
			// Sync shell RC file to match current config
			if shellCfg, err := GetShellConfig(); err == nil && len(m.state.ResolvedAPIKeys) > 0 {
				_ = shellCfg.SyncAllShellExports(m.state.ResolvedAPIKeys)
				shellCfg.SourceAllNow(m.state.ResolvedAPIKeys)
				shellCfg.WriteEnvFile(m.state.ResolvedAPIKeys)
			}
			m.state.HasChanges = false
			m.state.OriginalCfg = deepCopyConfig(m.state.Config)
			// Snapshot resolved keys so future edits are detected
			m.state.OriginalResolvedKeys = make(map[string]string, len(m.state.ResolvedAPIKeys))
			for k, v := range m.state.ResolvedAPIKeys {
				m.state.OriginalResolvedKeys[k] = v
			}
		}
		return m, tea.Quit
	case 6: // Quit without saving
		if m.state.HasUnsavedChanges() {
			m.state.ShowConfirm = true
			m.state.ConfirmCursor = 0 // Default to Yes (user initiated quit)
			m.state.ConfirmMessage = "You have unsaved changes. Quit without saving?"
			m.state.ConfirmAction = func() bool {
				return true // Allow quit
			}
			return m, nil
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m *WizardModel) handleProvidersEnter() (tea.Model, tea.Cmd) {
	// Get provider at cursor position
	providers := m.getProviderList()
	if m.state.ProviderCursor < len(providers) {
		providerName := providers[m.state.ProviderCursor]
		m.state.EditRouteName = providerName
		if providerCfg, ok := m.state.Config.Providers[providerName]; ok {
			m.state.NewProviderName = providerName
			m.state.NewProviderBaseURL = providerCfg.BaseURL
			m.state.NewProviderTransformer = providerCfg.Transformer
			m.state.NewProviderModels = strings.Join(providerCfg.Models, "\n")
			expanded := os.ExpandEnv(providerCfg.APIKey)
			if expanded == "" && strings.Contains(providerCfg.APIKey, "${") {
				// Env var not set — show the placeholder so user knows which var is needed
				m.state.NewProviderAPIKey = providerCfg.APIKey
			} else {
				m.state.NewProviderAPIKey = expanded
			}
		}
		m.state.CurrentScreen = ScreenAddProvider1
		m.state.ProviderPreset = "custom"
		m.state.EditingProvider = true
		m.state.AddToShellConfig = true
		m.state.SourceImmediately = true
	}
	return m, nil
}

func (m *WizardModel) handleProvidersDelete() (tea.Model, tea.Cmd) {
	providers := m.getProviderList()
	if len(providers) == 0 || m.state.ProviderCursor >= len(providers) {
		return m, nil
	}

	providerName := providers[m.state.ProviderCursor]
	m.state.ShowConfirm = true
	m.state.ConfirmCursor = 1 // Default to No (safer)
	m.state.ConfirmMessage = fmt.Sprintf("Delete provider \"%s\"?", providerName)
	m.state.ConfirmAction = func() bool {
		delete(m.state.Config.Providers, providerName)
		delete(m.state.ResolvedAPIKeys, providerName)

		// Remove from shell RC file
		if shellCfg, err := GetShellConfig(); err == nil {
			_ = shellCfg.RemoveFromShellConfig(providerName)
		}

		// Clamp cursor
		newProviders := make([]string, 0, len(m.state.Config.Providers))
		for name := range m.state.Config.Providers {
			newProviders = append(newProviders, name)
		}
		sort.Strings(newProviders)
		if m.state.ProviderCursor >= len(newProviders) {
			m.state.ProviderCursor = len(newProviders) - 1
		}

		// Clean up routes that reference the deleted provider
		for routeName, routeStr := range m.state.Config.Router.Routes {
			targets := config.ParseRoute(routeStr)
			remaining := make([]config.RouteTarget, 0, len(targets))
			for _, t := range targets {
				if t.Provider != providerName {
					remaining = append(remaining, t)
				}
			}
			if len(remaining) == 0 {
				delete(m.state.Config.Router.Routes, routeName)
			} else {
				parts := make([]string, 0, len(remaining))
				for _, t := range remaining {
					parts = append(parts, t.Provider+":"+t.Model)
				}
				m.state.Config.Router.Routes[routeName] = strings.Join(parts, ";")
			}
		}

		m.state.HasChanges = true
		return false
	}
	return m, nil
}

func (m *WizardModel) handleRoutesEnter() (tea.Model, tea.Cmd) {
	routes := m.getRouteList()
	if m.state.RouteCursor < len(routes) {
		routeName := routes[m.state.RouteCursor]
		m.state.EditRouteName = routeName
		m.state.EditRouteChain = config.ParseRoute(m.state.Config.Router.Routes[routeName])
		m.state.EditRouteChainCursor = 0
		m.state.ShowDropdown = false
		m.state.DropdownCursor = 0
		m.state.ShowModelDropdown = false
		m.state.ModelDropdownCursor = 0
		m.state.ShowRouteNameDropdown = false
		m.state.RouteNameDropdownCursor = 0
		m.state.CurrentScreen = ScreenEditRoute
	}
	return m, nil
}

func (m *WizardModel) handleRoutesDelete() (tea.Model, tea.Cmd) {
	routes := m.getRouteList()
	if len(routes) == 0 || m.state.RouteCursor >= len(routes) {
		return m, nil
	}

	routeName := routes[m.state.RouteCursor]
	m.state.ShowConfirm = true
	m.state.ConfirmCursor = 1 // Default to No (safer)
	m.state.ConfirmMessage = fmt.Sprintf("Delete route \"%s\"? This cannot be undone.", routeName)
	m.state.ConfirmAction = func() bool {
		delete(m.state.Config.Router.Routes, routeName)

		// Clamp cursor
		remaining := m.getRouteList()
		if m.state.RouteCursor >= len(remaining) && len(remaining) > 0 {
			m.state.RouteCursor = len(remaining) - 1
		}

		m.state.HasChanges = true
		return false
	}
	return m, nil
}

func (m *WizardModel) checkPortAvailability() tea.Cmd {
	m.state.PortStatus = ""
	host := strings.TrimSpace(m.state.ServerHost)
	portStr := strings.TrimSpace(m.state.ServerPort)
	if host != "localhost" && host != "127.0.0.1" {
		return nil
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil
	}
	m.state.PortTesting = true
	return func() tea.Msg {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return portTestDoneMsg{status: "Port is already in use"}
		}
		listener.Close()
		return portTestDoneMsg{status: "Port availability test PASS"}
	}
}

func (m *WizardModel) handleServerSave() (tea.Model, tea.Cmd) {
	host := strings.TrimSpace(m.state.ServerHost)
	portStr := strings.TrimSpace(m.state.ServerPort)

	if host == "" {
		m.state.ErrorMessage = "Host cannot be empty"
		return m, nil
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1024 || port > 65535 {
		m.state.ErrorMessage = "Port must be between 1024 and 65535"
		return m, nil
	}

	m.state.Config.Server.Host = host
	m.state.Config.Server.Port = port
	m.state.HasChanges = true
	m.state.CurrentScreen = ScreenMainMenu
	m.state.ErrorMessage = ""
	return m, nil
}

func (m *WizardModel) handleLoggingSave() (tea.Model, tea.Cmd) {
	m.state.Config.Logging.Enabled = m.state.LoggingEnabled
	m.state.Config.Logging.Level = m.state.LoggingLevel
	m.state.Config.Logging.Destination = m.state.LoggingDestination
	m.state.Config.Logging.FilePath = m.state.LoggingFilePath
	m.state.HasChanges = true
	m.state.CurrentScreen = ScreenMainMenu
	m.state.ErrorMessage = ""
	return m, nil
}

func (m *WizardModel) handleAddProvider1Enter() (tea.Model, tea.Cmd) {
	// Validate input
	name := strings.TrimSpace(m.state.NewProviderName)
	baseURL := strings.TrimSpace(m.state.NewProviderBaseURL)
	models := strings.TrimSpace(m.state.NewProviderModels)

	if name == "" {
		m.state.ErrorMessage = "Provider name is required"
		return m, nil
	}
	if baseURL == "" {
		m.state.ErrorMessage = "Base URL is required"
		return m, nil
	}
	if models == "" {
		m.state.ErrorMessage = "At least one model is required"
		return m, nil
	}

	// Parse models (one per line)
	modelList := strings.Split(models, "\n")
	var validModels []string
	for _, m := range modelList {
		m = strings.TrimSpace(m)
		if m != "" {
			validModels = append(validModels, m)
		}
	}

	if len(validModels) == 0 {
		m.state.ErrorMessage = "At least one model is required"
		return m, nil
	}

	m.state.NewProviderModels = strings.Join(validModels, "\n")
	if !m.state.EditingProvider {
		m.state.NewProviderAPIKey = "" // Clear for new providers to prevent stale data
	}
	m.state.CurrentScreen = ScreenAddProvider2
	m.state.ErrorMessage = ""
	return m, nil
}

func (m *WizardModel) handleAddProvider2Enter() (tea.Model, tea.Cmd) {
	// Save the provider
	providerName := strings.TrimSpace(m.state.NewProviderName)
	apiKey := m.state.NewProviderAPIKey

	if apiKey == "" {
		m.state.ErrorMessage = "API key is required"
		return m, nil
	}

	// Generate env var name
	envVarName := GenerateEnvVarName(providerName)

	// Create provider config
	m.state.Config.Providers[providerName] = config.ProviderConfig{
		APIKey:      "${" + envVarName + "}",
		BaseURL:     strings.TrimSpace(m.state.NewProviderBaseURL),
		Transformer: strings.TrimSpace(m.state.NewProviderTransformer),
		Models:      strings.Split(strings.TrimSpace(m.state.NewProviderModels), "\n"),
	}

	m.state.HasChanges = true

	// Track resolved API key
	if m.state.ResolvedAPIKeys == nil {
		m.state.ResolvedAPIKeys = make(map[string]string)
	}
	m.state.ResolvedAPIKeys[providerName] = apiKey

	// Shell integration is deferred to "Save & Exit" (SyncAllShellExports/SourceAllNow)

	// Reset state and go back to providers
	m.resetAddProviderState()
	m.state.CurrentScreen = ScreenProviders
	m.state.ErrorMessage = ""
	return m, nil
}

func (m *WizardModel) handleEditRouteEnter() (tea.Model, tea.Cmd) {
	// When route name field is focused (field 0), handle dropdown
	if m.focusedField == 0 {
		if m.state.ShowRouteNameDropdown {
			// Select the highlighted route name and close dropdown
			matches := m.getRouteNameDropdownList()
			if m.state.RouteNameDropdownCursor < len(matches) {
				m.state.EditRouteName = matches[m.state.RouteNameDropdownCursor]
			}
			m.state.ShowRouteNameDropdown = false
			m.state.RouteNameDropdownCursor = 0
			return m, nil
		}
		// Dropdown not open — open it
		m.state.ShowRouteNameDropdown = true
		m.state.RouteNameDropdownCursor = 0
		return m, nil
	}

	// When chain list is focused (field 1), handle dropdown selection
	if m.focusedField == 1 {
		// If provider dropdown is open, select provider and open model dropdown
		if m.state.ShowDropdown {
			providers := m.getChainProviderList()
			if m.state.DropdownCursor < len(providers) {
				selectedProvider := providers[m.state.DropdownCursor]
				cursor := m.state.EditRouteChainCursor
				if cursor < len(m.state.EditRouteChain) {
					m.state.EditRouteChain[cursor].Provider = selectedProvider
					m.state.EditRouteChain[cursor].Model = ""
				}
				m.state.ShowDropdown = false
				m.state.DropdownCursor = 0
				// Open model dropdown for selected provider
				m.state.ShowModelDropdown = true
				m.state.ModelDropdownCursor = 0
			}
			return m, nil
		}
		// If model dropdown is open, select model and close
		if m.state.ShowModelDropdown {
			models := m.getChainModelList()
			if m.state.ModelDropdownCursor < len(models) {
				selectedModel := models[m.state.ModelDropdownCursor]
				cursor := m.state.EditRouteChainCursor
				if cursor < len(m.state.EditRouteChain) {
					m.state.EditRouteChain[cursor].Model = selectedModel
				}
			}
			m.state.ShowModelDropdown = false
			m.state.ModelDropdownCursor = 0
			return m, nil
		}
		// No dropdown open — save the route
		return m.saveRouteFromEdit()

	}

	// focusedField == 0: save the route (fallback — combobox Enter now opens dropdown)
	return m.saveRouteFromEdit()
}

// saveRouteFromEdit validates and saves the current route being edited.
func (m *WizardModel) saveRouteFromEdit() (tea.Model, tea.Cmd) {
	routeName := strings.TrimSpace(m.state.EditRouteName)
	if routeName == "" {
		m.state.ErrorMessage = "Route name is required"
		return m, nil
	}

	var chainParts []string
	for _, target := range m.state.EditRouteChain {
		if target.Provider != "" && target.Model != "" {
			chainParts = append(chainParts, fmt.Sprintf("%s:%s", target.Provider, target.Model))
		}
	}

	if len(chainParts) == 0 {
		m.state.ErrorMessage = "At least one provider:model is required"
		return m, nil
	}

	m.state.Config.Router.Routes[routeName] = strings.Join(chainParts, ";")
	m.state.HasChanges = true
	m.state.CurrentScreen = ScreenRoutes
	m.state.ErrorMessage = ""
	return m, nil
}

func (m *WizardModel) handleFormInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle input based on focused field
	switch m.state.CurrentScreen {
	case ScreenAddProvider1:
		switch m.focusedField {
		case 0: // Provider name
			if msg.String() == "backspace" && len(m.state.NewProviderName) > 0 {
				m.state.NewProviderName = m.state.NewProviderName[:len(m.state.NewProviderName)-1]
				m.state.ShowDropdown = true
				m.state.DropdownCursor = 0
			} else if msg.Paste {
				m.state.NewProviderName += string(msg.Runes)
				m.state.ShowDropdown = true
				m.state.DropdownCursor = 0
			} else if len(msg.String()) == 1 {
				m.state.NewProviderName += msg.String()
				m.state.ShowDropdown = true
				m.state.DropdownCursor = 0
			}
		case 1: // Base URL
			if msg.String() == "backspace" && len(m.state.NewProviderBaseURL) > 0 {
				m.state.NewProviderBaseURL = m.state.NewProviderBaseURL[:len(m.state.NewProviderBaseURL)-1]
			} else if msg.Paste {
				m.state.NewProviderBaseURL += string(msg.Runes)
			} else if len(msg.String()) == 1 {
				m.state.NewProviderBaseURL += msg.String()
			}
		case 2: // Models (textarea)
			removedNewline := false
			if msg.String() == "backspace" && len(m.state.NewProviderModels) > 0 {
				if m.state.NewProviderModels[len(m.state.NewProviderModels)-1] == '\n' {
					removedNewline = true
				}
				m.state.NewProviderModels = m.state.NewProviderModels[:len(m.state.NewProviderModels)-1]
			} else if msg.Paste {
				m.state.NewProviderModels += string(msg.Runes)
			} else if len(msg.String()) == 1 {
				m.state.NewProviderModels += msg.String()
			}
			if !removedNewline {
				m.state.ShowModelDropdown = true
				m.state.ModelDropdownCursor = 0
			}
		}

	case ScreenAddProvider2:
		if msg.String() == "backspace" && len(m.state.NewProviderAPIKey) > 0 {
			m.state.NewProviderAPIKey = m.state.NewProviderAPIKey[:len(m.state.NewProviderAPIKey)-1]
		} else if msg.Paste {
			m.state.NewProviderAPIKey += string(msg.Runes)
		} else if len(msg.String()) == 1 {
			m.state.NewProviderAPIKey += msg.String()
		}
	}
	return m, nil
}

func (m *WizardModel) handleServerInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.focusedField {
	case 0: // Host
		if msg.String() == "backspace" && len(m.state.ServerHost) > 0 {
			m.state.ServerHost = m.state.ServerHost[:len(m.state.ServerHost)-1]
		} else if msg.Paste {
			m.state.ServerHost += string(msg.Runes)
		} else if len(msg.String()) == 1 {
			m.state.ServerHost += msg.String()
		}
	case 1: // Port
		if msg.String() == "backspace" && len(m.state.ServerPort) > 0 {
			m.state.ServerPort = m.state.ServerPort[:len(m.state.ServerPort)-1]
		} else if msg.Paste {
			for _, r := range msg.Runes {
				if r >= '0' && r <= '9' {
					m.state.ServerPort += string(r)
				}
			}
		} else if len(msg.String()) == 1 && msg.String() >= "0" && msg.String() <= "9" {
			m.state.ServerPort += msg.String()
		}
		return m, m.checkPortAvailability()
	}
	return m, nil
}

func (m *WizardModel) handleLoggingInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.focusedField {
	case 0: // Toggle enabled
		if msg.String() == " " {
			m.state.LoggingEnabled = !m.state.LoggingEnabled
		}
	case 1: // Level - dropdown handled by handleNavigation/handleEnter
	case 2: // Destination - dropdown handled by handleNavigation/handleEnter
	}
	return m, nil
}

func (m *WizardModel) handleRouteEditInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.focusedField == 0 {
		// Route name input
		if msg.String() == "backspace" && len(m.state.EditRouteName) > 0 {
			m.state.EditRouteName = m.state.EditRouteName[:len(m.state.EditRouteName)-1]
		} else if msg.Paste {
			m.state.EditRouteName += string(msg.Runes)
		} else if len(msg.String()) == 1 {
			m.state.EditRouteName += msg.String()
		}
		// If dropdown is open, filter and reset cursor
		if m.state.ShowRouteNameDropdown {
			m.state.RouteNameDropdownCursor = 0
		}
	}
	// focusedField == 1 (chain list): handled by character key cases for 'a' and backspace
	return m, nil
}

func (m *WizardModel) cycleLoggingLevel(delta int) {
	levels := LogLevelOptions
	currentIdx := 0
	for i, l := range levels {
		if l == m.state.LoggingLevel {
			currentIdx = i
			break
		}
	}
	newIdx := (currentIdx + delta + len(levels)) % len(levels)
	m.state.LoggingLevel = levels[newIdx]
}

func (m *WizardModel) cycleLoggingDestination(delta int) {
	dests := LogDestinationOptions
	currentIdx := 0
	for i, d := range dests {
		if d == m.state.LoggingDestination {
			currentIdx = i
			break
		}
	}
	newIdx := (currentIdx + delta + len(dests)) % len(dests)
	m.state.LoggingDestination = dests[newIdx]
}

func (m *WizardModel) getMaxFields() int {
	switch m.state.CurrentScreen {
	case ScreenAddProvider1:
		return 3
	case ScreenAddProvider2:
		return 1
	case ScreenServer:
		return 2
	case ScreenLogging:
		return 3
	case ScreenEditRoute:
		return 2
	default:
		return 0
	}
}

// Helper methods

func (m *WizardModel) getProviderList() []string {
	providers := make([]string, 0, len(m.state.Config.Providers))
	for name := range m.state.Config.Providers {
		providers = append(providers, name)
	}
	sort.Strings(providers)
	return providers
}

func (m *WizardModel) getRouteList() []string {
	routes := make([]string, 0, len(m.state.Config.Router.Routes))
	for name := range m.state.Config.Router.Routes {
		routes = append(routes, name)
	}
	sort.Strings(routes)
	return routes
}

// getChainProviderList returns configured provider names for the chain dropdown.
func (m *WizardModel) getChainProviderList() []string {
	return m.getProviderList()
}

// getChainModelList returns models for the currently selected chain item's provider.
func (m *WizardModel) getChainModelList() []string {
	if m.state.EditRouteChainCursor >= len(m.state.EditRouteChain) {
		return nil
	}
	providerName := m.state.EditRouteChain[m.state.EditRouteChainCursor].Provider
	if providerName == "" {
		return nil
	}
	// Check config providers first
	if p, ok := m.state.Config.Providers[providerName]; ok {
		models := make([]string, len(p.Models))
		copy(models, p.Models)
		sort.Strings(models)
		return models
	}
	// Fall back to preset models
	if preset, ok := ProviderPresets[providerName]; ok {
		models := make([]string, len(preset.Models))
		copy(models, preset.Models)
		sort.Strings(models)
		return models
	}
	return nil
}

// getRouteNameDropdownList returns predefined route names filtered by the current input.
// If EditRouteName is empty, all predefined names are returned.
func (m *WizardModel) getRouteNameDropdownList() []string {
	input := strings.ToLower(m.state.EditRouteName)
	var result []string
	for _, name := range PredefinedRouteNames {
		if input == "" || strings.HasPrefix(strings.ToLower(name), input) {
			result = append(result, name)
		}
	}
	return result
}

// isValidProviderModel checks if a provider:model pair exists in the config.
// Empty provider or model strings return false (placeholder entries are not valid).
func (m *WizardModel) isValidProviderModel(provider, model string) bool {
	if provider == "" || model == "" {
		return false
	}
	p, ok := m.state.Config.Providers[provider]
	if !ok {
		return false
	}
	for _, m := range p.Models {
		if m == model {
			return true
		}
	}
	return false
}

// renderChainStyled renders a route chain string with styling based on selection and validity.
// selected indicates whether the overall row is selected (affects background).
func (m *WizardModel) renderChainStyled(chain string, width int, selected bool) string {
	targets := config.ParseRoute(chain)
	if len(targets) == 0 {
		if selected {
			return ListItemSelectedStyle.Width(width).Render("")
		}
		return ListItemStyle.Width(width).Render("")
	}

	// Build plain text first (no ANSI codes)
	parts := make([]string, len(targets))
	for i, t := range targets {
		parts[i] = t.Provider + ":" + t.Model
	}
	plainText := strings.Join(parts, ";")

	// Truncate plain text (no ANSI codes to corrupt width calculation)
	truncatedText := truncate(plainText, width)

	// Determine style: if any target is invalid, use invalid style
	hasInvalid := false
	for _, t := range targets {
		if !m.isValidProviderModel(t.Provider, t.Model) {
			hasInvalid = true
			break
		}
	}

	var style lipgloss.Style
	switch {
	case selected && hasInvalid:
		style = ListItemInvalidSelectedStyle
	case selected:
		style = ListItemSelectedStyle
	case hasInvalid:
		style = ListItemInvalidStyle
	default:
		style = ListItemStyle
	}

	return style.Width(width).Render(truncatedText)
}

func (m *WizardModel) resetAddProviderState() {
	m.state.NewProviderName = ""
	m.state.NewProviderBaseURL = ""
	m.state.NewProviderTransformer = "anthropic"
	m.state.NewProviderModels = ""
	m.state.NewProviderAPIKey = ""
	m.state.AddToShellConfig = true
	m.state.SourceImmediately = true
	m.state.ProviderPreset = "anthropic"
	m.state.EditingProvider = false
	m.state.ShowDropdown = false
	m.state.DropdownCursor = 0
	m.state.ShowModelDropdown = false
	m.state.ModelDropdownCursor = 0
	m.focusedField = 0
}

func (m *WizardModel) saveConfig() error {
	return config.Save(m.state.Config, m.state.ConfigPath)
}

func (m *WizardModel) exportConfig() error {
	data, err := json.MarshalIndent(m.state.Config, "", "  ")
	if err != nil {
		return err
	}
	exportPath := m.state.ConfigPath + ".export"
	return os.WriteFile(exportPath, data, 0644)
}

// View renders the wizard UI.
func (m *WizardModel) View() string {
	// Ensure minimum dimensions
	if m.width < 64 {
		m.width = 64
	}
	if m.height < 20 {
		m.height = 20
	}

	// Render based on current screen
	switch m.state.CurrentScreen {
	case ScreenMainMenu:
		return m.renderMainMenu()
	case ScreenProviders:
		return m.renderProviders()
	case ScreenAddProvider1:
		return m.renderAddProvider1()
	case ScreenAddProvider2:
		return m.renderAddProvider2()
	case ScreenRoutes:
		return m.renderRoutes()
	case ScreenEditRoute:
		return m.renderEditRoute()
	case ScreenServer:
		return m.renderServer()
	case ScreenLogging:
		return m.renderLogging()
	case ScreenViewConfig:
		return m.renderViewConfig()
	default:
		return m.renderMainMenu()
	}
}

// View with modal overlay
func (m *WizardModel) renderWithModal(content string) string {
	if m.state.ShowConfirm {
		modal := m.renderConfirmModal()
		return lipgloss.Place(
			m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			modal,
			lipgloss.WithWhitespaceBackground(PanelBackground),
			lipgloss.WithWhitespaceChars(" "),
		)
	}
	return content
}

// renderConfirmModal renders the confirmation modal.
func (m *WizardModel) renderConfirmModal() string {
	modalWidth := 50
	modalHeight := 9
	contentWidth := modalWidth - 4 // 46: fills content area after padding

	// Override modal alignment to Left — prevents modal-level centering
	modal := ModalStyle.Width(modalWidth).Height(modalHeight).Align(lipgloss.Left)

	yesBtn := ButtonStyle.Render(" Yes ")
	noBtn := ButtonStyle.Render(" No ")
	if m.state.ConfirmCursor == 0 {
		yesBtn = ButtonActiveStyle.Render(" Yes ")
	} else {
		noBtn = ButtonActiveStyle.Render(" No ")
	}

	// Title: left-aligned, fills full content width
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(HeaderText).
		Width(contentWidth).
		Align(lipgloss.Left).
		Render(m.state.ConfirmMessage)

	// Buttons row: centered within content width
	buttonsRow := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render(lipgloss.JoinHorizontal(lipgloss.Center, yesBtn, noBtn))

	// Help row: centered within content width
	helpRow := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render(HelpTextStyle.Render("[←/→] Choose   [Enter] Confirm   [Esc] Cancel"))

	// Push content to top, help to bottom — modal content area is ~5 lines (9 height - 2 padding - 2 border)
	spacer := lipgloss.NewStyle().Width(contentWidth).Render("")
	content := lipgloss.JoinVertical(lipgloss.Left, title, spacer, buttonsRow, spacer, helpRow)

	return modal.Render(content)
}

// Main menu rendering
func (m *WizardModel) renderMainMenu() string {
	providerCount := len(m.state.Config.Providers)
	routeCount := len(m.state.Config.Router.Routes)
	_ = routeCount
	serverInfo := fmt.Sprintf("%s:%d", m.state.Config.Server.Host, m.state.Config.Server.Port)
	logLevel := m.state.Config.Logging.Level
	if logLevel == "" {
		logLevel = "info"
	}
	logDest := m.state.Config.Logging.Destination
	if logDest == "" {
		logDest = "stdout"
	}

	menuItems := []struct {
		label   string
		info    string
		cursor  int
	}{
		{"[1] Providers", fmt.Sprintf("Manage API providers (%d configured)", providerCount), 0},
		{"[2] Routes", "Configure routing rules", 1},
		{"[3] Proxy", fmt.Sprintf("Host: %s", serverInfo), 2},
		{"[4] Logging", fmt.Sprintf("Level: %s, Destination: %s", logLevel, logDest), 3},
		{"[5] View Config", "Browse current configuration", 4},
		{"[6] Save & Exit", "Write changes to disk", 5},
		{"[7] Quit without saving", "Exit without saving changes", 6},
	}

	var menuLines []string
	const labelColumnWidth = 28 // Fixed width for menu label column

	for _, item := range menuItems {
		// Render label with fixed width to prevent word wrap
		var labelStr string
		if item.cursor == m.state.ProviderCursor {
			labelStr = MenuItemSelectedStyle.Width(labelColumnWidth).Render(item.label)
		} else {
			labelStr = MenuItemStyle.Width(labelColumnWidth).Render(item.label)
		}

		// Always render both label and info for consistent spacing
		infoWidth := m.contentWidth() - labelColumnWidth
		if infoWidth < 10 {
			infoWidth = 10
		}

		var infoStr string
		if item.cursor == m.state.ProviderCursor {
			// Selected: show actual info
			infoStr = MenuItemDescriptionStyle.Width(infoWidth).Render(item.info)
		} else {
			// Unselected: show empty placeholder to maintain consistent spacing
			infoStr = MenuItemDimmedStyle.Width(infoWidth).Render("")
		}
		line := lipgloss.JoinHorizontal(lipgloss.Top, labelStr, infoStr)
		menuLines = append(menuLines, line)
	}

	title := TitleStyle.Width(m.contentWidth()).Render("Configuration Wizard")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		m.blankLine(),
		lipgloss.JoinVertical(lipgloss.Left, menuLines...),
		m.blankLine(),
		HelpTextStyle.Width(m.contentWidth()).Render("[↑/↓] Navigate   [Enter] Select"),
	)

	if m.state.HasUnsavedChanges() {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			m.blankLine(),
			ErrorStyle.Width(m.contentWidth()).Render("⚠ Unsaved changes"),
		)
	}

	mainBox := MainContainerStyle.Width(m.width - 2).Render(content)
	return m.renderWithModal(mainBox)
}

// Providers screen rendering
func (m *WizardModel) renderProviders() string {
	title := SectionHeaderStyle.Width(m.contentWidth()).Render("Providers")
	var providerLines []string
	providers := m.getProviderList()

	for i, name := range providers {
		pc := m.state.Config.Providers[name]
		models := strings.Join(pc.Models, ", ")

		var line string
		if i == m.state.ProviderCursor {
			line = ListItemSelectedStyle.Width(m.contentWidth()).Render(fmt.Sprintf("▶ %s", name))
		} else {
			line = ListItemStyle.Width(m.contentWidth()).Render(fmt.Sprintf("  %s", name))
		}
		providerLines = append(providerLines, line)
		providerLines = append(providerLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("   ├─ "+pc.BaseURL))
		providerLines = append(providerLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("   └─ "+models))
	}

	if len(providers) == 0 {
		providerLines = append(providerLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("No providers configured"))
	}

	providerLines = append(providerLines, m.blankLine())
	providerLines = append(providerLines, HelpTextStyle.Width(m.contentWidth()).Render("[↑/↓] Navigate   [Enter] Edit   [a] Add   [T] Test   [⌫] Delete   [Esc] Back"))

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.fullWidth(title),
		m.blankLine(),
		lipgloss.JoinVertical(lipgloss.Left, providerLines...),
	)

	if m.state.ErrorMessage != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			m.blankLine(),
			ErrorStyle.Width(m.contentWidth()).Render(m.state.ErrorMessage),
		)
	}

	mainBox := MainContainerStyle.Width(m.width - 2).Render(content)
	return m.renderWithModal(mainBox)
}

// getPresetMatches returns preset names matching the current provider name input.
func (m *WizardModel) getPresetMatches() []string {
	input := strings.ToLower(m.state.NewProviderName)
	var matches []string
	for name := range ProviderPresets {
		if _, exists := m.state.Config.Providers[name]; exists {
			continue
		}
		if input == "" || strings.HasPrefix(strings.ToLower(name), input) {
			matches = append(matches, name)
		}
	}
	sort.Strings(matches)
	if len(matches) > 4 {
		matches = matches[:4]
	}
	return matches
}

// applyPreset fills in provider fields from a preset and hides the dropdown.
func (m *WizardModel) applyPreset(name string) {
	if preset, ok := ProviderPresets[name]; ok {
		m.state.NewProviderName = name
		m.state.NewProviderBaseURL = preset.BaseURL
		m.state.NewProviderTransformer = preset.Transformer
		// Don't auto-populate models — users can use the suggestions dropdown
	}
	m.state.ShowDropdown = false
	m.state.DropdownCursor = 0
}

// getModelSuggestions returns model names matching the current provider preset
// filtered by the current line prefix in the models field.
func (m *WizardModel) getModelSuggestions() []string {
	// Find the preset matching the current provider name
	providerName := strings.TrimSpace(strings.ToLower(m.state.NewProviderName))
	var models []string
	for key, preset := range ProviderPresets {
		if strings.ToLower(key) == providerName {
			models = preset.Models
			break
		}
	}
	if len(models) == 0 {
		return nil
	}

	// Get the current line (text after last newline) for prefix filtering
	text := m.state.NewProviderModels
	currentLine := text
	if idx := strings.LastIndex(text, "\n"); idx >= 0 {
		currentLine = text[idx+1:]
	}
	prefix := strings.ToLower(currentLine)

	// Filter models by prefix
	var matches []string
	for _, model := range models {
		if prefix == "" || strings.HasPrefix(strings.ToLower(model), prefix) {
			matches = append(matches, model)
		}
	}

	// Exclude models already present in the text (full lines)
	existingLines := make(map[string]bool)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			existingLines[strings.ToLower(line)] = true
		}
	}

	var filtered []string
	for _, match := range matches {
		if !existingLines[strings.ToLower(match)] {
			filtered = append(filtered, match)
		}
	}

	if len(filtered) > 6 {
		filtered = filtered[:6]
	}
	return filtered
}

// insertModelFromDropdown replaces the current line in the models field with the
// selected model and appends a newline, then closes the model dropdown.
func (m *WizardModel) insertModelFromDropdown(model string) {
	text := m.state.NewProviderModels
	// Find the current line (text after last newline)
	var prefix string
	if idx := strings.LastIndex(text, "\n"); idx >= 0 {
		prefix = text[:idx+1]
	}
	// Replace current line with the selected model and add a newline
	m.state.NewProviderModels = prefix + model + "\n"
	// Keep dropdown open for next model suggestion (reset cursor to top)
	m.state.ShowModelDropdown = true
	m.state.ModelDropdownCursor = 0
}

// Add Provider Step 1 rendering
func (m *WizardModel) renderAddProvider1() string {
	titleText := "Add Provider (1/2)"
	if m.state.EditingProvider {
		titleText = "Edit Provider (1/2)"
	}
	title := SectionHeaderStyle.Width(m.contentWidth()).Render(titleText)

	// Input fields
	nameLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Provider Name:")
	nameInput := m.state.NewProviderName
	if m.focusedField == 0 {
		nameInput = InputFieldFocusedStyle.Width(m.inputFieldWidth()).Render(nameInput + "_")
	} else {
		nameInput = InputFieldStyle.Width(m.inputFieldWidth()).Render(nameInput)
	}

	urlLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Base URL:")
	urlInput := m.state.NewProviderBaseURL
	if m.focusedField == 1 {
		urlInput = InputFieldFocusedStyle.Width(m.inputFieldWidth()).Render(urlInput + "_")
	} else {
		urlInput = InputFieldStyle.Width(m.inputFieldWidth()).Render(urlInput)
	}

	modelsLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Models (one per line):")
	modelsInput := m.state.NewProviderModels
	if m.focusedField == 2 {
		modelsInput = InputFieldFocusedStyle.Width(m.inputFieldWidth()).Height(1).Render(modelsInput + "_")
	} else {
		modelsInput = InputFieldStyle.Width(m.inputFieldWidth()).Height(1).Render(modelsInput)
	}

	// Build content — always show all fields, dropdown inserted inline when visible
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title, m.blankLine(),
		nameLabel, m.fullWidth(nameInput),
	)

	// Insert dropdown between name field and URL field when visible
	if m.state.ShowDropdown && m.focusedField == 0 {
		matches := m.getPresetMatches()
		var dropdownItems []string
		dropdownContentWidth := m.inputFieldWidth() - DropdownStyle.GetHorizontalFrameSize()
		for i, match := range matches {
			if i == m.state.DropdownCursor {
				dropdownItems = append(dropdownItems, ListItemSelectedStyle.Width(dropdownContentWidth).Render(match))
			} else {
				dropdownItems = append(dropdownItems, ListItemStyle.Width(dropdownContentWidth).Render(match))
			}
		}
		dropdown := DropdownStyle.Width(m.inputFieldWidth()).Render(
			lipgloss.JoinVertical(lipgloss.Left, dropdownItems...),
		)
		content = lipgloss.JoinVertical(lipgloss.Left, content, dropdown)
	}

	// Append remaining fields
	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		urlLabel, m.fullWidth(urlInput),
		modelsLabel, m.fullWidth(modelsInput),
	)

	// Insert model dropdown below models field when visible
	if m.state.ShowModelDropdown && m.focusedField == 2 {
		modelMatches := m.getModelSuggestions()
		if len(modelMatches) > 0 {
			var dropdownItems []string
			dropdownContentWidth := m.inputFieldWidth() - DropdownStyle.GetHorizontalFrameSize()
			for i, match := range modelMatches {
				if i == m.state.ModelDropdownCursor {
					dropdownItems = append(dropdownItems, ListItemSelectedStyle.Width(dropdownContentWidth).Render(match))
				} else {
					dropdownItems = append(dropdownItems, ListItemStyle.Width(dropdownContentWidth).Render(match))
				}
			}
			dropdown := DropdownStyle.Width(m.inputFieldWidth()).Render(
				lipgloss.JoinVertical(lipgloss.Left, dropdownItems...),
			)
			content = lipgloss.JoinVertical(lipgloss.Left, content, dropdown)
		}
	}

	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		m.blankLine(),
	)

	// Append help text based on dropdown state
	if m.state.ShowDropdown && m.focusedField == 0 {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			HelpTextStyle.Width(m.contentWidth()).Render("[↑/↓] Select preset   [Enter] Apply   [Esc] Close"),
		)
	} else if m.state.ShowModelDropdown && m.focusedField == 2 {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			HelpTextStyle.Width(m.contentWidth()).Render("[↑/↓] Select model   [Enter] Insert   [Esc] Close"),
		)
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			HelpTextStyle.Width(m.contentWidth()).Render("[Esc] Cancel   [Tab] Next field   [Enter] Next →"),
		)
	}

	if m.state.ErrorMessage != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			m.blankLine(),
			ErrorStyle.Width(m.contentWidth()).Render(m.state.ErrorMessage),
		)
	}

	mainBox := MainContainerStyle.Width(m.width - 2).Render(content)
	return m.renderWithModal(mainBox)
}

// Add Provider Step 2 rendering
func (m *WizardModel) renderAddProvider2() string {
	title := SectionHeaderStyle.Width(m.contentWidth()).Render("Environment Setup (2/2)")

	// Input field
	apiKeyLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Enter API Key:")
	var maskedKey string
	if strings.HasPrefix(m.state.NewProviderAPIKey, "${") && strings.HasSuffix(m.state.NewProviderAPIKey, "}") {
		maskedKey = ""
	} else {
		maskedKey = strings.Repeat("*", len(m.state.NewProviderAPIKey))
	}
	if maskedKey == "" {
		maskedKey = "________________________________"
	}
	apiKeyInput := InputFieldFocusedStyle.Width(m.inputFieldWidth()).Render(maskedKey)

	// Checkboxes (read-only indicators — shell integration happens on "Save & Exit")
	addToShell := CheckboxCheckedStyle.Render()
	sourceNow := CheckboxCheckedStyle.Render()

	// Preview
	preview := GetExportPreview(m.state.NewProviderName, m.state.NewProviderAPIKey)

	// Buttons
	backBtn := ButtonStyle.Render("[← Back]")
	saveBtn := ButtonPrimaryStyle.Render("[Save Provider]")

	// Side-by-side: Shell Configuration | Preview
	shellWidth := m.contentWidth() * 55 / 100
	previewWidth := m.contentWidth() - shellWidth - 2 // -2 for gap
	leftCol := lipgloss.JoinVertical(
		lipgloss.Left,
		MenuItemDimmedStyle.Width(shellWidth).Render("Shell Configuration:"),
		lipgloss.JoinHorizontal(lipgloss.Left, addToShell, " Add to shell config (~/.zshrc)"),
		lipgloss.JoinHorizontal(lipgloss.Left, sourceNow, " Source environment now"),
	)
	rightCol := lipgloss.JoinVertical(
		lipgloss.Left,
		MenuItemDimmedStyle.Width(previewWidth).Render("Preview:"),
		InputFieldStyle.Width(previewWidth).Height(2).Render(preview),
	)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.fullWidth(title),
		m.blankLine(),
		MenuItemDimmedStyle.Width(m.contentWidth()).Render(fmt.Sprintf("Provider: %s", m.state.NewProviderName)),
		m.blankLine(),
		apiKeyLabel,
		m.fullWidth(apiKeyInput),
		m.blankLine(),
		lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "  ", rightCol),
		m.blankLine(),
		m.fullWidth(lipgloss.JoinHorizontal(lipgloss.Center, backBtn, saveBtn)),
		m.blankLine(),
		m.renderAddProvider2Hints(),
	)

	mainBox := MainContainerStyle.Width(m.width - 2).Render(content)
	return m.renderWithModal(mainBox)
}

// renderAddProvider2Hints builds the hints bar for Add Provider step 2.
// When an error is present, it is displayed right-aligned in the hints bar.
func (m *WizardModel) renderAddProvider2Hints() string {
	const hintsText = "[Esc] Back   [Enter] Save   [⌘V/Ctrl+V] Paste"
	if m.state.ErrorMessage == "" {
		return HelpTextStyle.Width(m.contentWidth()).Render(hintsText)
	}
	hintsLeft := HelpTextStyle.Render(hintsText)
	errorHint := ErrorStyle.Render(m.state.ErrorMessage)
	return lipgloss.NewStyle().Width(m.contentWidth()).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, hintsLeft,
			lipgloss.NewStyle().Width(m.contentWidth()-runewidth.StringWidth(hintsText)-HelpTextStyle.GetHorizontalFrameSize()).Align(lipgloss.Right).Render(errorHint)),
	)
}

// Routes screen rendering
func (m *WizardModel) renderRoutes() string {
	title := SectionHeaderStyle.Width(m.contentWidth()).Render("Routes")

	// Table header
	headerRow := lipgloss.JoinHorizontal(
		lipgloss.Left,
		SectionHeaderStyle.Width(20).Render("Route"),
		SectionHeaderStyle.Width(m.contentWidth() - 20).Render("Chain"),
	)

	var routeLines []string
	routes := m.getRouteList()

	for i, name := range routes {
		chain := m.state.Config.Router.Routes[name]
		selected := i == m.state.RouteCursor

		var line string
		if selected {
			line = lipgloss.JoinHorizontal(
				lipgloss.Left,
				ListItemSelectedStyle.Width(20).Render(name),
				m.renderChainStyled(chain, m.contentWidth()-20, true),
			)
		} else {
			line = lipgloss.JoinHorizontal(
				lipgloss.Left,
				ListItemStyle.Width(20).Render(name),
				m.renderChainStyled(chain, m.contentWidth()-20, false),
			)
		}
		routeLines = append(routeLines, m.fullWidth(line))
	}

	if len(routes) == 0 {
		routeLines = append(routeLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("No routes configured"))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.fullWidth(title),
		m.blankLine(),
		headerRow,
		m.divider(),
		lipgloss.JoinVertical(lipgloss.Left, routeLines...),
		m.blankLine(),
		HelpTextStyle.Width(m.contentWidth()).Render("[↑/↓] Navigate   [Enter] Edit   [a] Add   [⌫] Delete   [Esc] Back"),
	)

	mainBox := MainContainerStyle.Width(m.width - 2).Render(content)
	return m.renderWithModal(mainBox)
}

// Edit Route rendering
func (m *WizardModel) renderEditRoute() string {
	title := SectionHeaderStyle.Width(m.contentWidth()).Render("Add/Edit Route")

	routeNameLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Route Name:")
	routeNameInput := m.state.EditRouteName
	if m.focusedField == 0 {
		indicator := " ▼"
		routeNameInput = InputFieldFocusedStyle.Width(m.inputFieldWidth()).Render(routeNameInput + "_" + indicator)
	} else {
		routeNameInput = InputFieldStyle.Width(m.inputFieldWidth()).Render(routeNameInput)
	}

	// Chain list
	var chainLines []string
	for i, target := range m.state.EditRouteChain {
		num := fmt.Sprintf("[%d]", i+1)
		display := fmt.Sprintf("%s:%s", target.Provider, target.Model)
		if target.Provider == "" {
			display = "(select provider)"
		} else if target.Model == "" {
			display = target.Provider + ": (select model)"
		}
		isSelected := m.focusedField == 1 && i == m.state.EditRouteChainCursor
		isInvalid := target.Provider != "" && target.Model != "" && !m.isValidProviderModel(target.Provider, target.Model)
		var style lipgloss.Style
		switch {
		case isSelected && isInvalid:
			style = ListItemInvalidSelectedStyle
		case isSelected:
			style = ListItemSelectedStyle
		case isInvalid:
			style = ListItemInvalidStyle
		default:
			style = ListItemStyle
		}
		item := lipgloss.JoinHorizontal(
			lipgloss.Left,
			style.Width(5).Render(num),
			style.Width(m.contentWidth() - 5).Render(display),
		)
		chainLines = append(chainLines, m.fullWidth(item))
	}

	if len(chainLines) == 0 {
		chainLines = append(chainLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("No providers in chain — press [a] to add"))
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		m.blankLine(),
		routeNameLabel,
		m.fullWidth(routeNameInput),
	)

	// Route name dropdown
	if m.state.ShowRouteNameDropdown && m.focusedField == 0 {
		routeNames := m.getRouteNameDropdownList()
		if len(routeNames) > 0 {
			var dropdownItems []string
			dropdownContentWidth := m.inputFieldWidth() - DropdownStyle.GetHorizontalFrameSize()
			for i, name := range routeNames {
				if i == m.state.RouteNameDropdownCursor {
					dropdownItems = append(dropdownItems, ListItemSelectedStyle.Width(dropdownContentWidth).Render(name))
				} else {
					dropdownItems = append(dropdownItems, ListItemStyle.Width(dropdownContentWidth).Render(name))
				}
			}
			dropdown := DropdownStyle.Width(m.inputFieldWidth()).Render(
				lipgloss.JoinVertical(lipgloss.Left, dropdownItems...),
			)
			content = lipgloss.JoinVertical(lipgloss.Left, content, dropdown)
		}
	}

	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		m.blankLine(),
		m.divider(),
		m.blankLine(),
		MenuItemDimmedStyle.Width(m.contentWidth()).Render("Failover Chain:"),
		lipgloss.JoinVertical(lipgloss.Left, chainLines...),
	)

	// Provider dropdown
	if m.state.ShowDropdown && m.focusedField == 1 {
		providers := m.getChainProviderList()
		if len(providers) > 0 {
			var dropdownItems []string
			dropdownContentWidth := m.inputFieldWidth() - DropdownStyle.GetHorizontalFrameSize()
			for i, p := range providers {
				if i == m.state.DropdownCursor {
					dropdownItems = append(dropdownItems, ListItemSelectedStyle.Width(dropdownContentWidth).Render(p))
				} else {
					dropdownItems = append(dropdownItems, ListItemStyle.Width(dropdownContentWidth).Render(p))
				}
			}
			dropdown := DropdownStyle.Width(m.inputFieldWidth()).Render(
				lipgloss.JoinVertical(lipgloss.Left, dropdownItems...),
			)
			content = lipgloss.JoinVertical(lipgloss.Left, content, dropdown)
		}
	}

	// Model dropdown
	if m.state.ShowModelDropdown && m.focusedField == 1 {
		models := m.getChainModelList()
		if len(models) > 0 {
			var dropdownItems []string
			dropdownContentWidth := m.inputFieldWidth() - DropdownStyle.GetHorizontalFrameSize()
			for i, model := range models {
				if i == m.state.ModelDropdownCursor {
					dropdownItems = append(dropdownItems, ListItemSelectedStyle.Width(dropdownContentWidth).Render(model))
				} else {
					dropdownItems = append(dropdownItems, ListItemStyle.Width(dropdownContentWidth).Render(model))
				}
			}
			dropdown := DropdownStyle.Width(m.inputFieldWidth()).Render(
				lipgloss.JoinVertical(lipgloss.Left, dropdownItems...),
			)
			content = lipgloss.JoinVertical(lipgloss.Left, content, dropdown)
		}
	}

	// Hints bar
	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		m.blankLine(),
	)

	if m.state.ShowRouteNameDropdown && m.focusedField == 0 {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			HelpTextStyle.Width(m.contentWidth()).Render("[↑/↓] Select   [Enter] Pick   [Esc] Close   [Type] Filter"),
		)
	} else if m.state.ShowDropdown && m.focusedField == 1 {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			HelpTextStyle.Width(m.contentWidth()).Render("[↑/↓] Select model   [Enter] Select   [Esc] Close"),
		)
	} else if m.focusedField == 1 {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			HelpTextStyle.Width(m.contentWidth()).Render("[Esc] Back   [Tab] Next   [Enter] Save   [a] Add   [⌫] Delete"),
		)
	} else if m.focusedField == 0 {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			HelpTextStyle.Width(m.contentWidth()).Render("[Esc] Back   [Tab] Next   [Enter] Show options"),
		)
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			HelpTextStyle.Width(m.contentWidth()).Render("[Esc] Back   [Tab] Next   [Enter] Save"),
		)
	}

	if m.state.ErrorMessage != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			m.blankLine(),
			ErrorStyle.Width(m.contentWidth()).Render(m.state.ErrorMessage),
		)
	}

	mainBox := MainContainerStyle.Width(m.width - 2).Render(content)
	return m.renderWithModal(mainBox)
}

// Server settings rendering
func (m *WizardModel) renderServer() string {
	title := SectionHeaderStyle.Width(m.contentWidth()).Render("Proxy Settings")

	hostLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Host:")
	hostInput := m.state.ServerHost
	if m.focusedField == 0 {
		hostInput = InputFieldFocusedStyle.Width(m.inputFieldWidth()).Render(hostInput + "_")
	} else {
		hostInput = InputFieldStyle.Width(m.inputFieldWidth()).Render(hostInput)
	}

	portLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Port:")
	portInput := m.state.ServerPort
	if m.focusedField == 1 {
		portInput = InputFieldFocusedStyle.Width(m.inputFieldWidth()).Render(portInput + "_")
	} else {
		portInput = InputFieldStyle.Width(m.inputFieldWidth()).Render(portInput)
	}

	note := MenuItemDimmedStyle.Render("Note: Must be between 1024-65535")
	if m.state.PortTesting {
		note = lipgloss.JoinHorizontal(lipgloss.Left, note, "  ", StatusPendingStyle.Render("Testing port..."))
	} else if m.state.PortStatus != "" {
		var statusMsg string
		if strings.Contains(m.state.PortStatus, "PASS") {
			statusMsg = StatusOKStyle.Render("✓ " + m.state.PortStatus)
		} else {
			statusMsg = WarningStyle.Render("⚠ " + m.state.PortStatus)
		}
		note = lipgloss.JoinHorizontal(lipgloss.Left, note, "  ", statusMsg)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		m.blankLine(),
		hostLabel, m.fullWidth(hostInput),
		m.blankLine(),
		portLabel, m.fullWidth(portInput),
		m.blankLine(),
		note,
		m.blankLine(),
		HelpTextStyle.Width(m.contentWidth()).Render("[Esc/Enter] Apply & Back   [Tab] Next field"),
	)

	if m.state.ErrorMessage != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			m.blankLine(),
			ErrorStyle.Width(m.contentWidth()).Render(m.state.ErrorMessage),
		)
	}

	mainBox := MainContainerStyle.Width(m.width - 2).Render(content)
	return m.renderWithModal(mainBox)
}

// Logging settings rendering
func (m *WizardModel) renderLogging() string {
	title := SectionHeaderStyle.Width(m.contentWidth()).Render("Logging Settings")

	// Enable Logging checkbox — use focused styles when field 0 is focused
	enabledCheckbox := CheckboxUncheckedStyle.Render()
	if m.state.LoggingEnabled {
		enabledCheckbox = CheckboxCheckedStyle.Render()
	}
	checkboxFocused := m.focusedField == 0
	if checkboxFocused {
		if m.state.LoggingEnabled {
			enabledCheckbox = CheckboxCheckedFocusedStyle.Render()
		} else {
			enabledCheckbox = CheckboxUncheckedFocusedStyle.Render()
		}
	}

	loggingDisabled := !m.state.LoggingEnabled

	// Level field — dimmed when logging is disabled
	levelLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Level:")
	levelValue := m.state.LoggingLevel
	if loggingDisabled {
		levelValue = InputFieldDisabledStyle.Width(m.inputFieldWidth()).Render(levelValue)
	} else if m.focusedField == 1 {
		levelValue = InputFieldFocusedStyle.Width(m.inputFieldWidth()).Render(levelValue + " ▾")
	} else {
		levelValue = InputFieldStyle.Width(m.inputFieldWidth()).Render(levelValue)
	}

	// Destination field — dimmed when logging is disabled
	destLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Destination:")
	destValue := m.state.LoggingDestination
	if loggingDisabled {
		destValue = InputFieldDisabledStyle.Width(m.inputFieldWidth()).Render(destValue)
	} else if m.focusedField == 2 {
		destValue = InputFieldFocusedStyle.Width(m.inputFieldWidth()).Render(destValue + " ▾")
	} else {
		destValue = InputFieldStyle.Width(m.inputFieldWidth()).Render(destValue)
	}

	// File path display — show both log locations
	fileLabel := MenuItemDimmedStyle.Width(m.contentWidth()).Render("Log Files:")
	defaultLogPath := m.state.LoggingFilePath
	if defaultLogPath == "" {
		defaultLogPath = "~/.cc-modelrouter/router.log"
	}
	instanceLogPath := "~/.cc-modelrouter/logs/inst_<timestamp>.log"

	// Build base content
	checkboxRow := lipgloss.JoinHorizontal(lipgloss.Left, enabledCheckbox, " Enable Logging")
	if checkboxFocused {
		checkboxRow = FocusedRowStyle.Width(m.contentWidth()).Render(checkboxRow)
	} else {
		checkboxRow = lipgloss.NewStyle().Padding(0, 1).Width(m.contentWidth()).Render(checkboxRow)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		m.blankLine(),
		checkboxRow,
		m.blankLine(),
		levelLabel,
		m.fullWidth(levelValue),
	)

	// Level dropdown
	if m.state.ShowLogLevelDropdown {
		var dropdownItems []string
		dropdownContentWidth := m.inputFieldWidth() - DropdownStyle.GetHorizontalFrameSize()
		for i, level := range LogLevelOptions {
			if i == m.state.LogLevelDropdownCursor {
				dropdownItems = append(dropdownItems, ListItemSelectedStyle.Width(dropdownContentWidth).Render(level))
			} else {
				dropdownItems = append(dropdownItems, ListItemStyle.Width(dropdownContentWidth).Render(level))
			}
		}
		dropdown := DropdownStyle.Width(m.inputFieldWidth()).Render(
			lipgloss.JoinVertical(lipgloss.Left, dropdownItems...),
		)
		content = lipgloss.JoinVertical(lipgloss.Left, content, dropdown)
	}

	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		m.blankLine(),
		destLabel,
		m.fullWidth(destValue),
	)

	// Destination dropdown
	if m.state.ShowLogDestDropdown {
		var dropdownItems []string
		dropdownContentWidth := m.inputFieldWidth() - DropdownStyle.GetHorizontalFrameSize()
		for i, dest := range LogDestinationOptions {
			if i == m.state.LogDestDropdownCursor {
				dropdownItems = append(dropdownItems, ListItemSelectedStyle.Width(dropdownContentWidth).Render(dest))
			} else {
				dropdownItems = append(dropdownItems, ListItemStyle.Width(dropdownContentWidth).Render(dest))
			}
		}
		dropdown := DropdownStyle.Width(m.inputFieldWidth()).Render(
			lipgloss.JoinVertical(lipgloss.Left, dropdownItems...),
		)
		content = lipgloss.JoinVertical(lipgloss.Left, content, dropdown)
	}

	if m.state.LoggingDestination == "file" {
		pathStyle := MenuItemDimmedStyle.Width(m.contentWidth())
		if loggingDisabled {
			pathStyle = MenuItemDimmedStyle.Width(m.contentWidth()).Foreground(SecondaryText)
		}
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			m.blankLine(),
			fileLabel,
			pathStyle.Render("  Default:   " + defaultLogPath),
			pathStyle.Render("  Instance:  " + instanceLogPath),
		)
	}

	// Context-sensitive hints
	var hints string
	if m.state.ShowLogLevelDropdown || m.state.ShowLogDestDropdown {
		hints = "[↑/↓] Select   [Enter] Apply   [Esc] Close"
	} else {
		hints = "[Esc/Enter] Apply & Back   [Tab] Next field   [Space] Toggle"
	}

	content = lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		m.blankLine(),
		HelpTextStyle.Width(m.contentWidth()).Render(hints),
	)

	if m.state.ErrorMessage != "" {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			content,
			m.blankLine(),
			ErrorStyle.Width(m.contentWidth()).Render(m.state.ErrorMessage),
		)
	}

	mainBox := MainContainerStyle.Width(m.width - 2).Render(content)
	return m.renderWithModal(mainBox)
}

// View Config rendering
func (m *WizardModel) renderViewConfig() string {
	title := SectionHeaderStyle.Width(m.contentWidth()).Render("Current Configuration (Read-only)")
	backHint := HelpTextStyle.Render("[Esc] Back")

	var configLines []string

	// Server
	configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("Server:"))
	configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  ├─ Host: "+m.state.Config.Server.Host))
	configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  └─ Port: "+strconv.Itoa(m.state.Config.Server.Port)))

	// Providers
	providerCount := len(m.state.Config.Providers)
	configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render(fmt.Sprintf("Providers (%d):", providerCount)))
	for name, pc := range m.state.Config.Providers {
		models := strings.Join(pc.Models, ", ")
		configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  ├─ "+name))
		configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  │   ├─ URL: "+pc.BaseURL))
		configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  │   ├─ Transformer: "+pc.Transformer))
		configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  │   └─ Models: "+models))
	}

	// Routes
	routeCount := len(m.state.Config.Router.Routes)
	configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render(fmt.Sprintf("Routes (%d):", routeCount)))
	for name, chain := range m.state.Config.Router.Routes {
		configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  ├─ "+name+" → "+chain))
	}

	// Logging
	configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("Logging:"))
	configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  ├─ Enabled: "+strconv.FormatBool(m.state.Config.Logging.Enabled)))
	configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  ├─ Level: "+m.state.Config.Logging.Level))
	configLines = append(configLines, MenuItemDimmedStyle.Width(m.contentWidth()).Render("  └─ Destination: "+m.state.Config.Logging.Destination))

	closeBtn := ButtonStyle.Render("[Close]")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.fullWidth(title+"  "+backHint),
		m.blankLine(),
		lipgloss.JoinVertical(lipgloss.Left, configLines...),
		m.blankLine(),
		m.fullWidth(lipgloss.JoinHorizontal(lipgloss.Center, closeBtn)),
		m.blankLine(),
		HelpTextStyle.Width(m.contentWidth()).Render("[P] Export to file   [Esc] Close"),
	)

	mainBox := MainContainerStyle.Width(m.width - 2).Render(content)
	return m.renderWithModal(mainBox)
}