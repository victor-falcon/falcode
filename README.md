# falcode

A terminal multiplexer for multi-agent git worktree workflows.

Each outer tab (workspace) maps to a git worktree discovered in the current repository. Each workspace has inner tabs that run configurable tools — AI coding agent, lazygit, or an interactive shell — each in its own PTY. The workflow it enables: one worktree per feature branch, each with its own agent running in parallel.

```
┌─[ main ]──[ feature/auth ]──[ feature/api ]─────────────────────────────┐
│ [ Agent ]  [ Git ]  [ Console ]  [+]                                     │
│                                                                           │
│  opencode running in this worktree's PTY                                 │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

## Prerequisites

- **Go 1.21+** — [install](https://go.dev/dl/)
- **Git** — worktree discovery requires a git repository
- **opencode** *(optional)* — used by the default Agent tab
- **lazygit** *(optional)* — used by the default Git tab

## Installation

### Homebrew (macOS / Linux)

```sh
brew tap victor-falcon/falcode
brew install falcode
```

### go install

```sh
go install github.com/victor-falcon/falcode@latest
```

### Download binary

Download the latest pre-built binary for your platform from the [GitHub Releases](https://github.com/victor-falcon/falcode/releases) page, extract it, and move it to a directory in your `$PATH`.

## Usage

Run from inside any git repository:

```sh
falcode
```

falcode discovers all git worktrees in the repository and opens one workspace tab per worktree. Each workspace starts with the configured inner tabs (Agent, Git, Console by default).

Mouse clicks on tabs are supported in addition to keybinds.

When a command tab (Agent, Git) exits, a restart banner appears at the bottom of the pane — press `Enter` to relaunch it.

## Default Keybinds

The default prefix key is `Ctrl+B` (tmux-style). Press the prefix to enter command mode, then:

| Sequence | Action |
|----------|--------|
| `Ctrl+B` `q` | Quit |
| `Ctrl+B` `Ctrl+B` | Send prefix key through to the active pane |
| `Ctrl+B` `t` `l` | Next inner tab |
| `Ctrl+B` `t` `h` | Previous inner tab |
| `Ctrl+B` `t` `n` | New console tab |
| `Ctrl+B` `t` `x` | Close current tab |
| `Ctrl+B` `w` `l` | Next workspace |
| `Ctrl+B` `w` `h` | Previous workspace |
| `Ctrl+B` `w` `n` | Create new workspace |
| `Ctrl+B` `w` `x` | Delete current workspace |

Press `Esc` at any point to cancel the current prefix sequence and return to normal mode.

Navigation keys (`l` / `h`) keep you in the sub-layer so you can press them repeatedly without re-entering the prefix. Structural actions (`n` / `x`) automatically return to normal mode after executing.

## Configuration

falcode looks for configuration in this order:

1. `./falcode.json` — project-local (checked into the repo)
2. `~/.config/falcode/config.json` — user-global
3. Built-in defaults

To bootstrap a user config file:

```sh
falcode write-default-config
```

This writes the built-in defaults to `~/.config/falcode/config.json` which you can then edit.

### config.json

```json
{
  "tabs": [
    { "name": "Agent", "command": "opencode" },
    { "name": "Git",   "command": "lazygit" },
    { "name": "Console" }
  ],
  "ui": {
    "theme": "default",
    "theme_scheme": "system",
    "hide_footer": false,
    "new_tab_button": true,
    "new_workspace_button": true,
    "close_tab_button": "focus",
    "close_workspace_button": "none",
    "compact_tabs": false
  },
  "keybinds": {
    "prefix": "ctrl+b",
    "bindings": []
  }
}
```

Tabs with no `"command"` run an interactive `$SHELL`.

### UI options

| Option | Values | Default | Description |
|--------|--------|---------|-------------|
| `theme` | string | `"default"` | Name of the theme to load from `~/.config/falcode/themes/` |
| `theme_scheme` | `"system"` `"dark"` `"light"` | `"system"` | Color scheme — `"system"` detects macOS appearance at launch |
| `hide_footer` | bool | `false` | Hide the bottom status/hint bar |
| `new_tab_button` | bool | `true` | Show the `[+]` new-tab button in the inner tab bar |
| `new_workspace_button` | bool | `true` | Show the `[+]` new-workspace button in the workspace bar |
| `close_tab_button` | `"all"` `"focus"` `"none"` | `"focus"` | Show `[x]` close button on all tabs, only the focused one, or none |
| `close_workspace_button` | `"all"` `"focus"` `"none"` | `"none"` | Show `[x]` close button on workspace tabs |
| `compact_tabs` | bool | `false` | Merge the workspace bar and inner tab bar into a single row |

### Keybinds

Keybinds live under the `"keybinds"` key in `config.json`. The full structure:

```json
{
  "keybinds": {
    "prefix": "ctrl+b",
    "bindings": [
      { "key": "q", "description": "Quit", "action": "quit" },
      {
        "key": "t", "description": "Tab",
        "bindings": [
          { "key": "l", "description": "Next tab",     "action": "next_tab" },
          { "key": "h", "description": "Prev tab",     "action": "prev_tab" },
          { "key": "n", "description": "New console",  "actions": ["new_tab", "lock"] },
          { "key": "x", "description": "Close tab",    "actions": ["close_tab", "lock"] }
        ]
      },
      {
        "key": "w", "description": "Workspace",
        "bindings": [
          { "key": "l", "description": "Next workspace",   "action": "next_workspace" },
          { "key": "h", "description": "Prev workspace",   "action": "prev_workspace" },
          { "key": "n", "description": "New workspace",    "actions": ["new_workspace", "lock"] },
          { "key": "x", "description": "Delete workspace", "actions": ["delete_workspace", "lock"] }
        ]
      }
    ]
  }
}
```

Available action names: `quit`, `next_tab`, `prev_tab`, `new_tab`, `close_tab`, `next_workspace`, `prev_workspace`, `new_workspace`, `delete_workspace`, `passthrough`, `go_to_tab`, `go_to_workspace`, `lock`.

The `lock` action exits command mode. Omit it from navigation bindings to stay in the sub-layer and press the key repeatedly.

Use `"action"` for a single action or `"actions"` to chain multiple actions.

## Themes

falcode ships with a built-in dark-purple theme. You can create and share custom themes as plain JSON files.

### Create a custom theme

1. Export the built-in theme as a starting point:

   ```sh
   falcode write-default-theme mytheme
   ```

   This writes `~/.config/falcode/themes/mytheme.json`.

2. Edit the file — change any color values you want. Colors are `#RRGGBB` hex strings or named aliases defined in the `"defs"` block. Use `"transparent"` to inherit the terminal background.

3. Activate it in `config.json`:

   ```json
   { "ui": { "theme": "mytheme" } }
   ```

### Theme file format

```json
{
  "defs": {
    "accent": "#7B61FF",
    "accent_dim": "#5B41DF"
  },
  "colors": {
    "workspace_active":    { "dark": "accent",    "light": "accent_dim" },
    "workspace_active_fg": { "dark": "#FFFFFF",   "light": "#FFFFFF" },
    "inner_active":        { "dark": "accent",    "light": "accent_dim" }
  }
}
```

Each color token requires both `"dark"` and `"light"` values. Omitted tokens fall back to the built-in defaults. The full list of color tokens and their descriptions is in [`internal/config/themes/schema.json`](internal/config/themes/schema.json).

### Share a theme

Themes are self-contained JSON files — share them however you like (gist, repo, etc.). Users drop the file into `~/.config/falcode/themes/` and set `"theme": "<filename-without-extension>"` in their config.

## Releasing a new version

Releases are automated with [goreleaser](https://goreleaser.com). Make sure you have it installed (`brew install goreleaser`) and that your `GITHUB_TOKEN` has `repo` write access.

```sh
make release TAG=v1.2.3
```

This will:
1. Create and push the git tag
2. Build binaries for macOS (arm64, amd64) and Linux (amd64, arm64)
3. Create a GitHub Release with archives and checksums
4. Push the updated Homebrew formula to the tap repo

## Contributing

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<you>/falcode`
3. Build: `go build -o falcode .`
4. Make your changes and verify the build still works
5. Open a pull request against `main`

Bug reports and feature requests are welcome as GitHub Issues.
