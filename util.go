package main

import "strconv"

func itoa(n int) string { return strconv.Itoa(n) }

// plural renders "1 commit" / "3 commits".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return itoa(n) + " " + word + "s"
}

// padRight pads s with spaces to width n (no truncation; labels are short).
func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}

// truncate shortens s to n display runes, appending "…" when clipped.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// shortHash clamps a git abbreviated hash to a stable 7 columns; repos differ on
// default abbrev length (Cerberus emits 8), and the commit list assumes 7.
func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return padRight(h, 7)
}
