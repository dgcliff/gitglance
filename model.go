package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// ---- messages ---------------------------------------------------------------

type tickMsg time.Time

// loadedMsg carries one repo's data. The commit-log and daily-count fetches
// fail independently so a count error never wipes a good commit list.
type loadedMsg struct {
	idx       int
	commits   []Commit
	commitErr error
	counts    map[string]int
	countErr  error
}

// ---- key map ----------------------------------------------------------------

type keyMap struct {
	Focus   key.Binding
	Up      key.Binding
	Down    key.Binding
	Top     key.Binding
	Refresh key.Binding
	Quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Focus, k.Up, k.Down, k.Top, k.Refresh, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Focus}, {k.Up, k.Down, k.Top}, {k.Refresh, k.Quit}}
}

var keys = keyMap{
	Focus:   key.NewBinding(key.WithKeys("tab", "shift+tab", "left", "right", "h", "l"), key.WithHelp("tab/←→", "focus")),
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Top:     key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Quit:    key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
}

// ---- model ------------------------------------------------------------------

type model struct {
	repos   []Repo
	author  string
	weeks   int
	every   time.Duration
	palette palette

	commits   [2][]Commit
	counts    [2]map[string]int
	commitErr [2]error
	countErr  [2]error
	loaded    [2]bool

	vps  [2]viewport.Model
	spin spinner.Model
	help help.Model
	keys keyMap

	focus      int
	width      int
	height     int
	ready      bool
	lastUpdate time.Time
}

func newModel(repos []Repo, author string, weeks int, every time.Duration, pal palette) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#539bf5"))

	h := help.New()

	return model{
		repos:   repos,
		author:  author,
		weeks:   weeks,
		every:   every,
		palette: pal,
		spin:    s,
		help:    h,
		keys:    keys,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.loadRepo(0), m.loadRepo(1), m.scheduleTick())
}

// loadRepo reads commits + daily counts for one repo off the UI thread, keeping
// the two failure modes separate.
func (m model) loadRepo(idx int) tea.Cmd {
	if idx >= len(m.repos) {
		return nil
	}
	repo, author, weeks := m.repos[idx], m.author, m.weeks
	return func() tea.Msg {
		commits, cErr := recentCommits(repo, author, 200)
		counts, kErr := dailyCounts(repo, author, lookbackStart(fetchDays(weeks)))
		return loadedMsg{idx: idx, commits: commits, commitErr: cErr, counts: counts, countErr: kErr}
	}
}

func (m model) scheduleTick() tea.Cmd {
	return tea.Tick(m.every, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) reloadAll() tea.Cmd {
	return tea.Batch(m.loadRepo(0), m.loadRepo(1))
}

func (m *model) markReloading() {
	for i := range m.repos {
		m.loaded[i] = false
	}
}

func (m model) allLoaded() bool {
	for i := range m.repos {
		if !m.loaded[i] {
			return false
		}
	}
	return true
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.layout()
		return m, nil

	case loadedMsg:
		if msg.idx < len(m.commits) {
			m.commits[msg.idx] = msg.commits
			m.commitErr[msg.idx] = msg.commitErr
			m.counts[msg.idx] = msg.counts
			m.countErr[msg.idx] = msg.countErr
			m.loaded[msg.idx] = true
			m.setViewportContent(msg.idx)
		}
		m.lastUpdate = time.Now()
		return m, nil

	case tickMsg:
		m.markReloading()
		return m, tea.Batch(m.spin.Tick, m.reloadAll(), m.scheduleTick())

	case spinner.TickMsg:
		if m.allLoaded() {
			return m, nil // stop animating once everything is in
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case tea.MouseMsg:
		var cmd tea.Cmd
		m.vps[m.focus], cmd = m.vps[m.focus].Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Refresh):
			m.markReloading()
			return m, tea.Batch(m.spin.Tick, m.reloadAll())
		case key.Matches(msg, m.keys.Focus):
			m.focus = (m.focus + 1) % len(m.repos)
			return m, nil
		case key.Matches(msg, m.keys.Top):
			m.vps[m.focus].GotoTop()
			return m, nil
		}
		// Anything else (up/down/j/k/pgup/pgdn) drives the focused viewport.
		var cmd tea.Cmd
		m.vps[m.focus], cmd = m.vps[m.focus].Update(msg)
		return m, cmd
	}
	return m, nil
}

// ---- layout / sizing --------------------------------------------------------

// colWidth returns the outer width of column idx, giving the last column any
// odd-width remainder so the panes fill the full screen width.
func (m model) colWidth(idx int) int {
	base := m.width / len(m.repos)
	if idx == len(m.repos)-1 {
		return m.width - base*(len(m.repos)-1)
	}
	return base
}

// splitHeights divides the space between header and footer 70/30 between the
// commit panes (top) and the activity charts (bottom).
func (m model) splitHeights() (top, bottom int) {
	avail := m.height - 1 /*header*/ - lipgloss.Height(m.footerView())
	if avail < 8 {
		avail = 8
	}
	bottom = avail * 30 / 100
	if bottom < 4 { // border(2) + title + axis is the floor for a chart pane
		bottom = 4
	}
	top = avail - bottom
	if top < 4 {
		top = 4
	}
	return top, bottom
}

// commitPaneHeight is the outer height of the top commit panes (the 70% band).
func (m model) commitPaneHeight() int {
	top, _ := m.splitHeights()
	return top
}

// layout (re)sizes each viewport to fit its pane after a resize, and refreshes
// the help width so the footer truncates correctly.
func (m *model) layout() {
	m.help.Width = m.width
	paneH := m.commitPaneHeight()
	for i := range m.repos {
		// box = outer - border(2); text area = box - padding(2); the viewport
		// gets the box height minus the title row and the scroll-counter row.
		boxW := m.colWidth(i) - 2
		m.vps[i].Width = boxW - 2
		m.vps[i].Height = paneH - 2 - 1 - 1
		if m.vps[i].Height < 1 {
			m.vps[i].Height = 1
		}
		m.setViewportContent(i)
	}
}

// setViewportContent fills a viewport with the commit table once its repo has
// loaded; while loading/errored/empty the pane body is drawn separately, so the
// viewport content is cleared.
func (m *model) setViewportContent(idx int) {
	if !m.loaded[idx] || m.commitErr[idx] != nil || len(m.commits[idx]) == 0 {
		m.vps[idx].SetContent("")
		return
	}
	m.vps[idx].SetContent(m.commitTable(idx))
}

// commitTable renders the commit rows as a borderless three-column table so the
// width/alignment is owned by lipgloss rather than hand-tuned padding. Subjects
// are pre-truncated to keep one commit per line (so the scroll counter is exact).
func (m model) commitTable(idx int) string {
	subjW := m.vps[idx].Width - (7 + 2 + 5 + 2) // hash + pad + date + pad
	if subjW < 6 {
		subjW = 6
	}
	t := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderRow(false).BorderHeader(false).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch col {
			case 0:
				return cellHashStyle
			case 1:
				return cellDateStyle
			default:
				return lipgloss.NewStyle()
			}
		})
	for _, c := range m.commits[idx] {
		t.Row(shortHash(c.Hash), c.When.Format("01-02"), truncate(c.Subject, subjW))
	}
	return t.Render()
}

// ---- styles -----------------------------------------------------------------

var (
	appNameStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#39d353"))
	metaStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#768390"))

	cellHashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#539bf5")).PaddingRight(2)
	cellDateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#768390")).PaddingRight(2)
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#e5534b"))

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#373e47")).
			Padding(0, 1)
	paneFocusStyle  = paneStyle.BorderForeground(lipgloss.Color("#539bf5"))
	paneTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#768390"))
	focusTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#539bf5"))
)

func (m model) View() string {
	if !m.ready {
		return "loading…"
	}

	header := m.headerView()
	top := m.renderCommitColumns(m.commitPaneHeight())
	charts := m.renderCharts()
	footer := m.footerView()

	return strings.Join([]string{header, top, charts, footer}, "\n")
}

func (m model) headerView() string {
	left := appNameStyle.Render("gitglance") + metaStyle.Render("  ·  "+authorLabel(m.author))
	right := metaStyle.Render("updated " + updatedLabel(m.lastUpdate) + "  ·  every " + m.every.String())
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m model) footerView() string {
	return m.help.View(m.keys)
}

func (m model) renderCommitColumns(height int) string {
	cols := make([]string, len(m.repos))
	for i := range m.repos {
		cols[i] = m.renderCommitPane(i, m.colWidth(i), height)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}

func (m model) renderCommitPane(idx, outerW, outerH int) string {
	style := paneStyle
	titleStyle := paneTitleStyle
	marker := "  "
	if idx == m.focus {
		style = paneFocusStyle
		titleStyle = focusTitleStyle
		marker = "▸ "
	}
	boxW := outerW - 2
	if boxW < 10 {
		boxW = 10
	}
	textW := boxW - 2

	title := titleStyle.Render(marker + m.repos[idx].Name)

	var body string
	switch {
	case !m.loaded[idx]:
		body = m.spin.View() + metaStyle.Render(" loading…")
	case m.commitErr[idx] != nil:
		body = errStyle.Render(truncate("git error: "+m.commitErr[idx].Error(), textW))
	case len(m.commits[idx]) == 0:
		body = metaStyle.Render("no commits by " + authorLabel(m.author))
	default:
		body = m.vps[idx].View() + "\n" + m.scrollHint(idx)
	}

	content := title + "\n" + body
	return style.Width(boxW).Height(outerH - 2).Render(content)
}

// scrollHint renders the "12–28 of 200" position line beneath a commit list.
func (m model) scrollHint(idx int) string {
	total := len(m.commits[idx])
	first := m.vps[idx].YOffset + 1
	last := min(m.vps[idx].YOffset+m.vps[idx].Height, total)
	if total == 0 {
		return ""
	}
	return metaStyle.Render(fmt.Sprintf("  %d–%d of %d", first, last, total))
}

func (m model) renderCharts() string {
	// Each chart sits in a bordered pane the width of the commit pane above it,
	// filling the bottom 30% band — balanced chrome with the top quadrants.
	_, bottomH := m.splitHeights()
	charts := make([]string, len(m.repos))
	for i := range m.repos {
		charts[i] = m.renderChartPane(i, m.colWidth(i), bottomH)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, charts...)
}

func (m model) renderChartPane(idx, outerW, outerH int) string {
	boxW := outerW - 2
	if boxW < 10 {
		boxW = 10
	}
	textW := boxW - 2
	barH := outerH - 2 /*border*/ - 1 /*title*/ - 1 /*axis*/
	if barH < 1 {
		barH = 1
	}
	chart := renderBars(m.repos[idx].Name, m.counts[idx], m.weeks, textW, barH, m.palette)
	return paneStyle.Width(boxW).Height(outerH - 2).Render(chart)
}

func authorLabel(a string) string {
	if a == "" {
		return "everyone"
	}
	return a
}

func updatedLabel(t time.Time) string {
	if t.IsZero() {
		return "…"
	}
	return t.Format("15:04:05")
}
