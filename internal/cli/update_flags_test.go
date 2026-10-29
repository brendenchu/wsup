package cli

import (
	"reflect"
	"testing"

	"github.com/spf13/pflag"
)

func TestUpdateExposesOnlyContentUpdateOptions(t *testing.T) {
	command, _, err := newCommand(session{}).Find([]string{"update"})
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		names = append(names, flag.Name)
	})
	if !reflect.DeepEqual(names, []string{"apply", "keep"}) {
		t.Fatalf("unexpected content update options: %v", names)
	}
}
