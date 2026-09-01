# AutoOps

智能运维平台 - 以证据驱动的 Agent Runtime 连接 Prometheus、Minikube、维护文档和受控 Kubernetes 操作。

## 项目简介

AutoOps 面向本地 Minikube 测试集群，提供三个相互隔离的业务 Agent：

- **RAG (检索增强生成)** - 基于向量数据库的知识检索，将内部运维文档、告警处理手册转化为可检索的知识库，为 Agent 提供领域知识支撑

- **Incident Coordinator/Workers** - 故障排查协调器派发 Kubernetes、Prometheus、日志和维护文档专家，汇总根因和恢复方案

- **Background Remediation** - 后台以 Plan-Execute-Replan 处理低风险告警，失败或严重故障自动接力 Incident

三类 Agent 共享统一 Runtime、工具注册和审计时间线，但 Prompt、会话、权限和上下文完全隔离。任何 Kubernetes 写操作都必须经过 Proposal、Policy、用户确认、Executor 和 Verifier。

## 功能特性

- **故障排查** - Incident Coordinator 与 Kubernetes、Prometheus、日志、维护文档专家协作，并与人工多轮对话
- **后台自动处置** - Supervisor 监听告警，Remediation Agent 以 Plan-Execute-Replan 低风险修复，失败自动创建 Incident
- **日常运维工单** - Work Order Agent 生成部署计划、YAML、Shell、风险和回滚步骤，不自动执行
- **维护文档 / RAG** - Markdown 文档自动解析、向量化与类型化检索

## 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                         AutoOps                             │
├─────────────────────────────────────────────────────────────┤
│  API Layer (Gin)                                            │
│  ├── /upload    - 文件上传 & 知识库索引                      │
│││  └── /incidents - 故障排查与 Agent 交互                       │
├─────────────────────────────────────────────────────────────┤
│  Agent Runtime (CloudWeGo Eino)                             │
│  ├── Incident Coordinator + read-only Specialists            │
│  ├── Background Remediation Plan-Execute-Replan              │
│  └── Work Order Agent                                         │
├─────────────────────────────────────────────────────────────┤
│  Tools                                                      │
│  ├── Observation / Knowledge / Proposal tools                 │
│  └── Action Broker (Policy → Executor → Verifier)             │
├─────────────────────────────────────────────────────────────┤
│  Storage                                                    │
│  ├── Qdrant      - 向量数据库                                │
│  └── Embedding API - OpenAI-compatible（Ollama 可选）        │
└─────────────────────────────────────────────────────────────┘
```

## 快速开始

### 前置依赖

- Go 1.25+
- OpenAI-compatible Embedding API（默认）或 [Ollama](https://ollama.ai/)（可选）
- [Qdrant](https://qdrant.tech/) (向量数据库)
- OpenAI 兼容 API (LLM 服务)
- Prometheus (可选，用于告警分析)
- Minikube + Helm（用于模拟 Kubernetes 运维场景）

### 安装步骤

1. **克隆项目**

```bash
git clone <repository-url>
cd AutoOps
```

2. **启动依赖服务**

```bash
# Embedding 默认调用外部 OpenAI-compatible API；如需本地模式可启动 Ollama
ollama pull nomic-embed-text

# 启动 Qdrant
docker run -p 6333:6333 -p 6334:6334 qdrant/qdrant
```

3. **配置文件**

准备环境变量文件并修改：

```bash
cp backend/.env.example backend/.env
```

编辑 `backend/.env`，填入 API Key 和服务地址。

4. **运行服务**

```bash
cd backend
go mod tidy
AUTOOPS_ENV_FILE=.env go run ./cmd
```

服务将在 `http://localhost:8819` 启动。

### 启动 AutoOps 前端

```bash
cd frontend
npm install
npm run dev
```

前端默认运行在 `http://localhost:5173`，Vite 会将 `/api` 请求代理到后端 `http://localhost:8819`。工作台包含后台任务、故障排查、日常工单、Minikube 集群观测和维护文档页面。

### 推荐：使用 Minikube 模拟运维集群

AutoOps 在宿主机运行，Minikube 只承载测试业务服务和监控组件。Qdrant、Embedding API/Ollama 等依赖独立运行。

```bash
minikube start
eval $(minikube docker-env)
docker build -t autoops-prometheus-test-server:local -f tests/prometheus-test-server/Dockerfile tests/prometheus-test-server
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install monitoring prometheus-community/kube-prometheus-stack -n monitoring --create-namespace \
  --set prometheus.service.type=NodePort --set prometheus.service.nodePort=30900
helm upgrade --install autoops-test ./deploy/helm/autoops-test
```

配置统一使用 `backend/.env`（模板为 `.env.example`），配置 Minikube Prometheus 的 NodePort 地址，并配置本机 kubeconfig。AutoOps 通过 Kubernetes API 查询 Pod、Deployment、Events 和 Pod 日志，不执行写操作。

## 配置说明

### backend/.env

完整配置示例见 `backend/.env.example`。

| 配置项 | 说明 |
|--------|------|
| `server.host/port` | HTTP 服务地址 |
| `embedding.*` | Embedding Provider、模型、API 地址、Key 和维度配置 |
| `qdrant.*` | Qdrant 向量数据库配置 |
| `openai.*` | LLM API 配置 (兼容 OpenAI 格式) |
| `prometheus.url` | Prometheus 服务地址 |
| `kubernetes.*` | Minikube kubeconfig、context、namespace 和启用开关 |

也可以使用环境变量覆盖 Kubernetes 连接：`AUTOOPS_K8S_KUBECONFIG`、`AUTOOPS_K8S_CONTEXT`、`AUTOOPS_K8S_NAMESPACE`、`AUTOOPS_K8S_ENABLED`。AutoOps 只需要以下只读权限：`get/list/watch` pods、deployments、events，以及 `get` pods/log。

## API 文档

### 健康检查

```http
GET /ping
```

**响应:**
```json
{"message": "pong"}
```

### 文件上传 (知识库索引)

```http
POST /upload
Content-Type: multipart/form-data

file: <markdown-file>
```

将 Markdown 文档上传到知识库，自动解析并建立向量索引。

**响应:**
```json
{"message": "上传成功"}
```

### 对话

```http
POST /incidents/:id/chat
Content-Type: application/json

{
  "question": "如何处理服务下线告警？",
  "id": "session-id"
}
```

**响应:**
```json
{
  "message": "根据知识库，服务下线可能因为服务 panic..."
}
```

### 流式对话

```http
POST /incidents/:id/chatStream
Content-Type: application/json

{
  "question": "当前有哪些告警？",
  "id": "session-id"
}
```

**响应:** Server-Sent Events (SSE) 流式数据

```
data: 根据 Prometheus 查询结果...

data: [DONE]
```

### 运维计划分析

```http
POST /work-orders/:id/chatStream
```

自动获取 Prometheus 活跃告警，检索内部知识库，生成分析报告。

**响应:**
```json
{
  "message": "分析结果...",
  "details": ["步骤1...", "步骤2..."]
}
```

## 项目结构

```
AutoOps/
├── backend/                    # Go 后端模块
│   ├── cmd/
│   └── main.go                 # 程序入口
│   └── .env.example            # 环境变量模板
│   ├── docs/                   # 知识库文档目录
│   ├── internal/
│   ├── handler/                # HTTP 处理器
│   │   ├── chat.go
│   │   ├── file.go
│   │   └── plan.go
│   ├── router/                 # 路由配置
│   │   └── init.go
│   ├── repo/                   # 数据访问层
│   │   └── qrdant/            # Qdrant 向量库
│   │       ├── init/
│   │       ├── indexer/
│   │       └── retriever/
│   └── server/                 # 业务逻辑层
│       ├── ai/
│       │   ├── agent/{incident,workorder,background,knowledge_index}
│       │   ├── runtime/        # Persona、工具权限和运行时
│       │   ├── embeder/        # Embedding 服务
│       │   └── tools/           # 观测、RAG、生成和动作提案工具
│       ├── background/          # Supervisor + Remediation 工作流
│       ├── incident/action/     # Proposal/Policy/Executor/Verifier
│       ├── operations/          # SQLite 持久化协调与锁
│       └── cases/               # Incident、工单、消息和时间线
│   ├── pkg/                    # 配置、日志和通用工具
│   ├── scripts/                # 后端辅助脚本
│   ├── tests/prometheus-test-server/ # Prometheus 测试服务器
│   ├── go.mod
│   └── go.sum
├── deploy/helm/autoops-test/   # Minikube 测试 Helm Chart
└── frontend/                   # Vite 前端
```

## 核心组件

### 1. Agent Runtime 与 Persona 注册

`backend/agents/*.md` 使用 YAML Front Matter 描述 Agent 的能力、工具白名单、禁用工具、权限模式和最大轮数。Runtime 统一负责 Persona 加载、超时、取消、流式输出和执行边界。

### 2. Incident Coordinator / Specialists

Coordinator 将故障拆分给 Kubernetes、Observability、Logs 和 Runbook 专家。专家只返回结构化证据，Coordinator 汇总事实、假设、风险和下一步；写操作只能先创建待确认 Proposal。

### 3. Background Remediation

Supervisor 只监听 Alertmanager/Prometheus、去重并调度任务。Remediation Agent 负责 Plan → Policy → Execute → Observe → Replan，最多三轮；仅允许策略批准的受控 Pod 自动修复，其他情况可靠接力 Incident。

### 4. Action Broker

Incident 动作经过 Proposal、Policy、ResourceVersion 前置校验、用户确认、Executor 和 Verifier。支持删除受控 Pod、滚动重启、回滚和有限扩缩容，不通过 shell 调用 kubectl。

### 3. RAG 工具

基于 Qdrant 向量数据库的检索工具：
- 文档向量化存储
- 语义相似度检索
- 知识库自动更新

### 4. Prometheus 工具

告警查询工具：
- 获取所有活跃告警
- 告警信息去重
- 持续时间计算

## 开发指南

### 添加新工具

在 `internal/server/ai/tools/` 目录创建新工具：

```go
package tools

import (
    "context"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/components/tool/utils"
)

type MyToolInput struct {
    Query string `json:"query" jsonschema:"description=查询参数"`
}

func NewMyTool() (tool.InvokableTool, error) {
    return utils.InferTool("my_tool",
        "工具描述",
        func(ctx context.Context, input MyToolInput) (string, error) {
            // 实现逻辑
            return "result", nil
        })
}
```

### 扩展知识库

将 Markdown 文档放入 `backend/docs/` 目录，通过 `/upload` 接口上传。

文档格式建议：
- 使用一级标题作为文档标题
- 标题会获得更高的检索权重
- 保持文档结构清晰

## 技术栈

- **框架**: [Gin](https://gin-gonic.com/) + [CloudWeGo Eino](https://github.com/cloudwego/eino)
- **向量数据库**: [Qdrant](https://qdrant.tech/)
- **Embedding**: OpenAI-compatible `/v1/embeddings`（Ollama 可选）
- **LLM**: OpenAI 兼容 API
- **监控**: [Prometheus](https://prometheus.io/)

## License

MIT
