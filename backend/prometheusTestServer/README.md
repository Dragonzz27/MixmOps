# AutoOps Prometheus 测试服务

该服务只负责产生可被 Prometheus 抓取的指标，不是 AutoOps 本身。Minikube 场景通过 `deploy/helm/autoops-test` 部署。

```bash
eval $(minikube docker-env)
docker build -t autoops-prometheus-test-server:local -f backend/prometheusTestServer/Dockerfile backend
```

场景由 `AUTOOPS_TEST_SCENARIO` 初始化，也可运行时调用 `/scenario?name=normal|degraded|alert` 切换。
