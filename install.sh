#!/bin/bash

set -e

BINARY_NAME="orbit"
INSTALL_DIR="$HOME/.local/bin"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SOURCE_BINARY="$SCRIPT_DIR/$BINARY_NAME"

check_architecture() {
    local machine
    machine="$(uname -m)"
    local expected_pat

    case "$machine" in
        x86_64|amd64)   expected_pat="x86-64" ;;
        aarch64|arm64)  expected_pat="aarch64" ;;
        *)
            echo "  Warning: unknown machine architecture '$machine', skipping check."
            return 0
            ;;
    esac

    if ! command -v file >/dev/null 2>&1; then
        echo "  Warning: 'file' command not found, skipping architecture check."
        return 0
    fi

    local info
    info="$(file -b "$SOURCE_BINARY" 2>/dev/null || true)"

    if echo "$info" | grep -qi "$expected_pat"; then
        echo "  Architecture check passed ($machine)."
    else
        echo "Error: binary architecture does not match the current system."
        echo "  Current system: $machine"
        echo "  Binary info:    $info"
        echo "Please download the correct architecture package."
        exit 1
    fi
}

print_usage() {
    echo "Usage: $0 [install|uninstall]"
    echo ""
    echo "  install    Install $BINARY_NAME to $INSTALL_DIR and configure PATH"
    echo "  uninstall  Uninstall $BINARY_NAME and clean up PATH"
    echo ""
    echo "Defaults to install when no argument is provided."
}

detect_shell_config() {
    local shell_name
    shell_name="$(basename "$SHELL")"
    case "$shell_name" in
        zsh)  echo "$HOME/.zshrc" ;;
        bash)
            if [ -f "$HOME/.bash_profile" ]; then
                echo "$HOME/.bash_profile"
            else
                echo "$HOME/.bashrc"
            fi
            ;;
        fish) echo "$HOME/.config/fish/config.fish" ;;
        *)    echo "$HOME/.profile" ;;
    esac
}

ensure_path() {
    local shell_config
    shell_config="$(detect_shell_config)"

    if echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
        echo "  $INSTALL_DIR is already in PATH."
        return
    fi

    local shell_name
    shell_name="$(basename "$SHELL")"

    if [ "$shell_name" = "fish" ]; then
        local line="fish_add_path $INSTALL_DIR"
        if ! grep -qF "$line" "$shell_config" 2>/dev/null; then
            echo "$line" >> "$shell_config"
            echo "  Added PATH entry to $shell_config"
        fi
    else
        local line="export PATH=\"$INSTALL_DIR:\$PATH\""
        if ! grep -qF "$INSTALL_DIR" "$shell_config" 2>/dev/null; then
            echo "" >> "$shell_config"
            echo "# Orbit agent" >> "$shell_config"
            echo "$line" >> "$shell_config"
            echo "  Added PATH entry to $shell_config"
        fi
    fi

    export PATH="$INSTALL_DIR:$PATH"
}

remove_path() {
    local shell_config
    shell_config="$(detect_shell_config)"

    if [ ! -f "$shell_config" ]; then
        return
    fi

    local shell_name
    shell_name="$(basename "$SHELL")"

    if [ "$shell_name" = "fish" ]; then
        if grep -qF "fish_add_path $INSTALL_DIR" "$shell_config"; then
            sed -i "\|fish_add_path $INSTALL_DIR|d" "$shell_config"
            echo "  Removed PATH entry from $shell_config."
        fi
    else
        if grep -qF "$INSTALL_DIR" "$shell_config"; then
            sed -i "\|# Orbit agent|d" "$shell_config"
            sed -i "\|$INSTALL_DIR|d" "$shell_config"
            echo "  Removed PATH entry from $shell_config."
        fi
    fi
}

do_install() {
    echo "Installing $BINARY_NAME ..."

    if [ ! -f "$SOURCE_BINARY" ]; then
        echo "Error: $SOURCE_BINARY not found."
        echo "Please make sure the $BINARY_NAME binary is in the same directory as this script."
        exit 1
    fi

    check_architecture

    mkdir -p "$INSTALL_DIR"

    cp "$SOURCE_BINARY" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    echo "  Copied $BINARY_NAME to $INSTALL_DIR/"

    ensure_path

    echo ""
    echo "Installation complete!"
    echo "Run the following command to apply changes, or restart your terminal:"
    echo ""
    echo "  source $(detect_shell_config)"
    echo ""
    echo "Then you can use: $BINARY_NAME"
}

do_uninstall() {
    echo "Uninstalling $BINARY_NAME ..."

    if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
        rm "$INSTALL_DIR/$BINARY_NAME"
        echo "  Removed $INSTALL_DIR/$BINARY_NAME"
    else
        echo "  $INSTALL_DIR/$BINARY_NAME does not exist, skipping."
    fi

    remove_path

    echo ""
    echo "Uninstallation complete!"
}

ACTION="${1:-install}"

case "$ACTION" in
    install)
        do_install
        ;;
    uninstall)
        do_uninstall
        ;;
    -h|--help|help)
        print_usage
        ;;
    *)
        echo "Unknown action: $ACTION"
        print_usage
        exit 1
        ;;
esac
