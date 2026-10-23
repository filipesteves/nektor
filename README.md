# nektor

A personal command recall tool. Save infrequently-used shell commands — SSH into
Raspberry Pis, upgrading Portainer, restarting DNS — and bring them back instantly
with a fuzzy-searchable TUI.

```
nk add <command>  → save a command directly, prompts for a description
nektor add        → pick from shell history, add a description to each
nk                → fuzzy-search saved commands, place on prompt (e edit, d delete)
nektor edit pi    → edit a saved command in a TUI form
nektor delete pi  → delete with confirmation prompt
```

## Install

### Option A: From source

```sh
git clone https://github.com/filipesteves/nektor
cd nektor
make install        # builds and copies to ~/.local/bin
```

### Option B: Cross-compile for Raspberry Pi / Linux ARM64

```sh
make build-linux-arm64
scp dist/nektor-linux-arm64 pi@raspberrypi.local:/usr/local/bin/nektor
```

The binary is fully static (no libc dependency, no CGO) so it runs on any
Linux ARM64 system.

### First-time setup

```sh
nektor install
```

This opens a TUI to choose your output mode, then (for shell mode) adds the
`nk` wrapper function to `~/.zshrc` or `~/.bashrc`. Running it again updates an
existing `nk` function in place, so re-run it after upgrading nektor.

```sh
source ~/.zshrc   # or open a new terminal
```

## Usage

### `nk` — quick recall

Type `nk` at any prompt, fuzzy-search your saved commands, hit Enter. The
selected command is placed on the readline buffer — ready to review or edit
before running.

```
❯ nk
  nektor

  2 items

 ▶  SSH into living room Pi
    $ ssh pi@192.168.1.42

    Pull and restart Portainer
    $ docker pull portainer/portainer-ce:latest && docker stop portainer …

  ↑/k up • ↓/j down • / filter • enter select • e edit • d delete • q quit
```

| Key     | Action                                              |
|---------|-----------------------------------------------------|
| `/`     | Fuzzy-filter by description or command              |
| `enter` | Place the highlighted command on the prompt         |
| `e`     | Edit the highlighted command's description/command  |
| `d`     | Delete the highlighted command (confirm with `y`)   |
| `q`     | Quit                                                |

Edits and deletes are saved immediately, and an active filter is kept.

`nk` with arguments forwards them to `nektor`, so `nk add …`, `nk edit …` etc.
work too. If you installed the wrapper before this was supported, re-run
`nektor install` to update it.

### `nektor add <command>` — save a command directly

```sh
nk add ssh pi@192.168.1.42
nk add 'docker ps | grep portainer'   # quote anything with |, &&, >, ;, etc.
```

Saves the command and prompts for a description. Without quotes, your shell
interprets pipes and `&&` itself, so only the part before them would be saved.

### `nektor add` — save from history

```sh
nektor add
```

Opens your shell history in a filterable list. `Tab` toggles selection on each
entry; `Enter` confirms. For each selected entry a short form appears:

- **Description** — what the command does; this is what you search by in `nk`
- **Command** — pre-filled from history, editable (required)

Press `Enter` to save, `Tab` to move between fields, `Esc` to cancel. Commands
that are already saved are skipped.

Each command also gets an alias, generated from its description
(`SSH into living room Pi` → `ssh-into-living-room-pi`). Aliases are used by
`nektor edit <alias>` and `nektor delete <alias>`, and are listed in
`commands.toml`.

### `nektor edit`

```sh
nektor edit                  # opens commands.toml in $EDITOR
nektor edit pi-ssh-living-room  # opens a TUI form pre-filled with that entry
```

### `nektor delete`

```sh
nektor delete pi-ssh-living-room
# or
nektor rm pi-ssh-living-room
```

Prints the command details and asks for `y/N` confirmation before deleting.

## Config files

Both files live in `~/.config/nektor/`. They are created automatically on first
use — you never need to create them manually.

### `~/.config/nektor/commands.toml`

```toml
[[commands]]
alias = "pi-ssh-living-room"
description = "SSH into living room Pi"
tags = ["ssh", "pi"]
command = "ssh pi@192.168.1.42"

[[commands]]
alias = "portainer-upgrade"
description = "Pull and restart Portainer"
tags = ["docker", "home"]
command = "docker pull portainer/portainer-ce:latest && docker stop portainer && docker rm portainer && docker run -d ..."
```

### `~/.config/nektor/config.toml`

```toml
output_mode = "shell"   # "shell" | "clipboard"
shell       = "zsh"     # "zsh"   | "bash"
```

- **`shell` mode** — requires the `nk` wrapper (set up via `nektor install`). The
  selected command is placed on the readline buffer so you can review it before
  hitting Enter.
- **`clipboard` mode** — no shell modification needed. The command is copied to
  the clipboard; paste it wherever you need it.

## Output modes

| Mode        | How it works                               | Requirement          |
|-------------|--------------------------------------------|----------------------|
| `shell`     | Places command on prompt via `print -z`    | `nk` wrapper in rc   |
| `clipboard` | Copies to clipboard via OS APIs            | none                 |

## Build targets

```sh
make build               # nektor (current platform)
make build-linux-arm64   # dist/nektor-linux-arm64 (static, no CGO)
make install             # build + copy to ~/.local/bin
make tidy                # go mod tidy
make clean               # remove build artifacts
```

## Shell history parsing

nektor reads history from:

| Shell | Default path      | Env override |
|-------|-------------------|--------------|
| zsh   | `~/.zsh_history`  | `$HISTFILE`  |
| bash  | `~/.bash_history` | `$HISTFILE`  |

Zsh's extended history format (`: timestamp:duration;command`) is handled
transparently. Entries are deduplicated and shown most-recent-first.
