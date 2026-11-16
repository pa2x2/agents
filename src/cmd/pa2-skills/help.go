package main

import (
	"errors"
	"fmt"
	"io"
)

type commandHelp struct {
	name    string
	section string
	// brief is the one-line description in the command list and shell completion.
	brief   string
	usage   string
	summary string
	details string
}

var helpSections = []string{"Install and update", "Author skills", "Setup"}

var commandHelps = []commandHelp{
	{
		name:    "install",
		section: "Install and update",
		brief:   "Install or refresh skills and rules",
		usage:   "install <name>...|all [--scope user|project] [--harness <harnesses>|all] [--conflict ask|overwrite|skip] [--dry-run]",
		summary: "Install skills and rules from the managed source checkout, or refresh existing installations. all selects every available skill and rule.\nA rule is written between pa2-skills markers in the harness's CLAUDE.md or AGENTS.md; text outside the markers is never changed.",
		details: `Flags:
  --scope user|project             install for the user or for the current Git project
                                   (default: $PA2_SKILLS_SCOPE, else user)
  --harness <harnesses>|all        comma-separated harnesses: claude, codex, opencode; all selects every one
                                   (default: $PA2_SKILLS_HARNESS, else the harnesses pa2-skills already
                                   manages, else the ones whose configuration directory exists)
  --conflict ask|overwrite|skip    how to resolve local changes (default: ask)
                                   overwrite also replaces local customizations the source has not changed
  --dry-run                        report what would change without changing files
`,
	},
	{
		name:    "remove",
		section: "Install and update",
		brief:   "Delete managed installations",
		usage:   "remove <name>...|all [--scope user|project] [--harness <harnesses>|all] [--force] [--dry-run]",
		summary: "Delete managed installations and stop tracking them. all selects every managed installation in the scope.\nFiles and rule text pa2-skills did not install are never touched.",
		details: `Flags:
  --scope user|project             remove from the user or from the current Git project
                                   (default: $PA2_SKILLS_SCOPE, else user)
  --harness <harnesses>|all        comma-separated harnesses: claude, codex, opencode; all selects every one
                                   (default: $PA2_SKILLS_HARNESS, else every harness)
  --force                          also remove installations with local changes
  --dry-run                        report what would change without changing files
`,
	},
	{
		name:    "update",
		section: "Install and update",
		brief:   "Update the binary, the source checkout and every installation",
		usage:   "update [--check] [--binary-only|--skills-only] [--conflict ask|overwrite|skip]",
		summary: "Upgrade the binary, then pull the source checkout and refresh every managed installation.",
		details: `Flags:
  --check                          report available updates without changing files
  --binary-only                    only update the pa2-skills binary
  --skills-only                    only update the source checkout and managed skills and rules
  --conflict ask|overwrite|skip    how to resolve skills and rules with local and remote changes (default: ask)
`,
	},
	{name: "list", section: "Install and update", brief: "List available skills and rules and where they are installed", usage: "list", summary: "List skills and rules available from the managed source checkout.\nINSTALLED shows the harnesses with a user-scope installation, then after \"project:\" those with one in the current Git project."},
	{
		name:    "status",
		section: "Install and update",
		brief:   "Show managed installations and whether they are current",
		usage:   "status [name...] [--scope user|project] [--harness <harnesses>|all] [-v|--verbose]",
		summary: "Show managed installations and how they compare with the local source checkout.\nInstallations of an item that share a status are shown on one row.",
		details: `Flags:
  --scope user|project             only show installations in this scope
  --harness <harnesses>|all        only show installations for these comma-separated harnesses
  -v, --verbose                    show one row per installation, with its target

Statuses:
  current     matches the source checkout
  outdated    the source checkout changed; run pa2-skills install or update to refresh it
  modified    changed locally since pa2-skills last wrote it
  diverged    changed both locally and in the source checkout
  missing     the installed skill directory or rule block was deleted
  orphaned    the skill or rule is no longer in the source checkout

Run 'pa2-skills update --check' to see whether the remote source has changes.
`,
	},
	{
		name:    "diff",
		section: "Install and update",
		brief:   "Show how installations differ from the source",
		usage:   "diff [name...] [--scope user|project] [--harness <harnesses>|all]",
		summary: "Show how each selected installation that is not current differs from the source checkout.\nLines removed (-) are in the installed copy; lines added (+) are in the source.",
		details: `Flags:
  --scope user|project             only compare installations in this scope
  --harness <harnesses>|all        only compare installations for these comma-separated harnesses
`,
	},
	{name: "add", section: "Author skills", brief: "Copy a local skill into the source checkout", usage: "add <skill-path> [--name <name>]", summary: "Copy a local skill into the managed source checkout without Git operations.", details: `Flags:
  --name <name>    tracked skill name (default: the directory name)
`},
	{name: "discover", section: "Author skills", brief: "Find skills below a directory", usage: "discover [path]", summary: "Report skills found below the current or supplied directory."},
	{name: "cd", section: "Author skills", brief: "Open a shell in the source checkout", usage: "cd [path]", summary: "Launch a child shell in the managed source checkout or one of its paths."},
	{name: "source-path", section: "Author skills", brief: "Print the source checkout path", usage: "source-path", summary: "Print the managed source checkout path."},
	{name: "doctor", section: "Setup", brief: "Check the installation and its prerequisites", usage: "doctor", summary: "Check prerequisites, the managed source checkout, the binary on PATH, managed installations and shell completion."},
	{name: "completion", section: "Setup", brief: "Print shell completion", usage: "completion zsh|bash|fish", summary: "Print dynamic shell completion.", details: `Enable it with:
  zsh     eval "$(pa2-skills completion zsh)"     in ~/.zshrc
  bash    eval "$(pa2-skills completion bash)"    in ~/.bashrc
  fish    pa2-skills completion fish | source     in ~/.config/fish/config.fish
`},
	{name: "version", section: "Setup", brief: "Print the installed version", usage: "version", summary: "Print the installed command version."},
	{name: "help", section: "Setup", brief: "Show usage for a command", usage: "help [command]", summary: "Show usage for all commands or one command."},
}

func lookupCommand(name string) (commandHelp, bool) {
	if name == "--version" {
		name = "version"
	}
	for _, help := range commandHelps {
		if help.name == name {
			return help, true
		}
	}
	return commandHelp{}, false
}

func printHelp(arguments []string, stdout io.Writer) error {
	if len(arguments) > 1 {
		return errors.New("usage: pa2-skills help [command]")
	}
	if len(arguments) == 0 {
		printUsage(stdout)
		return nil
	}
	help, found := lookupCommand(arguments[0])
	if !found {
		return unknownCommand(arguments[0])
	}
	printCommandHelp(stdout, help)
	return nil
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: pa2-skills <command> [arguments]")
	width := 0
	for _, help := range commandHelps {
		width = max(width, len(help.name))
	}
	for _, section := range helpSections {
		fmt.Fprintf(writer, "\n%s:\n", section)
		for _, help := range commandHelps {
			if help.section == section {
				fmt.Fprintf(writer, "  %-*s   %s\n", width, help.name, help.brief)
			}
		}
	}
	fmt.Fprintln(writer, "\nRun 'pa2-skills help <command>' for a command's flags and details.")
}

func printCommandHelp(writer io.Writer, help commandHelp) {
	fmt.Fprintf(writer, "Usage: pa2-skills %s\n\n%s\n", help.usage, help.summary)
	if help.details != "" {
		fmt.Fprintf(writer, "\n%s", help.details)
	}
}
