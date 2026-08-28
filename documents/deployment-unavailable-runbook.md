---
title: Deployment 可用副本不足故障预案
type: incident-runbook
description: Deployment 可用副本少于期望副本时的诊断、止损和恢复流程
tags:
  - kubernetes
  - deployment
  - availability
  - incident
---

# Deployment 可用副本不足故障预案

## 告警说明

`AutoOpsDeploymentUnavailable` 表示 Deployment 的 `availableReplicas` 少于期望副本。该告警标记为 Critical，应转入 Incident，不允许直接自动删除 Pod。

## 收集状态

```bash
kubectl -n autoops-test get deployment autoops-test-server -o yaml
kubectl -n autoops-test get replicaset
kubectl -n autoops-test get pods -o wide
kubectl -n autoops-test get events --sort-by=.lastTimestamp
```

重点检查：

- `spec.replicas`、`status.availableReplicas` 和 `status.updatedReplicas`；
- Deployment Conditions；
- ReplicaSet 是否成功创建 Pod；
- Pod 是否 Pending、CrashLoopBackOff 或 ImagePullBackOff；
- namespace 是否有 ResourceQuota 或调度限制。

## 常见原因

- Deployment 被误缩容为 0；
- 新版本镜像无法拉取或无法启动；
- 探针配置错误；
- 节点资源不足；
- ConfigMap/Secret 缺失；
- 调度约束无法满足。

## 恢复策略

如果确认是测试脚本缩容：

```bash
kubectl -n autoops-test scale deployment autoops-test-server --replicas=1
```

如果确认是失败发布：

```bash
kubectl -n autoops-test rollout undo deployment/autoops-test-server
kubectl -n autoops-test rollout status deployment/autoops-test-server
```

## 验证恢复

```bash
kubectl -n autoops-test get deployment,pods
curl -s http://<minikube-ip>:30900/api/v1/alerts
```

必须同时确认 Pod Ready、Deployment 可用副本恢复、Prometheus 告警解除。

