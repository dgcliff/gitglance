package main

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const dayKey = "2006-01-02"

// brailleBits maps a dot at [row][col] (row 0 = top, col 0 = left) of a braille
// cell's 2×4 grid to its bit in the U+2800 block.
var brailleBits = [4][2]byte{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

// palette is a height-ramp for the bars: a low→high list of colour stops plus a
// floor that lifts the usable bottom off the darkest stops (so they don't wash
// out against a dark background).
type palette struct {
	stops []lipgloss.Color
	floor float64
}

// palettes are the selectable bar colour schemes (see --palette).
var palettes = map[string]palette{
	// "plasma": the matplotlib plasma colormap (indigo→magenta→orange→yellow),
	// floored above its darkest blues which collide with a dark-navy terminal bg.
	"plasma": {
		stops: []lipgloss.Color{
			"#0d0887", "#46039f", "#7201a8", "#9c179e", "#bd3786",
			"#d8576b", "#ed7953", "#fa9e3b", "#fdc926", "#f0f921",
		},
		floor: 0.30,
	},
	// "muted": a calm, low-saturation ramp — slate-mauve at the baseline through
	// dusty taupe to a soft gold at the peaks. Light enough to read on navy, but
	// far quieter than plasma; only the busiest days warm up.
	"muted": {
		stops: []lipgloss.Color{
			"#6f6a82", "#807a90", "#938a93", "#a68f8a",
			"#b6977c", "#c2a06e", "#cbab63", "#d3b65d",
		},
		floor: 0.0,
	},
}

const defaultPalette = "muted"

// resolvePalette returns the named palette, falling back to the default.
func resolvePalette(name string) palette {
	if p, ok := palettes[name]; ok {
		return p
	}
	return palettes[defaultPalette]
}

// barColor samples a palette at t in [0,1] (0 = baseline, 1 = peak), rescaled so
// the usable bottom is the palette's floor.
func barColor(t float64, p palette) lipgloss.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	t = p.floor + t*(1-p.floor)
	idx := int(t*float64(len(p.stops)-1) + 0.5)
	return p.stops[idx]
}

var (
	chartTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#39d353"))
	chartMetaStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#768390"))
)

// maxLookbackDays caps how far back we ever query/show (~53 weeks), so an
// enormous terminal can't ask for an unbounded git history.
const maxLookbackDays = 372

// midnightToday returns local midnight at the start of the current day.
func midnightToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

// fetchDays is how many days of daily counts the loader pulls. With weeks <= 0
// (auto) it fetches the full lookback so the chart can fill any terminal width;
// a positive --weeks caps it. The chart packs two days per character (braille is
// two dots wide), so a pane W chars wide shows up to 2W days.
func fetchDays(weeks int) int {
	if weeks <= 0 {
		return maxLookbackDays
	}
	if d := weeks * 7; d < maxLookbackDays {
		return d
	}
	return maxLookbackDays
}

// lookbackStart is the `git log --since` bound for a given day span.
func lookbackStart(days int) time.Time {
	return midnightToday().AddDate(0, 0, -(days - 1))
}

// renderBars draws a repo's recent activity as a braille bar chart filling barH
// rows and the full available width: one bar per dot-column (two days per
// character), oldest left / today right, so the window length tracks the pane
// width — wider terminal, more history. weeks > 0 caps the span. Bars rise from
// the baseline scaled to that repo's busiest day, coloured by height via the
// plasma ramp (dark indigo low, bright yellow at the peaks).
func renderBars(name string, counts map[string]int, weeks, availWidth, barH int, pal palette) string {
	if barH < 1 {
		barH = 1
	}
	days := availWidth * 2 // two braille dot-columns per character cell
	if weeks > 0 && days > weeks*7 {
		days = weeks * 7
	}
	if days > maxLookbackDays {
		days = maxLookbackDays
	}
	if days < 1 {
		days = 1
	}

	dotW := days
	dotH := barH * 4
	chars := (dotW + 1) / 2

	start := lookbackStart(days)
	total, maxV := 0, 0
	vals := make([]int, days)
	for i := 0; i < days; i++ {
		c := counts[start.AddDate(0, 0, i).Format(dayKey)]
		vals[i] = c
		total += c
		if c > maxV {
			maxV = c
		}
	}
	if maxV < 1 {
		maxV = 1
	}

	// Bar height per column, measured in dots from the baseline.
	colDots := make([]int, dotW)
	for x, v := range vals {
		if v <= 0 {
			continue
		}
		h := (v*dotH + maxV - 1) / maxV // ceil(v/maxV * dotH)
		if h < 1 {
			h = 1
		}
		if h > dotH {
			h = dotH
		}
		colDots[x] = h
	}

	// One plasma colour per character row, spanning the full ramp: the bottom row
	// is deep indigo, the top row bright yellow.
	rowStyle := make([]lipgloss.Style, barH)
	for cy := 0; cy < barH; cy++ {
		t := 1.0
		if barH > 1 {
			t = float64(barH-1-cy) / float64(barH-1)
		}
		rowStyle[cy] = lipgloss.NewStyle().Foreground(barColor(t, pal))
	}

	rows := make([]string, barH)
	for cy := 0; cy < barH; cy++ {
		var b strings.Builder
		for cx := 0; cx < chars; cx++ {
			var mask byte
			for col := 0; col < 2; col++ {
				x := cx*2 + col
				if x >= dotW || colDots[x] == 0 {
					continue
				}
				filledFrom := dotH - colDots[x] // topmost filled global dot row
				for row := 0; row < 4; row++ {
					if cy*4+row >= filledFrom {
						mask |= brailleBits[row][col]
					}
				}
			}
			if mask == 0 {
				b.WriteByte(' ')
			} else {
				b.WriteString(rowStyle[cy].Render(string(rune(0x2800 + int(mask)))))
			}
		}
		rows[cy] = b.String()
	}

	title := chartTitleStyle.Render(name) +
		chartMetaStyle.Render("  ·  "+plural(total, "commit"))
	axis := chartMetaStyle.Render(itoa(days) + " days → today")

	return title + "\n" + strings.Join(rows, "\n") + "\n" + axis
}
