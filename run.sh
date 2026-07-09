#!/bin/bash
# Exit immediately if any command exits with a non-zero status
set -e

echo "=== Stashing local changes ==="
git stash

echo "=== Pulling latest changes ==="
git pull

echo "=== Building application ==="
go build cmd/main.go

echo "=== Running application ==="
./main
