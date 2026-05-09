#!/usr/bin/env bash
# Cross-compile the freetodolist CLI for macOS, Linux, and Windows and
# package each binary as a tar.gz (zip on Windows) ready to upload to a
# GitHub release.
#
# Output: dist/freetodolist-<os>-<arch>.{tar.gz,zip}
#         dist/SHA256SUMS
#
# Archive filenames intentionally omit the version so GitHub's
# `releases/latest/download/<file>` URL pattern resolves to the newest
# release. The version is preserved inside the archive (the staging
# directory is named with it) so an extracted tree is self-describing.
#
# Usage: VERSION=v0.1.0 ./build.sh   (or VERSION will default to "dev")
set -euo pipefail

VERSION="${VERSION:-dev}"
HERE="$(cd "$(dirname "$0")" && pwd)"
DIST="$HERE/dist"

rm -rf "$DIST"
mkdir -p "$DIST"

# (GOOS, GOARCH) pairs. macOS Apple Silicon first since that's most users.
TARGETS=(
  "darwin arm64"
  "darwin amd64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
)

cd "$HERE"

for t in "${TARGETS[@]}"; do
  GOOS="${t% *}"
  GOARCH="${t#* }"
  out="freetodolist"
  [[ "$GOOS" == "windows" ]] && out="freetodolist.exe"

  stage="$DIST/freetodolist-$VERSION-$GOOS-$GOARCH"
  mkdir -p "$stage"

  echo "  building ${GOOS}/${GOARCH}..."
  GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w" -o "$stage/$out" .

  if [[ "$GOOS" == "windows" ]]; then
    (cd "$DIST" && zip -qr "freetodolist-$GOOS-$GOARCH.zip" "$(basename "$stage")")
  else
    (cd "$DIST" && tar -czf "freetodolist-$GOOS-$GOARCH.tar.gz" "$(basename "$stage")")
  fi
  rm -rf "$stage"
done

echo
echo "Artifacts:"
ls -lh "$DIST"

# Generate a checksums file so users (and the release notes) can verify.
cd "$DIST"
shasum -a 256 *.tar.gz *.zip 2>/dev/null > SHA256SUMS
echo
echo "SHA256SUMS:"
cat SHA256SUMS
