package tui

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mrtuuro/go-switcher/internal/progress"
	"github.com/mrtuuro/go-switcher/internal/switcher"
)

type Service interface {
	ListLocal() ([]string, error)
	ListRemote(context.Context) ([]string, error)
	Current(cwd string) (switcher.ActiveVersion, error)
	InstallWithProgress(context.Context, string, progress.Reporter) (string, error)
	UseWithProgress(context.Context, string, switcher.Scope, string, progress.Reporter) (string, string, error)
	DeleteInstalledWithProgress(context.Context, string, string, progress.Reporter) (switcher.DeleteResult, error)
}

type listMode int

const (
	modeLocal listMode = iota
	modeRemote
)

type model struct {
	ctx context.Context
	svc Service
	cwd string

	mode       listMode
	scope      switcher.Scope
	cursor     int
	listOffset int
	width      int
	height     int

	searchQuery  string
	searchActive bool

	localVersions  []string
	remoteVersions []string
	activeVersion  string
	activeScope    switcher.Scope

	busy         bool
	status       string
	lastError    string
	spinner      spinner.Model
	hasRemoteHit bool
	progressCh   <-chan progress.Event
	doneCh       <-chan tea.Msg

	scopeInitialized bool
}

type versionsMsg struct {
	mode     listMode
	versions []string
	err      error
}

type currentMsg struct {
	version string
	scope   switcher.Scope
	err     error
}

type installDoneMsg struct {
	version string
	err     error
}

type useDoneMsg struct {
	version     string
	lintVersion string
	active      switcher.ActiveVersion
	err         error
}

type progressMsg struct {
	event progress.Event
}

type asyncClosedMsg struct{}

type deleteDoneMsg struct {
	result switcher.DeleteResult
	err    error
}

func Run(ctx context.Context, svc Service, cwd string) error {
	m := newModel(ctx, svc, cwd)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(ctx context.Context, svc Service, cwd string) model {
	spin := spinner.New()
	spin.Spinner = spinner.MiniDot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	return model{
		ctx:          ctx,
		svc:          svc,
		cwd:          cwd,
		mode:         modeLocal,
		scope:        switcher.ScopeGlobal,
		status:       "Loading local versions...",
		busy:         true,
		spinner:      spin,
		activeScope:  switcher.ScopeGlobal,
		hasRemoteHit: false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadLocalCmd(),
		m.loadCurrentCmd(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch typed := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(typed)
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.ensureCursorVisible()
	case spinner.TickMsg:
		if m.busy {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	case progressMsg:
		if typed.event.Message != "" {
			m.status = typed.event.Message
		}
		m.lastError = ""
		if m.doneCh != nil || m.progressCh != nil {
			cmds = append(cmds, m.waitAsyncCmd())
		}
	case asyncClosedMsg:
		m.busy = false
		m.progressCh = nil
		m.doneCh = nil
	case versionsMsg:
		m.busy = false
		if typed.err != nil {
			m.lastError = typed.err.Error()
			m.status = "Failed to load versions"
			return m, tea.Batch(cmds...)
		}

		if typed.mode == modeLocal {
			m.localVersions = typed.versions
			if len(m.localVersions) > 0 && m.cursor >= len(m.localVersions) {
				m.cursor = len(m.localVersions) - 1
			}
			if len(m.localVersions) == 0 {
				m.cursor = 0
				m.listOffset = 0
			}
			m.status = fmt.Sprintf("Loaded %d local versions", len(m.localVersions))
		} else {
			m.remoteVersions = typed.versions
			m.hasRemoteHit = true
			if m.mode == modeRemote {
				if len(m.remoteVersions) > 0 && m.cursor >= len(m.remoteVersions) {
					m.cursor = len(m.remoteVersions) - 1
				}
				if len(m.remoteVersions) == 0 {
					m.cursor = 0
					m.listOffset = 0
				}
			}
			m.status = fmt.Sprintf("Loaded %d remote versions", len(m.remoteVersions))
		}
		m.ensureCursorVisible()
		m.lastError = ""
	case currentMsg:
		if typed.err != nil {
			if typed.err == switcher.ErrNoActiveVersion {
				m.activeVersion = ""
				m.activeScope = switcher.ScopeGlobal
				return m, tea.Batch(cmds...)
			}
			m.lastError = typed.err.Error()
			return m, tea.Batch(cmds...)
		}
		if !m.scopeInitialized {
			m.scope = typed.scope
			m.scopeInitialized = true
		}
		m.activeVersion = typed.version
		m.activeScope = typed.scope
	case installDoneMsg:
		m.busy = false
		m.progressCh = nil
		m.doneCh = nil
		if typed.err != nil {
			m.lastError = typed.err.Error()
			m.status = "Install failed"
			return m, tea.Batch(cmds...)
		}
		m.lastError = ""
		m.status = fmt.Sprintf("Installed %s", typed.version)
		cmds = append(cmds, m.loadLocalCmd(), m.loadCurrentCmd())
		if m.mode == modeRemote {
			m.cursor = 0
		}
	case useDoneMsg:
		m.busy = false
		m.progressCh = nil
		m.doneCh = nil
		if typed.err != nil {
			m.lastError = typed.err.Error()
			m.status = "Switch failed"
			return m, tea.Batch(cmds...)
		}
		m.activeVersion = typed.active.Version
		m.activeScope = typed.active.Scope
		m.lastError = ""
		if typed.active.Version == typed.version && typed.active.Scope == m.scope {
			m.status = fmt.Sprintf("Using %s (%s), golangci-lint %s", typed.active.Version, typed.active.Scope, typed.lintVersion)
		} else {
			m.status = fmt.Sprintf("Set %s scope to %s; effective active is %s (%s)", m.scope, typed.version, typed.active.Version, typed.active.Scope)
		}
	case deleteDoneMsg:
		m.busy = false
		m.progressCh = nil
		m.doneCh = nil
		if typed.err != nil {
			m.lastError = typed.err.Error()
			m.status = "Delete failed"
			return m, tea.Batch(cmds...)
		}

		m.lastError = ""
		result := typed.result
		switch {
		case result.WasActive && result.SwitchedToNewest && result.ActiveAfter.Version != "":
			m.status = fmt.Sprintf("Deleted %s; switched to %s (%s)", result.DeletedVersion, result.ActiveAfter.Version, result.ActiveAfter.Scope)
		case result.WasActive && result.ActiveAfter.Version == "":
			m.status = fmt.Sprintf("Deleted %s; no installed versions remain", result.DeletedVersion)
		default:
			m.status = fmt.Sprintf("Deleted %s", result.DeletedVersion)
		}

		if result.ToolSyncWarning != "" {
			m.lastError = "Tool sync warning: " + result.ToolSyncWarning
		}

		cmds = append(cmds, m.loadLocalCmd(), m.loadCurrentCmd())
	}

	return m, tea.Batch(cmds...)
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	if key == "ctrl+c" || key == "q" {
		return m, tea.Quit
	}

	if m.busy {
		return m, nil
	}

	if updated, handled := m.handleSearchKey(msg); handled {
		return updated, nil
	}

	current := m.currentList()

	switch key {
	case "up", "k":
		if len(current) > 0 && m.cursor > 0 {
			m.cursor--
			m.ensureCursorVisible()
		}
	case "down", "j":
		if len(current) > 0 && m.cursor < len(current)-1 {
			m.cursor++
			m.ensureCursorVisible()
		}
	case "pgup", "ctrl+b":
		if len(current) > 0 {
			m.cursor -= m.pageSize()
			m.clampCursor()
		}
	case "pgdown", "ctrl+f":
		if len(current) > 0 {
			m.cursor += m.pageSize()
			m.clampCursor()
		}
	case "home", "g":
		if len(current) > 0 {
			m.cursor = 0
			m.ensureCursorVisible()
		}
	case "end", "G":
		if len(current) > 0 {
			m.cursor = len(current) - 1
			m.ensureCursorVisible()
		}
	case "tab":
		if m.mode == modeLocal {
			m.mode = modeRemote
			m.cursor = 0
			m.listOffset = 0
			m.status = "Remote versions"
			if !m.hasRemoteHit {
				m.busy = true
				m.status = "Loading remote versions..."
				return m, tea.Batch(m.spinner.Tick, m.loadRemoteCmd())
			}
		} else {
			m.mode = modeLocal
			m.cursor = 0
			m.listOffset = 0
			m.status = "Local versions"
		}
		m.ensureCursorVisible()
		if m.searchQuery != "" {
			m.status = m.searchStatusText()
		}
	case "s":
		if m.scope == switcher.ScopeGlobal {
			m.scope = switcher.ScopeLocal
		} else {
			m.scope = switcher.ScopeGlobal
		}
		m.scopeInitialized = true
		m.status = fmt.Sprintf("Scope set to %s", m.scope)
	case "r":
		m.busy = true
		m.status = "Refreshing information..."
		if m.mode == modeLocal {
			return m, tea.Batch(m.spinner.Tick, m.loadLocalCmd(), m.loadCurrentCmd())
		}
		return m, tea.Batch(m.spinner.Tick, m.loadRemoteCmd())
	case "x", "X":
		if m.mode != modeLocal {
			m.status = "Delete works in local mode only"
			return m, nil
		}
		if len(current) == 0 {
			m.status = "No installed version selected"
			return m, nil
		}
		version := current[m.cursor]
		return m.startDelete(version)
	case "i":
		if m.mode != modeRemote {
			m.status = "Switch to remote mode (Tab) to install"
			return m, nil
		}
		if len(current) == 0 {
			m.status = "No remote version selected"
			return m, nil
		}
		version := current[m.cursor]
		return m.startInstall(version)
	case "enter":
		if len(current) == 0 {
			m.status = "No version selected"
			return m, nil
		}
		version := current[m.cursor]
		return m.startUse(version)
	}

	return m, nil
}

func (m model) handleSearchKey(msg tea.KeyMsg) (model, bool) {
	key := msg.String()

	if m.searchActive {
		switch key {
		case "esc":
			m.searchActive = false
			m.searchQuery = ""
			m.cursor = 0
			m.listOffset = 0
			m.ensureCursorVisible()
			m.status = "Search cleared"
			return m, true
		case "enter":
			m.searchActive = false
			m.status = m.searchStatusText()
			return m, true
		case "backspace", "ctrl+h":
			if m.searchQuery == "" {
				m.searchActive = false
				m.status = "Search cleared"
				return m, true
			}

			_, size := utf8.DecodeLastRuneInString(m.searchQuery)
			if size > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-size]
			}
			m.cursor = 0
			m.listOffset = 0
			m.ensureCursorVisible()
			m.status = m.searchStatusText()
			return m, true
		case "ctrl+u":
			m.searchQuery = ""
			m.cursor = 0
			m.listOffset = 0
			m.ensureCursorVisible()
			m.status = "Search cleared"
			return m, true
		}

		if len(msg.Runes) > 0 && msg.Type == tea.KeyRunes {
			m.searchQuery += string(msg.Runes)
			m.cursor = 0
			m.listOffset = 0
			m.ensureCursorVisible()
			m.status = m.searchStatusText()
			return m, true
		}

		return m, false
	}

	switch key {
	case "/":
		m.searchActive = true
		if m.searchQuery == "" {
			m.status = "Search mode: type to filter versions, Enter to apply, Esc to clear"
		} else {
			m.status = m.searchStatusText()
		}
		return m, true
	case "esc":
		if m.searchQuery == "" {
			return m, false
		}
		m.searchQuery = ""
		m.cursor = 0
		m.listOffset = 0
		m.ensureCursorVisible()
		m.status = "Search cleared"
		return m, true
	default:
		return m, false
	}
}

func (m model) searchStatusText() string {
	if m.searchQuery == "" {
		return "Search cleared"
	}

	matches := len(m.currentList())
	unit := "matches"
	if matches == 1 {
		unit = "match"
	}

	return fmt.Sprintf("Search %q: %d %s", m.searchQuery, matches, unit)
}

func (m model) loadLocalCmd() tea.Cmd {
	return func() tea.Msg {
		versions, err := m.svc.ListLocal()
		return versionsMsg{mode: modeLocal, versions: versions, err: err}
	}
}

func (m model) loadRemoteCmd() tea.Cmd {
	return func() tea.Msg {
		versions, err := m.svc.ListRemote(m.ctx)
		return versionsMsg{mode: modeRemote, versions: versions, err: err}
	}
}

func (m model) loadCurrentCmd() tea.Cmd {
	return func() tea.Msg {
		active, err := m.svc.Current(m.cwd)
		if err != nil {
			return currentMsg{err: err}
		}
		return currentMsg{version: active.Version, scope: active.Scope}
	}
}

func (m model) startInstall(version string) (tea.Model, tea.Cmd) {
	progressCh := make(chan progress.Event, 128)
	doneCh := make(chan tea.Msg, 1)

	go func() {
		reporter := func(event progress.Event) {
			select {
			case progressCh <- event:
			default:
			}
		}

		installed, err := m.svc.InstallWithProgress(m.ctx, version, reporter)
		close(progressCh)
		doneCh <- installDoneMsg{version: installed, err: err}
		close(doneCh)
	}()

	m.busy = true
	m.lastError = ""
	m.status = fmt.Sprintf("Starting installation for %s...", version)
	m.progressCh = progressCh
	m.doneCh = doneCh

	return m, tea.Batch(m.spinner.Tick, m.waitAsyncCmd())
}

func (m model) startUse(version string) (tea.Model, tea.Cmd) {
	progressCh := make(chan progress.Event, 128)
	doneCh := make(chan tea.Msg, 1)

	go func() {
		reporter := func(event progress.Event) {
			select {
			case progressCh <- event:
			default:
			}
		}

		selected, lintVersion, err := m.svc.UseWithProgress(m.ctx, version, m.scope, m.cwd, reporter)
		if err != nil {
			close(progressCh)
			doneCh <- useDoneMsg{err: err}
			close(doneCh)
			return
		}

		active, err := m.svc.Current(m.cwd)
		close(progressCh)
		if err != nil {
			doneCh <- useDoneMsg{version: selected, lintVersion: lintVersion, err: err}
			close(doneCh)
			return
		}

		doneCh <- useDoneMsg{version: selected, lintVersion: lintVersion, active: active}
		close(doneCh)
	}()

	m.busy = true
	m.lastError = ""
	m.status = fmt.Sprintf("Switching to %s (%s)...", version, m.scope)
	m.progressCh = progressCh
	m.doneCh = doneCh

	return m, tea.Batch(m.spinner.Tick, m.waitAsyncCmd())
}

func (m model) startDelete(version string) (tea.Model, tea.Cmd) {
	progressCh := make(chan progress.Event, 128)
	doneCh := make(chan tea.Msg, 1)

	go func() {
		reporter := func(event progress.Event) {
			select {
			case progressCh <- event:
			default:
			}
		}

		result, err := m.svc.DeleteInstalledWithProgress(m.ctx, m.cwd, version, reporter)
		close(progressCh)
		doneCh <- deleteDoneMsg{result: result, err: err}
		close(doneCh)
	}()

	m.busy = true
	m.lastError = ""
	m.status = fmt.Sprintf("Deleting %s...", version)
	m.progressCh = progressCh
	m.doneCh = doneCh

	return m, tea.Batch(m.spinner.Tick, m.waitAsyncCmd())
}

func (m model) waitAsyncCmd() tea.Cmd {
	progressCh := m.progressCh
	doneCh := m.doneCh

	return func() tea.Msg {
		if progressCh == nil && doneCh == nil {
			return asyncClosedMsg{}
		}

		if progressCh == nil {
			msg, ok := <-doneCh
			if !ok {
				return asyncClosedMsg{}
			}
			return msg
		}

		if doneCh == nil {
			event, ok := <-progressCh
			if !ok {
				return asyncClosedMsg{}
			}
			return progressMsg{event: event}
		}

		select {
		case msg, ok := <-doneCh:
			if !ok {
				return asyncClosedMsg{}
			}
			return msg
		case event, ok := <-progressCh:
			if !ok {
				msg, ok := <-doneCh
				if !ok {
					return asyncClosedMsg{}
				}
				return msg
			}
			return progressMsg{event: event}
		}
	}
}

func (m model) currentList() []string {
	list := m.unfilteredList()
	if strings.TrimSpace(m.searchQuery) == "" {
		return list
	}

	query := strings.ToLower(strings.TrimSpace(m.searchQuery))
	filtered := make([]string, 0, len(list))
	for _, version := range list {
		if strings.Contains(strings.ToLower(version), query) {
			filtered = append(filtered, version)
		}
	}

	return filtered
}

func (m model) unfilteredList() []string {
	if m.mode == modeRemote {
		return m.remoteVersions
	}
	return m.localVersions
}

func (m model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	subtleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Bold(true)
	activeCursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).Underline(true)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	currentMode := "Local"
	if m.mode == modeRemote {
		currentMode = "Remote"
	}

	header := titleStyle.Render("Go Switcher")
	header += "\n"
	header += subtleStyle.Render("Tab: local/remote  /:search  Enter: use  i:install(remote)  X:delete(local)  s:scope  r:refresh  Esc:clear search  q:quit")

	active := "none"
	if m.activeVersion != "" {
		active = fmt.Sprintf("%s (%s)", m.activeVersion, m.activeScope)
	}
	meta := fmt.Sprintf("Mode: %s  Scope: %s  Active: %s", currentMode, m.scope, active)
	if m.activeScope == switcher.ScopeLocal && m.scope == switcher.ScopeGlobal {
		meta += "\n" + subtleStyle.Render("Local override is active; switching global will not change effective active version here")
	}

	if m.searchQuery != "" || m.searchActive {
		rawCount := len(m.unfilteredList())
		filteredCount := len(m.currentList())
		searchLine := fmt.Sprintf("Search: %q (%d/%d)", m.searchQuery, filteredCount, rawCount)
		if m.searchActive {
			searchLine += " [typing]"
		}
		meta += "\n" + subtleStyle.Render(searchLine)
	} else {
		meta += "\n" + subtleStyle.Render("Press / to search versions")
	}

	list := m.currentList()
	if len(list) == 0 {
		if m.searchQuery != "" {
			list = []string{"<no matches>"}
		} else {
			list = []string{"<empty>"}
		}
	}

	pageSize := m.pageSize()
	start, end := m.visibleRange(pageSize, len(list))

	rows := make([]string, 0, end-start+2)
	if start > 0 {
		rows = append(rows, subtleStyle.Render("... older versions above ..."))
	}

	for i := start; i < end; i++ {
		version := list[i]
		prefix := "  "
		isCursor := i == m.cursor
		isActive := version == m.activeVersion

		if isCursor {
			prefix = "> "
		}
		line := prefix + version
		if isActive {
			line += "  [active]"
		}

		switch {
		case isActive && isCursor:
			line = activeCursorStyle.Render(line)
		case isActive:
			line = activeStyle.Render(line)
		case isCursor:
			line = cursorStyle.Render(line)
		}

		rows = append(rows, line)
	}

	if end < len(list) {
		rows = append(rows, subtleStyle.Render("... more versions below ..."))
	}

	body := strings.Join(rows, "\n")

	if len(m.currentList()) > 0 {
		position := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.currentList()))
		body += "\n" + subtleStyle.Render(position)
	}

	status := subtleStyle.Render(m.status)
	if m.busy {
		status = fmt.Sprintf("%s %s", m.spinner.View(), subtleStyle.Render(m.status))
	}

	footer := status
	if m.lastError != "" {
		footer += "\n" + errorStyle.Render(m.lastError)
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", header, meta, body, footer)
}

func (m *model) pageSize() int {
	if m.height <= 0 {
		return 15
	}

	reserved := 9
	if m.lastError != "" {
		reserved++
	}

	size := m.height - reserved
	if size < 5 {
		size = 5
	}

	return size
}

func (m *model) clampCursor() {
	list := m.currentList()
	if len(list) == 0 {
		m.cursor = 0
		m.listOffset = 0
		return
	}

	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(list) {
		m.cursor = len(list) - 1
	}

	m.ensureCursorVisible()
}

func (m *model) ensureCursorVisible() {
	list := m.currentList()
	if len(list) == 0 {
		m.listOffset = 0
		return
	}

	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(list) {
		m.cursor = len(list) - 1
	}

	pageSize := m.pageSize()
	if m.listOffset > m.cursor {
		m.listOffset = m.cursor
	}
	if m.cursor >= m.listOffset+pageSize {
		m.listOffset = m.cursor - pageSize + 1
	}

	maxOffset := len(list) - pageSize
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.listOffset > maxOffset {
		m.listOffset = maxOffset
	}
	if m.listOffset < 0 {
		m.listOffset = 0
	}
}

func (m model) visibleRange(pageSize int, total int) (int, int) {
	if total <= 0 {
		return 0, 0
	}

	start := m.listOffset
	if start < 0 {
		start = 0
	}
	if start >= total {
		start = total - 1
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return start, end
}
