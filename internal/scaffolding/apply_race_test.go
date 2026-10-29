package scaffolding

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyDoesNotCommitOverStateChangedDuringContentWrites(t *testing.T) {
	root, prepared := prepareFixture(t)
	ownerState := append(bytes.Clone(prepared.stateBefore), '\n')
	result, err := prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
		if err := writeManaged(handle, name, body, exclusive); err != nil {
			return err
		}
		if name == "README.md" {
			return os.WriteFile(filepath.Join(root, filepath.FromSlash(StatePath)), ownerState, 0600)
		}
		return nil
	})
	if err == nil || result.Applied || !strings.Contains(err.Error(), "state changed") ||
		!strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("state change during writes not surfaced: %+v %v", result, err)
	}
	current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(StatePath)))
	if err != nil || !bytes.Equal(current, ownerState) {
		t.Fatal("concurrent owner state replaced")
	}
}

func TestApplyExclusiveNewFileRefusesLateCollision(t *testing.T) {
	root, prepared := prepareFixture(t)
	result, err := prepared.applyWith(func(handle *os.Root, name string, body []byte, exclusive bool) error {
		if name == "new/nested.md" {
			writeFixture(t, root, name, []byte("late owner content"))
		}
		return writeManaged(handle, name, body, exclusive)
	})
	if err == nil || result.Applied || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("late destination collision not surfaced: %+v %v", result, err)
	}
	current, err := os.ReadFile(filepath.Join(root, "new/nested.md"))
	if err != nil || string(current) != "late owner content" {
		t.Fatal("late owner file overwritten")
	}
	current, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(StatePath)))
	if err != nil || !bytes.Equal(current, prepared.stateBefore) {
		t.Fatal("late collision committed new state")
	}
}

func TestPreparedUpdateCopiesBundleBytes(t *testing.T) {
	root, _ := nativeFixture(t, map[string][]byte{"README.md": []byte("old")})
	body := []byte("new")
	var prepared preparedUpdate
	if _, err := planUpdate(root, "0.2.0", map[string][]byte{"README.md": body}, nil, nil, &prepared); err != nil {
		t.Fatal(err)
	}
	copy(body, "bad")
	if _, err := prepared.apply(); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || string(current) != "new" {
		t.Fatal("caller changed the prepared output")
	}
}
