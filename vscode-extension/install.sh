#!/bin/bash
# Install Gwen VSCode Extension

set -e

echo "Installing Gwen VSCode Extension..."

# Check if vsce is installed
if ! command -v vsce &> /dev/null; then
    echo "Installing vsce..."
    npm install -g @vscode/vsce
fi

# Package the extension
echo "Packaging extension..."
vsce package

# Install to VSCode
echo "Installing to VSCode..."
code --install-extension gwen-lang-*.vsix

echo "✓ Gwen extension installed!"
echo "Open a .gw file to start coding in Gwen."
