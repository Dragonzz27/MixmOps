#!/usr/bin/env bash
set -Eeuo pipefail
source "$(dirname "$0")/common.sh"
kubectl -n "$NAMESPACE" get deployment,pods,events --sort-by=.metadata.creationTimestamp || true
need curl
echo "--- AutoOps background tasks ---"
curl -fsS "$AUTOOPS_URL/background/tasks" || true
echo
echo "--- AutoOps incidents ---"
curl -fsS "$AUTOOPS_URL/incidents" || true
echo
