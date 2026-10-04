#!/bin/sh
# Build the playground into playground/web.
# Usage: playground/build.sh [serve]
set -eu
cd "$(dirname "$0")/.."
: "${GOTOOLCHAIN:=go1.27.1}"
export GOTOOLCHAIN
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o playground/web/monadz.wasm ./playground/wasm
rm -f playground/web/wasm_exec.js
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" playground/web/wasm_exec.js
chmod 644 playground/web/wasm_exec.js
echo "built playground/web"
if [ "${1:-}" = "serve" ]; then
	cd playground/web
	echo "open http://localhost:${PORT:-8080}/"
	exec python3 -m http.server "${PORT:-8080}"
fi
