---
title: AutoOps Minikube 测试集群架构
type: cluster-architecture
description: AutoOps 本地开发环境的组件边界、网络路径、命名空间和观测数据流
tags:
  - minikube
  - kubernetes
  - prometheus
  - autoops
---

# AutoOps Minikube 测试集群架构

## 架构目标

Minikube 只模拟被运维的 Kubernetes 集群。AutoOps 后端、前端、Qdrant 和模型服务均运行在集群外，避免测试集群故障影响 AutoOps 自身。

## 组件边界

```text
宿主机
├── AutoOps Backend :8819
├── AutoOps Frontend :5173
├── Qdrant :6333/:6334
├── OpenAI-compatible LLM API
└── Embedding API
        │
        ├── kubeconfig → Kubernetes API
        └── Minikube NodePort 30900 → Prometheus

Minikube
├── kube-system
│   ├── kube-apiserver
│   ├── scheduler/controller-manager
│   ├── CoreDNS
│   └── storage-provisioner
├── monitoring
│   ├── Prometheus
│   ├── Alertmanager
│   ├── Prometheus Operator
│   ├── kube-state-metrics
│   ├── node-exporter
│   └── Grafana
└── autoops-test
    ├── autoops-test-server Deployment
    ├── autoops-test-server NodePort Service
    ├── ServiceMonitor
    └── PrometheusRule
```

## 命名空间

| Namespace | 职责 |
|---|---|
| `autoops-test` | AutoOps 功能验证和故障注入 |
| `monitoring` | kube-prometheus-stack 组件 |
| `kube-system` | Kubernetes 控制面和系统组件 |

AutoOps 的 Kubernetes 查询必须限制在 `autoops-test`，不得读取或修改其他业务命名空间。

## 网络路径

- AutoOps 使用宿主机 `~/.kube/config` 和 `minikube` context 访问 Kubernetes API。
- Prometheus 通过 `monitoring-kube-prometheus-prometheus` Service 的 NodePort `30900` 暴露。
- WSL/Minikube 环境下 Prometheus 地址通常为 `http://<minikube-ip>:30900`，不能假定 `127.0.0.1:30900` 可用。
- 测试指标服务通过 NodePort `32112` 暴露，集群内 Service 端口为 `2112`。

## 监控数据流

```text
autoops-test-server /metrics
  → ServiceMonitor
  → Prometheus
  → PrometheusRule
  → firing alert
  → Background Supervisor
  → Background Remediation Agent
  → 自动修复或 Incident
  → Incident Agent 人工排查
```

## 当前告警规则

- `AutoOpsHighErrorRate`：错误率超过 20%。
- `AutoOpsHighLatency`：P95 延迟超过 1 秒。
- `AutoOpsPodRestarting`：10 分钟内容器重启超过 2 次。
- `AutoOpsDeploymentUnavailable`：可用副本少于期望副本。

告警规则默认要求异常持续 30 秒，Background Supervisor 默认每 30 秒轮询一次。

## 安全边界

- Incident Agent 和 Background Remediation Agent 使用隔离的 Kubernetes 工具权限。
- Background Remediation Agent 只能提出修复建议。
- Policy 层是自动修复的最终准入点。
- 首版唯一允许的自动动作是删除由 ReplicaSet/Deployment 管理的异常 Pod。
- 禁止自动删除 Deployment、Service、Node，禁止扩缩容、Patch、Update 和 `kubectl exec`。
