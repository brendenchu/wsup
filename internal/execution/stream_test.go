//go:build unix

package execution

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestStreamConnectsStandardStreamsAndReportsExitCodes(t *testing.T) {
	t.Setenv("WSUP_EXECUTION_HELPER", "1")
	var stdout, stderr bytes.Buffer
	result, err := Stream(context.Background(), helperCommand(t, "echo"), t.TempDir(), 0, strings.NewReader("in"), &stdout, &stderr)
	if err != nil || result.Status != "passed" || *result.ExitCode != 0 || stdout.String() != "out:in" || stderr.String() != "err" {
		t.Fatalf("echo: %+v %v %q %q", result, err, &stdout, &stderr)
	}
	result, err = Stream(context.Background(), helperCommand(t, "fail"), t.TempDir(), 0, nil, &stdout, &stderr)
	if err != nil || result.Status != "failed" || *result.ExitCode != 7 {
		t.Fatalf("fail: %+v %v", result, err)
	}
	if _, err := Stream(context.Background(), []string{"/nonexistent/tool"}, t.TempDir(), 0, nil, &stdout, &stderr); err == nil {
		t.Fatal("missing tool launched")
	}
}

func TestStreamStopsOnTimeoutAndCancellation(t *testing.T) {
	t.Setenv("WSUP_EXECUTION_HELPER", "1")
	start := time.Now()
	result, err := Stream(context.Background(), helperCommand(t, "sleep"), t.TempDir(), 100*time.Millisecond, nil, nil, nil)
	if err != nil || result.Status != "timeout" || time.Since(start) > stopGrace {
		t.Fatalf("timeout: %+v %v", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	result, err = Stream(ctx, helperCommand(t, "sleep"), t.TempDir(), 0, nil, nil, nil)
	if err != nil || result.Status != "cancelled" {
		t.Fatalf("cancel: %+v %v", result, err)
	}
	if result, err := Stream(ctx, helperCommand(t, "sleep"), t.TempDir(), 0, nil, nil, nil); err != nil || result.Status != "cancelled" || result.ExitCode != nil {
		t.Fatalf("precancelled: %+v %v", result, err)
	}
}
