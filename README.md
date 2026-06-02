# gitglance

An at-a-glance terminal overview of two git repositories: the kind of thing you
glance at to see "am I making progress?" without opening a browser.

```
┌─ Cerberus ──────────────┐┌─ Atlas ─────────────────┐  top 70%: two side-by-
│ 3d68e26 06-02 …         ││ ef85c65 06-01 …         │  side scrollable commit
│ 0cf2570 06-02 …         ││ 469d939 06-01 …         │  lists, newest first
│ …                       ││ …                       │
└─────────────────────────┘└─────────────────────────┘
┌─ Cerberus · 419 commits ┐┌─ Atlas · 201 commits ───┐  bottom 30%: per-repo
│            ⢸     ⡆⡆      ││         ⢸   ⡇           │  braille daily-commit
│      ⣿ ⣷⢸ ⣿⡇⣼ ⣿⣿         ││  ⡆ ⡆ ⡄⡀⢰⡇ ⢸⢠⡇ ⣿ ⢠⢸      │  bar charts, plasma
│ ⣧ ⣸⡄ ⣿⢸⣿⢸⢸ ⣿⡇⣿ ⣿⣿ ⣿     ││ ⢠ ⡄ ⢰⣧ ⣧ ⡇⡇⢸⣷ ⢸⢸⣷ ⣷⢸ ⣿ │  colour-by-height
│ 92 days → today         ││ 92 days → today         │
└─────────────────────────┘└─────────────────────────┘
```

- **Top (70%)** — two side-by-side panes, one commit log per repo (viewport
  pagers: arrows/`j``k`, pgup/pgdn, mouse wheel; scroll counter beneath).
- **Bottom (30%)** — a vertical daily-commit **braille** bar chart per repo
  (oldest left, today right) filling the pane, scaled to that repo's busiest day
  and coloured by height (`--palette`): the default **muted** ramp runs
  slate-mauve at the baseline up to soft gold at the peaks; **plasma** is a vivid
  indigo→yellow alternative (its floor lifted off the darkest blues so low bars
  don't wash out on a dark background). Braille
  packs two days per character column, so the window grows to ~2× the pane width
  in days; `--weeks` caps the span. The pane title carries the window total.
- Data comes straight from the local checkouts via `git log` — offline, no auth.
- Auto-refreshes on an interval (default 60s); press `r` to refresh now.

## Build & run

```sh
go build -o gitglance ./...
./gitglance                 # defaults to Cerberus + Atlas
```

By default it watches:

- `Cerberus = /home/nakatomi/projects/cerberus`
- `Atlas    = /home/nakatomi/projects/atlas`

Track different repos by passing up to two `Name=/path` args:

```sh
./gitglance Frontend=~/code/web Backend=~/code/api
```

## Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--author` | `git config user.name` | Filter charts/commits to one author. `*` = everyone. |
| `--weeks`  | `0` (auto) | Cap the activity chart at this many weeks. `0` lets the window grow to fill the pane width (one bar per day). |
| `--palette` | `muted` | Bar colour scheme: `muted` (calm slate-mauve→soft gold) or `plasma` (vivid indigo→yellow, floored for dark backgrounds). |
| `--every`  | `1m` | Auto-refresh interval. |
| `--dump`   | off | Render one static frame to stdout and exit (no TTY needed). |

The author filter matches by *name*, so commits made under multiple emails by the
same person are counted together.

## Keys

| Key | Action |
|-----|--------|
| `tab` / `shift+tab` / `←` `→` / `h` `l` | switch which commit column is focused |
| `j` / `k` / `↑` `↓` | scroll the focused commit list |
| `pgup` / `pgdn` / `u` `d` / mouse wheel | page / half-page scroll the focused list |
| `g` / `home` | jump to top |
| `r` | refresh now |
| `q` / `esc` / `ctrl+c` | quit |

The keybinding footer is generated from the keymap (`bubbles/help` + `key`) and
truncates itself on narrow terminals. Commit lists are `bubbles/viewport`
pagers; rows are laid out with `lipgloss/table`; loading state shows a spinner.

## Notes

- Commits are read from each repo's **current branch** (`HEAD`), excluding merges.
  Run `git fetch` in the checkouts to keep the view current with the remote.
- The chart window grows to fill the pane width (braille packs two days per
  character, capped at ~53 weeks); pass `--weeks N` to pin a shorter span. Bar
  heights are scaled per-repo (relative to that repo's busiest day), so they show
  each repo's own rhythm — compare absolute volume via the commit total in each
  title. Bars need a font with braille glyphs (most modern monospace/Nerd Fonts
  have them) and a truecolor terminal for the plasma gradient.
