#!/usr/bin/env sh
set -eu

version="${1:-${VERSION:-}}"
if [ -z "$version" ]; then
  printf '%s\n' 'Usage: build-release.sh <version>' >&2
  exit 2
fi

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
output="${PA2_SKILLS_RELEASE_OUTPUT:-$root/dist}"
github_repository="${PA2_SKILLS_GITHUB_REPOSITORY:-${GITHUB_REPOSITORY:-}}"
if [ -z "$github_repository" ]; then
  origin="$(git -C "$root" remote get-url origin 2>/dev/null || true)"
  github_repository="$(printf '%s' "$origin" | sed -n 's#^\(https://github.com/\|git@github.com:\)\([^/][^/]*/[^/][^/]*\)\(.git\)\{0,1\}$#\2#p' | sed 's/\.git$//')"
fi
if [ -z "$github_repository" ]; then
  printf '%s\n' 'Set PA2_SKILLS_GITHUB_REPOSITORY to owner/repository.' >&2
  exit 2
fi
release_base_url="https://github.com/$github_repository/releases"
case "$output" in
  "$root/dist"|/tmp/pa2-skills-release.*) ;;
  *)
    printf '%s\n' 'PA2_SKILLS_RELEASE_OUTPUT must be the repository dist directory or a /tmp/pa2-skills-release.* directory.' >&2
    exit 2
    ;;
esac
rm -rf "$output"
mkdir -p "$output"

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  goos="${target%/*}"
  goarch="${target#*/}"
  name="pa2-skills_${goos}_${goarch}"
  directory="$output/$name"
  mkdir -p "$directory"
  (
    cd "$root"
    GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
      -trimpath -buildvcs=false \
      -ldflags "-s -w -X main.version=$version -X pa2-skills/internal/pa2skills.defaultReleaseBaseURL=$release_base_url" \
      -o "$directory/pa2-skills" ./cmd/pa2-skills
  )
  tar -C "$directory" -czf "$output/$name.tar.gz" pa2-skills
done

(
  cd "$output"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum pa2-skills_*.tar.gz > SHA256SUMS
  else
    shasum -a 256 pa2-skills_*.tar.gz > SHA256SUMS
  fi
)
