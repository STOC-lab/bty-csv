#!/bin/sh
# Linux (VPS) 向けビルド
set -e
export CGO_ENABLED=0
go build -trimpath -ldflags="-s -w" -o btycsv_linux .
echo "built: btycsv_linux"
ls -l btycsv_linux
sha256sum btycsv_linux
