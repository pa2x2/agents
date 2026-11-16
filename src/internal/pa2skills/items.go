package pa2skills

import (
	"errors"
	"fmt"
	"io"
	"sort"
)

// Kind tells how an item is stored in the source checkout and deployed to a harness.
type Kind string

const (
	KindSkill Kind = "skill"
	KindRule  Kind = "rule"
)

// kindHandler owns everything that differs between kinds. Deployment, status and
// removal compare the hashes it reports with the installation's baseline.
type kindHandler interface {
	kind() Kind
	names(sourceRoot string) ([]string, error)
	sourcePath(sourceRoot, name string) string
	targetPath(paths Paths, name string, scope Scope, harness, projectRoot string) (string, error)
	sourceHash(source string) (string, error)
	// targetHash returns "" when the item is absent from the target.
	targetHash(target, name string) (string, error)
	write(source, target, name string) error
	remove(target, name string) error
	saveBaseline(source, destination string) error
	diff(output io.Writer, target, source, name string) error
}

var kindHandlers = []kindHandler{skillHandler{}, ruleHandler{}}

func handlerFor(kind Kind) kindHandler {
	for _, handler := range kindHandlers {
		if handler.kind() == kind {
			return handler
		}
	}
	panic(fmt.Sprintf("unknown kind %q", kind))
}

// Item is a skill or rule available from the managed source checkout.
type Item struct {
	Kind   Kind
	Name   string
	Source string
}

var errUnknownItem = errors.New("unknown skill or rule")

// Items lists every skill and rule in the source checkout. Names are shared
// across kinds, so a name used by both is an error.
func (m Manager) Items() ([]Item, error) {
	var items []Item
	kinds := map[string]Kind{}
	for _, handler := range kindHandlers {
		names, err := handler.names(m.Paths.SourceRoot)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if existing, found := kinds[name]; found {
				return nil, fmt.Errorf("%q is both a %s and a %s in the source checkout; rename one of them", name, existing, handler.kind())
			}
			kinds[name] = handler.kind()
			items = append(items, Item{Kind: handler.kind(), Name: name, Source: handler.sourcePath(m.Paths.SourceRoot, name)})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (m Manager) findItem(name string) (Item, error) {
	items, err := m.Items()
	if err != nil {
		return Item{}, err
	}
	for _, item := range items {
		if item.Name == name {
			return item, nil
		}
	}
	return Item{}, fmt.Errorf("%w %q", errUnknownItem, name)
}

// unknownNameError reports a name that matches no candidate, suggesting the closest one.
func unknownNameError(name string, candidates []string) error {
	message := fmt.Sprintf("%q", name)
	if best := ClosestName(name, candidates); best != "" {
		message += fmt.Sprintf("; did you mean %q?", best)
	}
	return fmt.Errorf("%w %s\nRun 'pa2-skills list' to see available names.", errUnknownItem, message)
}

// ClosestName returns the candidate within two edits of name, or "" when none is that close.
func ClosestName(name string, candidates []string) string {
	best, bestDistance := "", 3
	for _, candidate := range candidates {
		if distance := editDistance(name, candidate); distance < bestDistance {
			best, bestDistance = candidate, distance
		}
	}
	return best
}

func editDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for i := 1; i <= len(left); i++ {
		current := make([]int, len(right)+1)
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 1
			if left[i-1] == right[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(right)]
}
