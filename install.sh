#!/bin/bash

BINARY_NAME="ai-commit"

echo "Installing $BINARY_NAME"

CGO_ENABLED=0 go build -ldflags="-s -w" -o "$BINARY_NAME" 

if sudo install -m 0755 $BINARY_NAME "/usr/local/bin"; then
    echo "Installation complete."
else
    echo "ERROR: Failed to install"
    echo "Please ensure you have sudo privileges and the directory is writable."
    rm -f "$BINARY_NAME"
    exit 1
fi

rm -f "$BINARY_NAME"
