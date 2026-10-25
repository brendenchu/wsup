//go:build !unix

package execution

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestUnsupportedPlatformRefusesExecution(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(context.Background(), []string{binary}, t.TempDir(), time.Second, 128)
	if err == nil || result.Status != "" || result.ExitCode != nil {
		t.Fatalf("unsupported supervision claimed success: %+v %v", result, err)
	}
}
