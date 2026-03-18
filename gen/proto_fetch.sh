#!/usr/bin/env bash
# Fetch Google Ads API proto definitions from googleapis/googleapis.
# The proto files are used by gen/codegen.go to generate internal/schema/data/schema.json.
#
# Usage:
#   gen/proto_fetch.sh            # fetches v18 (default)
#   gen/proto_fetch.sh v19        # fetches a specific version
#
# Requires: git

set -euo pipefail

VERSION="${1:-v18}"
GOOGLEAPIS_REPO="https://github.com/googleapis/googleapis.git"
PROTO_DIR="$(cd "$(dirname "$0")" && pwd)/proto"
SPARSE_PATHS=(
  "google/ads/googleads/${VERSION}/resources"
  "google/ads/googleads/${VERSION}/enums"
  "google/ads/googleads/${VERSION}/common"
  "google/ads/googleads/${VERSION}/services/google_ads_field_service.proto"
  "google/api"
)

echo "Fetching Google Ads API ${VERSION} proto definitions..."
echo "Target: ${PROTO_DIR}"

# Clean up existing proto directory
rm -rf "${PROTO_DIR}"
mkdir -p "${PROTO_DIR}"

cd "${PROTO_DIR}"

# Initialise a bare git repo with sparse checkout
git init --quiet
git remote add origin "${GOOGLEAPIS_REPO}"
git config core.sparseCheckout true

# Write the sparse-checkout paths
mkdir -p .git/info
printf '%s\n' "${SPARSE_PATHS[@]}" > .git/info/sparse-checkout

echo "Pulling proto files (sparse checkout, depth=1)..."
git pull --depth=1 --quiet origin master

RESOURCE_COUNT=$(find "${PROTO_DIR}/google/ads/googleads/${VERSION}/resources" -name '*.proto' 2>/dev/null | wc -l)
ENUM_COUNT=$(find "${PROTO_DIR}/google/ads/googleads/${VERSION}/enums" -name '*.proto' 2>/dev/null | wc -l)

echo ""
echo "Done."
echo "  Resource proto files : ${RESOURCE_COUNT}"
echo "  Enum proto files     : ${ENUM_COUNT}"
echo ""
echo "Run the following to regenerate the embedded schema:"
echo "  go run gen/codegen.go"
