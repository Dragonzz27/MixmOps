#!/usr/bin/env bash
set -Eeuo pipefail
NAMESPACE="${AUTOOPS_TEST_NAMESPACE:-autoops-test}"
DEPLOYMENT="${AUTOOPS_TEST_DEPLOYMENT:-autoops-test-server}"
SERVICE="${AUTOOPS_TEST_SERVICE:-autoops-test-server}"
TEST_PORT="${AUTOOPS_TEST_PORT:-2112}"
AUTOOPS_URL="${AUTOOPS_URL:-http://127.0.0.1:8819}"
need() { command -v "$1" >/dev/null 2>&1 || { echo "缺少命令: $1" >&2; exit 1; }; }
need kubectl
port_forward_test_server() {
  need curl
  kubectl -n "$NAMESPACE" port-forward "svc/$SERVICE" "$TEST_PORT:2112" >/tmp/autoops-test-port-forward.log 2>&1 &
  PF_PID=$!
  trap 'kill "$PF_PID" 2>/dev/null || true' EXIT
  for _ in {1..20}; do
    if curl -fsS "http://127.0.0.1:$TEST_PORT/metrics" >/dev/null 2>&1; then return; fi
    sleep 0.5
  done
  echo "无法连接测试服务，端口转发日志: /tmp/autoops-test-port-forward.log" >&2
  exit 1
}
