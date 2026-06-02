#!/usr/bin/env bash
# dev-mobile.sh — 一键启动后端 + Expo 移动端开发
#
# 用法:
#   ./scripts/dev-mobile.sh            # 快：后端 + expo start（JS 热更，连已装好的 App）
#   ./scripts/dev-mobile.sh --build    # 慢：后端 + 原生构建并安装（含 widget）到模拟器
#
# 改了 JS 用默认模式即可；改了原生（widget / app.config.js / 原生库）才需 --build。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

BUILD=false
if [[ "${1:-}" == "--build" || "${1:-}" == "-b" ]]; then
  BUILD=true
fi

echo "🔄 停止旧进程..."
pkill -f "go run ./cmd/server" 2>/dev/null || true
pkill -f "expo start" 2>/dev/null || true
pkill -f "expo run" 2>/dev/null || true
pkill -f "metro" 2>/dev/null || true
sleep 1

echo "🚀 启动后端..."
cd "$REPO_ROOT"
go run ./cmd/server/ &
BACKEND_PID=$!
sleep 2

cd "$REPO_ROOT/mobile"
if $BUILD; then
  echo "📱 原生构建并安装（含 widget，首次较慢）..."
  EXPO_WIDGET=1 npx expo run:ios &
  EXPO_PID=$!
else
  echo "📱 启动 Expo（JS 热更）..."
  # 默认让 metro 绑 127.0.0.1，模拟器走 loopback 直连，避开 VPN/代理选到
  # 198.18.0.1 这类不可达地址导致的 502/Could not connect。
  # 真机调试时在前面覆盖：REACT_NATIVE_PACKAGER_HOSTNAME=<你的局域网IP> ./scripts/dev-mobile.sh
  export REACT_NATIVE_PACKAGER_HOSTNAME="${REACT_NATIVE_PACKAGER_HOSTNAME:-127.0.0.1}"
  npx expo start --ios --clear &
  EXPO_PID=$!
fi

echo ""
echo "✅ 全部启动"
echo "   后端 PID: $BACKEND_PID (localhost:8080)"
echo "   Expo PID: $EXPO_PID (localhost:8081)"
$BUILD && echo "   模式: 原生构建（含 widget）" || echo "   模式: JS 热更（widget 需先跑过一次 --build）"
echo ""
echo "   按 Ctrl+C 停止全部"

trap "kill $BACKEND_PID $EXPO_PID 2>/dev/null; exit" INT TERM
wait
