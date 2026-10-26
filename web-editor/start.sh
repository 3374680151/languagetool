#!/bin/sh
# 一键启动校对台：LanguageTool 服务(8081) + 静态站(8091)
# 用法：./start.sh            启动（已在跑就跳过）
#       ./start.sh stop       停掉本脚本起的两个进程
set -u

LT_HOME="/Users/wang/tools/LanguageTool-6.6"
JAVA="/Users/wang/tools/java21/Contents/Home/bin/java"
PY="/Users/wang/.workbuddy-ai/binaries/python/versions/3.13.12/bin/python3"
WEB_DIR="$(cd "$(dirname "$0")" && pwd)"
PORT_LT=8081
PORT_WEB=8091
RUN_DIR="/tmp/lt-web-editor"

mkdir -p "$RUN_DIR"

alive() { curl -s -o /dev/null -m 2 "$1"; }

if [ "${1:-}" = "stop" ]; then
  for f in "$RUN_DIR/lt.pid" "$RUN_DIR/web.pid"; do
    [ -f "$f" ] && kill "$(cat "$f")" 2>/dev/null && rm -f "$f"
  done
  echo "已停止。"
  exit 0
fi

# 1) LanguageTool
if alive "http://localhost:$PORT_LT/v2/languages"; then
  echo "LanguageTool 已在 $PORT_LT 运行"
else
  echo "启动 LanguageTool（首次加载语言包需要十几秒）…"
  cd "$LT_HOME" || exit 1
  nohup "$JAVA" -Xmx2g -cp languagetool-server.jar org.languagetool.server.HTTPServer \
    --port "$PORT_LT" --allow-origin > "$RUN_DIR/lt.log" 2>&1 &
  echo $! > "$RUN_DIR/lt.pid"
  i=0
  while [ $i -lt 60 ]; do
    alive "http://localhost:$PORT_LT/v2/languages" && break
    i=$((i+1)); sleep 1
  done
  alive "http://localhost:$PORT_LT/v2/languages" && echo "LanguageTool 就绪" || echo "LanguageTool 启动失败，见 $RUN_DIR/lt.log"
fi

# 2) 静态站
if alive "http://127.0.0.1:$PORT_WEB/"; then
  echo "静态站已在 $PORT_WEB 运行"
else
  cd "$WEB_DIR" || exit 1
  nohup "$PY" -m http.server "$PORT_WEB" --bind 127.0.0.1 > "$RUN_DIR/web.log" 2>&1 &
  echo $! > "$RUN_DIR/web.pid"
  sleep 1
  echo "静态站就绪"
fi

echo ""
echo "打开： http://127.0.0.1:$PORT_WEB/"
