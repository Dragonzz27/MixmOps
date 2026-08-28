#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"
kubectl -n "$NAMESPACE" patch deployment "$DEPLOYMENT" --type='json' -p='[{"op":"add","path":"/spec/template/spec/containers/0/command","value":["/bin/sh","-c","exit 1"]}]'
kubectl -n "$NAMESPACE" rollout status deployment "$DEPLOYMENT" --timeout=90s || true
echo "已注入 CrashLoopBackOff。恢复请执行："
echo "kubectl -n $NAMESPACE rollout undo deployment/$DEPLOYMENT"
