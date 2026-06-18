package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"scripts/internal/config"
	"scripts/internal/executor"
	"scripts/internal/installer"
	"scripts/internal/tui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "install" {
		if err := installer.SelfInstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Installed to %s.\n", installer.InstallPath())
		return
	}

	if !installer.IsRunningFromInstall() && !installer.IsInstalled() {
		if installer.PromptAndInstall() {
			if err := installer.SelfInstall(); err != nil {
				fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Installed to %s.\n", installer.InstallPath())
		}
	}

	cfg, err := config.LoadOrPrompt()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	dir := "."
	if len(os.Args) > 1 && os.Args[1] != "install" {
		dir = os.Args[1]
	}

	scripts, err := executor.FindScripts(dir)
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
