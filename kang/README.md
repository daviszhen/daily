# MOI 智能日报

基于 [MatrixOne](https://www.matrixorigin.cn/) 数据库和 [MOI 智能平台](https://www.matrixorigin.cn/moi) 构建的 AI 团队日报系统。单 Go 二进制，前后端一体化部署。

## 功能概览

### 日报提交
- 用户输入工作内容 → LLM 流式生成摘要 + 风险检测 → 用户确认 → 存库 + 同步至 MOI Catalog
- 同一人同一天重复提交自动覆盖（不产生重复数据）
- 支持补报历史日报（指定日期）

### 历史日报导入
- 上传 Markdown 日报文件 → Python 解析 → 并行 LLM 提取（semaphore=50）→ 预览确认 → 批量写入
- 两步流程：preview（AI 提取 + 人员匹配）→ confirm（批量入库 + Catalog 同步）
- 支持 500+ section 的大文件（2-3 年日报），18 秒内完成

### 数据查询
- 自然语言查询 → MOI Data Asking（NL2SQL Agent）→ 结构化结果展示
- 思考过程实时展示（可折叠），显示推理步骤和耗时
- 查询结果 Markdown 渲染（表格、列表、代码块）

### 周报生成
- 自动汇总指定时间段的日报数据 → LLM 生成 Markdown 周报 → 支持下载

### 智能意图路由
- 4 种显式模式（汇报/补报/查询/周报）+ 无模式自动识别
- 模式 = 硬约束：选了模式后验证输入是否匹配，不匹配则引导切换
- 无模式下检测意图 → 自动切换模式 + 执行，或闲聊

## 技术架构

```
┌─────────────────────────────────────────────────────┐
│                    前端 (React)                       │
│  ChatInterface · Layout · DailyFeed · Stats          │
│  react-markdown · SSE 流式 · 模式切换                  │
└──────────────────────┬──────────────────────────────┘
                       │ HTTP / SSE
┌──────────────────────▼──────────────────────────────┐
│                  Go 后端 (Gin)                        │
│                                                      │
│  handler/chat.go     意图路由 + 模式验证               │
│  handler/import.go   两步导入（preview + confirm）     │
│  service/ai.go       LLM 调用 + prompt 管理           │
│  service/catalog_sync.go  MOI Catalog 数据同步        │
│  service/daily.go    日报 CRUD                        │
│  service/session.go  会话管理                          │
│  middleware/auth.go  JWT 认证                         │
└───────┬──────────────────────┬──────────────────────┘
        │                      │
        ▼                      ▼
┌───────────────┐    ┌─────────────────────┐
│  MatrixOne DB │    │    MOI 智能平台       │
│               │    │                     │
│  members      │    │  LLM Proxy (Qwen)   │
│  daily_entries│◄──►│  Data Asking (NL2SQL)│
│  daily_summaries│  │  Catalog (数据同步)   │
│  sessions     │    │  NL2SQL Knowledge    │
│  messages     │    │                     │
└───────────────┘    └─────────────────────┘
```

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 前端 | React 18 + TypeScript + Vite | SPA，嵌入 Go 二进制 |
| UI | Tailwind CSS + Lucide Icons | 响应式，ChatGPT 风格侧边栏 |
| Markdown | react-markdown + remark-gfm | 查询结果渲染（表格、列表、代码块） |
| 后端 | Go 1.23 + Gin | 单二进制，embed 前端静态文件 |
| ORM | GORM v2 | 自动建表、批量操作 |
| 数据库 | MatrixOne | 兼容 MySQL 协议的云原生数据库 |
| AI 模型 | Qwen3-Max（通过 MOI LLM Proxy） | 摘要、风险检测、意图分类、内容提取 |
| NL2SQL | MOI Data Asking | 自然语言查询 → SQL → 结构化结果 |
| 数据同步 | MOI Catalog + SDK | 日报数据同步至 MOI 平台供 Data Asking 查询 |
| 认证 | JWT | 登录 → Token → 中间件校验 |
| 通信 | SSE (Server-Sent Events) | 流式输出 token、思考过程、模式切换事件 |

## MOI 平台集成

### LLM Proxy
通过 MOI 的 LLM Proxy 调用大模型（Qwen3-Max），用于：
- 工作内容摘要生成（流式）
- 风险项检测
- 意图分类（report / query / chat）
- 工作内容提取（多轮对话合并）
- 内容充分性检查
- 周报生成

所有 prompt 统一注入当天日期上下文（`todayContext()`），避免时间推断错误。

### Data Asking（NL2SQL）
用户自然语言提问 → MOI Data Asking Agent 自动：
1. 分析问题 → 探索表结构 → 生成 SQL → 执行 → 返回结构化结果
2. 思考过程通过 SSE 实时推送到前端展示
3. 结果渲染：≤2 列 → bullet list，>2 列 → Markdown 表格

### Catalog 数据同步
日报数据通过 MOI SDK 同步至 Catalog，供 Data Asking 查询：
- 每次日报提交/导入后增量同步
- CSV 格式上传 → 异步 Task 导入 → 轮询完成状态
- Task 轮询使用直接 HTTP 调用（绕过 SDK 解析 bug）

### NL2SQL Knowledge
启动时自动初始化 NL2SQL Knowledge 条目，包括：
- 表结构描述（members、daily_entries、daily_summaries）
- 字段语义（中文名 → 字段映射）
- 常见查询 SQL 示例
- 业务逻辑规则（日期计算、提交率统计）

## 工程化设计

### Prompt 工程
- `todayContext()` 统一注入日期上下文到所有时间敏感的 prompt
- 意图分类器区分疑问句（query）和陈述句（report）
- 模式验证：选了模式后用 ClassifyIntent（无历史）验证输入匹配性

### 对话历史管理
- 前端每条消息标记 mode（report/query/chat）
- 后端 `buildHistoryFiltered(mode)` 按模式过滤历史，避免跨模式污染
- 滑动窗口：最近 5 轮对话

### 数据一致性
- Chat 提交：同人同天 DELETE + INSERT（覆盖，不追加）
- Import 导入：同人同天同来源 DELETE + INSERT
- Catalog 同步：ConflictPolicyReplace + 异步 Task 轮询

### SSE 事件协议
| 事件 | 数据 | 用途 |
|------|------|------|
| `token` | `{token: "..."}` | 流式文本输出 |
| `result` | `{summary, risks}` | 日报摘要确认 |
| `thinking` | `{text: "..."}` | Data Asking 思考步骤 |
| `mode_switch` | `{mode: "report"}` | 自动切换前端模式 |
| `done` | `{}` | 流结束 |

## 快速开始

### 环境要求
- Go 1.23+
- Node.js 20+
- MatrixOne 数据库（或 FreeTier 账号）
- MOI 平台 API Key

### 本地开发

```bash
# 1. 编辑配置
cp server/etc/config-dev.yaml.example server/etc/config-dev.yaml
vim server/etc/config-dev.yaml  # 填入 api_key、数据库账密

# 2. 初始化数据库
mysql -h <host> -P 6001 -u <user> -p smart_daily < server/migration/init.sql

# 3. 启动开发模式（前端 :9872 热更新 + 后端 :9871）
make dev

# 4. 停止
make dev-stop
```

### 生产部署

```bash
# 编译前端 + Go 二进制，后台启动
make run

# 停止
make run-stop

# 查看日志
tail -f logs/backend.log
```

### Docker 部署

```bash
cp .env.example .env
vim .env  # 填入配置
docker compose up -d
```

## 项目结构

```
├── web/                        前端
│   ├── components/
│   │   ├── ChatInterface.tsx   聊天主界面 + Markdown 渲染
│   │   ├── Layout.tsx          侧边栏 + 导航 + 导入弹窗
│   │   ├── DailyFeed.tsx       团队动态页
│   │   └── Stats.tsx           数据洞察页
│   ├── services/
│   │   └── apiService.ts       API 调用 + SSE 解析
│   └── types.ts                TypeScript 类型定义
├── server/                     后端
│   ├── cmd/server/main.go      入口 + embed 前端
│   ├── internal/
│   │   ├── handler/
│   │   │   ├── chat.go         意图路由 + 模式验证
│   │   │   └── import.go       两步导入流程
│   │   ├── service/
│   │   │   ├── ai.go           LLM 调用 + prompt + Knowledge
│   │   │   ├── catalog_sync.go Catalog 同步 + Task 轮询
│   │   │   ├── daily.go        日报 CRUD
│   │   │   └── session.go      会话/消息持久化
│   │   ├── model/              数据模型
│   │   ├── config/             配置加载
│   │   ├── middleware/         JWT 认证
│   │   └── logger/             结构化日志
│   ├── etc/                    配置文件
│   └── migration/              SQL 初始化脚本
├── Makefile                    构建/启停命令
├── Dockerfile
└── docker-compose.yml
```

## 配置说明

配置优先级：`--config` 指定文件 > `etc/config-dev.yaml` > `/etc/smart-daily/config.yaml` > 环境变量

```yaml
server:
  port: 9871

moi:
  base_url: "https://xxx.matrixone.tech"  # MOI 平台地址
  api_key: "your-api-key"                 # MOI API Key
  catalog_id: 1                           # Catalog 目录 ID
  model: "qwen3-max"                      # 主模型
  fast_model: "qwen3-max"                 # 快速模型（意图分类等）

database:
  host: "xxx.matrixone.tech"
  port: 6001
  user: "your-user"
  password: "your-password"
  name: "smart_daily"
```
