package installer

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

func InstallPath() string {
	usr, err := user.Current()
	if err != nil {
		return "/usr/local/bin/scripty"
	}
	return filepath.Join(usr.HomeDir, ".local", "bin", "scripty")
}

func IsInstalled() bool {
	_, err := os.Stat(InstallPath())
	return err == nil
}

func IsRunningFromInstall() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return exe == InstallPath()
}

func SelfInstall() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	src, err := os.Open(exe)
	if err != nil {
		return fmt.Errorf("open self: %w", err)
	}
	defer src.Close()

	dstPath := InstallPath()
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return fmt.Errorf("create %s: %w", dstPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}

	return nil
}

func PromptAndInstall() bool {
	fmt.Print("scripty is not installed. Install to " + InstallPath() + "? [Y/n]: ")
	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(response)
	return response == "" || strings.HasPrefix(strings.ToLower(response), "y")
}
