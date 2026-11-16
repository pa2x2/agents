package pa2skills

import (
	"fmt"
	"io"
	"strings"
)

// outcome is what a run did, or in a dry run would do, to one installation.
// In its texts, {name} is the item, {where} the harnesses (and project) it
// applies to, and {flags} the flags a suggested command needs to select the
// same scope.
type outcome struct {
	tally   string
	done    string
	planned string // dry-run wording; empty when it is the same as done
}

var (
	outcomeInstalled          = outcome{"installed", "Installed {name} for {where}", "Would install {name} for {where}"}
	outcomeCurrent            = outcome{"current", "Already current: {name} for {where}", ""}
	outcomeUpdated            = outcome{"updated", "Updated {name} for {where}", "Would update {name} for {where}"}
	outcomeAdopted            = outcome{"adopted", "Adopted matching {name} for {where}", "Would adopt matching {name} for {where}"}
	outcomeAdoptedCustomized  = outcome{"adopted", "Adopted locally customized {name} for {where}", "Would adopt locally customized {name} for {where}"}
	outcomeReplacedUnmanaged  = outcome{"replaced", "Replaced unmanaged {name} for {where}", "Would replace unmanaged {name} for {where}"}
	outcomeReplacedCustomized = outcome{"replaced", "Replaced local changes to {name} for {where}", "Would replace local changes to {name} for {where}"}
	outcomeKeptCustomized     = outcome{"kept", "Kept locally customized {name} for {where}; run 'pa2-skills install {name}{flags} --conflict=overwrite' to replace it", "Would keep locally customized {name} for {where}"}
	outcomeSkippedSource      = outcome{"skipped", "Skipped source changes to {name} for {where}", "Would skip source changes to {name} for {where}"}
	outcomeConflict           = outcome{"unresolved", "Conflict: {name} for {where} has local changes, and the source changed too; see 'pa2-skills diff {name}{flags}'", "Would ask about {name} for {where}: it has local changes, and the source changed too"}
	outcomeUnmanagedConflict  = outcome{"unresolved", "Conflict: {name} for {where} already exists and isn't managed by pa2-skills", "Would ask about {name} for {where}: it already exists and isn't managed by pa2-skills"}
	outcomeMissing            = outcome{"missing", "Missing {name} for {where}; run 'pa2-skills install {name}{flags}' to restore it or 'pa2-skills remove {name}{flags}' to stop tracking it", ""}
	outcomeOrphaned           = outcome{"orphaned", "Skipped {name} for {where}: no longer in the source checkout; run 'pa2-skills remove {name}{flags}' to stop tracking it", ""}
	outcomeRemoved            = outcome{"removed", "Removed {name} for {where}", "Would remove {name} for {where}"}
	outcomeUntrackedMissing   = outcome{"removed", "Stopped tracking missing {name} for {where}", "Would stop tracking missing {name} for {where}"}
	outcomeUntrackedShared    = outcome{"removed", "Stopped tracking {name} for {where}; the other harnesses that share the file keep it", "Would stop tracking {name} for {where}; the other harnesses that share the file would keep it"}
	outcomeKeptOnRemove       = outcome{"kept", "Kept locally customized {name} for {where}; re-run with --force to remove it", "Would keep locally customized {name} for {where}"}
)

// report prints one line per item and outcome, listing every harness the
// outcome applies to, and ends a run that touched several installations with
// a tally.
type report struct {
	paths  Paths
	out    io.Writer
	dryRun bool
	// hideCurrent leaves out already-current lines, which only the tally counts.
	hideCurrent bool
	group       reportGroup
	lines       []reportLine
	printed     int
	hidden      int
	tally       map[string]int
	order       []string
}

// reportGroup is one item at one location; its outcomes are printed together.
type reportGroup struct{ name, scope, projectRoot string }

type reportLine struct {
	outcome   outcome
	harnesses []string
}

func (r *report) add(installation Installation, result outcome) {
	group := reportGroup{installation.Name, installation.Scope, installation.ProjectRoot}
	if group != r.group {
		r.flush()
		r.group = group
	}
	if r.tally[result.tally] == 0 {
		r.order = append(r.order, result.tally)
	}
	r.tally[result.tally]++
	for index := range r.lines {
		if r.lines[index].outcome == result {
			r.lines[index].harnesses = append(r.lines[index].harnesses, installation.Harness)
			return
		}
	}
	r.lines = append(r.lines, reportLine{result, []string{installation.Harness}})
}

func (r *report) flush() {
	for _, line := range r.lines {
		if r.hideCurrent && line.outcome == outcomeCurrent {
			r.hidden++
			continue
		}
		format := line.outcome.done
		if r.dryRun && line.outcome.planned != "" {
			format = line.outcome.planned
		}
		where, flags := strings.Join(line.harnesses, ", "), ""
		if r.group.scope == string(ScopeProject) {
			where += " in " + r.paths.Display(r.group.projectRoot)
			flags = " --scope project"
		}
		fmt.Fprintln(r.out, strings.NewReplacer("{name}", r.group.name, "{where}", where, "{flags}", flags).Replace(format))
		r.printed++
	}
	r.lines = nil
}

func (r *report) summary() {
	r.flush()
	if r.dryRun {
		fmt.Fprintln(r.out, "Dry run: no files were changed.")
		return
	}
	// A single line already says everything the tally would.
	if r.hidden == 0 && r.printed < 2 {
		return
	}
	parts := make([]string, 0, len(r.order))
	for _, tally := range r.order {
		parts = append(parts, fmt.Sprintf("%d %s", r.tally[tally], tally))
	}
	fmt.Fprintf(r.out, "Done: %s.\n", strings.Join(parts, ", "))
}
