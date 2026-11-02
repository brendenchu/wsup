// Package configuration resolves workspace-local settings and records their sources.
package configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wavykit/wsup/internal/registry"
)

// Config contains validated workspace defaults; individual checks may override timeout.
type Config struct {
	TimeoutSeconds   int    `json:"timeout_seconds"`
	OutputLimitBytes int    `json:"output_limit_bytes"`
	ReportDir        string `json:"report_dir"`
}

// Result exposes effective settings and the winning source for each setting.
type Result struct {
	Settings Config            `json:"settings"`
	Sources  map[string]string `json:"sources"`
}

// ReportsPath is the default report directory and the root every configured
// report_dir must stay inside. workbench/.local/ is ignored in every workspace.
const ReportsPath = "workbench/.local/wsup/reports"

// Load applies defaults, wsup.json settings, wsup.local.json, environment, then optional CLI
// timeout, without searching parent directories or creating report directories.
// lookup supplies environment access explicitly; nil timeout means no CLI override.
func Load(root string, timeout *int, lookup func(string) (string, bool)) (Result, error) {
	values := map[string]any{
		"timeout_seconds": json.Number("300"), "output_limit_bytes": json.Number("65536"),
		"report_dir": ReportsPath,
	}
	sources := map[string]string{}
	for key := range values {
		sources[key] = "default"
	}
	merge := func(raw any, label string) error {
		settings, ok := raw.(map[string]any)
		if !ok || !registry.Fields(settings, "timeout_seconds", "output_limit_bytes", "report_dir") {
			return fmt.Errorf("%s: unsupported configuration settings", label)
		}
		for key, value := range settings {
			values[key], sources[key] = value, label
		}
		return nil
	}
	for _, filename := range []string{registry.DocumentPath, "wsup.local.json"} {
		path := filepath.Join(root, filename)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return Result{}, err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return Result{}, err
		}
		var settings map[string]any
		if filename == registry.DocumentPath {
			document, err := registry.Decode(body)
			if err != nil {
				return Result{}, err
			}
			settings = document.Settings
			if settings == nil {
				continue
			}
		} else if settings, err = ParseSettings(filename, body); err != nil {
			return Result{}, err
		}
		if err := merge(settings, filename); err != nil {
			return Result{}, err
		}
	}
	for _, pair := range [][2]string{
		{"WSUP_TIMEOUT_SECONDS", "timeout_seconds"},
		{"WSUP_OUTPUT_LIMIT_BYTES", "output_limit_bytes"},
		{"WSUP_REPORT_DIR", "report_dir"},
	} {
		if text, exists := lookup(pair[0]); exists {
			var value any = text
			if pair[1] != "report_dir" {
				integer, err := strconv.Atoi(strings.TrimSpace(text))
				if err != nil {
					return Result{}, fmt.Errorf("%s must be an integer", pair[0])
				}
				value = json.Number(strconv.Itoa(integer))
			}
			if err := merge(map[string]any{pair[1]: value}, pair[0]); err != nil {
				return Result{}, err
			}
		}
	}
	if timeout != nil {
		if err := merge(map[string]any{"timeout_seconds": json.Number(strconv.Itoa(*timeout))}, "CLI"); err != nil {
			return Result{}, err
		}
	}
	numbers := map[string]int{}
	for _, limit := range []struct {
		key string
		max int
	}{{"timeout_seconds", 86400}, {"output_limit_bytes", 10485760}} {
		value, ok := registry.Integer(values[limit.key])
		if !ok || value < 1 || value > limit.max {
			return Result{}, fmt.Errorf("%s must be 1..%d", limit.key, limit.max)
		}
		numbers[limit.key] = value
	}
	reportDir, ok := values["report_dir"].(string)
	if !ok {
		return Result{}, fmt.Errorf("Use a nonempty relative forward-slash path")
	}
	if _, err := ReportDirectory(root, reportDir); err != nil {
		return Result{}, err
	}
	return Result{
		Settings: Config{TimeoutSeconds: numbers["timeout_seconds"],
			OutputLimitBytes: numbers["output_limit_bytes"], ReportDir: reportDir},
		Sources: sources,
	}, nil
}

// ParseSettings validates wsup.local.json's structure without merging away an
// explicitly recorded report policy. Load validates effective values separately.
func ParseSettings(filename string, body []byte) (map[string]any, error) {
	document, err := registry.ParseObject(filename, body)
	if err != nil {
		return nil, err
	}
	version, valid := registry.Integer(document["version"])
	if !valid || version != 1 || !registry.Fields(document, "version", "settings") {
		return nil, fmt.Errorf("%s: configuration requires version 1", filename)
	}
	raw, exists := document["settings"]
	if !exists {
		return map[string]any{}, nil
	}
	settings, ok := raw.(map[string]any)
	if !ok || !registry.Fields(settings, "timeout_seconds", "output_limit_bytes", "report_dir") {
		return nil, fmt.Errorf("%s: unsupported configuration settings", filename)
	}
	return settings, nil
}

// ReportDirectory rechecks containment without creating directories. Writers
// call this again at write time; earlier configuration validation is not a lock.
func ReportDirectory(root, name string) (string, error) {
	report, err := registry.RelativeTarget(root, name)
	if err != nil {
		return "", err
	}
	resolvedReport, err := registry.Resolve(report)
	if err != nil {
		return "", err
	}
	reports := filepath.Join(root, filepath.FromSlash(ReportsPath))
	resolvedReports, err := registry.Resolve(reports)
	if err != nil {
		return "", err
	}
	// Resolved containment alone would accept a redirected reports root, so no
	// component of it may be a symlink.
	redirected := false
	for name := ReportsPath; name != "."; name = path.Dir(name) {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name)))
		redirected = redirected || (err == nil && info.Mode()&os.ModeSymlink != 0)
	}
	if !registry.Within(resolvedReports, resolvedReport) || redirected {
		return "", fmt.Errorf("report_dir must be inside the local %s directory", ReportsPath)
	}
	return report, nil
}
