# MOI 智能日报

基于 [MatrixOne](https://www.matrixorigin.cn/) 数据库和 [MOI 智能平台](https://www.matrixorigin.cn/moi) 构建的 AI 团队日报系统。单 Go 二进制，前后端一体化部署。

## 功能概览

### 日报提交
- 用户输入工作内容 → 内容提取 + 充分性检查 → LLM 流式生成摘要 + 风险检测 → 用户确认 → 存库 + 同步至 MOI Catalog
- 同一人同一天多次提交，摘要自动合并（不丢失历史内容）
- 支持补填往期日报（指定日期）

### 历史日报导入
- 上传 Markdown 日报文件 → Python 脚本解析 → 并行 LLM 提取（semaphore=50）→ 预览确认 → 批量写入
- 两步流程：preview（AI 提取 + 人员匹配）→ confirm（批量入库 + Catalog 同步）
- 支持 500+ section 的大文件（2-3 年日报），18 秒内完成
- 权限控制：管理员可导入所有人，普通用户只能导入自己的日报

### 数据查询
- 自然语言查询 → MOI Data Asking（NL2SQL Agent）→ 结构化结果展示
- 思考过程实时展示（可折叠），显示推理步骤和耗时
- 查询结果 Markdown 渲染（表格、列表、代码块）
- 查询时自动将"我"替换为用户真名，让 Agent 准确定位数据

### 周报生成
- 自然语言指定时间范围（本周/上周/最近一周等）→ LLM 解析日期 → 查库汇总 → LLM 生成 Markdown 周报 → 支持下载
- 支持查他人周报（"帮我生成彭振上周的周报"）
- 周报按日期逐日列出，不遗漏任何有日报的日期
- 风险与阻塞、下周计划仅来自日报原文，不编造

### 智能意图路由
- 4 种显式模式（汇报/补填/查询/周报）+ 无模式自动识别
- 模式 = 硬约束：选了模式后验证输入是否匹配，不匹配则引导切换
- 无模式下检测意图 → 自动切换模式 + 执行，或闲聊

### 我的日历
- 月历视图，可前后翻月，显示每天提交状态
- 中国法定节假日 + 调休上班日自动标注（数据源：apihubs.cn → jsdelivr CDN 双源 fallback）
- 工作日填报率统计（进度条 + 百分比）
- 点击已提交日期 → 查看当天日报摘要和风险项
- 点击未提交日期 → 一键跳转补填模式（日期自动选好）

### Topic 自动提取与风险看板
- 日报提交/导入时自动提取 Topic（LLM 批量提取，20条/次）
- 风险看板：近 90 天活跃 Topic，按活跃天数/参与人数排序
- Topic 管理：重命名、标记已解决/重新打开、批量合并

### 团队动态
- 按成员查看：每人每天的工作内容
- 按 Topic 查看：同一 Topic 下所有成员的工作记录
- 快捷日期筛选（本周/上周/近7天/近30天）

### 权限控制
- 管理员（is_admin）：可导入所有人日报、修改成员信息、管理团队
- 普通用户：只能导入自己的日报，成员/团队管理为只读

### 认证机制
- JWT 认证，36 小时过期（每天打开不用重新登录，隔天过期）
- 服务重启后所有 token 自动失效（secret 每次启动随机生成）
- 剩余不到 12 小时时自动续期

## 技术架构

```
┌──────────────────────────────────────────────────────────┐
│                      前端 (React)                         │
│  ChatInterface · Layout · MyCalendar · DailyFeed · Stats  │
│  react-markdown · SSE 流式 · 模式切换                       │
└──────────────────────────┬───────────────────────────────┘
                           │ HTTP / SSE
┌──────────────────────────▼───────────────────────────────┐
│                    Go 后端 (Gin)                           │
│                                                           │
│  handler/chat.go       意图路由 + 模式验证 + 周报生成       │
│  handler/import.go     两步导入（preview + confirm）       │
│  handler/calendar.go   日历 + 日报详情                     │
│  handler/feed.go       团队动态 + 风险看板 + Topic 管理     │
│  handler/auth.go       登录认证                            │
│  handler/session.go    会话 CRUD                           │
│  service/ai.go         LLM 调用 + prompt 管理              │
│  service/holiday.go    节假日数据（双源 fallback + 缓存）    │
│  service/catalog_sync.go  MOI Catalog 数据同步             │
│  middleware/jwt.go     JWT 认证 + 自动续期 + AdminOnly      │
└───────┬──────────────────────────┬───────────────────────┘
        │                          │
        ▼                          ▼
┌───────────────┐        ┌─────────────────────┐
│  MatrixOne DB │        │    MOI 智能平台       │
│               │        │                     │
│  members      │        │  LLM Proxy (Qwen)   │
│  teams        │   ◄──► │  Data Asking (NL2SQL)│
│  daily_entries│        │  Catalog (数据同步)   │
│  daily_summaries│     │  NL2SQL Knowledge    │
│  topics       │        │  Session/Message     │
│  topic_activities│     │                     │
└───────────────┘        └─────────────────────┘
                                  │
                    ┌─────────────┴─────────────┐
                    │    节假日数据源（双源）      │
                    │  apihubs.cn → jsdelivr CDN │
                    └───────────────────────────┘
```

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 前端 | React 18 + TypeScript + Vite | SPA，嵌入 Go 二进制 |
| UI | Tailwind CSS + Lucide Icons | 响应式，ChatGPT 风格侧边栏 |
| Markdown | react-markdown + remark-gfm | 查询结果渲染（表格、列表、代码块） |
| 后端 | Go 1.24 + Gin | 单二进制，embed 前端静态文件 |
| ORM | GORM v2 | 自动建表、批量操作 |
| 数据库 | MatrixOne | 兼容 MySQL 协议的云原生数据库 |
| AI 模型 | Qwen3-Max（通过 MOI LLM Proxy） | 摘要、风险检测、意图分类、内容提取、Topic 提取 |
| NL2SQL | MOI Data Asking | 自然语言查询 → SQL → 结构化结果 |
| 数据同步 | MOI Catalog + SDK | 6 张表自动同步至 MOI 平台供 Data Asking 查询 |
| 会话存储 | MOI LLM Proxy Session API | 会话和消息持久化在 MOI 平台 |
| 节假日 | apihubs.cn + jsdelivr CDN | 中国法定节假日 + 调休，双源 fallback |
| 认证 | JWT（随机 secret + 36h 过期） | 重启失效 + 自动续期 |
| 通信 | SSE (Server-Sent Events) | 流式输出 token、思考过程、元数据 |
| 测试 | Go API 测试 + chromedp 浏览器 E2E | 29 个端到端测试 |

## 快速开始

### 环境要求
- Go 1.24+
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

# 3. 初始化 MOI Catalog + 语义配置（首次部署）
cd server && go run ./cmd/catalog_init/

# 4. 启动开发模式（前端 :9872 热更新 + 后端 :9871）
make dev

# 5. 停止
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

### 测试账号

| 账号 | 密码 | 权限 |
|------|------|------|
| kuaiweikang | 123456 | 管理员 |
| pengzhen | 123456 | 管理员 |
| test | 123456 | 普通用户 |

### 端到端测试

```bash
# 需要先启动服务（make run）
cd test/e2e
go test -v -timeout 300s
```

测试覆盖 29 个用例：14 个 API 测试（成员/团队/Feed/Topic/日历/认证）+ 15 个浏览器测试（登录/日报/查询/周报/会话/日历翻页）。

## 项目结构

```
├── web/                          前端
│   ├── components/
│   │   ├── ChatInterface.tsx     聊天主界面 + 模式切换 + Markdown 渲染
│   │   ├── Layout.tsx            侧边栏 + 导航（4 tab）+ 导入弹窗
│   │   ├── MyCalendar.tsx        个人日历（月历 + 节假日 + 日报详情）
│   │   ├── DailyFeed.tsx         团队动态页（按成员/按 Topic/成员管理）
│   │   ├── Stats.tsx             数据洞察页（风险看板）
│   │   └── ConfirmModal.tsx      确认弹窗组件
│   ├── services/
│   │   └── apiService.ts         API 调用 + SSE 解析
│   ├── types.ts                  TypeScript 类型定义
│   └── App.tsx                   路由 + 视图切换 + 补填联动
├── server/                       后端
│   ├── cmd/
│   │   ├── server/main.go        入口 + embed 前端 + 启动初始化
│   │   ├── catalog_init/         独立工具：初始化 Catalog + 语义配置
│   │   └── docx_parser/main.py   Python 脚本：解析 docx 日报文件
│   ├── internal/
│   │   ├── handler/
│   │   │   ├── chat.go           意图路由 + 模式验证 + 周报生成
│   │   │   ├── import.go         两步导入（权限过滤 + 批量写入）
│   │   │   ├── calendar.go       日历 API + 日报详情
│   │   │   ├── feed.go           团队动态 + 风险看板 + Topic 管理
│   │   │   ├── auth.go           登录（JWT 签发含 is_admin）
│   │   │   ├── member.go         成员/团队 CRUD
│   │   │   ├── export.go         日报导出 xlsx
│   │   │   └── session.go        会话 CRUD 接口
│   │   ├── service/
│   │   │   ├── ai.go             LLM 调用 + prompt 管理
│   │   │   ├── holiday.go        节假日数据（apihubs.cn → jsdelivr CDN）
│   │   │   ├── catalog_sync.go   Catalog 同步（6 张表 + 语义配置）
│   │   │   ├── import.go         导入逻辑（提取 + 入库 + Topic 提取）
│   │   │   ├── auth.go           登录验证（bcrypt）
│   │   │   ├── daily.go          日报 CRUD
│   │   │   └── session.go        MOI LLM Proxy 会话/消息 API
│   │   ├── repository/
│   │   │   ├── member.go         成员数据访问
│   │   │   ├── daily.go          日报数据访问（含 SubmittedDates）
│   │   │   └── topic.go          Topic 数据访问（含风险查询）
│   │   ├── model/                数据模型（Member/DailyEntry/Topic 等）
│   │   ├── config/               配置加载
│   │   ├── middleware/           JWT 认证 + AdminOnly + 自动续期
│   │   └── logger/               结构化日志
│   ├── etc/                      配置文件
│   └── migration/                SQL 初始化脚本
├── test/e2e/                     端到端测试
│   ├── api_test.go               API 测试（14 个）
│   └── e2e_test.go               浏览器测试（15 个，chromedp）
├── docs/
│   └── technical-solutions.md    核心技术方案文档
├── exports/                      周报导出文件（make clean 会清除）
├── Makefile                      构建/启停命令
├── Dockerfile
└── docker-compose.yml
```

## API 路由

### 公开接口
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/login | 登录（返回 JWT + 用户信息含 is_admin） |

### 认证接口（需 JWT）
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/chat | 日报提交（含确认） |
| POST | /api/chat/stream | 流式对话（SSE） |
| GET | /api/files/:name | 下载周报文件 |
| POST | /api/sessions | 创建会话 |
| GET | /api/sessions | 会话列表 |
| DELETE | /api/sessions/:id | 删除会话 |
| GET | /api/sessions/:id/messages | 会话消息 |
| POST | /api/import/preview | 导入预览（非 admin 自动过滤） |
| POST | /api/import/confirm | 导入确认（非 admin 自动过滤） |
| GET | /api/members | 成员列表 |
| GET | /api/teams | 团队列表 |
| GET | /api/feed/by-member | 按成员查看动态 |
| GET | /api/feed/by-topic | 按 Topic 查看动态 |
| GET | /api/insights | 风险看板（近 90 天） |
| GET | /api/topics/all | Topic 列表 |
| PUT | /api/topics/:id | 更新 Topic |
| PUT | /api/topics/:id/resolve | 标记已解决 |
| PUT | /api/topics/:id/reopen | 重新打开 |
| POST | /api/topics/merge | 合并 Topic |
| GET | /api/export/daily | 导出日报 xlsx |
| GET | /api/calendar | 月历数据（含节假日 + 提交状态） |
| GET | /api/calendar/day | 单日日报详情 |

### 管理员接口（需 JWT + is_admin）
| 方法 | 路径 | 说明 |
|------|------|------|
| PUT | /api/members/:id | 修改成员信息 |
| DELETE | /api/members/:id | 删除成员 |
| POST | /api/teams | 创建团队 |

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
  fast_model: "qwen3-max"                 # 快速模型（意图分类、日期解析等）

database:
  host: "xxx.matrixone.tech"
  port: 6001
  user: "your-user"
  password: "your-password"
  name: "smart_daily"
```
