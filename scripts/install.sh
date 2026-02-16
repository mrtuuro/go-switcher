#!/usr/bin/env sh
set -eu

REPO="${SWITCHER_REPO:-mrtuuro/go-switcher}"
INSTALL_DIR="${SWITCHER_INSTALL_DIR:-$HOME/.switcher/bin}"
VERSION="${SWITCHER_VERSION:-}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf "missing required command: %s\n" "$1" >&2
    exit 1
  fi
}

detect_os() {
  case "$(uname -s)" in
    Darwin) printf "darwin" ;;
    Linux) printf "linux" ;;
    *)
      printf "unsupported operating system: %s\n" "$(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) printf "amd64" ;;
    arm64|aarch64) printf "arm64" ;;
    *)
      printf "unsupported architecture: %s\n" "$(uname -m)" >&2
      exit 1
      ;;
  esac
}

fetch() {
  curl -fsSL "$1"
}

download_to() {
  url="$1"
  destination="$2"
  printf "Downloading %s\n" "$url"
  curl -fL --progress-bar "$url" -o "$destination"
}

resolve_latest_version() {
  metadata="$(fetch "https://api.github.com/repos/$REPO/releases/latest")"
  resolved="$(printf "%s" "$metadata" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"

  if [ -z "$resolved" ]; then
    printf "failed to resolve latest release from GitHub API\n" >&2
    printf "tip: set SWITCHER_VERSION=vX.Y.Z and retry\n" >&2
    exit 1
  fi

  printf "%s" "$resolved"
}

verify_checksum() {
  archive_path="$1"
  archive_name="$2"
  checksum_file="$3"

  expected="$(awk -v target="$archive_name" '$2 == target { print $1 }' "$checksum_file" | head -n 1)"
  if [ -z "$expected" ]; then
    printf "checksum not found for %s\n" "$archive_name" >&2
    exit 1
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive_path" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
  else
    printf "warning: no sha256 tool found, skipping checksum verification\n" >&2
    return
  fi

  if [ "$actual" != "$expected" ]; then
    printf "checksum mismatch for %s\n" "$archive_name" >&2
    printf "expected: %s\n" "$expected" >&2
    printf "actual:   %s\n" "$actual" >&2
    exit 1
  fi
}

install_binary() {
  extracted_dir="$1"
  mkdir -p "$INSTALL_DIR"

  source_bin="$extracted_dir/switcher"
  if [ ! -f "$source_bin" ]; then
    printf "switcher binary not found in archive\n" >&2
    exit 1
  fi

  cp "$source_bin" "$INSTALL_DIR/switcher"
  chmod +x "$INSTALL_DIR/switcher"
}

need_cmd curl
need_cmd tar
need_cmd awk
need_cmd sed

if [ -z "$VERSION" ]; then
  VERSION="$(resolve_latest_version)"
fi

OS="$(detect_os)"
ARCH="$(detect_arch)"
VERSION_STRIPPED="${VERSION#v}"
ARCHIVE="go-switcher_${VERSION_STRIPPED}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$VERSION"

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT INT TERM

archive_path="$tmp_dir/$ARCHIVE"
checksums_path="$tmp_dir/checksums.txt"

download_to "$BASE_URL/$ARCHIVE" "$archive_path"
download_to "$BASE_URL/checksums.txt" "$checksums_path"

printf "Verifying checksum...\n"
verify_checksum "$archive_path" "$ARCHIVE" "$checksums_path"

printf "Extracting archive...\n"
tar -xzf "$archive_path" -C "$tmp_dir"

printf "Installing switcher to %s...\n" "$INSTALL_DIR/switcher"
install_binary "$tmp_dir"

printf "\nInstalled switcher %s\n" "$VERSION"
printf "Add this to your shell profile if needed:\n"
printf "  export PATH=\"\$HOME/.switcher/bin:\$PATH\"\n"
printf "Then restart your shell and run: switcher help\n"
