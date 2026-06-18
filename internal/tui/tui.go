package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"scripts/internal/executor"
	"scripts/internal/openrouter"
)

type outputMsg string
type scriptDone struct{ err error }
type orResult string
type orError struct{ err error }

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#3B82F6")).
			Padding(0, 1)

	cursorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E3A5F")).
			Foreground(lipgloss.Color("#FFFFFF"))

	runningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22C55E")).
			Bold(true)

	orOnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22C55E")).
			Bold(true)

	orOffStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))

	idleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
)

type Model struct {
	scripts          []executor.Script
	cursor           int
	listOffset       int
	output           strings.Builder
	outputLines      []string
	vpWidth          int
	vpHeight         int
	vpOffset         int
	running          bool
	runErr           error
	currentScript    string
	orEnabled        bool
	apiKey           string
	width            int
	height           int
	ready            bool
	cancel           context.CancelFunc
}

func New(scripts []executor.Script, apiKey string) Model {
	return Model{
		scripts: scripts,
		apiKey:  apiKey,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.vpWidth = m.width*7/10 - 2
		m.vpHeight = m.height - 5
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}

		if m.running {
			switch msg.String() {
			case "up", "k":
				if m.vpOffset > 0 {
					m.vpOffset--
				}
			case "down", "j":
				if m.vpOffset < len(m.outputLines)-m.vpHeight {
					m.vpOffset++
				}
			case "pgup":
				m.vpOffset -= m.vpHeight / 2
				if m.vpOffset < 0 {
					m.vpOffset = 0
				}
			case "pgdown":
				m.vpOffset += m.vpHeight / 2
				maxOffset := len(m.outputLines) - m.vpHeight
				if maxOffset < 0 {
					maxOffset = 0
				}
				if m.vpOffset > maxOffset {
					m.vpOffset = maxOffset
				}
			case "g":
				m.vpOffset = 0
			case "G":
				m.vpOffset = len(m.outputLines) - m.vpHeight
				if m.vpOffset < 0 {
					m.vpOffset = 0
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.listOffset {
					m.listOffset = m.cursor
				}
			}
		case "down", "j":
			if m.cursor < len(m.scripts)-1 {
				m.cursor++
				maxVisible := m.height - 8
				if m.cursor-m.listOffset >= maxVisible {
					m.listOffset = m.cursor - maxVisible + 1
				}
			}
		case "enter":
			if !m.running && len(m.scripts) > 0 {
				s := m.scripts[m.cursor]
				ctx, cancel := context.WithCancel(context.Background())
				m.cancel = cancel
				m.running = true
				m.currentScript = s.Name
				m.output.Reset()
				m.outputLines = nil
				m.vpOffset = 0
				m.runErr = nil
				return m, runScript(ctx, s)
			}
		case "r":
			if !m.running {
				m.orEnabled = !m.orEnabled
			}
		}

	case outputMsg:
		line := string(msg)
		m.output.WriteString(line + "\n")
		m.outputLines = append(m.outputLines, line)
		if m.vpOffset >= len(m.outputLines)-m.vpHeight-1 {
			m.vpOffset = len(m.outputLines) - m.vpHeight
			if m.vpOffset < 0 {
				m.vpOffset = 0
			}
		}
		return m, nil

	case scriptDone:
		m.running = false
		m.runErr = msg.err
		m.cancel = nil
		if m.orEnabled && m.apiKey != "" && m.output.Len() > 0 {
			return m, processWithOR(m.output.String(), m.apiKey)
		}
		return m, nil

	case orResult:
		lines := strings.Split(string(msg), "\n")
		header := "--- OpenRouter Output ---"
		m.output.WriteString("\n" + header + "\n")
		m.outputLines = append(m.outputLines, "", header)
		for _, l := range lines {
			m.output.WriteString(l + "\n")
			m.outputLines = append(m.outputLines, l)
		}
		m.vpOffset = len(m.outputLines) - m.vpHeight
		if m.vpOffset < 0 {
			m.vpOffset = 0
		}
		return m, nil

	case orError:
		errLine := fmt.Sprintf("--- OpenRouter Error: %s ---", msg.err)
		m.output.WriteString("\n" + errLine + "\n")
		m.outputLines = append(m.outputLines, "", errLine)
		return m, nil
	}

	return m, nil
}

func runScript(ctx context.Context, s executor.Script) tea.Cmd {
	cmd, err := executor.Command(s)
	if err != nil {
		return func() tea.Msg {
			return scriptDone{err: err}
		}
	}

	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return func() tea.Msg {
			return scriptDone{err: err}
		}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return func() tea.Msg {
			return scriptDone{err: err}
		}
	}

	if err := cmd.Start(); err != nil {
		return func() tea.Msg {
			return scriptDone{err: err}
		}
	}

	reader := io.MultiReader(stdout, stderr)
	scanner := bufio.NewScanner(reader)
	done := make(chan error, 1)

	go func() {
		done <- cmd.Wait()
	}()

	var finished bool
	return func() tea.Msg {
		if finished {
			return nil
		}
		if scanner.Scan() {
			return outputMsg(scanner.Text())
		}
		finished = true
		err := <-done
		return scriptDone{err: err}
	}
}

func processWithOR(text, apiKey string) tea.Cmd {
	return func() tea.Msg {
		result, err := openrouter.SendText(text, apiKey)
		if err != nil {
			return orError{err: err}
		}
		return orResult(result)
	}
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	listWidth := m.width * 3 / 10
	outputWidth := m.width - listWidth
	statusWidth := m.width

	if listWidth < 20 {
		listWidth = 20
	}
	if outputWidth < 30 {
		outputWidth = 30
	}

	listPanel := m.renderList(listWidth, m.height-4)
	outputPanel := m.renderOutput(outputWidth, m.height-4)

	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel, outputPanel)

	statusBar := m.renderStatus(statusWidth)

	return lipgloss.JoinVertical(lipgloss.Left, body, statusBar)
}

func (m Model) renderList(width, height int) string {
	var b strings.Builder

	b.WriteString(titleStyle.Width(width - 2).Render(" Scripts "))
	b.WriteString("\n\n")

	maxVisible := height - 3
	if maxVisible < 1 {
		maxVisible = 1
	}

	start := m.listOffset
	end := start + maxVisible
	if end > len(m.scripts) {
		end = len(m.scripts)
	}

	displayed := m.scripts[start:end]
	for i, s := range displayed {
		line := "  " + s.Name
		if start+i == m.cursor {
			line = "▸ " + s.Name
			if m.running && s.Name == m.currentScript {
				line = cursorStyle.Render(line) + " " + runningStyle.Render("●")
			} else {
				line = cursorStyle.Render(line)
			}
		}

		b.WriteString(line)
		if i < len(displayed)-1 {
			b.WriteString("\n")
		}
	}

	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3B82F6"))

	return style.Render(b.String())
}

func (m Model) renderOutput(width, height int) string {
	var b strings.Builder

	title := " Output "
	if m.running {
		title = " Output (running) "
	}
	b.WriteString(titleStyle.Width(width - 2).Render(title))
	b.WriteString("\n\n")

	if len(m.outputLines) == 0 {
		if m.running {
			b.WriteString("Waiting for output...")
		} else {
			b.WriteString("Select a script and press Enter to run it.")
		}
	} else {
		start := m.vpOffset
		end := start + height - 3
		if end > len(m.outputLines) {
			end = len(m.outputLines)
		}
		if start < 0 {
			start = 0
		}
		for i := start; i < end; i++ {
			b.WriteString(m.outputLines[i])
			if i < end-1 {
				b.WriteString("\n")
			}
		}
	}

	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3B82F6"))

	return style.Render(b.String())
}

func (m Model) renderStatus(width int) string {
	scriptName := m.currentScript
	if scriptName == "" {
		scriptName = "none"
	}

	state := idleStyle.Render("Idle")
	if m.running {
		state = runningStyle.Render("Running")
	}

	orLabel := orOffStyle.Render("OR: OFF")
	if m.orEnabled {
		orLabel = orOnStyle.Render("OR: ON")
	}

	var errText string
	if m.runErr != nil {
		errText = fmt.Sprintf(" | Exit: %s", m.runErr)
	}

	left := fmt.Sprintf(" %s | %s | %s%s ",
		scriptName, state, orLabel, errText)
	right := " ↑↓ navigate | Enter run | r toggle OR | q quit "

	padding := width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 1 {
		padding = 1
	}
	sep := strings.Repeat(" ", padding)

	return lipgloss.NewStyle().
		Width(width).
		Background(lipgloss.Color("#1F2937")).
		Foreground(lipgloss.Color("#D1D5DB")).
		Render(left + sep + right)
}
