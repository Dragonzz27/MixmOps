# AutoOps Minikube 测试 Chart

该 Chart 只部署用于测试 AutoOps 的业务服务、ServiceMonitor 和 PrometheusRule，不部署 AutoOps 本身。

## 使用

先把测试服务镜像构建到 Minikube：

```bash
eval $(minikube docker-env)
docker build -t autoops-prometheus-test-server:local -f backend/prometheusTestServer/Dockerfile backend
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install monitoring prometheus-community/kube-prometheus-stack -n monitoring --create-namespace \
  --set prometheus.service.type=NodePort --set prometheus.service.nodePort=30900
helm upgrade --install autoops-test ./deploy/helm/autoops-test
```

PrometheusRule 和 ServiceMonitor 默认使用 `release: monitoring`，如 Helm release 名不同请通过 `--set monitoring.prometheusRelease=<name>` 覆盖。

```bash
kubectl -n autoops-test get pods,events
kubectl -n autoops-test port-forward svc/autoops-test-server 2112:2112
# 切换测试场景：normal、degraded 或 alert
curl 'http://127.0.0.1:2112/scenario?name=degraded'
```
