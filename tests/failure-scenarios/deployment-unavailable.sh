#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"
kubectl -n "$NAMESPACE" scale deployment "$DEPLOYMENT" --replicas=0
echo "Deployment 已缩容为 0，等待约 30 秒触发 AutoOpsDeploymentUnavailable。"
