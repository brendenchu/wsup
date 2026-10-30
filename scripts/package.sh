#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local

if [[ $# -ne 0 ]]; then
  printf 'Usage: bash scripts/package.sh\n' >&2
  exit 2
fi
if [[ "$(uname -s)" != Darwin || "$(uname -m)" != arm64 ]]; then
  printf 'Packaging requires the runtime-certified macOS ARM64 host.\n' >&2
  exit 2
fi
required_go="go$(awk '$1 == "go" { print $2 }' go.mod)"
if [[ "$(go env GOVERSION)" != "$required_go" ]]; then
  printf 'Packaging requires %s from go.mod.\n' "$required_go" >&2
  exit 2
fi

revision=$(git rev-parse HEAD)
status=$(git status --porcelain --untracked-files=all)
output=$(mktemp -d "${TMPDIR:-/tmp}/wsup-package.XXXXXX")
printf 'Local candidate staging: %s\n' "$output"
trap 'printf "Packaging failed; inspect retained staging: %s\n" "$output" >&2' ERR

go test -race ./...
go vet ./...
version=$(go run ./cmd/wsup --version)
name="wsup_${version}-local_darwin_arm64"
package="$output/$name"
mkdir "$package" "$package/licenses"
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
  go build -trimpath -buildvcs=false -o "$package/wsup" ./cmd/wsup
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
  go build -trimpath -buildvcs=false -o "$output/wsup-repeat" ./cmd/wsup
cmp "$package/wsup" "$output/wsup-repeat"
rm "$output/wsup-repeat"
cp LICENSE "$package/LICENSE"
cp docs/native-install.md "$package/README.md"
goroot=$(go env GOROOT)
cp "$goroot/LICENSE" "$package/licenses/go-LICENSE"
cp "$goroot/PATENTS" "$package/licenses/go-PATENTS"
for component in crypto net sys text; do
  cp "$goroot/src/vendor/golang.org/x/$component/LICENSE" \
    "$package/licenses/go-x-$component-LICENSE"
  cp "$goroot/src/vendor/golang.org/x/$component/PATENTS" \
    "$package/licenses/go-x-$component-PATENTS"
done
cobra=$(go list -m -f '{{.Dir}}' github.com/spf13/cobra)
pflag=$(go list -m -f '{{.Dir}}' github.com/spf13/pflag)
mousetrap=$(go list -m -f '{{.Dir}}' github.com/inconshreveable/mousetrap)
cp "$cobra/LICENSE.txt" "$package/licenses/cobra-LICENSE"
cp "$pflag/LICENSE" "$package/licenses/pflag-LICENSE"
cp "$mousetrap/LICENSE" "$package/licenses/mousetrap-LICENSE"
{
  printf 'Local candidate, not a published release\nSource: %s\n' "$revision"
  printf '\nWorking-tree status (empty means clean):\n%s\n' "$status"
  printf '\nToolchain:\n'
  go version
  printf '\nBinary build metadata:\n'
  go version -m "$package/wsup"
} > "$package/BUILD.txt"
chmod 755 "$package/wsup"
tar -czf "$output/$name.tar.gz" -C "$output" "$name"
(cd "$output" && shasum -a 256 "$name.tar.gz" > SHA256SUMS)
(cd "$output" && shasum -a 256 -c SHA256SUMS)

verification=$(mktemp -d "$output/verification.XXXXXX")
tar -xzf "$output/$name.tar.gz" -C "$verification"
diff -r "$package" "$verification/$name"
binary="$verification/$name/wsup"
test -x "$binary"
test "$(env PATH= "$binary" --version)" = "$version"
env PATH= "$binary" --help > "$verification/help.txt"
env PATH= "$binary" init "$verification/workspace"
env PATH= "$binary" --root "$verification/workspace" validate
env PATH= "$binary" --root "$verification/workspace" guidance > "$verification/guidance.json"
env PATH= "$binary" --root "$verification/workspace" add journal --apply
env PATH= "$binary" --root "$verification/workspace" update --apply
mv "$verification/workspace" "$verification/relocated"
env PATH= "$binary" --root "$verification/relocated" validate
env PATH= "$binary" --root "$verification/relocated" init "$verification/child"
env PATH= "$binary" --root "$verification/child" guidance > "$verification/child-guidance.json"
env PATH= "$binary" init "$verification/project" --template project
env PATH= "$binary" --root "$verification/project" guidance > "$verification/project-guidance.json"

if [[ "$(git rev-parse HEAD)" != "$revision" ||
      "$(git status --porcelain --untracked-files=all)" != "$status" ]]; then
  printf 'Source revision or working-tree status changed during packaging; discard this candidate.\n' >&2
  exit 1
fi
rm -r "$verification"
printf '\nVerified local candidate: %s/%s.tar.gz\n' "$output" "$name"
printf 'Checksum manifest: %s/SHA256SUMS\n' "$output"
printf 'No tag, publication, installation, or PATH changes performed.\n'
