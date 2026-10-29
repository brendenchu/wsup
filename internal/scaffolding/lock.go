package scaffolding

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/brendenchu/wsup/internal/catalogue"
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
		if !portableFile(name) || name == LockPath || !lockDigestPattern.MatchString(digest) {
			return Lock{}, fmt.Errorf("wsup.lock: invalid file entry %q", name)
		}
	}
	for key, digest := range lock.Blocks {
		match := blockKeyPattern.FindStringSubmatch(key)
		if match == nil || !portableFile(match[1]) || !lockDigestPattern.MatchString(digest) {
			return Lock{}, fmt.Errorf("wsup.lock: invalid block entry %q", key)
		}
	}
	for name, target := range lock.Links {
		if !portableFile(name) || target == "" || path.IsAbs(target) || strings.ContainsAny(target, "\\:\x00") {
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
