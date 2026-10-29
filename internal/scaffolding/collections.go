package scaffolding

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/registry"
)

// CollectionPlan distinguishes listing available names from explicit additions.
// Selected is a pointer so plain listings omit the selection field.
type CollectionPlan struct {
	Available []string  `json:"available,omitempty"`
	Selected  *[]string `json:"selected,omitempty"`
	Actions   []Action  `json:"actions"`
	Conflicts []string  `json:"conflicts"`
	Applied   bool      `json:"applied"`
}

// AddCollections previews or adds optional homes without adopting existing
// unmanaged files, even when their bytes match the embedded template.
func AddCollections(root string, names []string, apply bool) (CollectionPlan, error) {
	result, prepared, err := planCollections(root, names)
	if err != nil {
		return result, err
	}
	if len(names) == 0 {
		if apply {
			return result, fmt.Errorf("select a collection with --add before using --apply")
		}
		return result, nil
	}
	if apply {
		applied, err := prepared.apply()
		if err != nil {
			return result, err
		}
		result.Applied = applied.Applied
	}
	return result, nil
}

func planCollections(root string, names []string) (CollectionPlan, preparedUpdate, error) {
	selected, err := catalogue.SelectCollections(names)
	result := CollectionPlan{Actions: []Action{}, Conflicts: []string{}}
	if err != nil {
		return result, preparedUpdate{}, err
	}
	if len(selected) == 0 {
		result.Available = catalogue.CollectionNames()
		return result, preparedUpdate{}, nil
	}
	result.Selected = &selected
	loaded, err := LoadState(root)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	state := loaded.State
	state.Collections = state.SelectedCollections()
	if state.Version != StateVersion || state.Template != "workspace" {
		return result, preparedUpdate{}, fmt.Errorf("adding collections requires a managed workspace")
	}
	// Render for the whole resulting selection: base templates depend on it.
	bundle, err := catalogue.BundleSelected("workspace", append(slices.Clone(state.Collections), selected...))
	if err != nil {
		return result, preparedUpdate{}, err
	}
	installed, err := catalogue.BundleSelected("workspace", state.Collections)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	root, err = registry.Resolve(root)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	defer handle.Close()
	paths := []string{}
	// The addition covers the selected folders, skills not yet deployed, and
	// files whose rendered content depends on the selection (such as the
	// .gitignore block). Files only an already-selected collection installs are
	// unchanged by definition and left alone.
	for name, body := range bundle {
		prior, deployed := installed[name]
		if slices.Contains(selected, strings.SplitN(name, "/", 2)[0]) || !deployed || !bytes.Equal(prior, body) {
			paths = append(paths, name)
		}
	}
	slices.Sort(paths)
	prepared := preparedUpdate{
		root: root, stateBefore: loaded.Original,
		observed: map[string]observation{}, files: map[string][]byte{},
	}
	for _, name := range paths {
		current, err := observeFile(handle, name)
		if err != nil {
			return result, preparedUpdate{}, err
		}
		prepared.observed[name] = current
		baseline, managed := state.Files[name]
		prior, deployed := installed[name]
		if managed && deployed && !bytes.Equal(prior, bundle[name]) {
			// A rendered file changes with the selection. An unedited copy is
			// replaced and takes the new baseline. An edited copy keeps its owner
			// text and baseline: only its generated sections are replaced, and
			// only when the owner left them as wsup wrote them.
			switch {
			case !current.regular:
				result.Conflicts = append(result.Conflicts, name)
			case current.digest == baseline:
				result.Actions = append(result.Actions, Action{Path: name, Action: "write"})
				prepared.files[name] = bundle[name]
				state.Files[name] = contentDigest(bundle[name])
			default:
				body, err := handle.ReadFile(name)
				if err != nil {
					return result, preparedUpdate{}, err
				}
				spliced, ok := spliceBlocks(body, prior, bundle[name])
				if !ok || contentDigest(body) != current.digest {
					result.Conflicts = append(result.Conflicts, name)
					break
				}
				result.Actions = append(result.Actions, Action{Path: name, Action: "write"})
				prepared.files[name] = spliced
			}
			continue
		}
		if managed {
			// A managed skill is shared, so it never belongs to an unselected collection.
			enabled := isSkill(name) || slices.Contains(state.Collections, strings.SplitN(name, "/", 2)[0])
			formerDefault := slices.Contains([]string{"journal/README.md", "journal/TEMPLATE.md", "learning/README.md", "reference/README.md", "research/README.md"}, name)
			if (!enabled && !formerDefault) || !current.regular {
				result.Conflicts = append(result.Conflicts, name)
			}
		} else if current.exists {
			result.Conflicts = append(result.Conflicts, name)
		} else {
			result.Actions = append(result.Actions, Action{Path: name, Action: "write"})
			prepared.files[name] = bundle[name]
			state.Files[name] = contentDigest(bundle[name])
		}
	}
	state.Collections = append(state.Collections, selected...)
	slices.Sort(state.Collections)
	state.Collections = slices.Compact(state.Collections)
	prepared.stateAfter, err = EncodeNativeState(state)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	prepared.plan = UpdatePlan{Actions: result.Actions, Conflicts: result.Conflicts}
	return result, prepared, nil
}

func isSkill(name string) bool {
	return strings.HasPrefix(name, ".agents/skills/")
}
