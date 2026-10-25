package execution

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestResolvePythonTokenPreservesOriginalArguments(t *testing.T) {
	original := []string{"{python}", "", "$(touch unsafe)", "space value", "{python}", "prefix{python}"}
	command := append([]string(nil), original...)
	calls := 0
	resolved, err := resolveCommand(command, func(name string) (string, error) {
		calls++
		if name != "python3" {
			t.Fatalf("looked up %q", name)
		}
		return "/synthetic/python3", nil
	})
	expected := append([]string(nil), original...)
	expected[0], expected[4] = "/synthetic/python3", "/synthetic/python3"
	if err != nil || calls != 1 || !reflect.DeepEqual(resolved, expected) ||
		!reflect.DeepEqual(command, original) {
		t.Fatalf("resolved=%q original=%q calls=%d error=%v", resolved, command, calls, err)
	}
	resolved[1] = "changed"
	if command[1] != "" {
		t.Fatal("resolved command aliases original arguments")
	}
}

func TestMissingPythonIsExplicitWithoutPartialCommand(t *testing.T) {
	resolved, err := resolveCommand([]string{"{python}", "-c", "pass"},
		func(string) (string, error) { return "", exec.ErrNotFound })
	if !errors.Is(err, exec.ErrNotFound) || resolved != nil {
		t.Fatalf("missing interpreter produced %q, %v", resolved, err)
	}
}

func TestExplicitCommandsDoNotRequirePython(t *testing.T) {
	command := []string{"specific-interpreter", "{python}-suffix", ""}
	resolved, err := resolveCommand(command, func(name string) (string, error) {
		t.Fatalf("unexpected Python lookup: %s", name)
		return "", exec.ErrNotFound
	})
	if err != nil || !reflect.DeepEqual(command, resolved) {
		t.Fatalf("literal arguments changed: %q %v", resolved, err)
	}
}

func TestInvalidCommandsFailBeforeLookup(t *testing.T) {
	for _, command := range [][]string{nil, {""}, {"tool", "\x00"}} {
		if _, err := resolveCommand(command, func(string) (string, error) {
			t.Fatal("invalid command triggered interpreter lookup")
			return "", exec.ErrNotFound
		}); err == nil {
			t.Fatalf("invalid command accepted: %q", command)
		}
	}

}

func TestResolveCommandUsesPathAndReportsMissingPython(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("PATH", directory)
	command := []string{"{python}", "-c", "pass"}
	if resolved, err := ResolveCommand(command); !errors.Is(err, exec.ErrNotFound) || resolved != nil {
		t.Fatalf("missing PATH interpreter: %q %v", resolved, err)
	}
	name := "python3"
	if runtime.GOOS == "windows" {
		name += ".exe"
		t.Setenv("PATHEXT", ".EXE")
	}
	filename := filepath.Join(directory, name)
	if err := os.WriteFile(filename, []byte("synthetic executable; lookup only\n"), 0700); err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveCommand(command)
	if err != nil || len(resolved) != len(command) || resolved[0] != filename ||
		command[0] != "{python}" {
		t.Fatalf("PATH resolution: %q %v", resolved, err)
	}
}
