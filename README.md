## Bootstrap

```sh
curl -fsSL https://raw.githubusercontent.com/pa2x2/agents/master/src/bootstrap.sh | sh
```

The managed source checkout is `~/.local/share/pa2-skills/agents` by default and private deployment state is `~/.local/state/pa2-skills`. These follow `XDG_DATA_HOME` and `XDG_STATE_HOME` when set.

Ensure `~/.local/bin` is on `PATH` if needed, then inspect the available skills and rules and install the ones you want:

```sh
pa2-skills list
pa2-skills install all
```

Without `--harness`, `install` uses `$PA2_SKILLS_HARNESS`, else the harnesses pa2-skills already manages, else the harnesses whose configuration directory exists (`~/.claude`, `~/.codex`, `~/.config/opencode`). It prints the harnesses it picked.

## Skills and rules

A skill is a directory under `skills/<name>/` with a `SKILL.md`. It is copied into each harness's skills directory.

A rule is a single Markdown file, `rules/<name>.md`, holding general instructions such as commit conventions. It is written into the harness instruction file between marker comments:

```markdown
<!-- pa2-skills:begin git-commits -->
...rule text...
<!-- pa2-skills:end git-commits -->
```

| Harness | User scope | Project scope |
|---|---|---|
| `claude` | `~/.claude/CLAUDE.md` | `CLAUDE.md` |
| `codex` | `~/.codex/AGENTS.md` | `AGENTS.md` |
| `opencode` | `~/.config/opencode/AGENTS.md` | `AGENTS.md` |

Text outside the markers is never changed, so a rule can sit next to your own instructions. Change a rule's text in the source checkout, not between the markers: an edit inside a block counts as a local customization, just like an edited skill. Codex and opencode share the project `AGENTS.md` and one block in it, and a symlinked `CLAUDE.md` is followed rather than replaced.

Skills and rules share one set of names, and every command below accepts either kind.

## Commands reference

| Command | Purpose |
|---|---|
| `pa2-skills install <name>...\|all [--scope user\|project] [--harness <harnesses>] [--conflict ask\|overwrite\|skip] [--dry-run]` | Install or refresh skills and rules (`all` selects every available one) for the selected scope and comma-separated harnesses (`all` selects every supported harness). Scope defaults to `$PA2_SKILLS_SCOPE` or `user`. `--conflict=overwrite` also replaces local customizations. |
| `pa2-skills remove <name>...\|all [--scope user\|project] [--harness <harnesses>] [--force] [--dry-run]` | Delete managed installations, from every harness unless `--harness` is given, and stop tracking them; locally customized ones need `--force`. |
| `pa2-skills update [--check] [--binary-only\|--skills-only] [--conflict ask\|overwrite\|skip]` | Upgrade the binary, pull the source checkout and refresh all managed installations. |
| `pa2-skills list` | List skills and rules available from the managed source checkout, with the harnesses each is installed for (user scope, then `project:` for the current Git project). |
| `pa2-skills status [name...] [--scope] [--harness] [-v]` | Show managed installations and whether each is current, outdated, locally modified, diverged, missing, or orphaned. `-v` shows one row per installation with its target. |
| `pa2-skills diff [name...] [--scope] [--harness]` | Show how installations that are not current differ from the source checkout. |
| `pa2-skills discover [path]` | Report skills found below the current or supplied directory. |
| `pa2-skills add <skill-path> [--name <name>]` | Copy a local skill into the managed source checkout without Git operations. |
| `pa2-skills version` | Print the installed command version. |
| `pa2-skills source-path` | Print the managed source checkout path. |
| `pa2-skills cd [path]` | Launch a child shell in the managed source checkout or one of its paths. |
| `pa2-skills completion zsh\|bash\|fish` | Print dynamic shell completion. |
| `pa2-skills doctor` | Check prerequisites, the source checkout, the binary on `PATH`, managed installations and shell completion. |
| `pa2-skills help [command]` | Show usage for all commands, or flags and details for one command (same as `<command> --help`). |

When a conflict needs a decision and stdin is not a terminal, `install` and `update` leave it unresolved, finish the other installations and exit non-zero; re-run with `--conflict=overwrite` or `--conflict=skip`.

## Shell completion

The completion is dynamic: it offers skill and rule names and valid flag values without fetching the network.

```sh
eval "$(pa2-skills completion zsh)"     # ~/.zshrc
eval "$(pa2-skills completion bash)"    # ~/.bashrc
pa2-skills completion fish | source     # ~/.config/fish/config.fish
```
