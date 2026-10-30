package scaffolding

import (
	"bufio"
	"bytes"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/brendenchu/wsup/internal/catalogue"
	"github.com/brendenchu/wsup/internal/guidance"
	"github.com/brendenchu/wsup/internal/registry"
)

// The tables below restate the v0.3.0 contract (sections 1–4 and 7). They are
// written out rather than read from the catalogue so that the matrix fails
// when the catalogue drifts from the contract. Change them together with the
// contract.

var (
	contractBaseSkills = []string{"check-workspace", "maintain-agents-md", "onboard-project", "plan-work", "recap-work", "run-checks", "update-workspace"}
	contractDevSkills  = []string{"diagnose-issue", "infer-conventions", "make-change", "review-change", "write-commit"}
	// The project starter has the development skills plus plan-work.
	contractProjectSkills = []string{"diagnose-issue", "infer-conventions", "make-change", "plan-work", "review-change", "write-commit"}

	contractBaseFiles = map[string][]string{
		"workspace": {".editorconfig", ".gitattributes", ".gitignore", "AGENTS.md", "README.md",
			"docs/README.md", "docs/workspace-model.md", "workbench/README.md", "wsup.json", LockPath},
		"project": {".editorconfig", ".gitattributes", ".gitignore", "AGENTS.md", "README.md",
			"docs/README.md", "workbench/README.md", LockPath},
	}

	// contractPreset is the development preset; it is also the init default.
	contractPreset = []string{"apps", "infra", "packages", "tools"}
)

// contractCollection is one row of the contract's collection table. Every
// collection installs its folder's README.md and AGENTS.md.
type contractCollection struct {
	extra  []string // files beyond README.md and AGENTS.md, relative to the folder
	ignore []string
	skills []string
}

var contractCollections = map[string]contractCollection{
	"apps":      {skills: contractDevSkills},
	"packages":  {skills: contractDevSkills},
	"tools":     {skills: contractDevSkills},
	"infra":     {skills: contractDevSkills},
	"sandbox":   {skills: contractDevSkills},
	"templates": {skills: contractDevSkills},
	"forks":     {skills: contractDevSkills},
	"research":  {skills: []string{"investigate-question", "promote-findings"}},
	"reference": {skills: []string{"investigate-question", "promote-findings"}},
	"decisions": {extra: []string{"TEMPLATE.md"}, skills: []string{"record-decision"}},
	"learning":  {},
	"journal":   {extra: []string{"TEMPLATE.md"}, skills: []string{"review-week"}},
	"archive":   {},
	"releases":  {skills: []string{"prepare-release"}},
	"issues":    {extra: []string{"TEMPLATE.md"}},
	"ai":        {},
	"design":    {},
	"content":   {},
	"data":      {ignore: []string{"data/**/*.local.*"}},
}

// contractAdapter is one row of the contract's adapter tables.
type contractAdapter struct {
	files    []string
	links    map[string]string
	ignore   []string
	starters []string
}

var contractAgents = map[string]contractAdapter{
	"claude": {files: []string{"CLAUDE.md"}, links: map[string]string{".claude/skills": "../.agents/skills"},
		ignore: []string{".claude/settings.local.json"}, starters: []string{"project", "workspace"}},
	"gemini": {files: []string{"GEMINI.md"}, starters: []string{"project", "workspace"}},
}

var contractEditors = map[string]contractAdapter{
	// The workspace name is the default, "My workspace", slugified.
	"vscode": {files: []string{"my-workspace.code-workspace"}, starters: []string{"workspace"}},
}

// matrixEntry is one starter selection. Selections are already resolved, as
// the CLI passes them to InitialiseWith.
type matrixEntry struct {
	name        string
	starter     string
	collections []string
	agents      []string
	editors     []string
}

func acceptanceMatrix() []matrixEntry {
	allAgents := slices.Sorted(maps.Keys(contractAgents))
	allEditors := slices.Sorted(maps.Keys(contractEditors))
	allCollections := slices.Sorted(maps.Keys(contractCollections))
	workspace := func(name string, collections []string) matrixEntry {
		return matrixEntry{name, "workspace", collections, allAgents, allEditors}
	}
	entries := []matrixEntry{
		workspace("base only", nil),
		workspace("development", contractPreset),
		workspace("overlapping project collections", []string{"apps", "tools"}),
		workspace("every project collection", []string{"apps", "forks", "infra", "packages", "sandbox", "templates", "tools"}),
		workspace("knowledge only", []string{"decisions", "journal", "reference", "research"}),
		workspace("mixed groups", []string{"data", "issues", "journal", "tools"}),
		workspace("every collection", allCollections),
		{"no adapters", "workspace", contractPreset, []string{}, []string{}},
		{"project, all agents", "project", nil, allAgents, nil},
		{"project, no agents", "project", nil, []string{}, nil},
	}
	for _, id := range allCollections {
		entries = append(entries, workspace("collection "+id, []string{id}))
	}
	for _, id := range allAgents {
		entries = append(entries,
			matrixEntry{"agent " + id, "workspace", contractPreset, []string{id}, allEditors},
			matrixEntry{"project, agent " + id, "project", nil, []string{id}, nil})
	}
	for _, id := range allEditors {
		entries = append(entries, matrixEntry{"editor " + id, "workspace", contractPreset, allAgents, []string{id}})
	}
	return entries
}

// TestMatrixCoversTheCatalogue keeps the contract tables and the catalogue in
// step, so a new module cannot ship without a matrix row.
func TestMatrixCoversTheCatalogue(t *testing.T) {
	for kind, want := range map[string][]string{
		catalogue.KindCollection: slices.Sorted(maps.Keys(contractCollections)),
		catalogue.KindAgent:      slices.Sorted(maps.Keys(contractAgents)),
		catalogue.KindEditor:     slices.Sorted(maps.Keys(contractEditors)),
	} {
		if got := catalogue.Modules(); !slices.Equal(idsOfKind(got, kind), want) {
			t.Errorf("%s modules: catalogue %v, contract %v", kind, idsOfKind(got, kind), want)
		}
	}
	if presets := catalogue.PresetNames(); !slices.Equal(presets, []string{"development"}) {
		t.Errorf("presets: %v", presets)
	}
	if development, err := catalogue.ComposeCollections("development", nil); err != nil || !slices.Equal(development, contractPreset) {
		t.Errorf("development preset: %v %v", development, err)
	}
	for kind, want := range map[string][]string{
		catalogue.KindAgent:  slices.Sorted(maps.Keys(contractAgents)),
		catalogue.KindEditor: slices.Sorted(maps.Keys(contractEditors)),
	} {
		if defaults, err := catalogue.ComposeAdapters(kind, nil); err != nil || !slices.Equal(defaults, want) {
			t.Errorf("default %s adapters: %v %v", kind, defaults, err)
		}
	}
	covered := map[string]bool{}
	for _, entry := range acceptanceMatrix() {
		for _, id := range slices.Concat(entry.collections, entry.agents, entry.editors) {
			covered[id] = true
		}
	}
	for _, module := range catalogue.Modules() {
		if module.Kind != catalogue.KindBase && module.Kind != catalogue.KindPreset && !covered[module.ID] {
			t.Errorf("no matrix entry selects %s", module.ID)
		}
	}
}

func idsOfKind(modules []catalogue.Module, kind string) []string {
	ids := []string{}
	for _, module := range modules {
		if module.Kind == kind {
			ids = append(ids, module.ID)
		}
	}
	return ids
}

func TestGeneratedOutputMatrix(t *testing.T) {
	for _, entry := range acceptanceMatrix() {
		t.Run(entry.name, func(t *testing.T) {
			t.Parallel()
			root := filepath.Join(t.TempDir(), entry.starter)
			selection := catalogue.Selection{Collections: entry.collections, Agents: entry.agents, Editors: entry.editors}
			if _, err := InitialiseWith(root, "0.1.0", entry.starter, selection); err != nil {
				t.Fatal(err)
			}
			files, links, skills, ignore := entry.expected()
			checkInventory(t, root, files, links)
			checkSkills(t, root, skills)
			checkSelectionRecorded(t, root, entry)
			checkMarkdownLinks(t, root)
			checkIgnoreBlock(t, root, ignore)
			if entry.starter == "workspace" {
				checkLayoutBlock(t, root, entry.collections)
			}
			plan, err := Update(root, "0.1.0", nil, false)
			if err != nil || len(plan.Actions) != 0 || len(plan.Conflicts) != 0 {
				t.Errorf("fresh starter does not settle: %+v %v", plan, err)
			}
		})
	}
}

// expected returns the contract's file inventory, links, skills, and ignore
// entries for the entry.
func (entry matrixEntry) expected() (files []string, links map[string]string, skills, ignore []string) {
	files = slices.Clone(contractBaseFiles[entry.starter])
	links = map[string]string{}
	skills = contractBaseSkills
	if entry.starter == "project" {
		skills = contractProjectSkills
	}
	for _, id := range entry.collections {
		collection := contractCollections[id]
		files = append(files, id+"/README.md", id+"/AGENTS.md")
		for _, name := range collection.extra {
			files = append(files, id+"/"+name)
		}
		skills = slices.Concat(skills, collection.skills)
		ignore = append(ignore, collection.ignore...)
	}
	adapters := []contractAdapter{}
	for _, id := range entry.agents {
		adapters = append(adapters, contractAgents[id])
	}
	for _, id := range entry.editors {
		adapters = append(adapters, contractEditors[id])
	}
	for _, adapter := range adapters {
		files = append(files, adapter.files...)
		maps.Copy(links, adapter.links)
		ignore = append(ignore, adapter.ignore...)
	}
	for _, skill := range skills {
		files = append(files, ".agents/skills/"+skill+"/SKILL.md")
	}
	slices.Sort(files)
	files = slices.Compact(files)
	skills = slices.Compact(slices.Sorted(slices.Values(skills)))
	slices.Sort(ignore)
	return files, links, skills, ignore
}

// checkInventory compares every regular file and symlink under root with the
// contract, and checks that each link resolves to its target directory.
func checkInventory(t *testing.T, root string, files []string, links map[string]string) {
	t.Helper()
	gotFiles, gotLinks := []string{}, map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(name)
			gotLinks[relative] = target
			return err
		}
		gotFiles = append(gotFiles, relative)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(gotFiles, files) {
		t.Errorf("files:\n got %v\nwant %v", gotFiles, files)
	}
	if !maps.Equal(gotLinks, links) {
		t.Errorf("links: got %v, want %v", gotLinks, links)
	}
	for name, target := range links {
		resolved, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(name)))
		want, _ := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(name), "..", filepath.FromSlash(target)))
		if err != nil || resolved != want {
			t.Errorf("link %s does not resolve to %s: %s %v", name, target, resolved, err)
		}
	}
}

// checkSkills compares the installed skills with the contract, through
// guidance (which also validates their metadata), and the AGENTS.md skills
// table, which must link exactly those skills.
func checkSkills(t *testing.T, root string, want []string) {
	t.Helper()
	inspection, err := guidance.Inspect(root)
	if err != nil || len(inspection.Errors) != 0 {
		t.Fatalf("guidance: %v %v", inspection.Errors, err)
	}
	installed := []string{}
	for _, skill := range inspection.Skills {
		installed = append(installed, skill.Name)
	}
	slices.Sort(installed)
	if !slices.Equal(installed, want) {
		t.Errorf("skills: got %v, want %v", installed, want)
	}
	routes := regexp.MustCompile(`\]\(\.agents/skills/([a-z0-9-]+)/SKILL\.md\)`)
	listed := []string{}
	for _, match := range routes.FindAllStringSubmatch(markedBlock(t, root, "AGENTS.md", "<!-- wsup:begin skills -->", "<!-- wsup:end skills -->"), -1) {
		listed = append(listed, match[1])
	}
	if !slices.Equal(listed, want) {
		t.Errorf("AGENTS.md skills table: got %v, want %v", listed, want)
	}
}

// checkSelectionRecorded checks that wsup.json (workspaces) and wsup.lock
// record exactly the resolved selection.
func checkSelectionRecorded(t *testing.T, root string, entry matrixEntry) {
	t.Helper()
	modules := []string{entry.starter + "-base"}
	modules = slices.Concat(modules, entry.collections, entry.agents, entry.editors)
	slices.Sort(modules)
	loaded, err := LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := slices.Sorted(maps.Keys(loaded.Lock.Modules)); !slices.Equal(got, modules) {
		t.Errorf("wsup.lock modules: got %v, want %v", got, modules)
	}
	if entry.starter != "workspace" {
		return
	}
	document, _, err := registry.LoadDocument(root)
	if err != nil {
		t.Fatal(err)
	}
	for field, pair := range map[string][2][]string{
		"collections": {document.Collections, entry.collections},
		"agents":      {document.Agents, entry.agents},
		"editors":     {document.Editors, entry.editors},
	} {
		if !slices.Equal(pair[0], pair[1]) {
			t.Errorf("wsup.json %s: got %v, want %v", field, pair[0], pair[1])
		}
	}
}

// checkMarkdownLinks resolves every relative Markdown link to a regular file
// inside root.
func checkMarkdownLinks(t *testing.T, root string) {
	t.Helper()
	links := regexp.MustCompile(`\]\(([^)]+)\)`)
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(name, ".md") {
			return err
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		for _, match := range links.FindAllSubmatch(body, -1) {
			target := string(match[1])
			if strings.Contains(target, "://") || strings.HasPrefix(target, "#") {
				continue
			}
			resolved := filepath.Join(filepath.Dir(name), strings.SplitN(target, "#", 2)[0])
			relative, err := filepath.Rel(root, resolved)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				t.Errorf("link escapes the starter: %s -> %s", name, target)
				continue
			}
			if info, err := os.Stat(resolved); err != nil || !info.Mode().IsRegular() {
				t.Errorf("unresolved link: %s -> %s", name, target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func checkIgnoreBlock(t *testing.T, root string, want []string) {
	t.Helper()
	got := []string{}
	scanner := bufio.NewScanner(strings.NewReader(markedBlock(t, root, ".gitignore", "# wsup:begin ignore", "# wsup:end ignore")))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			got = append(got, line)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf(".gitignore block: got %v, want %v", got, want)
	}
}

// checkLayoutBlock checks that the AGENTS.md layout lists the base folders
// and exactly the selected collections.
func checkLayoutBlock(t *testing.T, root string, collections []string) {
	t.Helper()
	folders := regexp.MustCompile("(?m)^- `([a-z0-9-]+)/`:")
	got := []string{}
	for _, match := range folders.FindAllStringSubmatch(markedBlock(t, root, "AGENTS.md", "<!-- wsup:begin layout -->", "<!-- wsup:end layout -->"), -1) {
		got = append(got, match[1])
	}
	if want := slices.Concat([]string{"docs", "workbench"}, collections); !slices.Equal(got, want) {
		t.Errorf("AGENTS.md layout: got %v, want %v", got, want)
	}
}

// markedBlock returns the text between a file's begin and end markers. A
// missing file or marker is a test failure.
func markedBlock(t *testing.T, root, name, begin, end string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := bytes.Cut(body, []byte(begin))
	block, _, closed := bytes.Cut(rest, []byte(end))
	if !found || !closed {
		t.Fatalf("%s has no %s block", name, begin)
	}
	return string(block)
}
