package config

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

const (
	keyDirRel = ".config/scripty/or"
	keyFile   = "key.secret"
)

type Config struct {
	OpenRouterKey string
	KeyPath       string
}

func LoadOrPrompt() (*Config, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	keyPath := filepath.Join(usr.HomeDir, keyDirRel, keyFile)
	cfg := &Config{KeyPath: keyPath}

	data, err := os.ReadFile(keyPath)
	if err == nil {
		cfg.OpenRouterKey = strings.TrimSpace(string(data))
		if cfg.OpenRouterKey != "" {
			return cfg, nil
		}
	}

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	fmt.Print("Enter your OpenRouter API key: ")
	reader := bufio.NewReader(os.Stdin)
	key, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("API key cannot be empty")
	}

	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create config dir %s: %w", dir, err)
	}
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		return nil, fmt.Errorf("write key file: %w", err)
	}

	cfg.OpenRouterKey = key
	fmt.Println("API key saved to", keyPath)
	return cfg, nil
}
