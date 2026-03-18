#!/usr/bin/env sh
# gads-cli installer
# Usage: curl -fsSL https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.sh | sh
#
# Private repo auth (set before running):
#   export GITHUB_TOKEN="ghp_xxx"
#   curl -fsSL https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.sh | sh
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

# Detect downloader
if command -v curl > /dev/null 2>&1; then
  DOWNLOADER="curl"
elif command -v wget > /dev/null 2>&1; then
  DOWNLOADER="wget"
else
  echo "Error: curl or wget is required to install gads" >&2
  exit 1
fi

# http_get <url> — fetch URL to stdout, with optional GitHub token auth
http_get() {
  _url="$1"
  if [ "${DOWNLOADER}" = "curl" ]; then
    if [ -n "${GITHUB_TOKEN}" ]; then
      curl -fsSL -H "Authorization: token ${GITHUB_TOKEN}" "${_url}"
    else
      curl -fsSL "${_url}"
    fi
  else
    if [ -n "${GITHUB_TOKEN}" ]; then
      wget -qO- --header="Authorization: token ${GITHUB_TOKEN}" "${_url}"
    else
      wget -qO- "${_url}"
    fi
  fi
}

# http_download <url> <dest> [accept-header] — download URL to file
http_download() {
  _url="$1"
  _dest="$2"
  _accept="${3:-}"
  if [ "${DOWNLOADER}" = "curl" ]; then
    _curl_args="-fsSL"
    [ -n "${GITHUB_TOKEN}" ] && _curl_args="${_curl_args} -H \"Authorization: token ${GITHUB_TOKEN}\""
    [ -n "${_accept}" ]      && _curl_args="${_curl_args} -H \"Accept: ${_accept}\""
    # Use eval to handle the quoted headers correctly
    eval curl ${_curl_args} -o "\"${_dest}\"" "\"${_url}\""
  else
    _wget_args="-q"
    [ -n "${GITHUB_TOKEN}" ] && _wget_args="${_wget_args} --header=\"Authorization: token ${GITHUB_TOKEN}\""
    [ -n "${_accept}" ]      && _wget_args="${_wget_args} --header=\"Accept: ${_accept}\""
    eval wget ${_wget_args} -O "\"${_dest}\"" "\"${_url}\""
  fi
}

# Fetch latest release JSON from GitHub API
echo "Fetching latest release..."
GITHUB_API="https://api.github.com/repos/${REPO}/releases/latest"
LATEST_JSON="$(http_get "${GITHUB_API}")"

# Parse tag_name (portable, no jq dependency)
LATEST_VERSION="$(printf '%s' "${LATEST_JSON}" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"

if [ -z "${LATEST_VERSION}" ]; then
  echo "Error: could not determine latest release version" >&2
  echo "Check that the repository has published releases." >&2
  if [ -n "${GITHUB_TOKEN}" ]; then
    echo "GITHUB_TOKEN is set — verify it has 'repo' scope for private repos." >&2
  else
    echo "For private repos, set GITHUB_TOKEN before running this script." >&2
  fi
  exit 1
fi

echo "Latest version: ${LATEST_VERSION}"

# Create a temp directory
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

BINARY_PATH="${TMP_DIR}/${BINARY_NAME}${EXT}"
CHECKSUM_PATH="${TMP_DIR}/checksums.txt"

echo "Downloading ${BINARY_FILE}..."

# For private repos: parse asset API URLs from the release JSON so we can
# download with token auth.  For public repos we fall back to browser_download_url.
# Asset URL pattern in JSON: {"name":"BINARY_FILE",...,"url":"https://api.github.com/..."}
ASSET_API_URL=""
CHECKSUM_API_URL=""
if [ -n "${GITHUB_TOKEN}" ]; then
  # Extract the API URL for each asset by finding "name":"FILE" then the nearest "url":"..."
  # We normalize whitespace first, then match the two fields on the same line after collapsing
  # the assets array into one line per object (split on '}' boundaries).
  # This avoids a jq dependency while remaining portable.
  ASSET_API_URL="$(printf '%s' "${LATEST_JSON}" \
    | tr -d '\n' \
    | grep -o '"name"[[:space:]]*:[[:space:]]*"'"${BINARY_FILE}"'"[^}]*"url"[[:space:]]*:[[:space:]]*"[^"]*"' \
    | grep '"url"' \
    | sed 's/.*"url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' \
    | head -1)"
  CHECKSUM_API_URL="$(printf '%s' "${LATEST_JSON}" \
    | tr -d '\n' \
    | grep -o '"name"[[:space:]]*:[[:space:]]*"checksums\.txt"[^}]*"url"[[:space:]]*:[[:space:]]*"[^"]*"' \
    | grep '"url"' \
    | sed 's/.*"url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' \
    | head -1)"
fi

# Fall back to browser_download_url if no asset API URL found
BASE_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}"
BINARY_DOWNLOAD_URL="${BASE_URL}/${BINARY_FILE}"
CHECKSUM_DOWNLOAD_URL="${BASE_URL}/checksums.txt"

if [ -n "${ASSET_API_URL}" ]; then
  BINARY_DOWNLOAD_URL="${ASSET_API_URL}"
fi
if [ -n "${CHECKSUM_API_URL}" ]; then
  CHECKSUM_DOWNLOAD_URL="${CHECKSUM_API_URL}"
fi

# Download binary
# When using the GitHub asset API URL, pass Accept: application/octet-stream to get a redirect to the raw bytes
if [ -n "${ASSET_API_URL}" ]; then
  http_download "${BINARY_DOWNLOAD_URL}" "${BINARY_PATH}" "application/octet-stream" || {
    echo "Error: failed to download binary from ${BINARY_DOWNLOAD_URL}" >&2
    exit 1
  }
else
  http_download "${BINARY_DOWNLOAD_URL}" "${BINARY_PATH}" || {
    echo "Error: failed to download binary from ${BINARY_DOWNLOAD_URL}" >&2
    exit 1
  }
fi

# Download checksums (optional)
CHECKSUM_PATH_VALID=""
if [ -n "${CHECKSUM_API_URL}" ]; then
  http_download "${CHECKSUM_DOWNLOAD_URL}" "${CHECKSUM_PATH}" "application/octet-stream" && CHECKSUM_PATH_VALID="${CHECKSUM_PATH}" || {
    echo "Warning: checksums.txt not found — skipping checksum verification" >&2
  }
else
  http_download "${CHECKSUM_DOWNLOAD_URL}" "${CHECKSUM_PATH}" && CHECKSUM_PATH_VALID="${CHECKSUM_PATH}" || {
    echo "Warning: checksums.txt not found — skipping checksum verification" >&2
  }
fi

# Verify SHA256 checksum
if [ -n "${CHECKSUM_PATH_VALID}" ] && [ -f "${CHECKSUM_PATH_VALID}" ]; then
  echo "Verifying checksum..."

  EXPECTED_CHECKSUM="$(grep "${BINARY_FILE}" "${CHECKSUM_PATH_VALID}" | awk '{print $1}')"

  if [ -z "${EXPECTED_CHECKSUM}" ]; then
    echo "Warning: no checksum found for ${BINARY_FILE} in checksums.txt — skipping verification" >&2
  else
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
