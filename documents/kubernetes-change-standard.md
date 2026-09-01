---
title: Kubernetes 变更操作规范
type: change-standard
description: AutoOps 测试集群执行部署、配置、扩缩容和回滚操作时的最小变更规范
tags:
  - kubernetes
  - change
  - rollback
  - safety
---

# Kubernetes 变更操作规范

## 基本原则

- 所有变更必须限定明确 namespace、资源类型和资源名称。
- 执行前记录当前状态和配置。
- 执行前提供验证命令和回滚方案。
- 禁止使用未解析变量、宽泛通配符或全 namespace 批量写操作。
- Incident Agent 只能通过受控动作提案和人工确认执行变更。

## 变更前检查

```bash
kubectl config current-context
kubectl auth can-i update deployment -n autoops-test
kubectl -n autoops-test get deployment,pods,events
kubectl -n autoops-test get deployment <name> -o yaml
```

确认：

- context 为 `minikube`；
- namespace 为 `autoops-test`；
- 当前没有未结束的故障处置任务；
- 目标 Deployment 当前可用状态已记录。

## Deployment 变更

优先使用声明式 Helm 或 manifest 变更。发布后执行：

```bash
kubectl -n autoops-test rollout status deployment/<name> --timeout=120s
kubectl -n autoops-test get pods
kubectl -n autoops-test get events --sort-by=.lastTimestamp
```

## 回滚

```bash
kubectl -n autoops-test rollout history deployment/<name>
kubectl -n autoops-test rollout undo deployment/<name>
kubectl -n autoops-test rollout status deployment/<name>
```

## 禁止操作

- 未经确认删除 Deployment、Service 或 Node；
- 对生产或非测试 namespace 执行故障模拟脚本；
- 在证据不足时连续删除多个 Pod；
- 使用 `kubectl exec` 直接修改容器内部持久状态；
- 在无回滚方案时修改镜像、探针、资源限制或副本数。

## 变更完成条件

- Deployment 可用副本达到期望值；
- Pod 全部 Ready；
- 无新增 Warning Events；
- Prometheus 没有新增 firing 告警；
- AutoOps 后台任务和 Incident 状态已核对；
- 变更内容、结果和回滚点已记录。
