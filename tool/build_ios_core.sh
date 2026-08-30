#!/bin/bash
# Builds the sing-box core as an iOS arm64 c-archive for the PacketTunnel
# extension. Output: ios/PacketTunnel/Libs/{libclash.a,libclash.h}
set -uo pipefail
cd "$(dirname "$0")/../core-singbox"

SDK=$(xcrun --sdk iphoneos --show-sdk-path)
CC=$(xcrun --sdk iphoneos --find clang)

export CGO_ENABLED=1
export GOOS=ios
export GOARCH=arm64
export CC="$CC"
export CGO_CFLAGS="-isysroot $SDK -miphoneos-version-min=13.0 -arch arm64"
export CGO_LDFLAGS="-isysroot $SDK -miphoneos-version-min=13.0 -arch arm64"

OUT_DIR=../ios/PacketTunnel/Libs
mkdir -p "$OUT_DIR"

echo "== go version: $(go version)"
echo "== env: GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) CGO_ENABLED=$(go env CGO_ENABLED)"
go list -f 'GOFILES: {{.GoFiles}}' .
go list -f 'IGNORED: {{.IgnoredGoFiles}}' .

BUILD_OK=0
for attempt in 1 2 3; do
  echo "== build attempt $attempt"
  if go build -buildmode=c-archive \
      -tags "with_clash_api,with_tun" \
      -ldflags "-w -s" \
      -o "$OUT_DIR/libclash.a" .; then
    BUILD_OK=1
    break
  fi
  sleep 3
done

if [ "$BUILD_OK" != "1" ]; then
  echo "core build failed after retries"
  exit 1
fi

echo "built $OUT_DIR/libclash.a"
ls -la "$OUT_DIR"
