package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const watchRefreshInterval = 2 * time.Second

type watchConfig struct {
	taskDir  string
	listName string
	grep     string
	verbose  bool
	quiet    bool
}

type watchModel struct {
	config   watchConfig
	output   string
	err      error
	schedule func(time.Duration) tea.Cmd
}

type watchLoadResult struct {
	output string
	err    error
}

type watchTimerMsg struct{}

func newWatchModel(config watchConfig) watchModel {
	return watchModel{config: config, schedule: func(d time.Duration) tea.Cmd {
		return tea.Tick(d, func(time.Time) tea.Msg { return watchTimerMsg{} })
	}}
}

func (m watchModel) Init() tea.Cmd {
	return m.load()
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || (msg.Type == tea.KeyRunes && msg.String() == "q") {
			return m, tea.Quit
		}
	case watchLoadResult:
		if msg.err != nil {
			m.err = msg.err

			return m, m.schedule(watchRefreshInterval)
		}

		m.output = msg.output
		m.err = nil

		return m, m.schedule(watchRefreshInterval)
	case watchTimerMsg:
		return m, m.load()
	}

	return m, nil
}

func (m watchModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("%serror: %v\n", m.output, m.err)
	}

	return m.output
}

func (m watchModel) load() tea.Cmd {
	return func() tea.Msg {
		tasks, err := NewTaskList(m.config.taskDir, m.config.listName)
		if err != nil {
			return watchLoadResult{err: err}
		}

		var rendered strings.Builder
		tasks.List(&rendered, "tasks", m.config.verbose, m.config.quiet, m.config.grep)

		return watchLoadResult{output: rendered.String()}
	}
}
