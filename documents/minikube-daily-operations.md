---
title: Minikube 集群日常运维手册
type: operations-manual
description: AutoOps 本地测试集群的启动、检查、访问、停止和基础恢复操作
tags:
  - minikube
  - kubectl
  - operations
---

# Minikube 集群日常运维手册

## 启动检查

```bash
minikube status
kubectl config current-context
kubectl get nodes
kubectl get pods -A
```

当前 context 应为 `minikube`，节点应为 `Ready`。

## 检查 AutoOps 测试环境

```bash
kubectl -n autoops-test get deployment,service,pods
kubectl -n autoops-test get servicemonitor,prometheusrule
kubectl -n autoops-test get events --sort-by=.lastTimestamp
```

正常情况下 `autoops-test-server` Deployment 至少有一个 Ready Pod。

## 检查监控组件

```bash
kubectl -n monitoring get pods
kubectl -n monitoring get svc monitoring-kube-prometheus-prometheus
kubectl -n monitoring get prometheus,alertmanager
```

Prometheus、Alertmanager、Operator、kube-state-metrics 和 node-exporter 应处于 Running。

## 获取 Prometheus 地址

```bash
minikube ip
kubectl -n monitoring get svc monitoring-kube-prometheus-prometheus
```

推荐配置：

```text
PROMETHEUS_URL=http://<minikube-ip>:30900
```

若宿主机无法访问 Minikube IP，使用端口转发：

```bash
kubectl -n monitoring port-forward svc/monitoring-kube-prometheus-prometheus 9090:9090
```

并配置：

```text
PROMETHEUS_URL=http://127.0.0.1:9090
```

## 验证 AutoOps 后端

```bash
curl http://127.0.0.1:8819/ping
curl http://127.0.0.1:8819/background/status
curl http://127.0.0.1:8819/cluster/summary
```

`background/status.last_error` 应为空。

## 恢复测试 Deployment

```bash
kubectl -n autoops-test scale deployment autoops-test-server --replicas=1
kubectl -n autoops-test rollout undo deployment/autoops-test-server
kubectl -n autoops-test rollout status deployment/autoops-test-server
```

仅在确认故障注入已结束后执行恢复操作。

## 停止环境

```bash
minikube stop
```

停止前应确保没有正在执行的故障注入或后台自动修复任务。

