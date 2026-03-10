# falcode

Terminal multiplexer for multi-agent git worktree workflows. Each outer tab is a git worktree; each inner tab runs a configurable tool (AI agent, lazygit, shell) in its own PTY.

## Prerequisites

- **Go 1.26+** — [install](https://go.dev/dl/)
- **Git** — worktree discovery requires a git repository
- **opencode** *(optional)* — used by the default Agent tab
- **lazygit** *(optional)* — used by the default Git tab

## Build

```sh
git clone https://github.com/victor-falcon/falcode
cd falcode
go build -o falcode .
```

The `falcode` binary is placed in the current directory.

### Install to `$GOPATH/bin`

```sh
go install .
```

### Run without installing

```sh
go run . 
```

## Usage

Run from inside any git repository:

```sh
falcode
```

falcode discovers all git worktrees in the repository and opens one workspace tab per worktree.

## Keybinds

The default prefix key is `Ctrl+B` (tmux-style). Press the prefix, then:

| Key | Action |
|-----|--------|
| `q` | Quit |
| `Ctrl+B` | Send prefix through to the active terminal |
| `t` → `n` | Next inner tab |
| `t` → `p` | Previous inner tab |
| `t` → `c` | New console tab |
| `t` → `x` | Close current tab |
| `w` → `n` | Next workspace |
| `w` → `p` | Previous workspace |
| `w` → `d` | Delete current workspace |

Press `Esc` to cancel a prefix sequence. Mouse clicks on tabs also work.

When a command tab (Agent, Git) stops, a banner appears at the bottom of the pane — press `Enter` to restart the command.

## Configuration

falcode looks for configuration in this order:

1. `./falcode.json` (project-local)
2. `~/.config/falcode/config.json` (user-global)
3. Built-in defaults

### `config.json`

```json
{
  "tabs": [
    { "name": "Agent", "command": "opencode" },
    { "name": "Git",   "command": "lazygit" },
    { "name": "Console" }
  ],
  "theme": "default"
}
```

Tabs with no `"command"` run an interactive `$SHELL`.

### `keybinds.json` (`~/.config/falcode/keybinds.json`)

```json
{
  "prefix": "ctrl+b",
  "bindings": [
    { "key": "q", "description": "Quit", "action": "quit" },
    {
      "key": "t", "description": "Tab",
      "bindings": [
        { "key": "n", "description": "Next tab",    "action": "next_tab" },
        { "key": "p", "description": "Previous tab", "action": "prev_tab" },
        { "key": "c", "description": "New console",  "action": "new_tab" },
        { "key": "x", "description": "Close tab",    "action": "close_tab" }
      ]
    }
  ]
}
```

### Themes (`~/.config/falcode/themes/<name>.json`)

```json
{
  "defs": { "accent": "#7B61FF" },
  "colors": {
    "workspace_active": "accent",
    "workspace_active_fg": "#FFFFFF",
    "inner_active": "accent"
  }
}
```

Set `"theme": "<name>"` in `config.json` to activate a custom theme. Omitted color keys fall back to the built-in dark-purple defaults.
