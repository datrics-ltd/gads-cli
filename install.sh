#!/usr/bin/env sh
# gads-cli installer
# Usage: curl -fsSL https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.sh | sh
# With GitHub PAT (private repo): curl -fsSL -H "Authorization: token ghp_xxx" ... | sh
set -e

REPO="datrics-ltd/gads-cli"
BINARY_NAME="gads"
INSTALL_DIR=""

# Determine install directory
if [ "$(id -u)" = "0" ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="${HOME}/.local/bin"
fi

# Allow override via env var
if [ -n "${GADS_INSTALL_DIR}" ]; then
  INSTALL_DIR="${GADS_INSTALL_DIR}"
fi

# Detect OS
OS="$(uname -s)"
case "${OS}" in
  Linux)   OS="linux" ;;
  Darwin)  OS="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
  *)
    echo "Error: unsupported operating system: ${OS}" >&2
    exit 1
    ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Error: unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

# Windows only supports amd64 currently
if [ "${OS}" = "windows" ] && [ "${ARCH}" != "amd64" ]; then
  echo "Error: Windows is only supported on amd64" >&2
  exit 1
fi

# Determine binary suffix
EXT=""
if [ "${OS}" = "windows" ]; then
  EXT=".exe"
fi

BINARY_FILE="${BINARY_NAME}-${OS}-${ARCH}${EXT}"

echo "Detected platform: ${OS}/${ARCH}"

# Fetch the latest release tag from GitHub API
GITHUB_API="https://api.github.com/repos/${REPO}/releases/latest"

if command -v curl > /dev/null 2>&1; then
  DOWNLOADER="curl"
elif command -v wget > /dev/null 2>&1; then
  DOWNLOADER="wget"
else
  echo "Error: curl or wget is required to install gads" >&2
  exit 1
fi

echo "Fetching latest release..."
if [ "${DOWNLOADER}" = "curl" ]; then
  LATEST_JSON="$(curl -fsSL "${GITHUB_API}")"
else
  LATEST_JSON="$(wget -qO- "${GITHUB_API}")"
fi

# Parse tag_name from JSON (portable, no jq dependency)
LATEST_VERSION="$(printf '%s' "${LATEST_JSON}" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"

if [ -z "${LATEST_VERSION}" ]; then
  echo "Error: could not determine latest release version" >&2
  echo "Check that the repository has published releases." >&2
  exit 1
fi

echo "Latest version: ${LATEST_VERSION}"

# Build download URLs
BASE_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}"
BINARY_URL="${BASE_URL}/${BINARY_FILE}"
CHECKSUM_URL="${BASE_URL}/checksums.txt"

# Create a temp directory
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

BINARY_PATH="${TMP_DIR}/${BINARY_NAME}${EXT}"
CHECKSUM_PATH="${TMP_DIR}/checksums.txt"

echo "Downloading ${BINARY_FILE}..."
if [ "${DOWNLOADER}" = "curl" ]; then
  curl -fsSL -o "${BINARY_PATH}" "${BINARY_URL}" || {
    echo "Error: failed to download binary from ${BINARY_URL}" >&2
    exit 1
  }
  curl -fsSL -o "${CHECKSUM_PATH}" "${CHECKSUM_URL}" || {
    echo "Warning: checksums.txt not found — skipping checksum verification" >&2
    CHECKSUM_PATH=""
  }
else
  wget -qO "${BINARY_PATH}" "${BINARY_URL}" || {
    echo "Error: failed to download binary from ${BINARY_URL}" >&2
    exit 1
  }
  wget -qO "${CHECKSUM_PATH}" "${CHECKSUM_URL}" || {
    echo "Warning: checksums.txt not found — skipping checksum verification" >&2
    CHECKSUM_PATH=""
  }
fi

# Verify SHA256 checksum
if [ -n "${CHECKSUM_PATH}" ] && [ -f "${CHECKSUM_PATH}" ]; then
  echo "Verifying checksum..."

  EXPECTED_CHECKSUM="$(grep "${BINARY_FILE}" "${CHECKSUM_PATH}" | awk '{print $1}')"

  if [ -z "${EXPECTED_CHECKSUM}" ]; then
    echo "Warning: no checksum found for ${BINARY_FILE} in checksums.txt — skipping verification" >&2
  else
    # Compute actual checksum
    if command -v sha256sum > /dev/null 2>&1; then
      ACTUAL_CHECKSUM="$(sha256sum "${BINARY_PATH}" | awk '{print $1}')"
    elif command -v shasum > /dev/null 2>&1; then
      ACTUAL_CHECKSUM="$(shasum -a 256 "${BINARY_PATH}" | awk '{print $1}')"
    else
      echo "Warning: sha256sum/shasum not found — skipping checksum verification" >&2
      ACTUAL_CHECKSUM="${EXPECTED_CHECKSUM}"
    fi

    if [ "${ACTUAL_CHECKSUM}" != "${EXPECTED_CHECKSUM}" ]; then
      echo "Error: checksum mismatch!" >&2
      echo "  Expected: ${EXPECTED_CHECKSUM}" >&2
      echo "  Actual:   ${ACTUAL_CHECKSUM}" >&2
      echo "The downloaded file may be corrupted or tampered with." >&2
      exit 1
    fi

    echo "Checksum verified."
  fi
fi

# Make binary executable
chmod +x "${BINARY_PATH}"

# Ensure install directory exists
mkdir -p "${INSTALL_DIR}"

# Install binary
INSTALL_PATH="${INSTALL_DIR}/${BINARY_NAME}${EXT}"
mv "${BINARY_PATH}" "${INSTALL_PATH}"

echo ""
echo "gads ${LATEST_VERSION} installed to ${INSTALL_PATH}"

# Check if install dir is on PATH
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo ""
    echo "NOTE: ${INSTALL_DIR} is not in your PATH."
    echo "Add the following to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
    echo ""
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    echo ""
    ;;
esac

# Print next steps
echo ""
echo "Next steps:"
echo "  1. Set your developer token:   gads config set developer_token YOUR_TOKEN"
echo "  2. Set your OAuth2 client ID:  gads config set client_id YOUR_CLIENT_ID"
echo "  3. Set your client secret:     gads config set client_secret YOUR_SECRET"
echo "  4. Set your customer ID:       gads config set default_customer_id 123-456-7890"
echo "  5. Authenticate:               gads auth login"
echo ""
echo "Run 'gads --help' to see all available commands."
