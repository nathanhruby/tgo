package main

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/list"
)

const watchRefreshInterval = 2 * time.Second

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
	loading    bool
	schedule   func(time.Duration) tea.Cmd
}

type watchLoadResult struct {
	rows       []taskRow
	labelWidth int
	openCount  int
	err        error
}

type watchTimerMsg struct{}

func newWatchModel(config watchConfig) watchModel {
	return watchModel{config: config}
}

func (m watchModel) Init() tea.Cmd {
	return m.load()
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
		m.height = msg.Height

		return m, nil
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
	lines := make([]string, 0, 1+len(m.rows))

	if m.err != nil {
		lines = append(lines, watchErrorStyle.Render("error: "+m.err.Error()))
	}

	if body := m.renderBody(); body != "" {
		lines = append(lines, strings.Split(body, "\n")...)
	}

	if m.height > 0 && len(lines) > m.height {
		hidden := len(lines) - (m.height - 1)
		lines = append(lines[:m.height-1], fmt.Sprintf("… +%d more", hidden))
	}

	var content string
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}

	view := tea.NewView(content)
	view.AltScreen = true

	return view
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

		rows, labelWidth := taskRows(tasks.Tasks, m.config.verbose, m.config.quiet, m.config.grep)

		return watchLoadResult{rows: rows, labelWidth: labelWidth, openCount: len(tasks.Tasks)}
	}
}
