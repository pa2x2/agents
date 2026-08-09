# pa2 agent skills

Reusable, harness-selectable agent skills and the `pa2-skills` manager.

Skills live at the repository root in [`skills/`](skills). Everything that supports their installation and management lives in [`src/`](src).

## Bootstrap

The bootstrap script installs only the `pa2-skills` command and its managed source checkout. It does not install or enable any skill.

```sh
curl -fsSL https://raw.githubusercontent.com/pa2x2/agents/v0.1.0/src/bootstrap.sh | sh
```

The bootstrap resolves the matching Linux or macOS release binary, verifies it against the release `SHA256SUMS`, and installs it without Go. If no release is available—for example while developing from `master`—it builds the command from the managed source checkout and requires Go. Pin the script URL to an immutable release tag or commit.

To install a particular release binary, select the matching tag:

```sh
curl -fsSL https://raw.githubusercontent.com/pa2x2/agents/v0.1.0/src/bootstrap.sh | \
  PA2_SKILLS_VERSION=v0.1.0 sh
```

The managed source checkout is `~/.local/share/pa2-skills/agents` by default and private deployment state is `~/.local/state/pa2-skills`. These follow `XDG_DATA_HOME` and `XDG_STATE_HOME` when set. Neither directory is created in a project.

Ensure `~/.local/bin` is on `PATH` if needed, then inspect the available skills:

```sh
pa2-skills list
pa2-skills version
```

## Install a skill

Choose the scope and supported harnesses explicitly.

```sh
pa2-skills install create-agent-workspace \
  --scope user \
  --harness codex,claude

pa2-skills install create-agent-workspace \
  --scope project \
  --harness claude
```

User installs copy a skill into the requested harness's user location. Project installs copy it into the current Git worktree's corresponding harness directory. Project copies are ordinary files: review and commit them if the team should share them. `pa2-skills` never adds a project lock file, modifies `.gitignore`, or creates a project-local management directory.

| Harness | User target | Project target |
|---|---|---|
| Codex | `~/.agents/skills/<skill>` | `.agents/skills/<skill>` |
| Claude Code | `~/.claude/skills/<skill>` | `.claude/skills/<skill>` |
| OpenCode | `~/.config/opencode/skills/<skill>` | `.opencode/skills/<skill>` |

Harness selection only controls the installation target. It does not configure, deny, or otherwise control a harness that may later discover a compatible path.

## Update a skill

```sh
pa2-skills update create-agent-workspace \
  --scope project \
  --harness codex,claude
```

The manager fetches the source repository with `git pull --ff-only`, compares the current installed copy to its private baseline and the incoming source, then preserves local-only changes. If both source and installed copy changed, choose `diff`, `overwrite`, `all-overwrite`, `skip`, or `quit`. Use `--conflict=overwrite` or `--conflict=skip` for non-interactive use.

## Work on the source repository

```sh
pa2-skills cd
cd "$(pa2-skills source-path)"
```

`pa2-skills cd` launches a child shell in the managed source checkout. Use `source-path` when the current shell itself must change directories.

## Zsh completion

The completion is dynamic: it offers cached skill names and valid command values without fetching the network.

```zsh
autoload -Uz compinit && compinit
eval "$(pa2-skills completion zsh)"
```

Add that to a personal Zsh configuration only if you want persistent completion. Bootstrap deliberately does not edit shell startup files.
