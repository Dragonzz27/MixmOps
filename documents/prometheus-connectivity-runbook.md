---
title: AutoOps 无法访问 Prometheus 故障预案
type: incident-runbook
description: Background Supervisor 因 Prometheus 地址或网络问题无法发现告警时的排查步骤
tags:
  - prometheus
  - network
  - supervisor
  - incident
---

# AutoOps 无法访问 Prometheus 故障预案

## 典型现象

```bash
curl http://127.0.0.1:8819/background/status
```

返回的 `last_error` 包含 `connection refused`、`timeout` 或无法解析地址，且 `/background/tasks` 没有新任务。

## 检查 Prometheus

```bash
kubectl -n monitoring get pods
kubectl -n monitoring get svc monitoring-kube-prometheus-prometheus
minikube ip
```

直接测试 NodePort：

```bash
curl http://<minikube-ip>:30900/api/v1/alerts
```

## 地址选择

Minikube NodePort 不一定映射到宿主机回环地址。若 `127.0.0.1:30900` 拒绝连接，应使用：

```text
PROMETHEUS_URL=http://<minikube-ip>:30900
```

或建立端口转发：

```bash
kubectl -n monitoring port-forward svc/monitoring-kube-prometheus-prometheus 9090:9090
```

对应配置：

```text
PROMETHEUS_URL=http://127.0.0.1:9090
```

## 配置格式

`.env` 中必须使用纯文本 URL：

```text
PROMETHEUS_URL=http://192.168.49.2:30900
```

不得写成 Markdown 链接、添加方括号或圆括号。

修改配置后必须重启 AutoOps 后端，再检查：

```bash
curl http://127.0.0.1:8819/background/status
```

`last_error` 为空且 `last_poll_at` 持续更新才表示恢复。

