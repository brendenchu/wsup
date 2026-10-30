package cli

import (
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/scaffolding"
)

// checkTasks turns check definitions (argument lists or policy objects) into
// wsup.json tasks that run under wsup check.
func checkTasks(checks map[string]any) map[string]any {
	tasks := make(map[string]any, len(checks))
	for name, check := range checks {
		switch value := check.(type) {
		case map[string]any:
			task := map[string]any{"check": true}
			for key, field := range value {
				task[key] = field
			}
			tasks[name] = task
		default:
			tasks[name] = map[string]any{"command": value, "check": true}
		}
	}
	return tasks
}

// lockSelection returns a lock's installed modules as a selection.
func lockSelection(t *testing.T, lock scaffolding.Lock) catalogue.Selection {
	t.Helper()
	selection, err := lock.Selection()
	if err != nil {
		t.Fatal(err)
	}
	return selection
}
