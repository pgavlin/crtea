# crtea

An interactive terminal code review tool built with [Bubble Tea](https://charm.land/bubbletea).

This project is modeled after [tuicr](https://github.com/agavra/tuicr).

## Features

- **Commit picker** — select working tree changes and/or individual commits to review
- **Unified diff viewer** — syntax-highlighted, scrollable diff with context expansion
- **File tree** — collapsible directory tree for navigating changed files
- **Inline comments** — add notes, todos, and issues anchored to specific lines or ranges
- **Review sessions** — persistent sessions that track reviewed files and comments
- **Markdown export** — export review notes to clipboard as Markdown
- **Per-commit toggles** — enable/disable individual commits to focus your review
- **Review description** — auto-generated squash-style summary from commit messages
- **Themes** — dark, light, glamour-dark, glamour-light, with auto-detection

## Install

```
go install github.com/pgavlin/crtea@latest
```

## Usage

```
crtea [options]
```

With no flags, crtea opens a commit picker where you can select which commits
and/or working tree changes to review.

### Options

| Flag | Description |
|------|-------------|
| `-r, --revisions REV` | Review a specific revision range (e.g. `main~5..HEAD`) |
| `-t, --theme THEME` | Color theme: `dark`, `light`, `glamour-dark`, `glamour-light` (auto-detected if omitted) |

### Keybindings

**Navigation**

| Key | Action |
|-----|--------|
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `h` / `Left` | Scroll left |
| `l` / `Right` | Scroll right |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `{N}G` | Jump to line N |
| `Tab` | Switch focus between panels |
| `f` | Toggle file list |
| `D` | Toggle description / commit list |

**Review**

| Key | Action |
|-----|--------|
| `Space` | Toggle file reviewed / toggle commit |
| `c` | Add comment on current line |
| `C` | Add file-level comment |
| `v` | Start visual line selection for range comments |
| `e` | Expand context around diff gap |

**Other**

| Key | Action |
|-----|--------|
| `:` | Command mode |
| `/` | Search |
| `n` | Next search match |
| `?` | Help |
| `y` | Copy review as Markdown to clipboard |
| `q` | Quit |

## Embedding

The `ui` package exposes `App` as a reusable Bubble Tea component. Create one
with `ui.NewApp` or `ui.NewPickerApp` and embed it in your own `tea.Model`:

```go
import (
    "github.com/pgavlin/crtea/ui"
    "github.com/pgavlin/crtea/vcs"
    "github.com/pgavlin/crtea/theme"
)

// In your Update method, handle these messages from the component:
//   ui.DoneMsg     — user finished; Session field has the final review state
//   ui.ClipboardMsg — user wants to copy text to clipboard

app := ui.NewPickerApp(backend, theme.Dark(), highlighter, store)
app.SetSize(width, height)
```

## License

See [LICENSE](LICENSE) for details.
