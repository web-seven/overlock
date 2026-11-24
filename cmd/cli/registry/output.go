package registry

import (
	"encoding/json"
	"os"
	"sort"

	semver "github.com/Masterminds/semver/v3"
	"github.com/pterm/pterm"
)

// Exit codes let scripts tell a first push (registry reachable, nothing there yet)
// apart from a connection failure.
const (
	exitUnreachable = 1
	exitNotFound    = 2
)

func printList(output string, header string, items []string) error {
	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(items)
	}

	table := pterm.TableData{{header}}
	for _, item := range items {
		table = append(table, []string{item})
	}
	return pterm.DefaultTable.WithHasHeader().WithData(table).Render()
}

// sortTags orders tags by semver precedence when every tag parses as a version, falling
// back to lexical order otherwise, since registries commonly mix semver and non-semver tags.
func sortTags(tags []string, descending bool) {
	sort.Slice(tags, func(i, j int) bool {
		vi, ei := semver.NewVersion(tags[i])
		vj, ej := semver.NewVersion(tags[j])
		if ei == nil && ej == nil {
			if descending {
				return vi.GreaterThan(vj)
			}
			return vi.LessThan(vj)
		}
		if descending {
			return tags[i] > tags[j]
		}
		return tags[i] < tags[j]
	})
}
