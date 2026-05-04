#!/bin/sh
# install.sh — peek installer for macOS and Linux.
# Usage: curl -fsSL https://raw.githubusercontent.com/Pankaj3112/peek/main/install.sh | sh
#
# Installs the peek binary to ~/.local/bin/peek.

set -eu

GITHUB_REPO="Pankaj3112/peek"
INSTALL_DIR="${HOME}/.local/bin"
BINARY_NAME="peek"

# --- helpers ---

info() { printf '\033[1;34m▸\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!\033[0m %s\n' "$*"; }
error() { printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; }
success() { printf '\033[1;32m✓\033[0m %s\n' "$*"; }

# --- platform detection ---

detect_os() {
    case "$(uname -s)" in
        Darwin)        echo "macos" ;;
        Linux)         echo "linux" ;;
        CYGWIN*|MINGW*|MSYS*)
            error "Windows is not supported by this script."
            error "Install peek via Scoop instead:"
            error "  scoop bucket add peek https://github.com/Pankaj3112/scoop-bucket"
            error "  scoop install peek"
            error "Or download a Windows binary from:"
            error "  https://github.com/${GITHUB_REPO}/releases"
            exit 1 ;;
        *)
            error "Unsupported OS: $(uname -s)"
            exit 1 ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)
            error "Unsupported architecture: $(uname -m)"
            exit 1 ;;
    esac
}

# --- main ---

OS=$(detect_os)
ARCH=$(detect_arch)
info "Detected platform: ${OS}/${ARCH}"

# Resolve latest release version.
info "Resolving latest release..."
LATEST_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
TAG=$(curl -fsSL "${LATEST_URL}" | grep -E '^\s*"tag_name"' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
if [ -z "${TAG}" ]; then
    error "Could not resolve latest release tag from ${LATEST_URL}"
    error "Manual download: https://github.com/${GITHUB_REPO}/releases"
    exit 1
fi
VERSION="${TAG#v}"
info "Latest version: ${TAG}"

# Build archive URL. (Format must match .goreleaser.yaml's name_template.)
ARCHIVE="peek_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${TAG}/${ARCHIVE}"

# Create install dir.
mkdir -p "${INSTALL_DIR}"

# Download.
TMP=$(mktemp -d 2>/dev/null || mktemp -d -t peek-install)
trap 'rm -rf "${TMP}"' EXIT

info "Downloading ${ARCHIVE}..."
if ! curl -fsSL "${DOWNLOAD_URL}" -o "${TMP}/${ARCHIVE}"; then
    error "Download failed: ${DOWNLOAD_URL}"
    error "Manual download: https://github.com/${GITHUB_REPO}/releases"
    exit 1
fi

# Extract.
info "Extracting..."
if ! tar -xzf "${TMP}/${ARCHIVE}" -C "${TMP}"; then
    error "Extraction failed."
    exit 1
fi

# Install.
if [ ! -f "${TMP}/${BINARY_NAME}" ]; then
    error "Binary not found in archive (looked for ${TMP}/${BINARY_NAME})"
    exit 1
fi
mv "${TMP}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

success "Installed peek ${TAG} to ${INSTALL_DIR}/${BINARY_NAME}"

# --- PATH check ---

case ":${PATH}:" in
    *":${INSTALL_DIR}:"*)
        info "Run: peek --version"
        ;;
    *)
        warn "${INSTALL_DIR} is not on your PATH."
        warn "Add it by appending one of these to your shell config:"
        echo
        echo "  # Bash (~/.bashrc or ~/.bash_profile):"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo
        echo "  # Zsh (~/.zshrc):"
        echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo
        echo "  # Fish (~/.config/fish/config.fish):"
        echo "  fish_add_path \$HOME/.local/bin"
        echo
        warn "Then restart your shell (or 'source' the config) and run: peek --version"
        ;;
esac
