package scaffolding

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/registry"
)

// AddPlan previews adding modules: the requested ids, every module the addition
// installs (requested and required), and the resulting actions.
type AddPlan struct {
	Selected  []string `json:"selected"`
	Installs  []string `json:"installs"`
	Actions   []Action `json:"actions"`
	Conflicts []string `json:"conflicts"`
	Applied   bool     `json:"applied"`
}

// AddModules previews or installs collections and agent or editor adapters,
// with every module they require, without adopting existing unmanaged files,
// even when their bytes match the embedded template. A workspace's wsup.json
// records the new selection.
func AddModules(root string, ids []string, apply bool) (AddPlan, error) {
	result, prepared, err := planAdd(root, ids)
	if err != nil || !apply {
		return result, err
	}
	applied, err := prepared.apply()
	if err != nil {
		return result, err
	}
	result.Applied = applied.Applied
	return result, nil
}

func planAdd(root string, ids []string) (AddPlan, preparedUpdate, error) {
	result := AddPlan{Selected: []string{}, Installs: []string{}, Actions: []Action{}, Conflicts: []string{}}
	if len(ids) == 0 {
		return result, preparedUpdate{}, fmt.Errorf("name at least one module to add; wsup modules lists them")
	}
	loaded, err := LoadLock(root)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	lock := loaded.Lock
	have, err := lock.Selection()
	if err != nil {
		return result, preparedUpdate{}, err
	}
	want := catalogue.Selection{Collections: slices.Clone(have.Collections), Agents: slices.Clone(have.Agents), Editors: slices.Clone(have.Editors)}
	for _, id := range slices.Compact(slices.Sorted(slices.Values(ids))) {
		kind, ok := catalogue.KindOf(id)
		switch {
		case !ok:
			return result, preparedUpdate{}, fmt.Errorf("unknown module %q; wsup modules lists them", id)
		case kind == catalogue.KindCollection:
			want.Collections = append(want.Collections, id)
		case kind == catalogue.KindAgent:
			want.Agents = append(want.Agents, id)
		case kind == catalogue.KindEditor:
			want.Editors = append(want.Editors, id)
		default:
			return result, preparedUpdate{}, fmt.Errorf("module %q is a %s and cannot be added", id, kind)
		}
		result.Selected = append(result.Selected, id)
	}
	for _, list := range []*[]string{&want.Collections, &want.Agents, &want.Editors} {
		*list = slices.Compact(slices.Sorted(slices.Values(*list)))
	}
	modules, err := catalogue.Installed(lock.Starter, want)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	for _, id := range slices.Sorted(maps.Keys(modules)) {
		if _, installed := lock.Modules[id]; !installed {
			result.Installs = append(result.Installs, id)
		}
	}
	// A workspace's wsup.json records the selection and feeds generators; a
	// project has none.
	var document registry.Document
	documentBody, err := readDocument(root)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	if lock.Starter == "workspace" {
		if documentBody == nil {
			return result, preparedUpdate{}, fmt.Errorf("adding modules to a workspace requires wsup.json")
		}
		if document, err = registry.Decode(documentBody); err != nil {
			return result, preparedUpdate{}, err
		}
	}
	have.Registry, want.Registry = documentBody, documentBody
	// Render for the whole resulting selection: base templates depend on it.
	bundle, links, err := catalogue.Starter(lock.Starter, want)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	installed, _, err := catalogue.Starter(lock.Starter, have)
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
	// The addition covers the requested collections' folders (so a missing
	// managed file there is reported), files not yet deployed, and files whose
	// rendered content depends on the selection (such as the .gitignore block).
	// Other files are unchanged by definition and left alone.
	for name, body := range bundle {
		prior, deployed := installed[name]
		if slices.Contains(result.Selected, strings.SplitN(name, "/", 2)[0]) || !deployed || !bytes.Equal(prior, body) {
			paths = append(paths, name)
		}
	}
	slices.Sort(paths)
	prepared := preparedUpdate{
		root: root, lockBefore: loaded.Original,
		observed: map[string]observation{}, files: map[string][]byte{}, links: links,
	}
	next := lock
	next.Modules = modules
	next.Files = maps.Clone(lock.Files)
	next.Blocks = maps.Clone(lock.Blocks)
	if next.Blocks == nil {
		next.Blocks = map[string]string{}
	}
	// adopt records new content as this path's baseline, blocks included.
	adopt := func(name string, body []byte, baseline bool) {
		if baseline {
			next.Files[name] = lockDigest(body)
		}
		for key := range next.Blocks {
			if strings.HasPrefix(key, name+"#") {
				delete(next.Blocks, key)
			}
		}
		maps.Copy(next.Blocks, blockDigests(name, body))
	}
	for _, name := range paths {
		current, err := observeFile(handle, name)
		if err != nil {
			return result, preparedUpdate{}, err
		}
		prepared.observed[name] = current
		baseline, managed := lock.Files[name]
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
				adopt(name, bundle[name], true)
			default:
				body, err := handle.ReadFile(name)
				if err != nil {
					return result, preparedUpdate{}, err
				}
				spliced, ok := spliceBlocks(body, prior, bundle[name])
				if !ok || lockDigest(body) != current.digest {
					result.Conflicts = append(result.Conflicts, name)
					break
				}
				result.Actions = append(result.Actions, Action{Path: name, Action: "merge"})
				prepared.files[name] = spliced
				adopt(name, bundle[name], false)
			}
			continue
		}
		if managed {
			// A managed skill is shared, so it never belongs to an unselected collection.
			enabled := isSkill(name) || slices.Contains(have.Collections, strings.SplitN(name, "/", 2)[0])
			if !enabled || !current.regular {
				result.Conflicts = append(result.Conflicts, name)
			}
		} else if current.exists {
			result.Conflicts = append(result.Conflicts, name)
		} else {
			result.Actions = append(result.Actions, Action{Path: name, Action: "write"})
			prepared.files[name] = bundle[name]
			adopt(name, bundle[name], true)
		}
	}
	// New links are created where nothing exists yet; anything else there is a
	// conflict, never replaced.
	next.Links = links
	for _, name := range slices.Sorted(maps.Keys(links)) {
		if _, recorded := lock.Links[name]; recorded {
			continue
		}
		status, err := linkStatus(handle, name, links[name])
		if err != nil {
			return result, preparedUpdate{}, err
		}
		switch status {
		case linkOK:
		case linkMissing:
			result.Actions = append(result.Actions, Action{Path: name, Action: "link"})
		default:
			result.Conflicts = append(result.Conflicts, name)
		}
	}
	// wsup.json records the selection; it is rewritten in canonical form.
	if lock.Starter == "workspace" {
		document.Collections = mergeIDs(document.Collections, want.Collections)
		document.Agents = mergeIDs(document.Agents, want.Agents)
		document.Editors = mergeIDs(document.Editors, want.Editors)
		encoded, err := registry.Encode(document)
		if err != nil {
			return result, preparedUpdate{}, err
		}
		if !bytes.Equal(encoded, documentBody) {
			observed, err := observeFile(handle, registry.DocumentPath)
			if err != nil {
				return result, preparedUpdate{}, err
			}
			prepared.observed[registry.DocumentPath] = observed
			prepared.files[registry.DocumentPath] = encoded
			result.Actions = append(result.Actions, Action{Path: registry.DocumentPath, Action: "write"})
		}
	}
	slices.SortFunc(result.Actions, func(a, b Action) int { return strings.Compare(a.Path, b.Path) })
	prepared.lockAfter, err = EncodeLock(next)
	if err != nil {
		return result, preparedUpdate{}, err
	}
	prepared.plan = UpdatePlan{Actions: result.Actions, Conflicts: result.Conflicts}
	return result, prepared, nil
}

// mergeIDs returns the sorted union of two id lists.
func mergeIDs(current, added []string) []string {
	return slices.Compact(slices.Sorted(slices.Values(slices.Concat(current, added))))
}

func isSkill(name string) bool {
	return strings.HasPrefix(name, ".agents/skills/")
}
