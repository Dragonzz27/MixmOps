---
title: Pod CrashLoopBackOff 故障预案
type: incident-runbook
description: Pod 持续崩溃或频繁重启时的证据采集、判断、隔离和恢复步骤
tags:
  - kubernetes
  - pod
  - crashloopbackoff
  - incident
---

# Pod CrashLoopBackOff 故障预案

## 适用范围

适用于 Pod 状态为 `CrashLoopBackOff`、容器频繁重启或 `AutoOpsPodRestarting` 告警。

## 第一阶段：确认影响范围

```bash
kubectl -n autoops-test get pods -o wide
kubectl -n autoops-test get deployment
kubectl -n autoops-test get events --sort-by=.lastTimestamp
```

判断：

- 单个 Pod 异常还是多个 Pod 同时异常；
- Deployment 是否仍有可用副本；
- 是否存在镜像拉取、配置、探针、OOM 或调度问题；
- 异常 Pod 是否由 ReplicaSet/Deployment 管理。

## 第二阶段：读取证据

```bash
kubectl -n autoops-test describe pod <pod-name>
kubectl -n autoops-test logs <pod-name> --tail=200
kubectl -n autoops-test logs <pod-name> --previous --tail=200
```

常见原因：

- 启动命令退出；
- 配置或 Secret 缺失；
- 端口冲突；
- Readiness/Liveness Probe 失败；
- 内存限制导致 OOMKilled；
- 依赖服务无法连接。

## 自动修复准入

只有以下条件全部满足时才允许自动删除 Pod：

- 告警位于自动修复白名单；
- 只有一个异常 Pod；
- Deployment 仍有健康控制器；
- Pod 由 ReplicaSet 管理；
- 当前证据明确表明重建 Pod 风险较低；
- Policy 校验通过。

Critical、多 Pod、Deployment 全部不可用或证据不足时必须创建 Incident，等待人工介入。

## 人工恢复

若故障来自错误变更：

```bash
kubectl -n autoops-test rollout history deployment/autoops-test-server
kubectl -n autoops-test rollout undo deployment/autoops-test-server
kubectl -n autoops-test rollout status deployment/autoops-test-server
```

恢复后确认：

```bash
kubectl -n autoops-test get pods
kubectl -n autoops-test get deployment
curl -s http://127.0.0.1:8819/background/tasks
```

不得在原因未知时连续删除多个 Pod。

