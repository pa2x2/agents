## Bootstrap

```sh
curl -fsSL https://raw.githubusercontent.com/pa2x2/agents/master/src/bootstrap.sh | sh
```

To install a particular release binary, select the matching tag:

```sh
curl -fsSL https://raw.githubusercontent.com/pa2x2/agents/master/src/bootstrap.sh | \
  PA2_SKILLS_VERSION=v0.1.0 sh
```

The managed source checkout is `~/.local/share/pa2-skills/agents` by default and private deployment state is `~/.local/state/pa2-skills`. These follow `XDG_DATA_HOME` and `XDG_STATE_HOME` when set.

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

| Harness | User target | Project target |
|---|---|---|
| Codex | `~/.agents/skills/<skill>` | `.agents/skills/<skill>` |
| Claude Code | `~/.claude/skills/<skill>` | `.claude/skills/<skill>` |
| OpenCode | `~/.config/opencode/skills/<skill>` | `.opencode/skills/<skill>` |

## Sync a skill

```sh
pa2-skills sync create-agent-workspace \
  --scope project \
  --harness codex,claude
```

## Update pa2-skills

Check for a newer verified release binary, fast-forward the managed source
checkout, and synchronize every managed skill installation:

```sh
pa2-skills update
```

Use `pa2-skills update --check` to report binary and source updates without
changing files. `--binary-only` and `--skills-only` limit the operation.

## Commands reference

| Command | Purpose |
|---|---|
| `pa2-skills install <skill> --scope user\|project --harness <harnesses>` | Install a skill for the selected scope and comma-separated harnesses. |
| `pa2-skills sync <skill> --scope user\|project --harness <harnesses>` | Fetch the source repository and refresh one installed skill. |
| `pa2-skills update` | Upgrade the binary and synchronize the source and all managed skill installations. |
| `pa2-skills list` | List skills available from the managed source checkout. |
| `pa2-skills version` | Print the installed command version. |
| `pa2-skills source-path` | Print the managed source checkout path. |
| `pa2-skills cd [path]` | Launch a child shell in the managed source checkout or one of its paths. |
| `pa2-skills completion zsh` | Print dynamic Zsh completion. |
| `pa2-skills doctor` | Check the managed source checkout and local prerequisites. |

`install`, `sync`, and `update` accept `--conflict ask`, `--conflict overwrite`, or
`--conflict skip`. The default, `ask`, prompts before replacing a locally
modified installed copy.

## Zsh completion

The completion is dynamic: it offers cached skill names and valid command values without fetching the network.

```zsh
eval "$(pa2-skills completion zsh)"
```
