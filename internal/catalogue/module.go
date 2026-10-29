package catalogue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/brendenchu/wsup/templates"
)

// Module kinds. Every installable unit is a module; kinds only change how the
// CLI selects them.
const (
	KindBase       = "base"
	KindCollection = "collection"
	KindAgent      = "agent"
	KindEditor     = "editor"
	KindPreset     = "preset"
)

var (
	kinds          = []string{KindBase, KindCollection, KindAgent, KindEditor, KindPreset}
	starters       = []string{"workspace", "project"}
	idPattern      = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
)

// Manifest is the predefined module shape read from modules/<id>/module.json.
// Built-in and future external modules use exactly this format.
type Manifest struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	AppliesTo   []string `json:"applies_to"`
	Requires    []string `json:"requires"`
	Skills      []string `json:"skills"`
	Ignore      []string `json:"ignore"`
	Generate    []string `json:"generate"`
	Modules     []string `json:"modules,omitempty"`
}

// Module is a validated manifest with the files it installs, keyed by
// workspace-relative path.
type Module struct {
	Manifest
	Files map[string][]byte
}

// Catalogue holds every validated built-in module and the shared skill library.
type Catalogue struct {
	modules map[string]Module
	// skills maps a skill name to its files, keyed by path inside the skill.
	skills map[string]map[string][]byte
}

var builtIn = sync.OnceValues(func() (*Catalogue, error) { return load(templates.Files) })

// Load returns the validated embedded catalogue. The result is shared and must
// be treated as read-only.
func Load() (*Catalogue, error) {
	return builtIn()
}

// Module returns one module by id.
func (c *Catalogue) Module(id string) (Module, bool) {
	module, ok := c.modules[id]
	return module, ok
}

// IDs returns the sorted ids of every module of the given kind.
func (c *Catalogue) IDs(kind string) []string {
	ids := []string{}
	for id, module := range c.modules {
		if module.Kind == kind {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// Resolve returns the starter's base module plus the selected modules and
// everything they require, sorted with the base first. Presets expand to their
// modules and are not themselves returned.
func (c *Catalogue) Resolve(starter string, selected []string) ([]string, error) {
	if !slices.Contains(starters, starter) {
		return nil, fmt.Errorf("template must be workspace or project")
	}
	base := starter + "-base"
	seen := map[string]bool{}
	var visit func(id string) error
	visit = func(id string) error {
		module, ok := c.modules[id]
		if !ok {
			return fmt.Errorf("unknown module %q", id)
		}
		if module.Kind == KindPreset {
			for _, member := range module.Modules {
				if err := visit(member); err != nil {
					return err
				}
			}
			return nil
		}
		if seen[id] {
			return nil
		}
		if !slices.Contains(module.AppliesTo, starter) {
			return fmt.Errorf("module %q does not apply to a %s", id, starter)
		}
		if module.Kind == KindBase && id != base {
			return fmt.Errorf("module %q is a base for another starter", id)
		}
		seen[id] = true
		for _, required := range module.Requires {
			if err := visit(required); err != nil {
				return err
			}
		}
		return nil
	}
	for _, id := range append([]string{base}, selected...) {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		if id != base {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return append([]string{base}, ids...), nil
}

// Files returns the union of the resolved modules' files and their skills.
// Two modules writing the same path is a catalogue defect, not a merge.
func (c *Catalogue) Files(ids []string) (map[string][]byte, error) {
	files := map[string][]byte{}
	owners := map[string]string{}
	put := func(name string, body []byte, owner string) error {
		if previous, exists := owners[name]; exists && previous != owner {
			return fmt.Errorf("modules %q and %q both install %s", previous, owner, name)
		}
		owners[name] = owner
		files[name] = body
		return nil
	}
	for _, id := range ids {
		module, ok := c.modules[id]
		if !ok {
			return nil, fmt.Errorf("unknown module %q", id)
		}
		for name, body := range module.Files {
			if err := put(name, body, id); err != nil {
				return nil, err
			}
		}
		for _, skill := range module.Skills {
			for name, body := range c.skills[skill] {
				// Skills are shared, so any owning module installs identical bytes.
				if err := put(".agents/skills/"+skill+"/"+name, body, "skill:"+skill); err != nil {
					return nil, err
				}
			}
		}
	}
	return files, nil
}

func load(tree fs.FS) (*Catalogue, error) {
	c := &Catalogue{modules: map[string]Module{}, skills: map[string]map[string][]byte{}}
	skillDirs, err := fs.ReadDir(tree, "skills")
	if err != nil {
		return nil, fmt.Errorf("embedded skill library: %w", err)
	}
	for _, entry := range skillDirs {
		name := entry.Name()
		if !entry.IsDir() || !idPattern.MatchString(name) {
			return nil, fmt.Errorf("skill library entry %q must be a skill directory", name)
		}
		files, err := readTree(tree, "skills/"+name)
		if err != nil {
			return nil, err
		}
		if _, ok := files["SKILL.md"]; !ok {
			return nil, fmt.Errorf("skill %q has no SKILL.md", name)
		}
		c.skills[name] = files
	}
	moduleDirs, err := fs.ReadDir(tree, "modules")
	if err != nil {
		return nil, fmt.Errorf("embedded modules: %w", err)
	}
	for _, entry := range moduleDirs {
		module, err := loadModule(tree, entry)
		if err != nil {
			return nil, err
		}
		c.modules[module.ID] = module
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func loadModule(tree fs.FS, entry fs.DirEntry) (Module, error) {
	dir := "modules/" + entry.Name()
	if !entry.IsDir() {
		return Module{}, fmt.Errorf("%s must be a module directory", dir)
	}
	children, err := fs.ReadDir(tree, dir)
	if err != nil {
		return Module{}, err
	}
	module := Module{Files: map[string][]byte{}}
	for _, child := range children {
		switch {
		case child.Name() == "module.json" && child.Type().IsRegular():
		case child.Name() == "files" && child.IsDir():
			if module.Files, err = readTree(tree, dir+"/files"); err != nil {
				return Module{}, err
			}
		default:
			return Module{}, fmt.Errorf("%s contains unsupported entry %q", dir, child.Name())
		}
	}
	body, err := fs.ReadFile(tree, dir+"/module.json")
	if err != nil {
		return Module{}, fmt.Errorf("%s: %w", dir, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&module.Manifest); err != nil {
		return Module{}, fmt.Errorf("%s/module.json: %w", dir, err)
	}
	if module.ID != entry.Name() {
		return Module{}, fmt.Errorf("%s/module.json id %q must match its directory", dir, module.ID)
	}
	return module, nil
}

// readTree returns regular files below base keyed by base-relative slash path.
func readTree(tree fs.FS, base string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := fs.WalkDir(tree, base, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("module catalogue must contain only regular files: %s", name)
		}
		body, err := fs.ReadFile(tree, name)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(name, base+"/")] = body
		return nil
	})
	return files, err
}

func (c *Catalogue) validate() error {
	for id, module := range c.modules {
		fail := func(format string, args ...any) error {
			return fmt.Errorf("module %q: "+format, append([]any{id}, args...)...)
		}
		switch {
		case !idPattern.MatchString(id):
			return fail("id must be lowercase letters, digits, and hyphens")
		case !slices.Contains(kinds, module.Kind):
			return fail("unknown kind %q", module.Kind)
		case !versionPattern.MatchString(module.Version):
			return fail("version must be MAJOR.MINOR.PATCH")
		case strings.TrimSpace(module.Description) == "":
			return fail("description is required")
		case len(module.Generate) != 0:
			// Generators arrive with the editor and agent adapters.
			return fail("no generators are available yet")
		case len(module.Ignore) != 0:
			return fail("ignore entries are not supported yet")
		}
		for _, name := range module.Requires {
			if required, ok := c.modules[name]; !ok || required.Kind == KindPreset || required.Kind == KindBase {
				return fail("requires unknown or non-installable module %q", name)
			}
		}
		for _, skill := range module.Skills {
			if _, ok := c.skills[skill]; !ok {
				return fail("lists unknown skill %q", skill)
			}
		}
		for name := range module.Files {
			if !portable(name) || name == ".agents" || strings.HasPrefix(name, ".agents/skills/") {
				return fail("file %q must be a portable workspace path outside .agents/skills", name)
			}
		}
		if module.Kind == KindPreset {
			if len(module.Modules) == 0 || len(module.Files) != 0 || len(module.Skills) != 0 || len(module.Requires) != 0 {
				return fail("a preset only lists modules")
			}
			for _, member := range module.Modules {
				if target, ok := c.modules[member]; !ok || target.Kind == KindPreset || target.Kind == KindBase {
					return fail("preset lists unknown or non-installable module %q", member)
				}
			}
			continue
		}
		if len(module.Modules) != 0 {
			return fail("only presets list modules")
		}
		if len(module.AppliesTo) == 0 {
			return fail("applies_to is required")
		}
		for _, starter := range module.AppliesTo {
			if !slices.Contains(starters, starter) {
				return fail("applies_to must name workspace or project")
			}
		}
		if module.Kind == KindBase && (len(module.AppliesTo) != 1 || id != module.AppliesTo[0]+"-base") {
			return fail("a base module is named <starter>-base and applies to that starter only")
		}
	}
	for _, starter := range starters {
		if _, ok := c.modules[starter+"-base"]; !ok {
			return fmt.Errorf("catalogue has no %s-base module", starter)
		}
	}
	return nil
}

func portable(name string) bool {
	return name != "" && path.Clean(name) == name && !path.IsAbs(name) &&
		name != ".." && !strings.HasPrefix(name, "../") && !strings.ContainsAny(name, "\\:\x00")
}
