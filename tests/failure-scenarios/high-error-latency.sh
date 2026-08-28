#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"
port_forward_test_server
curl -fsS "http://127.0.0.1:$TEST_PORT/scenario?name=alert"
echo "已触发高错误率和高延迟场景，等待 PrometheusRule 持续约 30 秒。"
