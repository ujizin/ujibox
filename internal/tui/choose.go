package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Option is one line of a Choose list.
type Option struct {
	Label string
	Note  string // dim text after the label
}

// ErrCanceled is the result of Choose when the user presses Esc, q or Ctrl-C.
var ErrCanceled = errors.New("canceled")

// Choose shows a list in the terminal. The user moves with the arrow keys
// (or j and k) and picks with Enter. It returns the index of the choice.
func Choose(title string, options []Option, start int) (int, error) {
	m := chooser{title: title, options: options, cur: start}
	out, err := tea.NewProgram(m).Run()
	if err != nil {
		return 0, err
	}
	c := out.(chooser)
	if c.canceled {
		return 0, ErrCanceled
	}
	return c.cur, nil
}

type chooser struct {
	title    string
	options  []Option
	cur      int
	done     bool
	canceled bool
}

func (m chooser) Init() tea.Cmd { return nil }

func (m chooser) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "up", "k", "shift+tab":
		m.cur = (m.cur + len(m.options) - 1) % len(m.options)
	case "down", "j", "tab":
		m.cur = (m.cur + 1) % len(m.options)
	case "enter", " ":
		m.done = true
		return m, tea.Quit
	case "esc", "q", "ctrl+c":
		m.canceled = true
		return m, tea.Quit
	default:
		// 1, 2, … pick a line directly.
		if s := k.String(); len(s) == 1 && s[0] >= '1' && int(s[0]-'1') < len(m.options) {
			m.cur = int(s[0] - '1')
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m chooser) View() string {
	var b strings.Builder
	if m.done || m.canceled {
		// Leave one line in the terminal with the answer.
		answer := m.options[m.cur].Label
		if m.canceled {
			answer = dim.Render("canceled")
		}
		fmt.Fprintf(&b, "%s %s\n", m.title, green.Render(answer))
		return b.String()
	}
	b.WriteString(m.title + "\n")
	for i, o := range m.options {
		line := "  " + o.Label
		if i == m.cur {
			line = green.Render("❯ " + o.Label)
		}
		if o.Note != "" {
			line += "  " + dim.Render(o.Note)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(dim.Render("↑/↓ to move, Enter to choose, Esc to cancel") + "\n")
	return b.String()
}
