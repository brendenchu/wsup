package scaffolding

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/wavykit/wsup/internal/catalogue"
	"github.com/wavykit/wsup/internal/configuration"
	"github.com/wavykit/wsup/internal/registry"
)

const (
	// legacyReports is where wsup 0.2.0 wrote check reports.
	legacyReports = ".wsup/reports"
	// backupRoot holds a migration's copies of every file it replaces or
	// removes, under a timestamped folder.
	backupRoot = "workbench/.local/wsup/migrate"
)

// renamedSkills maps wsup 0.2.0 skill names to their replacements.
var renamedSkills = map[string]string{
	"workspace-check": "check-workspace", "registry-checks": "run-checks",
	"template-lifecycle": "update-workspace", "project-onboarding": "onboard-project",
	"ai-guidance-maintenance": "maintain-agents-md", "repository-change": "make-change",
	"commit-message": "write-commit",
}

// MigrationPlan previews converting a wsup 0.2.0 workspace or project. Actions
// are write (new or unedited file), remove (unedited file wsup no longer
// installs), keep (owner-edited file left as it is), move (the reports
// folder), and link. Notes list owner content to review by hand.
type MigrationPlan struct {
	FromVersion string   `json:"from_version"`
	ToVersion   string   `json:"to_version"`
	Starter     string   `json:"starter"`
	Modules     []string `json:"modules"`
	Actions     []Action `json:"actions"`
	Conflicts   []string `json:"conflicts"`
	Notes       []string `json:"notes"`
	// Backup is the folder holding copies of replaced and removed files, set
	// when the migration is applied.
	Backup  string `json:"backup,omitempty"`
	Applied bool   `json:"applied"`
}

// MigrationOptions selects the adapters a migrated starter gets. Nil lists
// mean every built-in adapter, as at init; editors apply to workspaces only.
type MigrationOptions struct {
	Agents  []string
	Editors []string
	// Keep lists existing owner files at paths the conversion installs to
	// keep instead of treating as conflicts (see keepOwnerFile).
	Keep []string
}

// preparedMigration is what apply needs: the plan, observed files, new bytes,
// paths to remove, the reports move, links, and the lock.
type preparedMigration struct {
	root       string
	plan       MigrationPlan
	observed   map[string]observation
	writes     map[string][]byte
	removes    []string
	moveReport bool
	links      map[string]string
	lock       []byte
}

// planMigration reads a 0.2.0 starter's state and registry and plans its
// conversion without writing anything.
func planMigration(root, version string, options MigrationOptions) (preparedMigration, error) {
	root, err := registry.Resolve(root)
	if err != nil {
		return preparedMigration{}, err
	}
	if _, err := os.Lstat(filepath.Join(root, LockPath)); err == nil {
		return preparedMigration{}, fmt.Errorf("%s already exists; this starter needs no migration", LockPath)
	}
	state, err := loadLegacyState(root)
	if err != nil {
		return preparedMigration{}, err
	}
	handle, err := os.OpenRoot(root)
	if err != nil {
		return preparedMigration{}, err
	}
	defer handle.Close()
	result := MigrationPlan{
		FromVersion: state.TemplateVersion, ToVersion: version, Starter: state.Template,
		Modules: []string{}, Actions: []Action{}, Conflicts: []string{}, Notes: []string{},
	}
	prepared := preparedMigration{root: root, observed: map[string]observation{}, writes: map[string][]byte{}}

	selection := catalogue.Selection{Collections: []string{}}
	// renamed maps former collection folders to their replacement ("" when
	// the collection was removed), noted once removals are known.
	renamed := map[string]string{}
	if selection.Agents, err = catalogue.ComposeAdapters(catalogue.KindAgent, options.Agents); err != nil {
		return preparedMigration{}, err
	}
	selection.Editors = []string{}
	if state.Template == "workspace" {
		if selection.Editors, err = catalogue.ComposeAdapters(catalogue.KindEditor, options.Editors); err != nil {
			return preparedMigration{}, err
		}
		selection.Collections, renamed = legacyCollections(state)
		document, notes, err := convertRegistry(handle, root, selection)
		if err != nil {
			return preparedMigration{}, err
		}
		result.Notes = append(result.Notes, notes...)
		if selection.Registry, err = registry.Encode(document); err != nil {
			return preparedMigration{}, err
		}
		prepared.writes[registry.DocumentPath] = selection.Registry
		result.Actions = append(result.Actions, Action{Path: registry.DocumentPath, Action: "write"})
		for _, name := range []string{"workspace.json", registry.DocumentPath} {
			if current, err := observeFile(handle, name); err != nil {
				return preparedMigration{}, err
			} else if current.exists {
				prepared.observed[name] = current
			}
		}
		prepared.removes = append(prepared.removes, "workspace.json")
		result.Actions = append(result.Actions, Action{Path: "workspace.json", Action: "remove"})
	}
	files, links, err := catalogue.Starter(state.Template, selection)
	if err != nil {
		return preparedMigration{}, err
	}
	modules, err := catalogue.Installed(state.Template, selection)
	if err != nil {
		return preparedMigration{}, err
	}
	result.Modules = slices.Sorted(maps.Keys(modules))
	lock := Lock{Version: LockVersion, WsupVersion: version, Starter: state.Template, Modules: modules,
		Files: map[string]string{}, Blocks: map[string]string{}, Links: links}

	names := slices.Concat(slices.Collect(maps.Keys(state.Files)), slices.Collect(maps.Keys(files)))
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		if name == "workspace.json" {
			continue
		}
		current, err := observeFile(handle, name)
		if err != nil {
			return preparedMigration{}, err
		}
		prepared.observed[name] = current
		hex, managed := state.Files[name]
		baseline := "sha256:" + hex
		target, supplied := files[name]
		write := func() {
			prepared.writes[name] = target
			lock.Files[name] = lockDigest(target)
			maps.Copy(lock.Blocks, blockDigests(name, target))
			result.Actions = append(result.Actions, Action{Path: name, Action: "write"})
		}
		switch {
		case current.exists && !current.regular:
			result.Conflicts = append(result.Conflicts, name)
		case supplied && current.regular && current.digest == lockDigest(target):
			// Already the new content; record it.
			lock.Files[name] = current.digest
			maps.Copy(lock.Blocks, blockDigests(name, target))
		case managed && (!current.exists || current.digest == baseline):
			if supplied {
				write()
			} else if current.exists {
				prepared.removes = append(prepared.removes, name)
				result.Actions = append(result.Actions, Action{Path: name, Action: "remove"})
			}
		case managed:
			// An owner edit is never overwritten or removed. A path wsup still
			// installs keeps its genuine 0.2.0 baseline, so update reports it.
			result.Actions = append(result.Actions, Action{Path: name, Action: "keep"})
			if supplied {
				lock.Files[name] = baseline
			} else {
				result.Notes = append(result.Notes, editedRetiredNote(name))
			}
		case current.exists && slices.Contains(options.Keep, name):
			keepOwnerFile(&lock, name, target)
			result.Actions = append(result.Actions, Action{Path: name, Action: "keep"})
		case current.exists:
			result.Conflicts = append(result.Conflicts, name)
		default:
			write()
		}
	}
	if err := checkKept(options.Keep, result.Actions); err != nil {
		return preparedMigration{}, err
	}
	for _, name := range slices.Sorted(maps.Keys(links)) {
		status, err := linkStatus(handle, name, links[name])
		if err != nil {
			return preparedMigration{}, err
		}
		switch status {
		case linkOK:
		case linkMissing:
			result.Actions = append(result.Actions, Action{Path: name, Action: "link"})
		default:
			result.Conflicts = append(result.Conflicts, name)
		}
	}
	prepared.links = links
	for _, old := range slices.Sorted(maps.Keys(renamed)) {
		if note := folderNote(handle, old, renamed[old], prepared.removes); note != "" {
			result.Notes = append(result.Notes, note)
		}
	}
	if notes, err := planReports(handle, &result, &prepared); err != nil {
		return preparedMigration{}, err
	} else {
		result.Notes = append(result.Notes, notes...)
	}
	if body, changed, err := convertLocalSettings(handle); err != nil {
		return preparedMigration{}, err
	} else if changed {
		current, _ := observeFile(handle, "wsup.local.json")
		prepared.observed["wsup.local.json"] = current
		prepared.writes["wsup.local.json"] = body
		result.Actions = append(result.Actions, Action{Path: "wsup.local.json", Action: "write"})
	}
	stateFile, err := observeFile(handle, StatePath)
	if err != nil {
		return preparedMigration{}, err
	}
	prepared.observed[StatePath] = stateFile
	prepared.removes = append(prepared.removes, StatePath)
	result.Actions = append(result.Actions, Action{Path: StatePath, Action: "remove"})
	slices.SortFunc(result.Actions, func(a, b Action) int { return strings.Compare(a.Path, b.Path) })
	slices.Sort(result.Conflicts)
	if prepared.lock, err = EncodeLock(lock); err != nil {
		return preparedMigration{}, err
	}
	prepared.plan = result
	return prepared, nil
}

// loadLegacyState reads 0.2.0 state, mapping retired collection names (which
// ParseState rejects for current workspaces) to their replacements first.
func loadLegacyState(root string) (State, error) {
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(StatePath))); err != nil {
		return State{}, fmt.Errorf("no wsup 0.2.0 state (%s) to migrate: %w", StatePath, err)
	}
	loaded, err := LoadState(root)
	if err == nil {
		return loaded.State, nil
	}
	body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(StatePath)))
	if readErr != nil {
		return State{}, readErr
	}
	var raw map[string]json.RawMessage
	var collections []string
	if json.Unmarshal(body, &raw) != nil || json.Unmarshal(raw["collections"], &collections) != nil {
		return State{}, err
	}
	accepted, retired := []string{}, []string{}
	for _, name := range collections {
		replacement, isRetired := retiredCollections[name]
		switch {
		case !isRetired:
			accepted = append(accepted, name)
		case replacement != "":
			accepted = append(accepted, replacement)
			retired = append(retired, name)
		default:
			retired = append(retired, name)
		}
	}
	raw["collections"], _ = json.Marshal(accepted)
	rewritten, _ := json.Marshal(raw)
	state, parseErr := ParseState(rewritten)
	if parseErr != nil {
		return State{}, parseErr
	}
	state.retired = retired
	return state, nil
}

// legacyCollections returns the collections a migrated workspace selects:
// recorded ones, former default homes it still manages (infrastructure/
// becomes infra), and replacements for renamed ones. It also returns each
// former folder and its replacement ("" for a removed collection).
func legacyCollections(state State) ([]string, map[string]string) {
	selected := state.SelectedCollections()
	renamed := map[string]string{}
	if _, managed := state.Files["infrastructure/.gitkeep"]; managed {
		selected = append(selected, "infra")
		renamed["infrastructure"] = "infra"
	}
	for _, old := range state.retired {
		renamed[old] = retiredCollections[old]
	}
	return slices.Compact(slices.Sorted(slices.Values(selected))), renamed
}

// folderNote explains a former collection folder that still holds files the
// migration does not remove, or returns "" when it will be empty.
func folderNote(handle *os.Root, old, replacement string, removes []string) string {
	remaining := false
	err := fs.WalkDir(handle.FS(), old, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && !slices.Contains(removes, name) {
			remaining = true
			return fs.SkipAll
		}
		return nil
	})
	switch {
	case err != nil || !remaining:
		return ""
	case replacement == "":
		return fmt.Sprintf("The %s collection was removed; %s/ is left as owner content. Keep restricted records outside the workspace.", old, old)
	default:
		return fmt.Sprintf("%s/ is now %s/ and holds owner content; move it yourself and update project paths in wsup.json.", old, replacement)
	}
}

// editedRetiredNote explains an owner-edited file wsup no longer installs.
func editedRetiredNote(name string) string {
	switch {
	case name == "AI.md" || strings.HasPrefix(name, "guidance/rules/"):
		return fmt.Sprintf("%s was edited and is kept; fold your text into AGENTS.md (procedures into skills), then delete it.", name)
	case strings.HasPrefix(name, ".agents/skills/"):
		skill := strings.SplitN(strings.TrimPrefix(name, ".agents/skills/"), "/", 2)[0]
		if renamed, ok := renamedSkills[skill]; ok {
			return fmt.Sprintf("%s was edited and is kept; the skill is now %s, so carry your changes there and delete the old one.", name, renamed)
		}
	}
	return fmt.Sprintf("%s was edited and is kept; wsup no longer installs it.", name)
}

// convertRegistry builds wsup.json from workspace.json and the 0.2.0
// settings-only wsup.json: checks become check tasks, the infrastructure kind
// becomes infra, and report paths move under the new reports folder.
func convertRegistry(handle *os.Root, root string, selection catalogue.Selection) (registry.Document, []string, error) {
	legacy, err := registry.LoadVersion1(root)
	if err != nil {
		return registry.Document{}, nil, fmt.Errorf("workspace.json: %w", err)
	}
	document := registry.Document{
		Schema: registry.SchemaURL, Name: legacy.Name,
		Collections: selection.Collections, Agents: selection.Agents, Editors: selection.Editors,
	}
	notes := []string{}
	for _, project := range legacy.Projects {
		kind, _ := project.Raw["kind"].(string)
		if kind == "infrastructure" {
			kind = "infra"
		}
		entry := registry.ProjectEntry{ID: project.ID, Path: project.Path, Kind: kind, Dependencies: project.Dependencies}
		entry.Repository, _ = project.Raw["repository"].(string)
		entry.Purpose, _ = project.Raw["purpose"].(string)
		entry.Owner, _ = project.Raw["owner"].(string)
		for _, name := range project.CheckNames {
			entry.Tasks = append(entry.Tasks, registry.Task{Name: name, IsCheck: true, Check: project.Checks[name]})
		}
		if strings.HasPrefix(project.Path, "infrastructure/") {
			notes = append(notes, fmt.Sprintf("Project %s is under infrastructure/; its path stays until you move it to infra/.", project.ID))
		}
		document.Projects = append(document.Projects, entry)
	}
	if body, err := handle.ReadFile(registry.DocumentPath); err == nil {
		settings, err := configuration.ParseSettings(registry.DocumentPath, body)
		if err != nil {
			return registry.Document{}, nil, err
		}
		if len(settings) != 0 {
			if document.Settings, err = movedReportDir(settings); err != nil {
				return registry.Document{}, nil, err
			}
		}
	} else if !os.IsNotExist(err) {
		return registry.Document{}, nil, err
	}
	return document, notes, nil
}

// convertLocalSettings rewrites a report_dir under the old reports folder in
// wsup.local.json, reporting whether anything changed.
func convertLocalSettings(handle *os.Root) ([]byte, bool, error) {
	body, err := handle.ReadFile("wsup.local.json")
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	settings, err := configuration.ParseSettings("wsup.local.json", body)
	if err != nil {
		return nil, false, err
	}
	moved, err := movedReportDir(settings)
	if err != nil || moved["report_dir"] == settings["report_dir"] {
		return nil, false, err
	}
	encoded, err := json.MarshalIndent(map[string]any{"version": 1, "settings": moved}, "", "  ")
	return append(encoded, '\n'), true, err
}

// movedReportDir returns settings with a report_dir under .wsup/reports moved
// under the new reports folder.
func movedReportDir(settings map[string]any) (map[string]any, error) {
	moved := maps.Clone(settings)
	value, exists := settings["report_dir"]
	if !exists {
		return moved, nil
	}
	name, _ := value.(string)
	if name != legacyReports && !strings.HasPrefix(name, legacyReports+"/") {
		return nil, fmt.Errorf("report_dir %q is outside %s; set it under %s before migrating", name, legacyReports, configuration.ReportsPath)
	}
	moved["report_dir"] = configuration.ReportsPath + strings.TrimPrefix(name, legacyReports)
	return moved, nil
}

// planReports moves .wsup/reports/ to the new reports folder when it exists,
// as long as nothing is there yet.
func planReports(handle *os.Root, result *MigrationPlan, prepared *preparedMigration) ([]string, error) {
	info, err := handle.Lstat(legacyReports)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		result.Conflicts = append(result.Conflicts, legacyReports)
		return nil, nil
	}
	if _, err := handle.Lstat(configuration.ReportsPath); err == nil {
		result.Conflicts = append(result.Conflicts, configuration.ReportsPath)
		return nil, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	prepared.moveReport = true
	result.Actions = append(result.Actions, Action{Path: legacyReports, Action: "move"})
	return []string{fmt.Sprintf("Reports in %s/ move to %s/.", legacyReports, configuration.ReportsPath)}, nil
}

// Migrate previews converting the 0.2.0 starter at root, or with apply
// converts it: a conflict blocks every write. Every file it replaces or
// removes is first copied to a timestamped folder under
// workbench/.local/wsup/migrate/, wsup.lock is written after the content, and
// the 0.2.0 state is removed last.
func Migrate(root, version string, options MigrationOptions, apply bool) (MigrationPlan, error) {
	prepared, err := planMigration(root, version, options)
	if err != nil || !apply {
		return prepared.plan, err
	}
	return prepared.apply(time.Now)
}

func (p preparedMigration) apply(now func() time.Time) (MigrationPlan, error) {
	result := p.plan
	if len(result.Conflicts) != 0 {
		return result, fmt.Errorf("migration conflicts require manual resolution; nothing was changed: %s", strings.Join(result.Conflicts, ", "))
	}
	root, err := os.OpenRoot(p.root)
	if err != nil {
		return result, err
	}
	defer root.Close()
	for _, name := range slices.Sorted(maps.Keys(p.observed)) {
		current, err := observeFile(root, name)
		if err != nil {
			return result, err
		}
		if current != p.observed[name] {
			return result, fmt.Errorf("%s changed during migration planning; nothing was changed", name)
		}
	}
	backup := path.Join(backupRoot, now().UTC().Format("20060102T150405Z"))
	if err := root.MkdirAll(backupRoot, 0755); err != nil {
		return result, err
	}
	if err := root.Mkdir(backup, 0755); err != nil {
		return result, fmt.Errorf("create migration backup: %w", err)
	}
	result.Backup = backup
	// Once anything is written, errors name the backup to restore from.
	fail := func(err error) (MigrationPlan, error) {
		return result, fmt.Errorf("migration incomplete; restore replaced and removed files from %s before retrying: %w", backup, err)
	}
	for _, name := range slices.Sorted(maps.Keys(p.observed)) {
		_, replaced := p.writes[name]
		if !p.observed[name].exists || (!replaced && !slices.Contains(p.removes, name)) {
			continue
		}
		body, err := root.ReadFile(name)
		if err != nil {
			return fail(err)
		}
		if err := writeManaged(root, path.Join(backup, name), body, true); err != nil {
			return fail(err)
		}
	}
	if p.moveReport {
		if err := root.MkdirAll(path.Dir(configuration.ReportsPath), 0755); err != nil {
			return fail(err)
		}
		if err := root.Rename(legacyReports, configuration.ReportsPath); err != nil {
			return fail(fmt.Errorf("move reports: %w", err))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(p.writes)) {
		if err := writeManaged(root, name, p.writes[name], !p.observed[name].exists); err != nil {
			return fail(err)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(p.links)) {
		if status, err := linkStatus(root, name, p.links[name]); err != nil {
			return fail(err)
		} else if status == linkMissing {
			if err := createLink(root, name, p.links[name]); err != nil {
				return fail(err)
			}
		}
	}
	for _, name := range p.removes {
		if name == StatePath || name == registry.DocumentPath {
			continue
		}
		if err := root.Remove(name); err != nil && !os.IsNotExist(err) {
			return fail(err)
		}
		pruneEmpty(root, path.Dir(name))
	}
	if err := writeManaged(root, LockPath, p.lock, true); err != nil {
		return fail(err)
	}
	if err := root.Remove(StatePath); err != nil {
		return fail(err)
	}
	pruneEmpty(root, path.Dir(StatePath))
	result.Applied = true
	return result, nil
}

// pruneEmpty removes name and its parents while they are empty folders,
// stopping at the root or at the first folder with anything in it.
func pruneEmpty(root *os.Root, name string) {
	for ; name != "." && name != ""; name = path.Dir(name) {
		if root.Remove(name) != nil {
			return
		}
	}
}

// PlanMigration previews converting the 0.2.0 starter at root.
func PlanMigration(root, version string, options MigrationOptions) (MigrationPlan, error) {
	return Migrate(root, version, options, false)
}
