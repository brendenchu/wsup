package cli

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/brendenchu/wsup/internal/catalogue"
)

const (
	readmePath = "../../README.md"
	modelPath  = "../../templates/modules/workspace-base/files/docs/workspace-model.md"
)

// section returns the Markdown under a "## heading" up to the next heading of
// the same level.
func section(t *testing.T, path, heading string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(body), "\n"+heading+"\n")
	if !found {
		t.Fatalf("%s has no %q section", path, heading)
	}
	level := strings.SplitN(heading, " ", 2)[0] + " "
	if end := strings.Index(rest, "\n"+level); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

// tableRows returns each table row's cells, keyed by the first word of its
// first cell with backticks removed.
func tableRows(text string) map[string][]string {
	rows := map[string][]string{}
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), " | ")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		key := strings.Fields(strings.Trim(cells[0], "`"))[0]
		rows[strings.Trim(key, "`")] = cells
	}
	return rows
}

// TestHelpAndDocsAgree keeps the README command table and the workspace
// model's command reference in step with the CLI: every command is documented
// in both, every documented command exists, and the reference names every
// flag of its command.
func TestHelpAndDocsAgree(t *testing.T) {
	commands := map[string]*cobra.Command{}
	for _, command := range newCommand(session{}).Commands() {
		if command.Name() != "help" {
			commands[command.Name()] = command
		}
	}
	readme := tableRows(section(t, readmePath, "## Commands"))
	reference := tableRows(section(t, modelPath, "## Command reference"))
	for name, command := range commands {
		if _, ok := readme[name]; !ok {
			t.Errorf("README command table lacks %s", name)
		}
		row, ok := reference[name]
		if !ok {
			t.Errorf("command reference lacks %s", name)
			continue
		}
		command.LocalFlags().VisitAll(func(flag *pflag.Flag) {
			if flag.Name != "help" && !strings.Contains(strings.Join(row, " "), "--"+flag.Name) {
				t.Errorf("command reference for %s omits --%s", name, flag.Name)
			}
		})
	}
	for _, table := range []map[string][]string{readme, reference} {
		for name := range table {
			if _, ok := commands[name]; !ok {
				t.Errorf("docs list a command wsup does not have: %s", name)
			}
		}
	}
}

// TestModuleCatalogueDocMatchesTheCatalogue checks the workspace model's
// collection table against every collection's manifest and files.
func TestModuleCatalogueDocMatchesTheCatalogue(t *testing.T) {
	codeChange := []string{"diagnose-issue", "infer-conventions", "make-change", "review-change", "write-commit"}
	rows := tableRows(section(t, modelPath, "## Module catalogue"))
	ticked := regexp.MustCompile("`([^`]+)`")
	listed := func(cell string) []string {
		values := []string{}
		for _, match := range ticked.FindAllStringSubmatch(cell, -1) {
			values = append(values, match[1])
		}
		return values
	}
	for _, module := range catalogue.Modules() {
		if module.Kind != catalogue.KindCollection {
			continue
		}
		row, ok := rows[module.ID]
		if !ok || len(row) != 5 {
			t.Errorf("catalogue table lacks a row for %s", module.ID)
			continue
		}
		if row[1]+"." != module.Description {
			t.Errorf("%s description: doc %q, manifest %q", module.ID, row[1], module.Description)
		}
		extra := []string{}
		for name := range module.Files {
			if base := strings.TrimPrefix(name, module.ID+"/"); base != "README.md" && base != "AGENTS.md" {
				extra = append(extra, base)
			}
		}
		slices.Sort(extra)
		if got := listed(row[2]); !slices.Equal(got, extra) {
			t.Errorf("%s extra files: doc %v, catalogue %v", module.ID, got, extra)
		}
		if got := listed(row[3]); !slices.Equal(got, module.Ignore) {
			t.Errorf("%s ignores: doc %v, manifest %v", module.ID, got, module.Ignore)
		}
		skills := map[string]string{"code-change": strings.Join(codeChange, ","), "—": ""}[row[4]]
		if skills != strings.Join(module.Skills, ",") {
			t.Errorf("%s skills: doc %q, manifest %v", module.ID, row[4], module.Skills)
		}
	}
	// The section also holds the two-column bases and presets tables.
	for id, row := range rows {
		if kind, _ := catalogue.KindOf(id); len(row) == 5 && kind != catalogue.KindCollection {
			t.Errorf("catalogue table lists %s, which is not a collection", id)
		}
	}
}
