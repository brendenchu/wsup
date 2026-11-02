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
	"text/template"

	"github.com/wavykit/wsup/templates"
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

// ValidID reports whether id has the built-in module id shape: lowercase
// letters and digits in hyphen-separated words.
func ValidID(id string) bool {
	return idPattern.MatchString(id)
}

// ValidVersion reports whether version is MAJOR.MINOR.PATCH without prefixes
// or leading zeros.
func ValidVersion(version string) bool {
	return versionPattern.MatchString(version)
}

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
	// Links maps a workspace-relative path to a relative symlink target. The
	// only link wsup creates points at the shared skill library.
	Links map[string]string `json:"links,omitempty"`
}

// skillLibrary is the one directory a module link may point to.
const skillLibrary = ".agents/skills"

// Module is a validated manifest with the files it installs, keyed by
// workspace-relative path. Templates are rendered against RenderData for the
// resolved selection before they are written.
type Module struct {
	Manifest
	Files     map[string][]byte
	Templates map[string]*template.Template
}

// Entry names one selected collection or deployed skill for templates.
// Summary is the description's first sentence, for compact tables.
type Entry struct {
	Name        string
	Description string
	Summary     string
}

// summary returns text up to and including its first sentence-ending period.
func summary(text string) string {
	if first, _, found := strings.Cut(text, ". "); found {
		return first + "."
	}
	return text
}

// RenderData is the fixed, read-only data set templates see. Templates get no
// functions that touch the filesystem, network, or environment.
type RenderData struct {
	// Collections are the selected collection modules, sorted by id.
	Collections []Entry
	// Skills are the deployed skills, sorted by name.
	Skills []Entry
	// Ignore holds every selected module's ignore entries, sorted and unique.
	Ignore []string
}

// Catalogue holds every validated built-in module and the shared skill library.
type Catalogue struct {
	modules map[string]Module
	// skills maps a skill name to its files, keyed by path inside the skill.
	skills map[string]map[string][]byte
	// descriptions maps a skill name to its SKILL.md frontmatter description.
	descriptions map[string]string
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

// Files returns the union of the resolved modules' files, rendered templates,
// and skills. Two modules writing the same path is a catalogue defect, not a merge.
func (c *Catalogue) Files(ids []string) (map[string][]byte, error) {
	data, err := c.renderData(ids)
	if err != nil {
		return nil, err
	}
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
		for name, tmpl := range module.Templates {
			var rendered bytes.Buffer
			if err := tmpl.Execute(&rendered, data); err != nil {
				return nil, fmt.Errorf("module %q: render %s: %w", id, name, err)
			}
			if err := put(name, rendered.Bytes(), id); err != nil {
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

// Links returns the symlinks the resolved modules install, keyed by path.
func (c *Catalogue) Links(ids []string) (map[string]string, error) {
	links := map[string]string{}
	for _, id := range ids {
		module, ok := c.modules[id]
		if !ok {
			return nil, fmt.Errorf("unknown module %q", id)
		}
		for name, target := range module.Links {
			if previous, exists := links[name]; exists && previous != target {
				return nil, fmt.Errorf("modules disagree on link %s", name)
			}
			links[name] = target
		}
	}
	return links, nil
}

// renderData describes the resolved selection for templates.
func (c *Catalogue) renderData(ids []string) (RenderData, error) {
	data := RenderData{Collections: []Entry{}, Skills: []Entry{}, Ignore: []string{}}
	skills := []string{}
	for _, id := range ids {
		module, ok := c.modules[id]
		if !ok {
			return RenderData{}, fmt.Errorf("unknown module %q", id)
		}
		if module.Kind == KindCollection {
			data.Collections = append(data.Collections, Entry{Name: id, Description: module.Description, Summary: summary(module.Description)})
		}
		skills = append(skills, module.Skills...)
		data.Ignore = append(data.Ignore, module.Ignore...)
	}
	slices.SortFunc(data.Collections, func(a, b Entry) int { return strings.Compare(a.Name, b.Name) })
	slices.Sort(skills)
	for _, skill := range slices.Compact(skills) {
		data.Skills = append(data.Skills, Entry{Name: skill, Description: c.descriptions[skill], Summary: summary(c.descriptions[skill])})
	}
	slices.Sort(data.Ignore)
	data.Ignore = slices.Compact(data.Ignore)
	return data, nil
}

func load(tree fs.FS) (*Catalogue, error) {
	c := &Catalogue{modules: map[string]Module{}, skills: map[string]map[string][]byte{}, descriptions: map[string]string{}}
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
		body, ok := files["SKILL.md"]
		if !ok {
			return nil, fmt.Errorf("skill %q has no SKILL.md", name)
		}
		description, err := skillDescription(name, body)
		if err != nil {
			return nil, err
		}
		c.skills[name] = files
		c.descriptions[name] = description
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
	module := Module{Files: map[string][]byte{}, Templates: map[string]*template.Template{}}
	for _, child := range children {
		switch {
		case child.Name() == "module.json" && child.Type().IsRegular():
		case child.Name() == "files" && child.IsDir():
			if module.Files, err = readTree(tree, dir+"/files"); err != nil {
				return Module{}, err
			}
		case child.Name() == "templates" && child.IsDir():
			sources, err := readTree(tree, dir+"/templates")
			if err != nil {
				return Module{}, err
			}
			for name, body := range sources {
				// missingkey=error turns a typo in a field name into a load failure.
				tmpl, err := template.New(name).Option("missingkey=error").Parse(string(body))
				if err != nil {
					return Module{}, fmt.Errorf("%s/templates: %w", dir, err)
				}
				module.Templates[name] = tmpl
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

// skillDescription reads the simple name/description frontmatter that every
// shared skill carries, requiring the name to match its directory.
func skillDescription(name string, body []byte) (string, error) {
	text, ok := strings.CutPrefix(string(body), "---\n")
	front, _, closed := strings.Cut(text, "\n---\n")
	if !ok || !closed {
		return "", fmt.Errorf("skill %q needs name and description frontmatter", name)
	}
	fields := map[string]string{}
	for _, line := range strings.Split(front, "\n") {
		key, value, _ := strings.Cut(line, ": ")
		fields[key] = value
	}
	if fields["name"] != name || strings.TrimSpace(fields["description"]) == "" {
		return "", fmt.Errorf("skill %q needs a matching name and a description", name)
	}
	if strings.Contains(fields["description"], "|") {
		// Descriptions are rendered into Markdown tables.
		return "", fmt.Errorf("skill %q description must not contain |", name)
	}
	return fields["description"], nil
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
		case id == NoPreset:
			return fail("id is reserved")
		case !slices.Contains(kinds, module.Kind):
			return fail("unknown kind %q", module.Kind)
		case !versionPattern.MatchString(module.Version):
			return fail("version must be MAJOR.MINOR.PATCH")
		case strings.TrimSpace(module.Description) == "":
			return fail("description is required")
		}
		for _, name := range module.Generate {
			if _, known := generators[name]; !known {
				return fail("names unknown generator %q", name)
			}
		}
		for _, entry := range module.Ignore {
			if strings.TrimSpace(entry) != entry || entry == "" || strings.HasPrefix(entry, "#") ||
				strings.ContainsFunc(entry, func(r rune) bool { return r < ' ' }) {
				return fail("ignore entry %q must be one trimmed, non-comment line", entry)
			}
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
		for name := range module.Templates {
			if _, clash := module.Files[name]; clash {
				return fail("installs %s as both a file and a template", name)
			}
		}
		for name := range mergedKeys(module.Files, module.Templates) {
			if !portable(name) || name == ".agents" || strings.HasPrefix(name, ".agents/skills/") {
				return fail("file %q must be a portable workspace path outside .agents/skills", name)
			}
			if Generated(name) {
				return fail("file %q has a generator's output path", name)
			}
		}
		for name, target := range module.Links {
			resolved := path.Join(path.Dir(name), target)
			if !portable(name) || strings.HasPrefix(name, ".agents") || path.IsAbs(target) || resolved != skillLibrary {
				return fail("link %s -> %s must be a relative link to %s", name, target, skillLibrary)
			}
			if mergedKeys(module.Files, module.Templates)[name] {
				return fail("installs %s as both a file and a link", name)
			}
		}
		if module.Kind == KindPreset {
			if len(module.Modules) == 0 || len(module.Files) != 0 || len(module.Templates) != 0 || len(module.Ignore) != 0 || len(module.Links) != 0 || len(module.Generate) != 0 || len(module.Skills) != 0 || len(module.Requires) != 0 {
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
	// Ignore entries are only written through the base's .gitignore template.
	for id, module := range c.modules {
		for _, starter := range module.AppliesTo {
			if _, ok := c.modules[starter+"-base"].Templates[".gitignore"]; len(module.Ignore) != 0 && !ok {
				return fmt.Errorf("module %q: ignore entries need a %s-base .gitignore template", id, starter)
			}
		}
	}
	return nil
}

func mergedKeys[V1, V2 any](a map[string]V1, b map[string]V2) map[string]bool {
	keys := make(map[string]bool, len(a)+len(b))
	for name := range a {
		keys[name] = true
	}
	for name := range b {
		keys[name] = true
	}
	return keys
}

func portable(name string) bool {
	return name != "" && path.Clean(name) == name && !path.IsAbs(name) &&
		name != ".." && !strings.HasPrefix(name, "../") && !strings.ContainsAny(name, "\\:\x00")
}
