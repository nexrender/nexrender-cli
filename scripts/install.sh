#!/bin/sh
set -eu

repository="nexrender/nexrender-cli"
version="${NEXRENDER_VERSION:-latest}"
binary_dir="${NEXRENDER_BIN_DIR:-${HOME}/.local/bin}"

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *)
    echo "Unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

archive="nexrender_${os}_${arch}.tar.gz"
if [ "$version" = "latest" ]; then
  release_url="https://github.com/${repository}/releases/latest/download"
else
  case "$version" in
    v*) tag="$version" ;;
    *) tag="v${version}" ;;
  esac
  release_url="https://github.com/${repository}/releases/download/${tag}"
fi

temporary_dir="$(mktemp -d)"
trap 'rm -rf "$temporary_dir"' EXIT HUP INT TERM

download() {
  curl --fail --silent --show-error --location "$1" --output "$2"
}

echo "Downloading ${archive} from https://github.com/${repository}"
download "${release_url}/${archive}" "${temporary_dir}/${archive}"
download "${release_url}/checksums.txt" "${temporary_dir}/checksums.txt"

checksum_line="$(awk -v file="$archive" '$2 == file || $2 == "*" file { print; exit }' "${temporary_dir}/checksums.txt")"
if [ -z "$checksum_line" ]; then
  echo "No checksum was published for ${archive}" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$temporary_dir" && printf '%s\n' "$checksum_line" | sha256sum --check --status)
elif command -v shasum >/dev/null 2>&1; then
  expected="$(printf '%s\n' "$checksum_line" | awk '{print $1}')"
  actual="$(shasum -a 256 "${temporary_dir}/${archive}" | awk '{print $1}')"
  [ "$expected" = "$actual" ] || { echo "Checksum verification failed" >&2; exit 1; }
else
  echo "Neither sha256sum nor shasum is available" >&2
  exit 1
fi

if command -v cosign >/dev/null 2>&1; then
  bundle="${temporary_dir}/checksums.txt.sigstore.json"
  if download "${release_url}/checksums.txt.sigstore.json" "$bundle" 2>/dev/null; then
    cosign verify-blob \
      --bundle "$bundle" \
      --certificate-identity-regexp '^https://github.com/nexrender/nexrender-cli/.github/workflows/release.yml@refs/tags/v.*$' \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com \
      "${temporary_dir}/checksums.txt" >/dev/null
  fi
fi

tar -xzf "${temporary_dir}/${archive}" -C "$temporary_dir"
mkdir -p "$binary_dir"
install -m 0755 "${temporary_dir}/nexrender" "${binary_dir}/nexrender"

echo "Installed nexrender to ${binary_dir}/nexrender"
case ":${PATH}:" in
  *":${binary_dir}:"*) ;;
  *) echo "Add ${binary_dir} to PATH before running nexrender." ;;
esac

if [ "${NEXRENDER_SKIP_SETUP:-0}" != "1" ] && [ -t 0 ] && [ -t 1 ]; then
  "${binary_dir}/nexrender" setup
fi
