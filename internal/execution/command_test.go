package execution

import (
	"reflect"
	"testing"
)

func TestResolveCommandPreservesLiteralArgumentsWithoutAliasing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	original := []string{"explicit-tool", "", "$(touch unsafe)", "space value", "{tool}", "prefix{tool}"}
	command := append([]string(nil), original...)
	resolved, err := ResolveCommand(command)
	if err != nil || !reflect.DeepEqual(resolved, original) || !reflect.DeepEqual(command, original) {
		t.Fatalf("resolved=%q original=%q error=%v", resolved, command, err)
	}
	resolved[1] = "changed"
	if command[1] != "" {
		t.Fatal("resolved command aliases original arguments")
	}
}

func TestInvalidCommandsFailBeforeLaunch(t *testing.T) {
	for _, command := range [][]string{nil, {""}, {"tool", "\x00"}} {
		if resolved, err := ResolveCommand(command); err == nil || resolved != nil {
			t.Fatalf("invalid command accepted: %q %v", resolved, err)
		}
	}
}
