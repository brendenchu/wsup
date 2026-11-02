package reporting

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wavykit/wsup/internal/configuration"
	"github.com/wavykit/wsup/internal/registry"
)

// Evidence deliberately has no command, environment, stdout, or stderr fields.
// Error contains a machine-readable failure code, never a tool's diagnostic text.
type Evidence struct {
	ExitCode        *int    `json:"exit_code"`
	DurationSeconds float64 `json:"duration_seconds"`
	OutputBytes     int64   `json:"output_bytes"`
	OutputTruncated bool    `json:"output_truncated"`
	Error           string  `json:"error,omitempty"`
}

// Result corresponds to one selection, in selection order. Skips have a reason
// and no Evidence; all executed checks have Evidence and no reason.
type Result struct {
	Status string
	Reason string
	*Evidence
}

type recordedResult struct {
	Project string `json:"project"`
	Check   string `json:"check"`
	Mode    string `json:"mode"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	*Evidence
}

type selection struct {
	Project       string `json:"project"`
	Check         string `json:"check"`
	Path          string `json:"path"`
	CommandSHA256 string `json:"command_sha256"`
}

type gitState struct {
	Available bool   `json:"available"`
	Head      string `json:"head,omitempty"`
	Dirty     *bool  `json:"dirty,omitempty"`
}

func (state gitState) MarshalJSON() ([]byte, error) {
	if !state.Available {
		return []byte(`{"available":false}`), nil
	}
	return json.Marshal(struct {
		Available bool   `json:"available"`
		Head      string `json:"head"`
		Dirty     *bool  `json:"dirty"`
	}{true, state.Head, state.Dirty})
}

type report struct {
	Version       int               `json:"version"`
	WsupVersion   string            `json:"wsup_version"`
	RecordedAt    string            `json:"recorded_at"`
	Runtime       Runtime           `json:"runtime"`
	Git           gitState          `json:"git"`
	ConfigSources map[string]string `json:"config_sources"`
	Inputs        map[string]string `json:"inputs"`
	Selection     []selection       `json:"selection"`
	Results       []recordedResult  `json:"results"`
	Summary       map[string]int    `json:"summary"`
}

var (
	failureCode = regexp.MustCompile(`^(launch_error:[0-9]+|pipe_cleanup_error)$`)
	gitObjectID = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
)

// Write records validated registry selections and sanitized execution results in
// exclusive, private JSON/Markdown reports. It returns the JSON
// path only after both writes succeed. A later I/O failure may leave a partial
// pair; the returned error names that path rather than claiming atomicity.
func Write(root, version string, config configuration.Result, selected []registry.Selection, results []Result) (string, error) {
	return write(root, version, config, selected, results, time.Now, rand.Reader, readGit)
}

func write(root, version string, config configuration.Result, selected []registry.Selection, results []Result,
	now func() time.Time, entropy io.Reader, git func(string) gitState) (string, error) {
	root, err := registry.Resolve(root)
	if err != nil {
		return "", err
	}
	data, err := build(version, config.Sources, selected, results)
	if err != nil {
		return "", err
	}
	data.Inputs = map[string]string{}
	for _, name := range []string{registry.DocumentPath, "wsup.local.json"} {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect report input %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("report input %s must be a regular file", name)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read report input %s: %w", name, err)
		}
		sum := sha256.Sum256(body)
		data.Inputs[name] = hex.EncodeToString(sum[:])
	}
	at := now().UTC()
	data.RecordedAt = at.Format(time.RFC3339Nano)
	data.Git = git(root)
	var random [6]byte
	if _, err := io.ReadFull(entropy, random[:]); err != nil {
		return "", fmt.Errorf("create report ID: %w", err)
	}
	stem := at.Format("20060102T150405") + "-" + hex.EncodeToString(random[:])
	body, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode report: %w", err)
	}
	markdown := "# Local verification\n\nRecorded: " + data.RecordedAt + "\n\n"
	for _, result := range data.Results {
		markdown += fmt.Sprintf("- %s:%s: %s\n", result.Project, result.Check, result.Status)
	}
	target, directory, err := openDirectory(root, config.Settings.ReportDir)
	if err != nil {
		return "", err
	}
	defer target.Close()
	for _, file := range []struct {
		extension string
		body      []byte
	}{{".json", append(body, '\n')}, {".md", []byte(markdown)}} {
		name := stem + file.extension
		if err := writeExclusive(target, name, file.body); err != nil {
			return "", fmt.Errorf("write report %s (a partial report pair may remain): %w",
				filepath.Join(directory, name), err)
		}
	}
	return filepath.Join(directory, stem+".json"), nil
}

// PrepareDirectory creates and opens the confined report directory before a
// check runs. Write rechecks it again; this is not a long-lived policy lock.
func PrepareDirectory(root, name string) error {
	root, err := registry.Resolve(root)
	if err != nil {
		return err
	}
	directory, _, err := openDirectory(root, name)
	if err != nil {
		return err
	}
	return directory.Close()
}

func openDirectory(root, name string) (*os.Root, string, error) {
	directory, err := configuration.ReportDirectory(root, name)
	if err != nil {
		return nil, "", err
	}
	resolvedDirectory, err := registry.Resolve(directory)
	if err != nil {
		return nil, "", err
	}
	relative, err := filepath.Rel(root, resolvedDirectory)
	if err != nil {
		return nil, "", err
	}
	owner, err := os.OpenRoot(root)
	if err != nil {
		return nil, "", err
	}
	defer owner.Close()
	if err := owner.MkdirAll(relative, 0755); err != nil {
		return nil, "", fmt.Errorf("create report directory: %w", err)
	}
	if _, err := configuration.ReportDirectory(root, name); err != nil {
		return nil, "", err
	}
	target, err := owner.OpenRoot(relative)
	if err != nil {
		return nil, "", fmt.Errorf("open report directory: %w", err)
	}
	return target, directory, nil
}

func build(version string, sources map[string]string, selected []registry.Selection, results []Result) (report, error) {
	if len(selected) != len(results) {
		return report{}, fmt.Errorf("report results must match the complete selection")
	}
	data := report{
		Version: SchemaVersion, WsupVersion: version, Runtime: CurrentRuntime(),
		ConfigSources: sources, Selection: []selection{}, Results: []recordedResult{}, Summary: map[string]int{},
	}
	for index, item := range selected {
		result := results[index]
		switch result.Status {
		case "skipped":
			if strings.TrimSpace(result.Reason) == "" || result.Evidence != nil {
				return report{}, fmt.Errorf("skipped report result requires a reason and no execution evidence")
			}
		case "passed", "failed", "findings", "error", "timeout", "cancelled":
			if result.Evidence == nil || result.Reason != "" {
				return report{}, fmt.Errorf("executed report result requires evidence and no skip reason")
			}
			if math.IsNaN(result.DurationSeconds) || math.IsInf(result.DurationSeconds, 0) ||
				result.DurationSeconds < 0 || result.OutputBytes < 0 ||
				(result.Error != "" && (result.Status != "error" || !failureCode.MatchString(result.Error))) {
				return report{}, fmt.Errorf("report execution evidence contains invalid metrics or failure code")
			}
			if result.Status == "passed" && (result.ExitCode == nil || *result.ExitCode != 0) {
				return report{}, fmt.Errorf("passed report result requires exit code zero")
			}
			if (result.Status == "failed" || result.Status == "findings") &&
				(result.ExitCode == nil || *result.ExitCode == 0) {
				return report{}, fmt.Errorf("failed/finding report result requires a nonzero exit code")
			}
		default:
			return report{}, fmt.Errorf("unknown report result status")
		}
		digest, err := CommandDigest(item.Check.Command)
		if err != nil {
			return report{}, err
		}
		data.Selection = append(data.Selection, selection{
			Project: item.Project.ID, Check: item.Name, Path: item.Project.Path, CommandSHA256: digest,
		})
		data.Results = append(data.Results, recordedResult{
			Project: item.Project.ID, Check: item.Name, Mode: item.Check.Mode, Status: result.Status,
			Reason: result.Reason, Evidence: result.Evidence,
		})
		data.Summary[result.Status]++
	}
	return data, nil
}

func writeExclusive(root *os.Root, name string, body []byte) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	written, writeErr := file.Write(body)
	if writeErr == nil && written != len(body) {
		writeErr = io.ErrShortWrite
	}
	return errors.Join(writeErr, file.Close())
}

// Git state is optional evidence. A missing tool/non-repository/timeout is
// represented explicitly as unavailable, never as a clean worktree.
func readGit(root string) gitState {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	head := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	head.Dir = root
	head.WaitDelay = 200 * time.Millisecond
	body, err := head.Output()
	if err != nil {
		return gitState{Available: false}
	}
	state := gitState{Available: true, Head: strings.TrimSpace(string(body))}
	if !gitObjectID.MatchString(state.Head) {
		return gitState{Available: false}
	}
	ctxStatus, cancelStatus := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelStatus()
	status := exec.CommandContext(ctxStatus, "git", "-c", "core.fsmonitor=false", "status", "--porcelain")
	status.Dir = root
	status.WaitDelay = 200 * time.Millisecond
	body, err = status.Output()
	if err == nil {
		dirty := len(body) != 0
		state.Dirty = &dirty
	}
	return state
}
