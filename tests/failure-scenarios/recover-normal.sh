#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"
port_forward_test_server
curl -fsS "http://127.0.0.1:$TEST_PORT/scenario?name=normal"
echo "已恢复 normal 场景。"
