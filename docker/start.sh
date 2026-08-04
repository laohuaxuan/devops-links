#!/bin/sh
set -eu

CONFIG_FILE="${CONFIG_FILE:-/app/config.yaml}"
export CONFIG_FILE

if [ ! -f "$CONFIG_FILE" ]; then
  echo "[start] ERROR: config file not found: $CONFIG_FILE" >&2
  exit 1
fi

echo "[start] starting devops-links backend (CONFIG_FILE=$CONFIG_FILE)"
/app/server &
SERVER_PID=$!

# 后端未就绪时不启动 nginx，避免对外只暴露静态页、API 502
i=0
while [ "$i" -lt 30 ]; do
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "[start] ERROR: backend exited during startup (check DB/config)" >&2
    wait "$SERVER_PID" || true
    exit 1
  fi
  if wget -q -O /dev/null "http://127.0.0.1:8090/api/health" 2>/dev/null \
    || curl -sf "http://127.0.0.1:8090/api/health" >/dev/null 2>&1; then
    echo "[start] backend is healthy on :8090"
    break
  fi
  i=$((i + 1))
  sleep 1
done

if ! wget -q -O /dev/null "http://127.0.0.1:8090/api/health" 2>/dev/null \
  && ! curl -sf "http://127.0.0.1:8090/api/health" >/dev/null 2>&1; then
  echo "[start] ERROR: backend health check failed on :8090/api/health" >&2
  kill -TERM "$SERVER_PID" 2>/dev/null || true
  wait "$SERVER_PID" 2>/dev/null || true
  exit 1
fi

echo "[start] starting nginx"
nginx -g "daemon off;" &
NGINX_PID=$!

cleanup() {
  echo "[start] shutting down..."
  kill -TERM "$SERVER_PID" "$NGINX_PID" 2>/dev/null || true
  wait "$SERVER_PID" 2>/dev/null || true
  wait "$NGINX_PID" 2>/dev/null || true
}

trap cleanup INT TERM

# 任一进程退出则结束容器，交给 K8s 重启
while kill -0 "$SERVER_PID" 2>/dev/null && kill -0 "$NGINX_PID" 2>/dev/null; do
  sleep 2
done

if ! kill -0 "$SERVER_PID" 2>/dev/null; then
  echo "[start] ERROR: backend exited unexpectedly" >&2
  wait "$SERVER_PID" || true
  cleanup
  exit 1
fi

echo "[start] ERROR: nginx exited unexpectedly" >&2
cleanup
exit 1
