package scaffolding

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMigrationConflictApplyNeverWritesOrDeletes(t *testing.T) {
	root := pythonFixture(t)
	writeFixture(t, root, ".wsup/wsup.py", []byte("Owner runtime change"))
	before := snapshot(t, root)
	result, err := ApplyMigration(root, "0.2.0", nil)
	if err == nil || result.Applied || !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatalf("conflict apply mutated content: %+v %v", result, err)
	}
}

func TestMigrationRetirementFailureAfterOneDeletePreservesRecoverableState(t *testing.T) {
	root, prepared := migrationFixture(t)
	injected := errors.New("synthetic second deletion denied")
	removed := []string{}
	result, err := prepared.applyWith(writeManaged, func(handle *os.Root, name string) error {
		if len(removed) == 1 {
			return injected
		}
		if err := handle.Remove(name); err != nil {
			return err
		}
		removed = append(removed, name)
		return nil
	})
	if !errors.Is(err, injected) || result.Applied || len(removed) != 1 ||
		!strings.Contains(err.Error(), "state committed but legacy retirement incomplete") {
		t.Fatalf("partial retirement hidden: %+v %v", result, err)
	}
	loaded, err := LoadState(root)
	if err != nil || loaded.State.Version != StateVersion {
		t.Fatal("partial retirement lost native state")
	}
	for name, body := range prepared.content.files {
		current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(current, body) {
			t.Fatalf("partial retirement lost installed content: %s", name)
		}
	}
}

func TestMigrationStateAndOutputsRevalidatedAfterCommitBeforeFirstRetirement(t *testing.T) {
	for _, change := range []string{StatePath, ".wsup/reports/old.json", ".wsup/templates/late-owner.txt"} {
		root, prepared := migrationFixture(t)
		removed := false
		result, err := prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
			if err := writeManaged(handle, name, body, exclusive); err != nil {
				return err
			}
			if name == StatePath {
				writeFixture(t, root, change, []byte("Owner changed after commit"))
			}
			return nil
		}, func(*os.Root, string) error { removed = true; return nil })
		if err == nil || result.Applied || removed || !strings.Contains(err.Error(), "state committed") {
			t.Fatalf("postcommit change retired sources: %s %+v %v", change, result, err)
		}
	}
}

func TestMigrationLateReportDestinationCollisionPreservesOwnerAndSources(t *testing.T) {
	root, prepared := migrationFixture(t)
	removed := false
	result, err := prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
		if name == ".wsup/reports/old.json" {
			writeFixture(t, root, name, []byte("Late owner report"))
		}
		return writeManaged(handle, name, body, exclusive)
	}, func(*os.Root, string) error { removed = true; return nil })
	if err == nil || result.Applied || removed {
		t.Fatalf("late collision retired sources: %+v %v", result, err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".wsup/reports/old.json"))
	if err != nil || string(body) != "Late owner report" {
		t.Fatal("late owner report overwritten")
	}
	loaded, err := LoadState(root)
	if err != nil || !bytes.Equal(loaded.Original, prepared.content.stateBefore) {
		t.Fatal("late report collision committed state")
	}
}

func TestMigrationRechecksStateDestinationAndSourceTypeBetweenDeletions(t *testing.T) {
	for _, change := range []string{StatePath, ".wsup/reports/z.json", ".wsup/wsup.py"} {
		t.Run(change, func(t *testing.T) {
			root := pythonFixture(t)
			writeFixture(t, root, ".work/quality/a.json", []byte("First evidence"))
			writeFixture(t, root, ".work/quality/z.json", []byte("Later evidence"))
			var prepared preparedMigration
			if _, err := planMigration(root, "0.2.0", nil, &prepared); err != nil {
				t.Fatal(err)
			}
			deletions := 0
			result, err := prepared.applyWith(writeManaged, func(handle *os.Root, name string) error {
				if err := handle.Remove(name); err != nil {
					return err
				}
				deletions++
				if deletions == 1 {
					if change == ".wsup/wsup.py" {
						if err := handle.Remove(change); err != nil {
							t.Fatal(err)
						}
						if err := handle.Mkdir(change, 0755); err != nil {
							t.Fatal(err)
						}
					} else {
						writeFixture(t, root, change, []byte("Changed between deletions"))
					}
				}
				return nil
			})
			if err == nil || result.Applied || !strings.Contains(err.Error(), "retirement incomplete") {
				t.Fatalf("between-deletion change ignored: %+v %v", result, err)
			}
			if change != ".wsup/wsup.py" {
				if _, err := os.Stat(filepath.Join(root, ".work/quality/z.json")); err != nil {
					t.Fatal("changed destination lost its source")
				}
			}
			info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(change)))
			if err != nil || (change == ".wsup/wsup.py" && !info.IsDir()) {
				t.Fatal("changed input was removed")
			}
		})
	}
}
