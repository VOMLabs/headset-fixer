package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	EVIOCGNAME = 0x82004506
	KEY_MICMUTE = 248
)

var (
	confDir = os.Getenv("HOME") + "/.config"
	echoCfg = confDir + "/pipewire/pipewire-pulse.conf.d/99-echo-cancel.conf"
	echoDis = echoCfg + ".disabled"
	srcRe   = regexp.MustCompile(`alsa_input\.usb-Logitech_PRO_X.*\.mono-fallback`)
	src     string
	restart bool
)

type inputEvent struct {
	_     [16]byte
	Type  uint16
	Code  uint16
	Value int32
}

type step int

const (
	stepPending step = iota
	stepRunning
	stepDone
	stepWarn
	stepFail
)

type task struct {
	name  string
	state step
	msg   string
	fn    func() (int, string)
}

type model struct {
	tasks   []task
	current int
	spin    spinner.Model
}

type taskDone struct {
	index int
	res   int
	msg   string
}

const (
	resOK = iota
	resWarn
	resFail
)

func initialModel() model {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	s.Spinner = spinner.Dot

	src = detectSource()

	tasks := []task{{
		name: "Detect Logitech PRO X source",
		fn: func() (int, string) {
			if src == "" {
				return resFail, "Not found — is it plugged in?"
			}
			return resOK, src
		},
	}}

	if src != "" {
		tasks = append(tasks,
			task{
				name: "Unload echo-cancel module",
				fn: func() (int, string) {
					if id := findEchoMod(); id >= 0 {
						exec.Command("pactl", "unload-module", strconv.Itoa(id)).Run()
						restart = true
						return resWarn, "Unloaded"
					}
					return resOK, "Not loaded"
				},
			},
			task{
				name: "Disable echo-cancel config",
				fn: func() (int, string) {
					if _, err := os.Stat(echoCfg); err == nil {
						os.Rename(echoCfg, echoDis)
						restart = true
						return resWarn, "Disabled"
					}
					return resOK, "Not active"
				},
			},
			task{
				name: "Fix default source",
				fn: func() (int, string) {
					if d := getDefSrc(); d != src {
						exec.Command("pactl", "set-default-source", src).Run()
						restart = true
						return resWarn, "Fixed"
					}
					return resOK, "Correct"
				},
			},
			task{
				name: "Set volume to 100%",
				fn: func() (int, string) {
					exec.Command("pactl", "set-source-volume", src, "65536").Run()
					return resOK, "100%"
				},
			},
			task{
				name: "Unmute mic",
				fn: func() (int, string) {
					exec.Command("pactl", "set-source-mute", src, "0").Run()
					return resOK, "Unmuted"
				},
			},
			task{
				name: "Check mute LED",
				fn: func() (int, string) {
					on, err := ledState()
					if err != nil || !on {
						return resOK, "OK"
					}
					if err := toggle(); err != nil {
						return resWarn, "Press button on cable"
					}
					restart = true
					return resWarn, "Toggled"
				},
			},
			task{
				name: "Restart PipeWire-Pulse",
				fn: func() (int, string) {
					if !restart {
						return resOK, "Not needed"
					}
					exec.Command("systemctl", "--user", "restart", "pipewire-pulse").Run()
					time.Sleep(2 * time.Second)
					return resOK, "Restarted"
				},
			},
			task{
				name: "Restart Discord",
				fn: func() (int, string) {
					if !restart {
						return resOK, "Not needed"
					}
					exec.Command("pkill", "discord").Run()
					time.Sleep(1 * time.Second)
					exec.Command("discord").Start()
					return resOK, "Restarted"
				},
			},
			task{
				name: "Restart Vesktop",
				fn: func() (int, string) {
					if !restart {
						return resOK, "Not needed"
					}
					exec.Command("pkill", "vesktop").Run()
					time.Sleep(1 * time.Second)
					exec.Command("vesktop").Start()
					return resOK, "Restarted"
				},
			},
			task{
				name: "Verify audio",
				fn: func() (int, string) {
					if verifyAudio(src) {
						return resOK, "Audio detected!"
					}
					return resWarn, "No audio — check mute button"
				},
			},
		)
	}

	return model{tasks: tasks, spin: s}
}

func (m model) Init() tea.Cmd {
	if len(m.tasks) == 0 {
		return tea.Quit
	}
	return m.runTask(0)
}

func (m model) runTask(i int) tea.Cmd {
	m.tasks[i].state = stepRunning
	return tea.Batch(
		m.spin.Tick,
		func() tea.Msg {
			res, msg := m.tasks[i].fn()
			return taskDone{index: i, res: res, msg: msg}
		},
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "esc" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case spinner.TickMsg:
		if m.current < len(m.tasks) && m.tasks[m.current].state == stepRunning {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
		return m, nil

	case taskDone:
		t := &m.tasks[msg.index]
		t.msg = msg.msg
		switch msg.res {
		case resOK:
			t.state = stepDone
		case resWarn:
			t.state = stepWarn
		case resFail:
			t.state = stepFail
		}
		m.current++

		if m.current < len(m.tasks) {
			return m, m.runTask(m.current)
		}
		return m, tea.Quit
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Render("Logitech PRO X Mic Fix")
	b.WriteString(title)
	b.WriteString("\n\n")

	for _, t := range m.tasks {
		b.WriteString(renderTask(t, m.spin.View()))
		b.WriteString("\n")
	}

	return b.String()
}

func renderTask(t task, spinView string) string {
	switch t.state {
	case stepPending:
		return fmt.Sprintf("  %s %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(t.name),
		)
	case stepRunning:
		return fmt.Sprintf("  %s %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(spinView),
			lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Render(t.name),
		)
	case stepDone:
		return fmt.Sprintf("  %s %s  %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓"),
			t.name,
			lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(t.msg),
		)
	case stepWarn:
		return fmt.Sprintf("  %s %s  %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("⚠"),
			t.name,
			lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(t.msg),
		)
	case stepFail:
		return fmt.Sprintf("  %s %s  %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗"),
			t.name,
			lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(t.msg),
		)
	}
	return ""
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func detectSource() string {
	b, _ := exec.Command("pactl", "list", "sources", "short").Output()
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && srcRe.MatchString(f[1]) {
			return f[1]
		}
	}
	return ""
}

func findEchoMod() int {
	b, _ := exec.Command("pactl", "list", "modules", "short").Output()
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "module-echo-cancel") && strings.Contains(line, "logitech") {
			f := strings.Fields(line)
			if len(f) >= 1 {
				id, _ := strconv.Atoi(f[0])
				return id
			}
		}
	}
	return -1
}

func getDefSrc() string {
	b, _ := exec.Command("pactl", "get-default-source").Output()
	return strings.TrimSpace(string(b))
}

func ledState() (bool, error) {
	es, err := os.ReadDir("/sys/class/leds")
	if err != nil {
		return false, err
	}
	for _, e := range es {
		if !strings.HasSuffix(e.Name(), "::mute") {
			continue
		}
		b, err := os.ReadFile("/sys/class/leds/" + e.Name() + "/brightness")
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(b)) == "1" {
			return true, nil
		}
	}
	return false, nil
}

func findEventDev() (string, error) {
	es, err := os.ReadDir("/dev/input")
	if err != nil {
		return "", err
	}
	for _, e := range es {
		if !strings.HasPrefix(e.Name(), "event") {
			continue
		}
		p := "/dev/input/" + e.Name()
		fd, err := syscall.Open(p, syscall.O_RDWR, 0)
		if err != nil {
			continue
		}
		var name [256]byte
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(EVIOCGNAME), uintptr(unsafe.Pointer(&name[0])))
		syscall.Close(fd)
		n := strings.TrimRight(string(name[:]), "\x00")
		if strings.Contains(n, "Logitech") && strings.Contains(n, "PRO X") {
			return p, nil
		}
	}
	return "", errors.New("not found")
}

func toggle() error {
	d, err := findEventDev()
	if err != nil {
		return err
	}
	fd, err := syscall.Open(d, syscall.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", d, err)
	}
	defer syscall.Close(fd)

	writeEv := func(v int32) error {
		var buf bytes.Buffer
		binary.Write(&buf, binary.LittleEndian, inputEvent{Type: 1, Code: KEY_MICMUTE, Value: v})
		b := buf.Bytes()
		_, _, e := syscall.Syscall(syscall.SYS_WRITE, uintptr(fd), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
		if e != 0 {
			return e
		}
		return nil
	}
	writeEv(1)
	time.Sleep(50 * time.Millisecond)
	writeEv(0)
	return nil
}

func verifyAudio(src string) bool {
	cmd := exec.Command("parec", "--device="+src, "--record", "--channels=1", "--rate=48000", "--format=s16le")
	r, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		return false
	}
	defer cmd.Process.Kill()

	rd := bufio.NewReader(r)
	buf := make([]byte, 256)
	dl := time.After(3 * time.Second)

	for {
		select {
		case <-dl:
			return false
		default:
			n, err := rd.Read(buf)
			if err != nil {
				return false
			}
			for i := 0; i+1 < n; i += 2 {
				if int16(buf[i])|int16(buf[i+1])<<8 > 15 ||
					int16(buf[i])|int16(buf[i+1])<<8 < -15 {
					return true
				}
			}
		}
	}
}
