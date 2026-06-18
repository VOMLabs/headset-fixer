package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

type Script struct {
	Name string
	Path string
}

func FindScripts(dir string) ([]Script, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var scripts []Script
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)

		switch ext {
		case ".sh", ".bash", ".zsh", ".fish",
			".py", ".pyw",
			".go",
			".js", ".mjs", ".cjs",
			".rb",
			".pl", ".pm",
			".lua",
			".ts", ".tsx":
			scripts = append(scripts, Script{Name: name, Path: filepath.Join(dir, name)})
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode()&0o111 != 0 {
			scripts = append(scripts, Script{Name: name, Path: filepath.Join(dir, name)})
			continue
		}

		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		var shebang [2]byte
		f.Read(shebang[:])
		f.Close()
		if shebang[0] == '#' && shebang[1] == '!' {
			scripts = append(scripts, Script{Name: name, Path: filepath.Join(dir, name)})
		}
	}

	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Name < scripts[j].Name
	})
	return scripts, nil
}

func Command(s Script) (*exec.Cmd, error) {
	ext := filepath.Ext(s.Name)
	switch ext {
	case ".sh", ".bash":
		return exec.Command("bash", s.Path), nil
	case ".zsh":
		return exec.Command("zsh", s.Path), nil
	case ".fish":
		return exec.Command("fish", s.Path), nil
	case ".py", ".pyw":
		return exec.Command("python3", s.Path), nil
	case ".js", ".mjs", ".cjs":
		return exec.Command("node", s.Path), nil
	case ".go":
		return exec.Command("go", "run", s.Path), nil
	case ".rb":
		return exec.Command("ruby", s.Path), nil
	case ".pl", ".pm":
		return exec.Command("perl", s.Path), nil
	case ".lua":
		return exec.Command("lua", s.Path), nil
	case ".ts", ".tsx":
		return exec.Command("npx", "tsx", s.Path), nil
	default:
		return exec.Command(s.Path), nil
	}
}
