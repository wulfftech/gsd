#!/usr/bin/env bash
# Build the GSD image with real version metadata stamped into the binary.
#
# A bare `docker compose build` leaves VERSION/COMMIT/BUILD_DATE as "dev",
# which makes deployed builds indistinguishable from each other. This wrapper
# fills them in from git, then hands off to compose. Any extra arguments are
# passed straight through (e.g. --no-cache).
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
export VERSION COMMIT BUILD_DATE

echo "Building GSD  version=${VERSION}  commit=${COMMIT}  date=${BUILD_DATE}"
docker compose build "$@"
