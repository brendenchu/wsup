package scaffolding

import (
	"fmt"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
)

// createLink makes one managed symlink inside root, creating its parent. The
// link itself is the only symlink wsup writes; its parent must be a real
// directory so the link cannot be redirected out of the workspace.
func createLink(root *os.Root, name, target string) error {
	parent := path.Dir(name)
	for dir := parent; dir != "."; dir = path.Dir(dir) {
		if info, err := root.Lstat(dir); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("link parent is a symlink: %s", dir)
		}
	}
	if err := root.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("create link parent %s: %w", name, err)
	}
	if err := root.Symlink(target, name); err != nil {
		return fmt.Errorf("create link %s: %w", name, err)
	}
	return nil
}

// linkStatus is linkOK when name is a symlink to target, linkMissing when
// nothing is there, and otherwise describes what occupies the path.
func linkStatus(root *os.Root, name, target string) (string, error) {
	for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
		info, err := root.Lstat(dir)
		if os.IsNotExist(err) {
			return linkMissing, nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "has a parent that is not a directory", nil
		}
	}
	info, err := root.Lstat(name)
	if os.IsNotExist(err) {
		return linkMissing, nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "is occupied by a file or directory", nil
	}
	current, err := root.Readlink(name)
	if err != nil {
		return "", err
	}
	if current != target {
		return "points to " + current, nil
	}
	return linkOK, nil
}

const (
	linkOK      = "ok"
	linkMissing = "is missing"
)

// CheckLinks reports every managed link that is missing, wrong, or broken.
// It reads only the state's link list; a starter without links has no findings.
func CheckLinks(root string) ([]string, error) {
	loaded, err := LoadState(root)
	if err != nil {
		return nil, err
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	findings := []string{}
	for _, name := range slices.Sorted(maps.Keys(loaded.State.Links)) {
		target := loaded.State.Links[name]
		status, err := linkStatus(handle, name, target)
		if err != nil {
			return nil, err
		}
		if status == linkOK {
			// The target must exist inside the root, through no further links.
			resolved := path.Join(path.Dir(name), target)
			if info, err := handle.Lstat(resolved); err != nil || !info.IsDir() || strings.HasPrefix(resolved, "../") {
				status = "is broken: " + target + " is not a directory"
			}
		}
		if status != linkOK {
			findings = append(findings, "Link "+name+" "+status)
		}
	}
	return findings, nil
}
