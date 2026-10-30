package scaffolding

import (
	"bytes"
	"fmt"
	"maps"
	"slices"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/registry"
)

// Update renders the executable's embedded catalogue for the modules wsup.lock
// records, with generators reading the current wsup.json, then previews or
// explicitly applies same-path content changes. It neither converts other lock
// versions nor downloads templates or binaries.
func Update(root, version string, keep []string, apply bool) (UpdatePlan, error) {
	return update(root, version, keep, nil, apply)
}

// RefreshEditor previews or applies only the vscode adapter's generated
// .code-workspace file, from the current wsup.json, with update's conflict
// rules. It fails when the adapter is not installed.
func RefreshEditor(root, version string, apply bool) (UpdatePlan, error) {
	loaded, err := LoadLock(root)
	if err != nil {
		return UpdatePlan{}, err
	}
	if _, installed := loaded.Lock.Modules["vscode"]; !installed {
		return UpdatePlan{}, fmt.Errorf("the vscode adapter is not installed; wsup add vscode installs it")
	}
	return update(root, version, nil, catalogue.Generated, apply)
}

func update(root, version string, keep []string, scope func(string) bool, apply bool) (UpdatePlan, error) {
	loaded, err := LoadLock(root)
	if err != nil {
		return UpdatePlan{}, err
	}
	selection, err := loaded.Lock.Selection()
	if err != nil {
		return UpdatePlan{}, err
	}
	if selection.Registry, err = readDocument(root); err != nil {
		return UpdatePlan{}, err
	}
	files, links, err := catalogue.Starter(loaded.Lock.Starter, selection)
	if err != nil {
		return UpdatePlan{}, err
	}
	modules, err := catalogue.Installed(loaded.Lock.Starter, selection)
	if err != nil {
		return UpdatePlan{}, err
	}
	var prepared preparedUpdate
	plan, err := planScoped(root, version, files, links, modules, keep, scope, &prepared)
	if err != nil {
		return plan, err
	}
	if scope == nil {
		plan.Modules = moduleReport(loaded.Lock, modules, selection.Registry)
	}
	if !bytes.Equal(loaded.Original, prepared.lockBefore) {
		return UpdatePlan{}, fmt.Errorf("wsup.lock changed during catalogue selection; no files updated")
	}
	if apply {
		return prepared.apply()
	}
	return plan, nil
}

// moduleReport compares the lock's modules with the versions this wsup
// installs and with wsup.json's selection. A missing or invalid wsup.json
// (validate reports it) offers nothing.
func moduleReport(lock Lock, available map[string]string, documentBody []byte) *ModuleReport {
	report := &ModuleReport{Updated: []VersionChange{}, Offered: []string{}, Unlisted: []string{}}
	for _, id := range slices.Sorted(maps.Keys(lock.Modules)) {
		if next, ok := available[id]; ok && next != lock.Modules[id] {
			report.Updated = append(report.Updated, VersionChange{ID: id, From: lock.Modules[id], To: next})
		}
	}
	document, err := registry.Decode(documentBody)
	if lock.Starter != "workspace" || documentBody == nil || err != nil {
		return report
	}
	listed := slices.Concat(document.Collections, document.Agents, document.Editors)
	for _, id := range slices.Sorted(slices.Values(listed)) {
		if _, installed := lock.Modules[id]; !installed {
			report.Offered = append(report.Offered, id)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(lock.Modules)) {
		kind, _ := catalogue.KindOf(id)
		if kind != catalogue.KindBase && !slices.Contains(listed, id) && !required(id, lock) {
			report.Unlisted = append(report.Unlisted, id)
		}
	}
	return report
}

// required reports whether another installed module requires id, so it is
// installed without being listed.
func required(id string, lock Lock) bool {
	for other := range lock.Modules {
		for _, module := range catalogue.Modules() {
			if module.ID == other && slices.Contains(module.Requires, id) {
				return true
			}
		}
	}
	return false
}
