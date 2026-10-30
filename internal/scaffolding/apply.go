package scaffolding

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
)

type preparedUpdate struct {
	root       string
	plan       UpdatePlan
	observed   map[string]observation
	lockBefore []byte
	lockAfter  []byte
	files      map[string][]byte
	// links holds the targets of planned "link" actions.
	links map[string]string
}

// ApplyUpdate plans and applies native same-path content changes. Conflicts and
// observed changes block all writes. Later I/O failures may leave partial content
// updates, reported explicitly; wsup.lock is replaced last. This is not a transaction
// or protection against concurrent writers after the final per-file recheck.
func ApplyUpdate(root, version string, files map[string][]byte, keep []string) (UpdatePlan, error) {
	var prepared preparedUpdate
	plan, err := planUpdate(root, version, files, nil, nil, keep, &prepared)
	if err != nil {
		return plan, err
	}
	return prepared.apply()
}

func (p preparedUpdate) apply() (UpdatePlan, error) {
	return p.applyWith(writeManaged)
}

func (p preparedUpdate) applyWith(write func(*os.Root, string, []byte, bool) error) (UpdatePlan, error) {
	result := p.plan
	if len(result.Conflicts) != 0 {
		return result, fmt.Errorf("template conflicts require manual resolution; no files updated: %s", strings.Join(result.Conflicts, ", "))
	}
	root, err := os.OpenRoot(p.root)
	if err != nil {
		return result, err
	}
	defer root.Close()
	checkLock := func() error {
		current, err := observeFile(root, LockPath)
		if err != nil {
			return err
		}
		if !current.regular || current.digest != lockDigest(p.lockBefore) {
			return fmt.Errorf("wsup.lock changed during update planning")
		}
		return nil
	}
	if err := checkLock(); err != nil {
		return result, err
	}
	names := make([]string, 0, len(p.observed))
	for name := range p.observed {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if err := p.recheck(root, name); err != nil {
			return result, err
		}
	}
	// Once any write is attempted, errors must disclose possible partial output.
	fail := func(err error) (UpdatePlan, error) {
		return result, fmt.Errorf("native update incomplete; content or directories may already be written; wsup.lock was not updated: %w", err)
	}
	for _, action := range result.Actions {
		if action.Action == "link" {
			// A link is created only where nothing exists yet.
			if status, err := linkStatus(root, action.Path, p.links[action.Path]); err != nil || status != linkMissing {
				return fail(fmt.Errorf("link path changed during update planning: %s", action.Path))
			}
			if err := createLink(root, action.Path, p.links[action.Path]); err != nil {
				return fail(err)
			}
			continue
		}
		if action.Action != "write" && action.Action != "merge" {
			continue
		}
		if err := p.recheck(root, action.Path); err != nil {
			return fail(err)
		}
		if err := write(root, action.Path, p.files[action.Path], !p.observed[action.Path].exists); err != nil {
			return fail(err)
		}
	}
	// Removals follow writes, so a failure never leaves neither old nor new file.
	for _, action := range result.Actions {
		if action.Action != "remove" {
			continue
		}
		if err := p.recheck(root, action.Path); err != nil {
			return fail(err)
		}
		if err := root.Remove(action.Path); err != nil {
			return fail(fmt.Errorf("remove replaced file %s: %w", action.Path, err))
		}
	}
	if err := checkLock(); err != nil {
		return fail(err)
	}
	if !bytes.Equal(p.lockBefore, p.lockAfter) {
		if err := write(root, LockPath, p.lockAfter, false); err != nil {
			return fail(err)
		}
	}
	result.Applied = true
	return result, nil
}

func (p preparedUpdate) recheck(root *os.Root, name string) error {
	current, err := observeFile(root, name)
	if err != nil {
		return err
	}
	if current != p.observed[name] {
		return fmt.Errorf("template file changed during update planning: %s", name)
	}
	return nil
}

// New files are always exclusive. Existing files use a same-directory temporary
// file and rename, never in-place truncation (which could affect hard links).
func writeManaged(root *os.Root, name string, body []byte, exclusive bool) (err error) {
	if _, err := observeFile(root, name); err != nil {
		return err
	}
	parent := path.Dir(name)
	if err := root.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("create template parent %s: %w", name, err)
	}
	directory, err := root.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer directory.Close()
	filename := path.Base(name)
	temporary := filename
	if !exclusive {
		var nonce [12]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return err
		}
		temporary = fmt.Sprintf(".wsup-%x", nonce)
	}
	file, err := directory.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create template file %s: %w", name, err)
	}
	if !exclusive {
		defer func() {
			if cleanup := directory.Remove(temporary); cleanup != nil && !os.IsNotExist(cleanup) {
				err = errors.Join(err, fmt.Errorf("remove template temporary file: %w", cleanup))
			}
		}()
	}
	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return fmt.Errorf("write template file %s: %w", name, err)
	}
	if !exclusive {
		if err := directory.Rename(temporary, filename); err != nil {
			return fmt.Errorf("replace template file %s: %w", name, err)
		}
	}
	return nil
}
