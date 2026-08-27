# AutoOps

智能运维值班代理系统 - 融合 RAG、ReAct 与 Plan-Execute-Replan 三大 AI 范式的智能运维平台。

## 项目简介

AutoOps 是一个面向运维场景的智能代理系统，深度融合三种 AI Agent 核心技术：

- **RAG (检索增强生成)** - 基于向量数据库的知识检索，将内部运维文档、告警处理手册转化为可检索的知识库，为 Agent 提供领域知识支撑

- **ReAct (推理+行动)** - 对话场景下的工具调用范式，Agent 通过"思考-行动-观察"循环，自主选择工具（时间查询、知识检索、告警获取）完成用户意图

- **Plan-Execute-Replan (规划-执行-重规划)** - 复杂运维任务的自主执行框架，Agent 先制定执行计划，按步骤执行，并根据执行结果动态调整后续计划，实现多步骤任务的闭环处理

三者协同工作：**RAG** 提供知识基础，**ReAct** 处理单轮工具调用，**Plan-Execute-Replan** 编排多步骤任务流程，共同实现从告警发现、知识检索到处理建议生成的全链路自动化运维分析。

## 功能特性

- **智能对话** - ReAct Agent 驱动的多轮对话，支持流式响应与工具自主调用
- **知识库管理** - Markdown 文档自动解析、向量化与索引构建
- **告警分析** - 自动获取 Prometheus 活跃告警，匹配内部处理方案
- **RAG 检索** - 语义相似度匹配，从知识库精准检索处理步骤
- **自主规划** - Plan-Execute-Replan 架构实现复杂任务的多步骤编排与动态调整

## 技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                         AutoOps                             │
├─────────────────────────────────────────────────────────────┤
│  API Layer (Gin)                                            │
│  ├── /upload    - 文件上传 & 知识库索引                      │
│  ├── /chat      - 智能对话                                   │
│  ├── /chatStream - 流式对话                                  │
│  └── /plan      - 运维计划分析                               │
├─────────────────────────────────────────────────────────────┤
│  Agent Layer (CloudWeGo Eino)                               │
│  ├── ReAct Agent     - 对话代理 (工具调用)                   │
│  └── Plan-Execute    - 运维分析代理 (多步骤任务)             │
├─────────────────────────────────────────────────────────────┤
│  Tools                                                      │
│  ├── Time Tool           - 获取当前时间                      │
│  ├── RAG Tool            - 知识库检索                        │
│  └── Prometheus Tool     - 告警查询                          │
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

准备配置目录并修改：

```bash
cp -r backend/config /tmp/autoops-config
```

编辑 `/tmp/autoops-config/base.yaml`，填入 API Key 和服务地址；按需修改 `/tmp/autoops-config/profiles/minikube.yaml`。

4. **运行服务**

```bash
cd backend
go mod tidy
AUTOOPS_CONFIG_DIR=./config AUTOOPS_PROFILE=minikube go run ./cmd
```

服务将在 `http://localhost:8819` 启动。

### 启动 AutoOps 前端

```bash
cd frontend
npm install
npm run dev
```

前端默认运行在 `http://localhost:5173`，Vite 会将 `/api` 请求代理到后端 `http://localhost:8819`。工作台包含告警中心、Minikube 集群观测、Agent 流式对话和知识库管理页面。

### 推荐：使用 Minikube 模拟运维集群

AutoOps 在宿主机运行，Minikube 只承载测试业务服务和监控组件。Qdrant、Embedding API/Ollama 等依赖独立运行。

```bash
minikube start
eval $(minikube docker-env)
docker build -t autoops-prometheus-test-server:local -f backend/prometheusTestServer/Dockerfile backend
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install monitoring prometheus-community/kube-prometheus-stack -n monitoring --create-namespace \
  --set prometheus.service.type=NodePort --set prometheus.service.nodePort=30900
helm upgrade --install autoops-test ./deploy/helm/autoops-test
```

配置目录使用 `base.yaml` + `profiles/minikube.yaml`，配置 Minikube Prometheus 的 NodePort 地址，并配置本机 kubeconfig。AutoOps 通过 Kubernetes API 查询 Pod、Deployment、Events 和 Pod 日志，不执行写操作。

## 配置说明

### backend/config/base.yaml 与 profiles/minikube.yaml

基础配置和 Minikube 环境配置分开维护，完整示例见 `backend/config/config.example.yaml`。

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
POST /chat
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
POST /chatStream
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
GET /plan
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
│   ├── config/
│   ├── base.yaml               # 应用基础配置
│   ├── config.example.yaml     # 完整配置示例
│   └── profiles/minikube.yaml  # Minikube 环境配置
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
│       │   ├── agent/          # AI Agent 实现
│       │   │   ├── chat/       # 对话 Agent
│       │   │   ├── knowledge_index/  # 知识库索引 Agent
│       │   │   └── plan_execute_replan/  # 运维 Agent
│       │   ├── embeder/        # Embedding 服务
│       │   ├── model/          # LLM 模型封装
│       │   └── tools/          # Agent 工具
│       │       ├── metrics_alerts.go  # Prometheus 工具
│       │       ├── rag.go      # RAG 检索工具
│       │       └── time.go     # 时间工具
│       ├── chatServer/         # 对话服务
│       ├── knowledge_index/    # 知识库索引服务
│       └── plan/               # 运维计划服务
│   ├── pkg/                    # 配置、日志和通用工具
│   ├── scripts/                # 后端辅助脚本
│   ├── prometheusTestServer/   # Prometheus 测试服务器镜像
│   ├── go.mod
│   └── go.sum
├── deploy/helm/autoops-test/   # Minikube 测试 Helm Chart
└── backend/config/             # 基础配置和环境 profile
```

## 核心组件

### 1. ReAct Agent (对话代理)

基于 ReAct (Reasoning + Acting) 模式的智能代理，能够：
- 自动选择合适的工具
- 多步推理和执行
- 记忆会话上下文

### 2. Plan-Execute-Replan Agent (运维代理)

多步骤任务执行框架：
- **Plan** - 根据目标生成执行计划
- **Execute** - 按计划逐步执行
- **Replan** - 根据执行结果动态调整计划

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
