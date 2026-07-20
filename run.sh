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

echo "=== Restarting systemctl service ==="
systemctl daemon-reload

echo "=== Restarting service ==="
systemctl restart povai-idn.service

echo "=== Checking service status ==="
systemctl status povai-idn.service
