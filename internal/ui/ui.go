// Package ui prints status lines, tables and colors for the command line.
package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

// IsTTY reports whether f is a terminal.
func IsTTY(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TIOCGETA)
	return err == nil
}

// Width returns the terminal width, or 100 when it is unknown.
func Width(f *os.File) int {
	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 {
		return 100
	}
	return int(ws.Col)
}

var colorOn = os.Getenv("NO_COLOR") == "" && IsTTY(os.Stdout)

func paint(code, s string) string {
	if !colorOn {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

// Color helpers. They print plain text when stdout is not a terminal or NO_COLOR is set.
func Green(s string) string  { return paint("32", s) }
func Red(s string) string    { return paint("31", s) }
func Yellow(s string) string { return paint("33", s) }
func Dim(s string) string    { return paint("2", s) }
func Bold(s string) string   { return paint("1", s) }

// OK, Fail, Warn and Info print one status line.
func OK(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Green("✔"), fmt.Sprintf(format, a...))
}
func Fail(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Red("✘"), fmt.Sprintf(format, a...))
}
func Warn(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Yellow("!"), fmt.Sprintf(format, a...))
}
func Info(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, "%s %s\n", Dim("•"), fmt.Sprintf(format, a...))
}

// Table prints rows with aligned columns. ANSI colors do not count toward width.
func Table(w io.Writer, header []string, rows [][]string) {
	all := rows
	if header != nil {
		h := make([]string, len(header))
		for i, c := range header {
			h[i] = Dim(c)
		}
		all = append([][]string{h}, rows...)
	}
	widths := map[int]int{}
	for _, r := range all {
		for i, c := range r {
			if n := visibleLen(c); n > widths[i] {
				widths[i] = n
			}
		}
	}
	for _, r := range all {
		var b strings.Builder
		for i, c := range r {
			b.WriteString(c)
			if i < len(r)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-visibleLen(c)+2))
			}
		}
		fmt.Fprintln(w, strings.TrimRight(b.String(), " "))
	}
}

func visibleLen(s string) int {
	n := 0
	in := false
	for _, r := range s {
		switch {
		case r == '\033':
			in = true
		case in && r == 'm':
			in = false
		case !in:
			n++
		}
	}
	return n
}

// Status shows one live line: a spinner, a label, the elapsed time and the
// last line of the tool output. On a non-terminal it prints nothing until Done.
type Status struct {
	mu    sync.Mutex
	label string
	last  string
	start time.Time
	tty   bool
	out   *os.File
	stop  chan struct{}
	done  chan struct{}
	buf   []byte
}

// NewStatus starts a status line on stderr.
func NewStatus(label string) *Status {
	s := &Status{label: label, start: time.Now(), out: os.Stderr, tty: IsTTY(os.Stderr),
		stop: make(chan struct{}), done: make(chan struct{})}
	if !s.tty {
		fmt.Fprintf(s.out, "%s %s…\n", Dim("•"), label)
		close(s.done)
		return s
	}
	go s.loop()
	return s
}

// Write takes tool output and keeps its last complete line.
func (s *Status) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(s.buf, p...)
	for {
		i := strings.IndexAny(string(s.buf), "\r\n")
		if i < 0 {
			break
		}
		if line := strings.TrimSpace(string(s.buf[:i])); line != "" {
			s.last = line
		}
		s.buf = s.buf[i+1:]
	}
	if len(s.buf) > 4096 {
		s.buf = s.buf[len(s.buf)-4096:]
	}
	return len(p), nil
}

func (s *Status) loop() {
	defer close(s.done)
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	t := time.NewTicker(120 * time.Millisecond)
	defer t.Stop()
	for i := 0; ; i++ {
		select {
		case <-s.stop:
			fmt.Fprint(s.out, "\r\033[K")
			return
		case <-t.C:
			s.mu.Lock()
			head := fmt.Sprintf("%s %s %s", frames[i%len(frames)], s.label, Elapsed(time.Since(s.start)))
			line := head
			if s.last != "" {
				room := Width(s.out) - utf8.RuneCountInString(head) - 4
				if room > 10 {
					line += Dim(" │ " + truncate(s.last, room))
				}
			}
			s.mu.Unlock()
			fmt.Fprint(s.out, "\r\033[K"+line)
		}
	}
}

// Done clears the live line.
func (s *Status) Done() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	<-s.done
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// Elapsed formats a duration as 1m23s or 4.2s.
func Elapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// Size formats bytes as 84.1 MB.
func Size(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// Ago formats how long ago t was: "just now", "5m ago", "3h ago", "2d ago".
func Ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// Plural returns "1 build" or "3 builds".
func Plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
