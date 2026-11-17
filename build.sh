#!/usr/bin/env bash
# Build server (Go, linux amd64+arm64) + webapp (esbuild) in throwaway containers and bundle the plugin tarball.
set -euo pipefail
cd "$(dirname "$0")"
GOCACHE_DIR=${GOCACHE_DIR:-$PWD/.cache/go}
mkdir -p "$GOCACHE_DIR"
docker run --rm -v "$PWD":/src -v "$GOCACHE_DIR":/go/pkg -e GOTOOLCHAIN=auto -w /src/server golang:1.24-bookworm sh -c '
  go vet ./... && go test -count=1 ./... &&
  CGO_ENABLED=0 GOARCH=amd64 go build -trimpath -o dist/plugin-linux-amd64 . &&
  CGO_ENABLED=0 GOARCH=arm64 go build -trimpath -o dist/plugin-linux-arm64 .'
docker run --rm -v "$PWD/webapp":/w -w /w node:20-bookworm-slim sh -c 'npm ci --no-audit --no-fund --loglevel=error 2>/dev/null || npm install --no-audit --no-fund --loglevel=error; npm run -s typecheck && npm run -s build'
ID=$(python3 -c 'import json;print(json.load(open("plugin.json"))["id"])')
VER=$(python3 -c 'import json;print(json.load(open("plugin.json"))["version"])')
rm -rf dist && mkdir -p "dist/$ID/server/dist" "dist/$ID/webapp/dist"
cp plugin.json "dist/$ID/"
cp server/dist/plugin-linux-* "dist/$ID/server/dist/"
cp webapp/dist/main.js "dist/$ID/webapp/dist/"
tar -C dist -czf "dist/$ID-$VER.tar.gz" "$ID"
ls -la "dist/$ID-$VER.tar.gz"
