// Command release prepares notices and verifies GoReleaser's native artifacts.
// GoReleaser owns builds, archives, checksums and Homebrew cask generation.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const stagingDirectory = ".release-staging"

type inputs struct {
	Version        string `json:"version"`
	ProductVersion string `json:"product_version"`
	Snapshot       bool   `json:"snapshot"`
	Revision       string `json:"revision"`
	Status         string `json:"status"`
	Fingerprint    string `json:"fingerprint"`
}

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func dispatch(args []string) error {
	if len(args) == 4 && args[0] == "prepare" {
		return prepare(args[1], args[2], args[3] == "true")
	}
	if len(args) == 3 && args[0] == "binary" {
		return verifyBinary(args[1], args[2])
	}
	if len(args) == 2 && args[0] == "verify" {
		return verifyRelease(args[1])
	}
	return fmt.Errorf("usage: go run ./scripts/release prepare DIST VERSION SNAPSHOT | binary PATH DIST | verify DIST")
}

func command(env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	body, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w\n%s", name, err, body)
	}
	return body, nil
}
func output(name string, args ...string) (string, error) {
	b, err := command(nil, name, args...)
	return strings.TrimSpace(string(b)), err
}
func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0644)
}
func fingerprint() (string, error) {
	paths, err := command(nil, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return "", err
	}
	names := strings.Split(strings.TrimSuffix(string(paths), "\x00"), "\x00")
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		info, err := os.Lstat(name)
		if os.IsNotExist(err) {
			fmt.Fprintf(h, "%s\x00deleted\x00", name)
			continue
		}
		if err != nil {
			return "", err
		}
		var body []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, e := os.Readlink(name)
			err = e
			body = []byte(target)
		} else if info.Mode().IsRegular() {
			body, err = os.ReadFile(name)
		} else {
			return "", fmt.Errorf("nonregular source input: %s", name)
		}
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%s\x00", name, info.Mode().String())
		h.Write(body)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func saveInputs(dist string, in inputs) error {
	body, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dist, "inputs.json"), append(body, '\n'), 0600)
}
func loadInputs(dist string) (inputs, error) {
	var in inputs
	b, err := os.ReadFile(filepath.Join(dist, "inputs.json"))
	if err == nil {
		err = json.Unmarshal(b, &in)
	}
	return in, err
}
func unchanged(in inputs) error {
	revision, err := output("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	fp, err := fingerprint()
	if err != nil {
		return err
	}
	if revision != in.Revision || fp != in.Fingerprint {
		return fmt.Errorf("source changed during packaging; discard artifacts and build again")
	}
	return nil
}
func prepare(dist, version string, snapshot bool) error {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return fmt.Errorf("only the verified macOS ARM64 runtime can prepare release artifacts")
	}
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		return err
	}
	pinned := ""
	for _, line := range strings.Split(string(mod), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			pinned = "go" + fields[1]
		}
	}
	if pinned != runtime.Version() {
		return fmt.Errorf("requires pinned %s; running %s", pinned, runtime.Version())
	}
	in := inputs{Version: version, Snapshot: snapshot}
	if in.Revision, err = output("git", "rev-parse", "HEAD"); err != nil {
		return err
	}
	if in.Status, err = output("git", "status", "--porcelain", "--untracked-files=all"); err != nil {
		return err
	}
	if !snapshot && in.Status != "" {
		return fmt.Errorf("release artifacts require a clean tagged checkout; use --snapshot for local changes")
	}
	product, err := command(nil, "go", "run", "./cmd/wsup", "--version")
	if err != nil {
		return err
	}
	in.ProductVersion = strings.TrimSpace(string(product))
	if !snapshot && in.ProductVersion != version {
		return fmt.Errorf("product version %s differs from artifact version %s", in.ProductVersion, version)
	}
	if !snapshot {
		tag, err := output("git", "describe", "--exact-match", "--tags", "HEAD")
		if err != nil {
			return err
		}
		if tag != "v"+version {
			return fmt.Errorf("tag %s does not match version %s", tag, version)
		}
	}
	if in.Fingerprint, err = fingerprint(); err != nil {
		return err
	}
	extra := filepath.Join(dist, "extra")
	if err = copyFile("LICENSE", filepath.Join(extra, "LICENSE")); err != nil {
		return err
	}
	if err = copyFile("docs/native-install.md", filepath.Join(extra, "README.md")); err != nil {
		return err
	}
	goroot, err := output("go", "env", "GOROOT")
	if err != nil {
		return err
	}
	for _, kind := range []string{"LICENSE", "PATENTS"} {
		if err = copyFile(filepath.Join(goroot, kind), filepath.Join(extra, "licenses", "go-"+kind)); err != nil {
			return err
		}
	}
	for _, component := range []string{"crypto", "net", "sys", "text"} {
		for _, kind := range []string{"LICENSE", "PATENTS"} {
			if err = copyFile(filepath.Join(goroot, "src/vendor/golang.org/x", component, kind), filepath.Join(extra, "licenses", "go-x-"+component+"-"+kind)); err != nil {
				return err
			}
		}
	}
	for _, dependency := range []struct{ module, name, file string }{{"github.com/spf13/cobra", "cobra", "LICENSE.txt"}, {"github.com/spf13/pflag", "pflag", "LICENSE"}, {"github.com/inconshreveable/mousetrap", "mousetrap", "LICENSE"}} {
		dir, err := output("go", "list", "-m", "-f", "{{.Dir}}", dependency.module)
		if err != nil {
			return err
		}
		if err = copyFile(filepath.Join(dir, dependency.file), filepath.Join(extra, "licenses", dependency.name+"-LICENSE")); err != nil {
			return err
		}
	}
	return saveInputs(dist, in)
}
func verifyBinary(binary, dist string) error {
	in, err := loadInputs(stagingDirectory)
	if err != nil {
		return err
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		return err
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
		if strings.HasPrefix(s.Key, "vcs") {
			return fmt.Errorf("binary contains implicit VCS metadata")
		}
	}
	if settings["CGO_ENABLED"] != "0" || settings["-trimpath"] != "true" {
		return fmt.Errorf("binary lacks the required cgo-disabled trimpath profile")
	}
	temp, err := os.MkdirTemp("", "wsup-release-repeat-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	repeat := filepath.Join(temp, "wsup")
	if _, err = command([]string{"CGO_ENABLED=0", "GOOS=darwin", "GOARCH=arm64"}, "go", "build", "-trimpath", "-buildvcs=false", "-ldflags=", "-o", repeat, "./cmd/wsup"); err != nil {
		return err
	}
	a, err := os.ReadFile(binary)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(repeat)
	if err != nil {
		return err
	}
	if !bytes.Equal(a, b) {
		return fmt.Errorf("release-profile repeated binary bytes differ")
	}
	if err = smoke(binary, in.ProductVersion); err != nil {
		return err
	}
	if err = unchanged(in); err != nil {
		return err
	}
	metadata, err := output("go", "version", "-m", binary)
	if err != nil {
		return err
	}
	_, metadata, _ = strings.Cut(metadata, "\n")
	label := "Tagged release"
	if in.Snapshot {
		label = "Local snapshot, not a published release"
	}
	body := fmt.Sprintf("%s\nArtifact version: %s\nProduct version: %s\nSource: %s\nWorking-tree status:\n%s\nSource fingerprint: %s\nToolchain: %s\n\nBinary metadata:\n%s\n", label, in.Version, in.ProductVersion, in.Revision, in.Status, in.Fingerprint, runtime.Version(), metadata)
	if err = os.WriteFile(filepath.Join(stagingDirectory, "extra", "BUILD.txt"), []byte(body), 0644); err != nil {
		return err
	}
	return saveInputs(dist, in)
}
func smoke(binary, version string) error {
	binary, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	temp, err := os.MkdirTemp("", "wsup-release-smoke-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	run := func(args ...string) error {
		cmd := exec.Command(binary, args...)
		cmd.Dir = temp
		cmd.Env = []string{"PATH="}
		body, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("standalone %v: %w\n%s", args, err, body)
		}
		if len(args) == 1 && args[0] == "--version" && strings.TrimSpace(string(body)) != version {
			return fmt.Errorf("unexpected product version: %s", body)
		}
		return nil
	}
	ws := filepath.Join(temp, "workspace")
	project := filepath.Join(temp, "project")
	steps := [][]string{{"--version"}, {"--help"}, {"init", ws, "--yes"}, {"--root", ws, "validate"}, {"--root", ws, "guidance"}, {"--root", ws, "add", "journal", "research", "--apply"}, {"--root", ws, "update", "--apply"}, {"init", project, "--template", "project", "--yes"}, {"--root", project, "guidance"}}
	for _, args := range steps {
		if err = run(args...); err != nil {
			return err
		}
	}
	moved := filepath.Join(temp, "relocated")
	if err = os.Rename(ws, moved); err != nil {
		return err
	}
	if err = run("--root", moved, "validate"); err != nil {
		return err
	}
	child := filepath.Join(temp, "child")
	if err = run("--root", moved, "init", child, "--yes"); err != nil {
		return err
	}
	return run("--root", child, "guidance")
}
func inventory() map[string]bool {
	files := map[string]bool{"wsup": true, "README.md": true, "LICENSE": true, "BUILD.txt": true, "licenses/go-LICENSE": true, "licenses/go-PATENTS": true, "licenses/cobra-LICENSE": true, "licenses/pflag-LICENSE": true, "licenses/mousetrap-LICENSE": true}
	for _, x := range []string{"crypto", "net", "sys", "text"} {
		for _, kind := range []string{"LICENSE", "PATENTS"} {
			files["licenses/go-x-"+x+"-"+kind] = true
		}
	}
	return files
}
func extractArchive(archive, dest, prefix string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	expected := inventory()
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeDir {
			if h.Name != prefix && h.Name != prefix+"/" && h.Name != prefix+"/licenses/" && h.Name != prefix+"/licenses" {
				return fmt.Errorf("unexpected archive directory: %s", h.Name)
			}
			continue
		}
		name, ok := strings.CutPrefix(h.Name, prefix+"/")
		if !ok || !expected[name] || seen[name] || h.Typeflag != tar.TypeReg {
			return fmt.Errorf("unexpected/duplicate archive entry: %s", h.Name)
		}
		if name == "wsup" && h.Mode&0111 == 0 {
			return fmt.Errorf("archived binary is not executable")
		}
		seen[name] = true
		target := filepath.Join(dest, name)
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(h.Mode)&0777)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("archive has %d files, expected %d", len(seen), len(expected))
	}
	return nil
}
func verifyRelease(dist string) error {
	in, err := loadInputs(dist)
	if err != nil {
		return err
	}
	prefix := "wsup_" + in.Version + "_darwin_arm64"
	archive := filepath.Join(dist, prefix+".tar.gz")
	body, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(body))
	manifest, err := os.ReadFile(filepath.Join(dist, "SHA256SUMS"))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(manifest)) != sum+"  "+filepath.Base(archive) {
		return fmt.Errorf("checksum manifest does not match the single supported archive")
	}
	temp, err := os.MkdirTemp("", "wsup-release-extracted-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err = extractArchive(archive, temp, prefix); err != nil {
		return err
	}
	for name := range inventory() {
		if name == "wsup" {
			continue
		}
		a, err := os.ReadFile(filepath.Join(temp, name))
		if err != nil {
			return err
		}
		b, err := os.ReadFile(filepath.Join(stagingDirectory, "extra", name))
		if err != nil {
			return err
		}
		if !bytes.Equal(a, b) {
			return fmt.Errorf("archived %s differs from prepared input", name)
		}
	}
	if err = smoke(filepath.Join(temp, "wsup"), in.ProductVersion); err != nil {
		return err
	}
	if err = unchanged(in); err != nil {
		return err
	}
	fmt.Printf("Verified %s: checksum, exact 17-file inventory and extracted-binary smoke checks passed.\n", archive)
	return nil
}
