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
	plan, err := planUpdate(root, version, files, links, modules, keep, &prepared)
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
