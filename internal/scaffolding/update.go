package scaffolding

import (
	"bytes"
	"fmt"

	"github.com/brendenchu/wsup/internal/catalogue"
)

// Update selects the executable's embedded template using this workspace's
// native state, then previews or explicitly applies same-path content changes.
// It neither converts unsupported schemas nor downloads templates or binaries.
func Update(root, version string, keep []string, apply bool) (UpdatePlan, error) {
	loaded, err := LoadState(root)
	if err != nil {
		return UpdatePlan{}, err
	}
	files, err := catalogue.BundleSelected(loaded.State.Template, loaded.State.SelectedCollections())
	if err != nil {
		return UpdatePlan{}, err
	}
	var prepared preparedUpdate
	plan, err := planUpdate(root, version, files, keep, &prepared)
	if err != nil {
		return plan, err
	}
	if !bytes.Equal(loaded.Original, prepared.stateBefore) {
		return UpdatePlan{}, fmt.Errorf("template state changed during catalogue selection; no files updated")
	}
	if apply {
		return prepared.apply()
	}
	return plan, nil
}
