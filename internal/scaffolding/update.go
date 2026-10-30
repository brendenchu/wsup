package scaffolding

import (
	"bytes"
	"fmt"

	"github.com/brendenchu/wsup/internal/catalogue"
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
	if !bytes.Equal(loaded.Original, prepared.lockBefore) {
		return UpdatePlan{}, fmt.Errorf("wsup.lock changed during catalogue selection; no files updated")
	}
	if apply {
		return prepared.apply()
	}
	return plan, nil
}
