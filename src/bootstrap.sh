#!/bin/sh
set -eu

repository_url="${PA2_SKILLS_REPOSITORY_URL:-https://github.com/pa2x2/agents.git}"
release_base_url="${PA2_SKILLS_RELEASE_BASE_URL:-https://github.com/pa2x2/agents/releases}"
release_version="${PA2_SKILLS_VERSION:-latest}"
data_home="${PA2_SKILLS_DATA_HOME:-${XDG_DATA_HOME:-$HOME/.local/share}}"
state_home="${PA2_SKILLS_STATE_HOME:-${XDG_STATE_HOME:-$HOME/.local/state}}"
bin_home="${XDG_BIN_HOME:-$HOME/.local/bin}"
source_root="$data_home/pa2-skills/agents"
binary="$bin_home/pa2-skills"

if ! command -v git >/dev/null 2>&1; then
  printf '%s\n' 'pa2-skills bootstrap requires git.' >&2
  exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
  printf '%s\n' 'pa2-skills bootstrap requires curl.' >&2
  exit 1
fi
if [ -e "$source_root/.git" ]; then
  git -C "$source_root" pull --ff-only
else
  mkdir -p "$(dirname "$source_root")"
  git clone "$repository_url" "$source_root"
fi

mkdir -p "$bin_home" "$state_home/pa2-skills"

platform() {
  case "$(uname -s)" in
  Linux) printf '%s' linux ;;
  Darwin) printf '%s' darwin ;;
  *)
    printf '%s\n' "Unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
  esac
}

architecture() {
  case "$(uname -m)" in
  x86_64 | amd64) printf '%s' amd64 ;;
  aarch64 | arm64) printf '%s' arm64 ;;
  *)
    printf '%s\n' "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
  esac
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    printf '%s\n' 'Install sha256sum or shasum to verify pa2-skills releases.' >&2
    return 1
  fi
}

os_name="$(platform)"
arch_name="$(architecture)"
asset_name="pa2-skills_${os_name}_${arch_name}.tar.gz"
temporary_dir="$(mktemp -d)"
trap 'rm -rf "$temporary_dir"' EXIT HUP INT TERM
asset_path="$temporary_dir/$asset_name"
checksums_path="$temporary_dir/SHA256SUMS"
asset_url="$release_base_url/$release_version/download/$asset_name"
checksums_url="$release_base_url/$release_version/download/SHA256SUMS"

if curl -fsSL "$asset_url" -o "$asset_path" && curl -fsSL "$checksums_url" -o "$checksums_path"; then
  expected="$(awk -v asset="$asset_name" '$2 == asset { print $1 }' "$checksums_path")"
  actual="$(sha256_file "$asset_path")"
  if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
    printf '%s\n' 'pa2-skills release checksum verification failed.' >&2
    exit 1
  fi
  tar -xzf "$asset_path" -C "$temporary_dir"
  install -m 0755 "$temporary_dir/pa2-skills" "$binary"
  printf 'Installed verified %s release at %s\n' "$release_version" "$binary"
elif command -v go >/dev/null 2>&1; then
  printf '%s\n' 'No matching release asset is available; building pa2-skills from the source checkout.' >&2
  (cd "$source_root/src" && go build -trimpath -buildvcs=false -o "$binary" ./cmd/pa2-skills)
  printf 'Installed pa2-skills from source at %s\n' "$binary"
else
  printf '%s\n' 'No matching release asset is available and Go is not installed.' >&2
  exit 1
fi

printf 'Add %s to PATH if necessary, then run: pa2-skills list\n' "$bin_home"
printf 'Enable zsh completion with: eval "$(pa2-skills completion zsh)"\n'
