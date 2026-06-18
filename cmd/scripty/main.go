package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"scripts/internal/config"
	"scripts/internal/executor"
	"scripts/internal/tui"
)

func main() {
	cfg, err := config.LoadOrPrompt()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	scripts, err := executor.FindScripts(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding scripts: %v\n", err)
		os.Exit(1)
	}

	model := tui.New(scripts, cfg.OpenRouterKey)
	program := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
