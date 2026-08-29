#!/bin/bash
# Builds the sing-box core as an iOS arm64 c-archive for the PacketTunnel
# extension. Output: ios/PacketTunnel/Libs/{libclash.a,clash.h}
set -euo pipefail
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

go build -buildmode=c-archive \
  -tags "with_clash_api,with_tun,with_gvisor" \
  -ldflags "-w -s" \
  -o "$OUT_DIR/libclash.a" .

echo "built $OUT_DIR/libclash.a"
