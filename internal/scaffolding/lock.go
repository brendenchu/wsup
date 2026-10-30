package scaffolding

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/registry"
)

const (
	// LockPath is the committed record of what wsup installed.
	LockPath = "wsup.lock"
	// LockVersion identifies the lock schema.
	LockVersion = 1
)

var (
	lockDigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	blockKeyPattern   = regexp.MustCompile(`^(.+)#([a-z][a-z0-9-]*)$`)
)

// Lock records the installed modules and the baseline of every file, marked
// block, and link wsup wrote. Like State, it holds baselines, never hashes of
// whatever is on disk now: reading an edited file must not adopt the edit.
type Lock struct {
	Version     int    `json:"version"`
	WsupVersion string `json:"wsup_version"`
	// Starter is workspace or project.
	Starter string `json:"starter"`
	// Modules maps each installed module id to its version.
	Modules map[string]string `json:"modules"`
	// Files maps workspace-relative paths to sha256: digests.
	Files map[string]string `json:"files"`
	// Blocks maps "path#name" to the digest of one generated section.
	Blocks map[string]string `json:"blocks,omitempty"`
	// Links maps each managed symlink to its relative target.
	Links map[string]string `json:"links,omitempty"`
}

// lockDigest returns the lock's digest notation for content.
func lockDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf("sha256:%x", sum)
}

// ParseLock accepts exactly one current-version lock object and validates its
// metadata and paths. It does not read the files it lists.
func ParseLock(body []byte) (Lock, error) {
	if !utf8.Valid(body) {
		return Lock{}, fmt.Errorf("wsup.lock must be UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var lock Lock
	if err := decoder.Decode(&lock); err != nil {
		return Lock{}, fmt.Errorf("wsup.lock: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Lock{}, fmt.Errorf("wsup.lock: unexpected trailing JSON")
	}
	if lock.Version != LockVersion {
		return Lock{}, fmt.Errorf("unsupported wsup.lock version %d; use a compatible wsup binary", lock.Version)
	}
	if strings.TrimSpace(lock.WsupVersion) == "" {
		return Lock{}, fmt.Errorf("wsup.lock: wsup_version is required")
	}
	if lock.Starter != "workspace" && lock.Starter != "project" {
		return Lock{}, fmt.Errorf("wsup.lock: starter must be workspace or project")
	}
	if _, ok := lock.Modules[lock.Starter+"-base"]; !ok {
		return Lock{}, fmt.Errorf("wsup.lock: modules must include %s-base", lock.Starter)
	}
	for id, version := range lock.Modules {
		if !catalogue.ValidID(id) || !catalogue.ValidVersion(version) {
			return Lock{}, fmt.Errorf("wsup.lock: module %q needs a valid id and MAJOR.MINOR.PATCH version", id)
		}
	}
	if lock.Files == nil {
		return Lock{}, fmt.Errorf("wsup.lock: files is required")
	}
	for name, digest := range lock.Files {
		if !portableFile(name) || name == LockPath || name == registry.DocumentPath || !lockDigestPattern.MatchString(digest) {
			return Lock{}, fmt.Errorf("wsup.lock: invalid file entry %q", name)
		}
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, exists := lock.Files[parent]; exists {
				return Lock{}, fmt.Errorf("wsup.lock: %s is both a file and the parent of %s", parent, name)
			}
		}
	}
	for key, digest := range lock.Blocks {
		match := blockKeyPattern.FindStringSubmatch(key)
		if match == nil || !portableFile(match[1]) || !lockDigestPattern.MatchString(digest) {
			return Lock{}, fmt.Errorf("wsup.lock: invalid block entry %q", key)
		}
	}
	for name, target := range lock.Links {
		if _, file := lock.Files[name]; file || !portableFile(name) || target == "" || path.IsAbs(target) || strings.ContainsAny(target, "\\:\x00") {
			return Lock{}, fmt.Errorf("wsup.lock: invalid link entry %q", name)
		}
	}
	return lock, nil
}

// EncodeLock writes a lock with sorted keys, two-space indentation, and a
// trailing newline, refusing any lock ParseLock would reject.
func EncodeLock(lock Lock) ([]byte, error) {
	if lock.Modules == nil {
		lock.Modules = map[string]string{}
	}
	if lock.Files == nil {
		lock.Files = map[string]string{}
	}
	if len(lock.Blocks) == 0 {
		lock.Blocks = nil
	}
	if len(lock.Links) == 0 {
		lock.Links = nil
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(lock); err != nil {
		return nil, err
	}
	if _, err := ParseLock(encoded.Bytes()); err != nil {
		return nil, fmt.Errorf("encode wsup.lock: %w", err)
	}
	return encoded.Bytes(), nil
}

// LoadedLock carries the lock's exact bytes for the apply-time change check.
type LoadedLock struct {
	Lock     Lock
	Original []byte
}

// LoadLock reads and validates wsup.lock at root. A symlinked or missing lock
// fails: a starter without one is not managed by this wsup.
func LoadLock(root string) (LoadedLock, error) {
	root, err := registry.Resolve(root)
	if err != nil {
		return LoadedLock{}, err
	}
	filename := filepath.Join(root, LockPath)
	info, err := os.Lstat(filename)
	if os.IsNotExist(err) {
		if _, stateErr := os.Lstat(filepath.Join(root, filepath.FromSlash(StatePath))); stateErr == nil {
			return LoadedLock{}, fmt.Errorf("this workspace was created by wsup 0.2.0 (%s, no %s); wsup migrate converts it", StatePath, LockPath)
		}
	}
	if err != nil {
		return LoadedLock{}, fmt.Errorf("inspect wsup.lock: %w", err)
	}
	if !info.Mode().IsRegular() {
		return LoadedLock{}, fmt.Errorf("wsup.lock must be a regular file")
	}
	body, err := os.ReadFile(filename)
	if err != nil {
		return LoadedLock{}, err
	}
	lock, err := ParseLock(body)
	if err != nil {
		return LoadedLock{}, err
	}
	return LoadedLock{Lock: lock, Original: body}, nil
}

// Selection returns the lock's installed modules as a catalogue selection.
// Every module must be a built-in module that fits the lock's starter.
func (l Lock) Selection() (catalogue.Selection, error) {
	selection := catalogue.Selection{Collections: []string{}, Agents: []string{}, Editors: []string{}}
	for _, id := range slices.Sorted(maps.Keys(l.Modules)) {
		kind, ok := catalogue.KindOf(id)
		switch {
		case !ok:
			return catalogue.Selection{}, fmt.Errorf("wsup.lock lists module %q, which this wsup does not provide", id)
		case kind == catalogue.KindCollection:
			selection.Collections = append(selection.Collections, id)
		case kind == catalogue.KindAgent:
			selection.Agents = append(selection.Agents, id)
		case kind == catalogue.KindEditor:
			selection.Editors = append(selection.Editors, id)
		case kind == catalogue.KindBase && id == l.Starter+"-base":
		default:
			return catalogue.Selection{}, fmt.Errorf("wsup.lock lists module %q, which a %s cannot install", id, l.Starter)
		}
	}
	return selection, nil
}

// newLock records the given modules and the baselines of files, their marked
// blocks, and links.
func newLock(version, starter string, modules map[string]string, files map[string][]byte, links map[string]string) Lock {
	lock := Lock{
		Version: LockVersion, WsupVersion: version, Starter: starter, Modules: maps.Clone(modules),
		Files: make(map[string]string, len(files)), Blocks: map[string]string{}, Links: maps.Clone(links),
	}
	for name, body := range files {
		lock.Files[name] = lockDigest(body)
		maps.Copy(lock.Blocks, blockDigests(name, body))
	}
	return lock
}

// blockDigests returns the digest of each marked block in a file, keyed
// "path#name". A file with unpaired markers has none.
func blockDigests(name string, body []byte) map[string]string {
	digests := map[string]string{}
	blocks, ok := findBlocks(body)
	if !ok {
		return digests
	}
	for _, b := range blocks {
		digests[name+"#"+b.name] = lockDigest(body[b.start:b.end])
	}
	return digests
}

// readDocument returns wsup.json's bytes for generators, or nil when it is
// absent. It never follows a symlink at that path.
func readDocument(root string) ([]byte, error) {
	root, err := registry.Resolve(root)
	if err != nil {
		return nil, err
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	current, err := observeFile(handle, registry.DocumentPath)
	if err != nil || !current.regular {
		return nil, err
	}
	return handle.ReadFile(registry.DocumentPath)
}
