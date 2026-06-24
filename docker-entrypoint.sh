#!/bin/sh
set -e

echo "Starting SmartX Matching Engine..."

# 如果需要初始化，可以在这里添加初始化逻辑

exec /app/server "$@"
