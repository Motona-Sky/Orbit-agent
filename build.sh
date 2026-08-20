#!/bin/bash
set -e

DIST_DIR="./dist"
ENTRY="./cmd/orbit/main.go"

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

echo "Building orbit for multiple platforms..."

build_and_package() {
    local os="$1"
    local arch="$2"
    local binary_name="$3"
    local install_script="$4"
    local archive_ext="$5"

    local base="orbit-$os-$arch"
    local staging="$DIST_DIR/$base"
    local archive="$DIST_DIR/$base.$archive_ext"

    mkdir -p "$staging"

    GOOS="$os" GOARCH="$arch" go build -o "$staging/$binary_name" "$ENTRY"
    cp "$install_script" "$staging/"

    if [ "$os" = "windows" ]; then
        (cd "$DIST_DIR" && zip -qr "$base.zip" "$base")
    else
        tar -czf "$archive" -C "$DIST_DIR" "$base"
    fi

    rm -rf "$staging"

    echo "  Built $archive"
}

build_and_package windows amd64 "orbit.exe" "install.ps1" "zip"
build_and_package windows arm64 "orbit.exe" "install.ps1" "zip"
build_and_package linux amd64 "orbit" "install.sh" "tar.gz"
build_and_package linux arm64 "orbit" "install.sh" "tar.gz"

echo "All builds completed. Output in $DIST_DIR/"