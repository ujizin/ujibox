// Package tui is the screen bare `uji` opens. It is a launcher: it picks
// devices and stored builds, then runs the matching uji command in the
// terminal and comes back when the command ends.
package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ujizin/ujibox/internal/config"
	"github.com/ujizin/ujibox/internal/device"
	"github.com/ujizin/ujibox/internal/proc"
	"github.com/ujizin/ujibox/internal/store"
	"github.com/ujizin/ujibox/internal/ui"
)

// Deps are what the screen reads.
type Deps struct {
	Ctx     context.Context
	Mgr     *device.Manager
	Store   *store.Store
	Filter  store.Filter // the current project's app, or empty for every app
	Group   string       // the current branch, marked in the list
	Title   string       // app name and branch for the header
	Project bool         // the working directory is a project checkout
}

// Run opens the screen.
func Run(d Deps) error {
	ti := textinput.New()
	ti.Placeholder = "myapp://path?query"
	ti.CharLimit = 2048
	m := &model{deps: d, selected: map[string]bool{}, input: ti}
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

const (
	paneDevices = iota
	paneBuilds
)

type model struct {
	deps     Deps
	devices  []device.Device
	builds   []*store.Manifest
	selected map[string]bool
	pane     int
	devCur   int
	buildCur int
	input    textinput.Model
	typing   bool
	confirm  string // build id waiting for a second x
	status   string
	width    int
	height   int
	loading  bool
}

type devicesMsg []device.Device
type execDoneMsg struct{ err error }

func (m *model) Init() tea.Cmd {
	m.loading = true
	m.builds = m.deps.Store.List(m.deps.Filter)
	return m.loadDevices()
}

func (m *model) loadDevices() tea.Cmd {
	return func() tea.Msg {
		devs, _ := m.deps.Mgr.List(m.deps.Ctx, device.AllPlatforms)
		return devicesMsg(devs)
	}
}

func key(d device.Device) string { return d.Platform + "/" + d.ID + "/" + d.Name }

// selector is how a command names a device: its ID when it has one, else its AVD name.
func selector(d device.Device) string {
	if d.ID != "" {
		return d.ID
	}
	if d.AVD != "" {
		return d.AVD
	}
	return d.Ref()
}

// targets are the selected devices, or none (every running device).
func (m *model) targets() []string {
	var out []string
	for _, d := range m.devices {
		if m.selected[key(d)] {
			out = append(out, selector(d))
		}
	}
	return out
}

func (m *model) run(args ...string) tea.Cmd {
	self, err := os.Executable()
	if err != nil {
		self = "uji"
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = proc.ShellQuote(a)
	}
	line := proc.ShellQuote(self) + " " + strings.Join(quoted, " ")
	script := line + `; printf '\n\033[2m(press enter to go back)\033[0m'; read _`
	c := exec.Command("/bin/sh", "-c", script)
	m.status = "uji " + strings.Join(args, " ")
	return tea.ExecProcess(c, func(err error) tea.Msg { return execDoneMsg{err} })
}

func (m *model) currentBuild() *store.Manifest {
	if m.buildCur >= 0 && m.buildCur < len(m.builds) {
		return m.builds[m.buildCur]
	}
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case devicesMsg:
		m.devices = msg
		m.loading = false
		if m.devCur >= len(m.devices) {
			m.devCur = max(0, len(m.devices)-1)
		}
		return m, nil
	case execDoneMsg:
		m.builds = m.deps.Store.List(m.deps.Filter)
		if m.buildCur >= len(m.builds) {
			m.buildCur = max(0, len(m.builds)-1)
		}
		m.loading = true
		return m, m.loadDevices()
	case tea.KeyMsg:
		if m.typing {
			return m.updateInput(msg)
		}
		return m.updateKeys(msg)
	}
	return m, nil
}

func (m *model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.typing = false
		m.input.Blur()
		return m, nil
	case "enter":
		url := strings.TrimSpace(m.input.Value())
		m.typing = false
		m.input.Blur()
		if url == "" {
			return m, nil
		}
		return m, m.run(append([]string{"deeplink", url}, m.targets()...)...)
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k != "x" {
		m.confirm = ""
	}
	switch k {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab", "left", "right", "h", "l":
		m.pane = 1 - m.pane
	case "up", "k":
		if m.pane == paneDevices && m.devCur > 0 {
			m.devCur--
		} else if m.pane == paneBuilds && m.buildCur > 0 {
			m.buildCur--
		}
	case "down", "j":
		if m.pane == paneDevices && m.devCur < len(m.devices)-1 {
			m.devCur++
		} else if m.pane == paneBuilds && m.buildCur < len(m.builds)-1 {
			m.buildCur++
		}
	case " ":
		if m.pane == paneDevices && m.devCur < len(m.devices) {
			kk := key(m.devices[m.devCur])
			m.selected[kk] = !m.selected[kk]
		}
	case "a":
		m.selected = map[string]bool{}
	case "R", "ctrl+r":
		m.loading = true
		m.builds = m.deps.Store.List(m.deps.Filter)
		return m, m.loadDevices()
	case "enter", "i":
		b := m.currentBuild()
		if b == nil {
			m.status = "no stored build; press b to build"
			return m, nil
		}
		if b.Platform == config.Desktop || b.Platform == config.Web {
			if !m.deps.Project {
				m.status = "launch a " + b.Platform + " build from its project checkout"
				return m, nil
			}
			return m, m.run("launch", b.Platform, "-b", b.ID)
		}
		return m, m.run(append([]string{"install", "-b", b.ID}, m.targets()...)...)
	case "I":
		if !m.deps.Project {
			m.status = "not in a project checkout"
			return m, nil
		}
		return m, m.run(append([]string{"install"}, m.targets()...)...)
	case "b":
		if !m.deps.Project {
			m.status = "not in a project checkout"
			return m, nil
		}
		return m, m.run(append([]string{"build"}, m.targets()...)...)
	case "d":
		m.typing = true
		m.input.SetValue("")
		return m, m.input.Focus()
	case "s":
		return m, m.run(append([]string{"screenshot"}, flagged(m.targets())...)...)
	case "r":
		return m, m.run(append([]string{"record"}, flagged(m.targets())...)...)
	case "y":
		return m, m.run(append([]string{"sync"}, m.targets()...)...)
	case "p":
		if b := m.currentBuild(); b != nil {
			if err := m.deps.Store.SetPinned(b, !b.Pinned); err != nil {
				m.status = err.Error()
			}
		}
	case "x":
		b := m.currentBuild()
		if b == nil {
			break
		}
		if m.confirm != b.ID {
			m.confirm = b.ID
			m.status = fmt.Sprintf("press x again to remove build %s", b.ShortID())
			break
		}
		m.confirm = ""
		if err := m.deps.Store.Remove(b); err != nil {
			m.status = err.Error()
		} else {
			m.status = "removed build " + b.ShortID()
		}
		m.builds = m.deps.Store.List(m.deps.Filter)
		if m.buildCur >= len(m.builds) {
			m.buildCur = max(0, len(m.builds)-1)
		}
	}
	return m, nil
}

// flagged turns device selectors into -d flags for record and screenshot.
func flagged(devs []string) []string {
	var out []string
	for _, d := range devs {
		out = append(out, "-d", d)
	}
	return out
}

var (
	title    = lipgloss.NewStyle().Bold(true)
	dim      = lipgloss.NewStyle().Faint(true)
	green    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellow   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	cursor   = lipgloss.NewStyle().Reverse(true)
	box      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	boxFocus = box.BorderForeground(lipgloss.Color("6"))
)

func (m *model) View() string {
	if m.width == 0 {
		return ""
	}
	header := title.Render("ujibox") + "  " + m.deps.Title
	targets := "targets: every running device"
	if t := m.targets(); len(t) > 0 {
		targets = fmt.Sprintf("targets: %d selected", len(t))
	}
	header += "\n" + dim.Render(targets)

	half := max(30, (m.width-4)/2)
	inner := m.height - 9
	devBox, buildBox := box, box
	if m.pane == paneDevices {
		devBox = boxFocus
	} else {
		buildBox = boxFocus
	}
	left := devBox.Width(half - 2).Height(inner).Render(m.viewDevices(half-4, inner))
	right := buildBox.Width(m.width - half - 4).Height(inner).Render(m.viewBuilds(m.width-half-6, inner))
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	footer := dim.Render("space select · enter/i install build · I build+install · b build · d deeplink · s screenshot · r record · y sync · p pin · x remove · R refresh · q quit")
	if m.typing {
		footer = "deeplink: " + m.input.View() + dim.Render("  (enter to open, esc to cancel)")
	} else if m.status != "" {
		footer = m.status + "\n" + footer
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *model) viewDevices(width, height int) string {
	var b strings.Builder
	b.WriteString(title.Render("Devices") + "\n")
	if m.loading && len(m.devices) == 0 {
		b.WriteString(dim.Render("loading…"))
		return b.String()
	}
	start := scroll(m.devCur, len(m.devices), height-1)
	for i := start; i < len(m.devices) && i < start+height-1; i++ {
		d := m.devices[i]
		mark := "  "
		if m.selected[key(d)] {
			mark = green.Render("● ")
		}
		state := dim.Render(d.State)
		if d.Ready() {
			state = green.Render(d.State)
		} else if d.State != device.StateOff {
			state = yellow.Render(d.State)
		}
		name := truncate(d.Label(), width-22)
		line := fmt.Sprintf("%s%-7s %s %s", mark, d.Platform, name, state)
		if d.OS != "" {
			line += dim.Render(" " + d.OS)
		}
		if m.pane == paneDevices && i == m.devCur {
			line = cursor.Render(line)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m *model) viewBuilds(width, height int) string {
	var b strings.Builder
	b.WriteString(title.Render("Builds") + "\n")
	if len(m.builds) == 0 {
		b.WriteString(dim.Render("no stored builds"))
		return b.String()
	}
	// One line per build; a header line when the app or branch changes.
	type row struct {
		text  string
		build int // -1 for a header
	}
	var rows []row
	lastApp, lastGroup := "", ""
	for i, x := range m.builds {
		if x.App.ID != lastApp {
			rows = append(rows, row{title.Render(x.App.Name), -1})
			lastApp, lastGroup = x.App.ID, ""
		}
		if x.Group() != lastGroup {
			g := x.Group()
			if g == m.deps.Group {
				g += green.Render(" ← HEAD")
			}
			rows = append(rows, row{" " + g, -1})
			lastGroup = x.Group()
		}
		flags := ""
		if x.Pinned {
			flags += yellow.Render(" pinned")
		}
		if x.Git.Dirty {
			flags += yellow.Render(" dirty")
		}
		ver := x.Version.Name
		if ver == "" {
			ver = x.ShortSHA()
		}
		text := fmt.Sprintf("   %-7s %-12s %-8s %8s", x.Platform, truncate(ver, 12), x.ShortSHA(), ui.Ago(x.CreatedAt))
		rows = append(rows, row{truncate(text, width) + flags, i})
	}
	cur := 0
	for i, r := range rows {
		if r.build == m.buildCur {
			cur = i
		}
	}
	start := scroll(cur, len(rows), height-1)
	for i := start; i < len(rows) && i < start+height-1; i++ {
		line := rows[i].text
		if rows[i].build >= 0 && rows[i].build == m.buildCur && m.pane == paneBuilds {
			line = cursor.Render(line)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func scroll(cur, total, visible int) int {
	if visible <= 0 || total <= visible {
		return 0
	}
	start := cur - visible/2
	return max(0, min(start, total-visible))
}

func truncate(s string, n int) string {
	r := []rune(s)
	if n <= 1 || len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
