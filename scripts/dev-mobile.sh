#!/usr/bin/env bash
# dev-mobile.sh — 一键启动后端 + Expo 移动端开发
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "🔄 停止旧进程..."
pkill -f "go run ./cmd/server" 2>/dev/null || true
pkill -f "expo start" 2>/dev/null || true
pkill -f "metro" 2>/dev/null || true
sleep 1

echo "🚀 启动后端..."
cd "$REPO_ROOT"
go run ./cmd/server/ &
BACKEND_PID=$!
sleep 2

echo "📱 启动 Expo..."
cd "$REPO_ROOT/mobile"
npx expo start --ios --clear &
EXPO_PID=$!

echo ""
echo "✅ 全部启动"
echo "   后端 PID: $BACKEND_PID (localhost:8080)"
echo "   Expo PID: $EXPO_PID (localhost:8081)"
echo ""
echo "   按 Ctrl+C 停止全部"

trap "kill $BACKEND_PID $EXPO_PID 2>/dev/null; exit" INT TERM
wait
