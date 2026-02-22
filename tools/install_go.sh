#!/usr/bin/env sh
set -eu

if [ ! -f go.mod ]; then
  echo "go.mod not found in current directory" >&2
  exit 1
fi

if grep -q '^toolchain ' go.mod; then
  desired_go_version="$(awk '/^toolchain / { sub(/^go/, "", $2); print $2; exit }' go.mod)"
else
  desired_go_version="$(awk '/^go / { print $2; exit }' go.mod)"
fi

# Build toolchain tarball names use full semver.
case "$desired_go_version" in
  *.*.*) ;;
  *.*) desired_go_version="${desired_go_version}.0" ;;
  *)
    echo "Could not parse Go version from go.mod: '$desired_go_version'" >&2
    exit 1
    ;;
esac

installed_go_version=""
if [ -x /usr/local/go/bin/go ]; then
  installed_go_version="$(/usr/local/go/bin/go version | awk '{ sub(/^go/, "", $3); print $3 }')"
fi

if [ "$installed_go_version" != "$desired_go_version" ]; then
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) go_arch="amd64" ;;
    aarch64|arm64) go_arch="arm64" ;;
    *)
      echo "Unsupported architecture: $arch" >&2
      exit 1
      ;;
  esac

  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  if [ "$os" != "linux" ]; then
    echo "Unsupported OS: $os" >&2
    exit 1
  fi

  echo "Installing Go $desired_go_version for $os/$go_arch" >&2
  rm -rf /usr/local/go
  curl -fsSL "https://go.dev/dl/go${desired_go_version}.${os}-${go_arch}.tar.gz" | tar -C /usr/local -xz
else
  echo "Go $installed_go_version already installed at /usr/local/go" >&2
fi

printf '%s\n' 'export PATH="/usr/local/go/bin:$PATH"'
