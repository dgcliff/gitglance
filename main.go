package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// defaultRepos are the two DRS checkouts this tool was built to watch.
var defaultRepos = []Repo{
	{Name: "Cerberus", Path: "/home/nakatomi/projects/cerberus"},
	{Name: "Atlas", Path: "/home/nakatomi/projects/atlas"},
}

func main() {
	author := flag.String("author", "", "filter charts/commits to this author name (default: git user.name; \"*\" for everyone)")
	weeks := flag.Int("weeks", 0, "cap the activity chart at this many weeks (0 = auto: fill the pane width)")
	every := flag.Duration("every", time.Minute, "auto-refresh interval")
	pal := flag.String("palette", defaultPalette, "bar colour scheme: plasma (vivid) or muted (calm)")
	dump := flag.Bool("dump", false, "render one static frame to stdout and exit (no TTY needed)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "gitglance — at-a-glance commit + activity overview for two repos\n\n")
		fmt.Fprintf(os.Stderr, "Usage: gitglance [flags] [Name=/path/to/repo ...]\n\n")
		fmt.Fprintf(os.Stderr, "With no repo args, defaults to Cerberus and Atlas.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Render true 24-bit colour rather than letting termenv auto-detect a lower
	// profile. Without this, terminals that don't advertise truecolor get the
	// greens downsampled to the 16-colour palette, where the darker greens
	// quantise to ANSI cyan — breaking the Less→More ramp into a teal jump.
	lipgloss.SetColorProfile(termenv.TrueColor)

	repos, err := parseRepos(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitglance:", err)
		os.Exit(1)
	}

	resolvedAuthor := *author
	switch resolvedAuthor {
	case "":
		resolvedAuthor = detectAuthor(repos)
	case "*":
		resolvedAuthor = "" // everyone
	}

	if *weeks < 0 {
		*weeks = 0 // 0 = auto (fill pane width)
	}

	m := newModel(repos, resolvedAuthor, *weeks, *every, resolvePalette(*pal))

	if *dump {
		dumpFrame(m)
		return
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "gitglance:", err)
		os.Exit(1)
	}
}

// parseRepos turns "Name=/path" args into Repos. Exactly two are tracked (the
// layout is two-up / two-down); extras are ignored, fewer falls back to defaults.
func parseRepos(args []string) ([]Repo, error) {
	if len(args) == 0 {
		return defaultRepos, nil
	}
	var repos []Repo
	for _, a := range args {
		name, path, ok := strings.Cut(a, "=")
		if !ok {
			path = name
			name = lastSegment(path)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("repo path not found: %s", path)
		}
		repos = append(repos, Repo{Name: name, Path: path})
		if len(repos) == 2 {
			break
		}
	}
	for len(repos) < 2 {
		repos = append(repos, defaultRepos[len(repos)])
	}
	return repos, nil
}

// dumpFrame loads data synchronously and prints one rendered frame, so the
// chart + commit layout can be eyeballed without an interactive terminal.
func dumpFrame(m model) {
	m.width, m.height = 100, 44
	m.ready = true
	m.layout()
	for i := range m.repos {
		if msg, ok := m.loadRepo(i)().(loadedMsg); ok {
			m.commits[i] = msg.commits
			m.commitErr[i] = msg.commitErr
			m.counts[i] = msg.counts
			m.countErr[i] = msg.countErr
			m.loaded[i] = true
			m.setViewportContent(i)
		}
	}
	m.lastUpdate = time.Now()
	fmt.Println(m.View())
}

func lastSegment(path string) string {
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}
