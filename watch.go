package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/list"
	"github.com/charmbracelet/x/ansi"
)

const (
	watchRefreshInterval = 2 * time.Second
	watchClockInterval   = time.Second
)

var (
	watchLabelStyle = lipgloss.NewStyle().Faint(true)
	watchErrorStyle = lipgloss.NewStyle().Bold(true)
)

type watchConfig struct {
	taskDir  string
	listName string
	grep     string
	verbose  bool
	quiet    bool
}

type watchModel struct {
	config     watchConfig
	rows       []taskRow
	labelWidth int
	openCount  int
	err        error
	height     int
	width      int
	loading    bool
	schedule   func(time.Duration) tea.Cmd
	clock      time.Time
	now        func() time.Time
	clockTick  func(time.Duration) tea.Cmd
}

type watchLoadResult struct {
	rows       []taskRow
	labelWidth int
	openCount  int
	err        error
}

type (
	watchTimerMsg struct{}
	watchClockMsg struct{}
)

func newWatchModel(config watchConfig) watchModel {
	now := time.Now

	return watchModel{
		config: config,
		now:    now,
		clock:  now(),
		clockTick: func(d time.Duration) tea.Cmd {
			return defaultClockTick(d)
		},
	}
}

// defaultClockTick fires on system-clock boundaries so the displayed second never drifts or skips.
func defaultClockTick(d time.Duration) tea.Cmd {
	return tea.Every(d, func(time.Time) tea.Msg { return watchClockMsg{} })
}

func (m watchModel) Init() tea.Cmd {
	return tea.Batch(m.load(), m.nextClockTick())
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.Keystroke() == "ctrl+c" || msg.Keystroke() == "q" {
			return m, tea.Quit
		}
	case watchLoadResult:
		m.loading = false

		if msg.err != nil {
			m.err = msg.err

			return m, m.scheduleRefresh()
		}

		m.rows = msg.rows
		m.labelWidth = msg.labelWidth
		m.openCount = msg.openCount
		m.err = nil

		return m, m.scheduleRefresh()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		return m, nil
	case watchClockMsg:
		m.clock = m.currentTime()

		return m, m.nextClockTick()
	case watchTimerMsg:
		if m.loading {
			return m, nil
		}

		m.loading = true

		return m, m.load()
	}

	return m, nil
}

func (m watchModel) View() tea.View {
	lines := m.screenLines()

	if m.width > 0 {
		for i, line := range lines {
			lines[i] = ansi.Truncate(line, m.width, "…")
		}
	}

	view := tea.NewView(strings.Join(lines, "\n") + "\n")
	view.AltScreen = true

	return view
}

// screenLines lays out the error row, task rows, overflow marker, and status bar.
// The bar is the last line, except that an error replaces it at height 1.
func (m watchModel) screenLines() []string {
	var errLines []string
	if m.err != nil {
		errLines = []string{watchErrorStyle.Render("error: " + m.err.Error())}
	}

	var tasks []string
	if body := m.renderBody(); body != "" {
		tasks = strings.Split(body, "\n")
	}

	if m.height <= 0 {
		return append(append(errLines, tasks...), m.statusBar())
	}

	budget := m.height - 1
	if budget == 0 {
		if len(errLines) > 0 {
			return errLines
		}

		return []string{m.statusBar()}
	}

	lines := errLines

	if avail := budget - len(errLines); len(tasks) <= avail {
		lines = append(lines, tasks...)
	} else if avail > 0 {
		lines = append(lines, tasks[:avail-1]...)
		lines = append(lines, fmt.Sprintf("… +%d more", len(tasks)-(avail-1)))
	}

	for len(lines) < budget {
		lines = append(lines, "")
	}

	return append(lines, m.statusBar())
}

// renderBody renders the task rows as a bulleted list.
func (m watchModel) renderBody() string {
	if len(m.rows) == 0 {
		return ""
	}

	items := make([]any, 0, len(m.rows))

	for _, row := range m.rows {
		if row.Label == "" {
			items = append(items, row.Text)

			continue
		}

		items = append(items, watchLabelStyle.Width(m.labelWidth).Render(row.Label)+" - "+row.Text)
	}

	return list.New(items...).Enumerator(list.Bullet).String()
}

// currentTime reads the injected clock, falling back to time.Now.
func (m watchModel) currentTime() time.Time {
	if m.now != nil {
		return m.now()
	}

	return time.Now()
}

// nextClockTick returns the command for the next clock tick.
func (m watchModel) nextClockTick() tea.Cmd {
	if m.clockTick != nil {
		return m.clockTick(watchClockInterval)
	}

	return defaultClockTick(watchClockInterval)
}

// statusBar renders the reverse-video bar: the clock on the left and the list summary on the right.
func (m watchModel) statusBar() string {
	var left string
	if !m.clock.IsZero() {
		left = m.clock.Format("Mon Jan 2 2006 · 15:04:05")
	}

	right := fmt.Sprintf("%s · %d open · q to quit", m.config.listName, m.openCount)
	bar := lipgloss.NewStyle().Reverse(true)

	if left == "" {
		return bar.Render(fitStatusRight(right, m.width))
	}

	switch {
	case m.width <= 0:
		return bar.Render(left + " " + right)
	case lipgloss.Width(left)+lipgloss.Width(right)+2 > m.width:
		return bar.Width(m.width).Render(ansi.Truncate(left, m.width, "…"))
	default:
		gap := strings.Repeat(" ", m.width-lipgloss.Width(left)-lipgloss.Width(right))

		return bar.Width(m.width).Render(left + gap + right)
	}
}

// fitStatusRight right-aligns the summary within width, truncating it when it cannot fit.
// An unknown width leaves it as is.
func fitStatusRight(right string, width int) string {
	switch {
	case width <= 0:
		return right
	case lipgloss.Width(right) > width:
		cut := ansi.Truncate(right, width, "…")

		return cut + strings.Repeat(" ", width-lipgloss.Width(cut))
	default:
		return strings.Repeat(" ", width-lipgloss.Width(right)) + right
	}
}

// scheduleRefresh returns the command for the next refresh tick.
func (m watchModel) scheduleRefresh() tea.Cmd {
	if m.schedule != nil {
		return m.schedule(watchRefreshInterval)
	}

	return tea.Tick(watchRefreshInterval, func(time.Time) tea.Msg { return watchTimerMsg{} })
}

func (m watchModel) load() tea.Cmd {
	return func() tea.Msg {
		tasks, err := NewTaskList(m.config.taskDir, m.config.listName)
		if err != nil {
			return watchLoadResult{err: err}
		}

		rows, labelWidth := taskRows(tasks.Tasks, m.config.verbose, m.config.quiet, false, m.config.grep)

		return watchLoadResult{rows: rows, labelWidth: labelWidth, openCount: len(tasks.Tasks)}
	}
}
