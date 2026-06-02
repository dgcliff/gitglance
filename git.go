package main

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Repo is a single tracked git checkout.
type Repo struct {
	Name string
	Path string
}

// Commit is a single author commit, newest first in the lists we build.
type Commit struct {
	Hash    string
	When    time.Time
	Subject string
}

// gitStrftime is the strftime format passed to git's `--date=format:`, and
// gitTimeLayout is the matching Go layout used to parse what comes back. They
// describe the same instant in the two libraries' incompatible notations.
const (
	gitStrftime   = "%Y-%m-%d %H:%M"
	gitTimeLayout = "2006-01-02 15:04"
)

// runGit shells out to `git -C <path> <args...>` and returns stdout.
func runGit(path string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
	out, err := cmd.Output()
	return string(out), err
}

// detectAuthor reads `git config user.name` from the first usable repo so the
// charts default to "my commits" without the user having to pass --author.
func detectAuthor(repos []Repo) string {
	for _, r := range repos {
		if out, err := runGit(r.Path, "config", "user.name"); err == nil {
			if name := strings.TrimSpace(out); name != "" {
				return name
			}
		}
	}
	return ""
}

// recentCommits returns up to limit commits authored by `author` on the current
// branch, newest first. An empty author means "everyone".
func recentCommits(r Repo, author string, limit int) ([]Commit, error) {
	args := []string{
		"log",
		"-n", strconv.Itoa(limit),
		"--no-merges",
		"--date=format:" + gitStrftime,
		"--pretty=format:%h\x1f%ad\x1f%s",
	}
	if author != "" {
		args = append(args, "--author="+author)
	}
	out, err := runGit(r.Path, args...)
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x1f", 3)
		if len(parts) != 3 {
			continue
		}
		when, _ := time.ParseInLocation(gitTimeLayout, parts[1], time.Local)
		commits = append(commits, Commit{
			Hash:    parts[0],
			When:    when,
			Subject: parts[2],
		})
	}
	return commits, nil
}

// dailyCounts returns a map of "YYYY-MM-DD" -> commit count for commits authored
// by `author` since the given date, used to drive the activity bar chart.
func dailyCounts(r Repo, author string, since time.Time) (map[string]int, error) {
	args := []string{
		"log",
		"--no-merges",
		"--since=" + since.Format("2006-01-02"),
		"--date=format:%Y-%m-%d",
		"--pretty=format:%ad",
	}
	if author != "" {
		args = append(args, "--author="+author)
	}
	out, err := runGit(r.Path, args...)
	if err != nil {
		return nil, err
	}

	counts := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		counts[line]++
	}
	return counts, nil
}
