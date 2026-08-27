#!/usr/bin/env bash
set -euo pipefail

APP_NAME="mahiru-dybot"
BUILD_DIR="dist"

# 清理旧构建
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

echo "=== 构建 $APP_NAME (linux/amd64) ==="
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o "$BUILD_DIR/$APP_NAME" .

echo "=== 复制 webui 资源 ==="
cp -r webui "$BUILD_DIR/webui"

echo "=== 构建完成: $BUILD_DIR/$APP_NAME ==="
ls -lh "$BUILD_DIR/$APP_NAME"
