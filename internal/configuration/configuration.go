// Package configuration resolves workspace-local settings and records their sources.
package configuration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/brendenchu/wsup/internal/registry"
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

// Load applies defaults, shared file, local file, environment, then optional CLI
// timeout, without searching parent directories or creating report directories.
// lookup supplies environment access explicitly; nil timeout means no CLI override.
func Load(root string, timeout *int, lookup func(string) (string, bool)) (Result, error) {
	values := map[string]any{
		"timeout_seconds": json.Number("300"), "output_limit_bytes": json.Number("65536"),
		"report_dir": ".wsup/reports",
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
	for _, filename := range []string{"wsup.json", "wsup.local.json"} {
		path := filepath.Join(root, filename)
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return Result{}, err
		}
		document, err := registry.ReadObject(path)
		if err != nil {
			return Result{}, err
		}
		version, valid := registry.Integer(document["version"])
		if !valid || version != 1 || !registry.Fields(document, "version", "settings") {
			return Result{}, fmt.Errorf("%s: configuration requires version 1", filename)
		}
		settings, exists := document["settings"]
		if !exists {
			settings = map[string]any{}
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
	report, err := registry.RelativeTarget(root, values["report_dir"])
	if err != nil {
		return Result{}, err
	}
	resolvedReport, err := registry.Resolve(report)
	if err != nil {
		return Result{}, err
	}
	reports := filepath.Join(root, ".wsup", "reports")
	resolvedReports, err := registry.Resolve(reports)
	if err != nil {
		return Result{}, err
	}
	isSymlink := func(name string) bool {
		info, err := os.Lstat(name)
		return err == nil && info.Mode()&os.ModeSymlink != 0
	}
	// Resolved containment alone would accept a redirected reports root.
	if !registry.Within(resolvedReports, resolvedReport) ||
		isSymlink(filepath.Join(root, ".wsup")) || isSymlink(reports) {
		return Result{}, fmt.Errorf("report_dir must be inside the local .wsup/reports directory")
	}
	return Result{
		Settings: Config{TimeoutSeconds: numbers["timeout_seconds"],
			OutputLimitBytes: numbers["output_limit_bytes"], ReportDir: values["report_dir"].(string)},
		Sources: sources,
	}, nil
}
