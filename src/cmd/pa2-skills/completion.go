package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"pa2-skills/internal/pa2skills"
)

// The scripts call back into pa2-skills for names: "completion names" lists
// available skills and rules, "completion installed" the managed ones. Neither
// reads the network.
func completion(arguments []string, manager pa2skills.Manager, stdout io.Writer) error {
	var validationErrors []error
	if len(arguments) == 0 {
		validationErrors = append(validationErrors, errors.New("completion shell is required (zsh, bash, or fish)"))
	} else if _, found := completionScripts[arguments[0]]; !found && arguments[0] != "names" && arguments[0] != "installed" {
		validationErrors = append(validationErrors, fmt.Errorf("unsupported completion shell %q (supported: zsh, bash, fish)", arguments[0]))
	}
	if len(arguments) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("unexpected argument(s): %s", strings.Join(arguments[1:], ", ")))
	}
	if err := invalidArguments(validationErrors...); err != nil {
		return err
	}
	switch arguments[0] {
	case "names":
		items, err := manager.Items()
		if err != nil {
			return err
		}
		for _, item := range items {
			fmt.Fprintln(stdout, item.Name)
		}
		return nil
	case "installed":
		names, err := manager.ManagedNames()
		if err != nil {
			return err
		}
		for _, name := range names {
			fmt.Fprintln(stdout, name)
		}
		return nil
	}
	fmt.Fprint(stdout, completionScripts[arguments[0]]())
	return nil
}

var completionScripts = map[string]func() string{
	"zsh": func() string {
		var commands strings.Builder
		for _, help := range commandHelps {
			fmt.Fprintf(&commands, "    '%s:%s'\n", help.name, help.brief)
		}
		return strings.Replace(zshCompletion, "@COMMANDS@\n", commands.String(), 1)
	},
	"bash": func() string {
		return strings.ReplaceAll(bashCompletion, "@COMMANDS@", strings.Join(commandNames(), " "))
	},
	"fish": func() string {
		var commands strings.Builder
		for _, help := range commandHelps {
			fmt.Fprintf(&commands, "complete -c pa2-skills -n __pa2_skills_needs_command -a %s -d '%s'\n", help.name, help.brief)
		}
		script := strings.Replace(fishCompletion, "@COMMANDS@\n", commands.String(), 1)
		return strings.Replace(script, "@HELP@", strings.Join(commandNames(), " "), 1)
	},
}

func commandNames() []string {
	names := make([]string, 0, len(commandHelps))
	for _, help := range commandHelps {
		names = append(names, help.name)
	}
	return names
}

const zshCompletion = `#compdef pa2-skills

_pa2_skills() {
  local command state line
  local -a commands names harnesses scopes conflicts
  commands=(
@COMMANDS@
  )
  harnesses=(all claude codex opencode)
  scopes=(user project)
  conflicts=(ask overwrite skip)
  command=$words[2]
  if (( CURRENT > 2 )); then
    words=($words[1] "${words[3,-1]}")
    (( CURRENT-- ))
  fi
  case $command in
    install)
      _arguments -s \
        '--scope=[installation scope]:scope:->scope' \
        '--harness=[comma-separated harnesses, or all]:harness:->harness' \
        '--conflict=[conflict policy]:policy:->conflict' \
        '--dry-run[report what would change without changing files]' \
        '*:name:->available'
      ;;
    remove)
      _arguments -s \
        '--scope=[installation scope]:scope:->scope' \
        '--harness=[comma-separated harnesses, or all]:harness:->harness' \
        '--force[also remove installations with local changes]' \
        '--dry-run[report what would change without changing files]' \
        '*:name:->removable'
      ;;
    status)
      _arguments -s \
        '--scope=[only show this scope]:scope:->scope' \
        '--harness=[only show these harnesses]:harness:->harness' \
        '(-v --verbose)'{-v,--verbose}'[one row per installation, with its target]' \
        '*:name:->installed'
      ;;
    diff)
      _arguments -s \
        '--scope=[only compare this scope]:scope:->scope' \
        '--harness=[only compare these harnesses]:harness:->harness' \
        '*:name:->installed'
      ;;
    update)
      _arguments -s \
        '--check[report available updates without changing files]' \
        '--binary-only[only update the pa2-skills binary]' \
        '--skills-only[only update source and managed skills and rules]' \
        '--conflict=[conflict policy]:policy:->conflict'
      ;;
    discover)
      _files -/
      ;;
    add)
      _arguments -s \
        '--name=[tracked skill name]:name:' \
        '1:skill directory:_files -/'
      ;;
    cd)
      _files -/ -W "$($words[1] source-path 2>/dev/null)"
      ;;
    completion)
      _values 'shell' zsh bash fish
      ;;
    help)
      _describe -t commands command commands
      ;;
    *)
      _describe -t commands command commands
      ;;
  esac
  case $state in
    scope) _values 'scope' $scopes ;;
    harness) _values -s , 'harness' $harnesses ;;
    conflict) _values 'conflict policy' $conflicts ;;
    available) names=(all "${(@f)$($words[1] completion names 2>/dev/null)}"); _describe -t names name names ;;
    removable) names=(all "${(@f)$($words[1] completion installed 2>/dev/null)}"); _describe -t names name names ;;
    installed) names=("${(@f)$($words[1] completion installed 2>/dev/null)}"); _describe -t names name names ;;
  esac
}

compdef _pa2_skills pa2-skills
`

const bashCompletion = `_pa2_skills() {
  local cur=${COMP_WORDS[COMP_CWORD]} prev=${COMP_WORDS[COMP_CWORD-1]} command=${COMP_WORDS[1]}
  # Bash splits --flag=value at the =.
  if [[ $cur == = ]]; then
    cur=""
  elif [[ $prev == = ]]; then
    prev=${COMP_WORDS[COMP_CWORD-2]}
  fi
  if (( COMP_CWORD == 1 )); then
    COMPREPLY=($(compgen -W "@COMMANDS@" -- "$cur"))
    return
  fi
  case $prev in
    --scope) COMPREPLY=($(compgen -W "user project" -- "$cur")); return ;;
    --conflict) COMPREPLY=($(compgen -W "ask overwrite skip" -- "$cur")); return ;;
    --name) return ;;
    --harness)
      local prefix=""
      [[ $cur == *,* ]] && prefix=${cur%,*},
      COMPREPLY=($(compgen -P "$prefix" -W "all claude codex opencode" -- "${cur##*,}"))
      return
      ;;
  esac
  local flags="" names=""
  case $command in
    install) flags="--scope --harness --conflict --dry-run"; names="all $("$1" completion names 2>/dev/null)" ;;
    remove) flags="--scope --harness --force --dry-run"; names="all $("$1" completion installed 2>/dev/null)" ;;
    status) flags="--scope --harness --verbose"; names=$("$1" completion installed 2>/dev/null) ;;
    diff) flags="--scope --harness"; names=$("$1" completion installed 2>/dev/null) ;;
    update) flags="--check --binary-only --skills-only --conflict" ;;
    add) flags="--name" ;;
    completion) names="zsh bash fish" ;;
    help) names="@COMMANDS@" ;;
  esac
  if [[ $cur == -* ]]; then
    COMPREPLY=($(compgen -W "$flags" -- "$cur"))
    return
  fi
  case $command in
    add|discover) COMPREPLY=($(compgen -d -- "$cur")) ;;
    cd) COMPREPLY=($(cd "$("$1" source-path 2>/dev/null)" 2>/dev/null && compgen -d -- "$cur")) ;;
    *) COMPREPLY=($(compgen -W "$names" -- "$cur")) ;;
  esac
}

complete -F _pa2_skills pa2-skills
`

const fishCompletion = `function __pa2_skills_needs_command
    test (count (commandline -opc)) -eq 1
end

function __pa2_skills_using
    set -l words (commandline -opc)
    test (count $words) -gt 1; and contains -- $words[2] $argv
end

function __pa2_skills_harnesses
    set -l prefix (string match -r '.*,' -- (commandline -ct))
    for harness in all claude codex opencode
        echo $prefix$harness
    end
end

function __pa2_skills_source_directories
    set -l root (pa2-skills source-path 2>/dev/null); or return
    for directory in $root/(commandline -ct)*/
        string replace -- $root/ '' $directory
    end
end

complete -c pa2-skills -f
@COMMANDS@
complete -c pa2-skills -n '__pa2_skills_using install remove' -l scope -x -a 'user project' -d 'installation scope'
complete -c pa2-skills -n '__pa2_skills_using status diff' -l scope -x -a 'user project' -d 'only this scope'
complete -c pa2-skills -n '__pa2_skills_using install remove status diff' -l harness -x -a '(__pa2_skills_harnesses)' -d 'comma-separated harnesses, or all'
complete -c pa2-skills -n '__pa2_skills_using install update' -l conflict -x -a 'ask overwrite skip' -d 'conflict policy'
complete -c pa2-skills -n '__pa2_skills_using install remove' -l dry-run -d 'report what would change without changing files'
complete -c pa2-skills -n '__pa2_skills_using remove' -l force -d 'also remove installations with local changes'
complete -c pa2-skills -n '__pa2_skills_using status' -s v -l verbose -d 'one row per installation, with its target'
complete -c pa2-skills -n '__pa2_skills_using update' -l check -d 'report available updates without changing files'
complete -c pa2-skills -n '__pa2_skills_using update' -l binary-only -d 'only update the pa2-skills binary'
complete -c pa2-skills -n '__pa2_skills_using update' -l skills-only -d 'only update source and managed skills and rules'
complete -c pa2-skills -n '__pa2_skills_using add' -l name -x -d 'tracked skill name'
complete -c pa2-skills -n '__pa2_skills_using install' -a 'all (pa2-skills completion names 2>/dev/null)'
complete -c pa2-skills -n '__pa2_skills_using remove' -a 'all (pa2-skills completion installed 2>/dev/null)'
complete -c pa2-skills -n '__pa2_skills_using status diff' -a '(pa2-skills completion installed 2>/dev/null)'
complete -c pa2-skills -n '__pa2_skills_using add discover' -a '(__fish_complete_directories)'
complete -c pa2-skills -n '__pa2_skills_using cd' -a '(__pa2_skills_source_directories)'
complete -c pa2-skills -n '__pa2_skills_using completion' -a 'zsh bash fish'
complete -c pa2-skills -n '__pa2_skills_using help' -a '@HELP@'
`
