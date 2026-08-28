---
title: Prometheus 告警查询与验证手册
type: operations-manual
description: 查询 Prometheus 告警、验证抓取目标和确认 AutoOps 后台监听状态的标准步骤
tags:
  - prometheus
  - alerts
  - monitoring
---

# Prometheus 告警查询与验证手册

## 查询当前告警

```bash
curl -s http://<minikube-ip>:30900/api/v1/alerts
```

重点检查：

- `status` 是否为 `success`；
- 告警 `state` 是否为 `firing`；
- `labels.alertname`、`labels.severity` 和 `labels.namespace`；
- `annotations.description`；
- 告警激活时间。

## 查询抓取目标

在 Prometheus UI 的 Targets 页面确认 `autoops-test-server` 为 UP，或调用：

```bash
curl -s http://<minikube-ip>:30900/api/v1/targets
```

若目标不存在，检查：

```bash
kubectl -n autoops-test get servicemonitor autoops-test-server -o yaml
kubectl -n autoops-test get svc autoops-test-server -o yaml
kubectl -n autoops-test get endpoints autoops-test-server
```

ServiceMonitor 的 `release` 标签必须与 Prometheus 的 selector 一致，当前为 `monitoring`。

## 验证测试指标

```bash
kubectl -n autoops-test port-forward svc/autoops-test-server 2112:2112
curl http://127.0.0.1:2112/metrics
```

触发高错误率和高延迟：

```bash
./tests/failure-scenarios/high-error-latency.sh
```

恢复：

```bash
./tests/failure-scenarios/recover-normal.sh
```

## 验证 AutoOps 后台监听

```bash
curl -s http://127.0.0.1:8819/background/status
curl -s http://127.0.0.1:8819/background/tasks
curl -s http://127.0.0.1:8819/incidents
```

如果 Prometheus 已 firing，但没有后台任务：

1. 检查 `background/status.last_error`；
2. 检查 `PROMETHEUS_URL` 是否能从 AutoOps 宿主机访问；
3. 等待一个 Supervisor 轮询周期；
4. 检查后端日志；
5. 确认告警 fingerprint 没有被活动任务去重。

