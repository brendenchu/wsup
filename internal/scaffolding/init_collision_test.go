package scaffolding

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentProjectInitializersNeverReplaceDestination(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "project")
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			<-start
			_, err := InitialiseProject(destination, "0.1.0")
			results <- err
		})
	}
	close(start)
	workers.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one exclusive initializer, got %d", successes)
	}
	if _, err := LoadState(destination); err != nil {
		t.Fatalf("winning initializer incomplete: %v", err)
	}
}

func TestProjectInitializationLateFileCollisionPreservesOwnerBytes(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "project")
	_, err := initialiseProject(destination, "0.1.0", map[string][]byte{"README.md": []byte("starter")}, func(root *os.Root, name string, body []byte, exclusive bool) error {
		if name == "README.md" {
			writeFixture(t, destination, name, []byte("concurrent owner"))
		}
		return writeManaged(root, name, body, exclusive)
	})
	if err == nil {
		t.Fatal("late file collision accepted")
	}
	body, err := os.ReadFile(filepath.Join(destination, "README.md"))
	if err != nil || string(body) != "concurrent owner" {
		t.Fatal("concurrent owner bytes overwritten or removed")
	}
	if _, err := os.Lstat(filepath.Join(destination, StatePath)); !os.IsNotExist(err) {
		t.Fatal("failed initializer wrote state")
	}
}
