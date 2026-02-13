# MOI 智能日报 — 技术能力需求与 SDK 适配报告

> 最后更新：2026-02-12（含 fin-data-agent 参考项目分析）

---

## 一、产品功能与所需技术能力

### 1.1 AI 助手 — 汇报今日工作

用户随时发送工作片段，AI 实时总结并生成结构化日报，支持一天多次汇报合并。

| 所需能力 | 说明 |
|----------|------|
| LLM 文本理解 | 判断用户输入是否为有效工作内容，过滤闲聊 |
| LLM 摘要生成 | 将口语化工作描述提炼为结构化要点 |
| LLM 风险检测 | 从工作内容中识别阻塞、延期、风险项 |
| 流式输出 | 摘要逐字生成，提升交互体验 |
| 数据库写入 | 每次汇报追加存储，同一天摘要合并 |

### 1.2 AI 助手 — 补填往期日报

选择历史日期，补录工作内容，逻辑同汇报但指定日期。

| 所需能力 | 说明 |
|----------|------|
| 同上全部 | 与汇报相同的 LLM + 存储能力 |
| 日期指定写入 | 数据库按指定日期存储而非当天 |

### 1.3 AI 助手 — 查询团队动态

用自然语言查询团队成员的工作进展，如"张伟这周做了什么"。

| 所需能力 | 说明 |
|----------|------|
| 数据库查询 | 按时间范围、成员检索日报数据 |
| LLM 问答 | 基于检索到的数据，用自然语言回答用户问题 |
| 流式输出 | 回答逐字生成 |
| **理想能力：NL2SQL** | 用户自然语言 → 自动生成 SQL → 查询数据库，无需手写查询逻辑 |

### 1.4 AI 助手 — 生成周报总结

基于本周日报数据，自动生成 Markdown 格式周报并提供下载。

| 所需能力 | 说明 |
|----------|------|
| 数据库查询 | 获取指定用户本周所有日报 |
| LLM 长文本生成 | 生成结构化 Markdown 周报 |
| 流式输出 | 周报逐字生成 |
| 文件生成与下载 | 将生成内容写入 .md 文件，提供下载链接 |

### 1.5 历史数据导入

上传 xlsx/csv 文件，批量导入团队历史日报。

| 所需能力 | 说明 |
|----------|------|
| 文件解析 | 解析 xlsx/csv 提取日报数据 |
| 批量写入 | 批量插入数据库 |
| **理想能力：文件存储** | 将原始文件存储到平台，便于追溯 |

### 1.6 用户认证

简单登录，区分团队成员身份。

| 所需能力 | 说明 |
|----------|------|
| 数据库用户表 | 存储账号密码（bcrypt） |
| JWT 认证 | 无状态 token 鉴权 |

---

## 二、MOI SDK 完整能力清单

SDK 分两层：`RawClient`（底层，一对一映射 API）和 `SDKClient`（高层，组合多个 API 完成一件事）。

### 2.1 LLM 对话代理（LLM Proxy）

**说白了**：MOI 平台代理了大模型的调用，你不用自己去对接通义千问/GPT，通过 MOI 统一调。

| 方法 | 干什么的 |
|------|----------|
| `CreateLLMSession` | 创建一个对话会话（类似 ChatGPT 的一个聊天窗口） |
| `ListLLMSessions` | 列出所有会话 |
| `GetLLMSession` | 获取某个会话详情 |
| `UpdateLLMSession` | 改会话标题、标签 |
| `DeleteLLMSession` | 删除会话 |
| `ListLLMSessionMessages` | 获取会话里的消息列表 |
| `GetLLMSessionLatestCompletedMessage` | 获取最新一条成功的消息 |
| `GetLLMSessionLatestMessage` | 获取最新一条消息（不管成功失败） |
| `ModifyLLMSessionMessageResponse` | 修改某条消息的回复内容 |
| `AppendLLMSessionMessageModifiedResponse` | 往某条消息的回复后面追加内容 |
| `CreateLLMChatMessage` | 创建一条聊天消息记录 |
| `GetLLMChatMessage` | 获取某条消息详情 |
| `UpdateLLMChatMessage` | 更新消息状态/内容 |
| `DeleteLLMChatMessage` | 删除消息 |
| `UpdateLLMChatMessageTags` | 给消息打标签 |
| `DeleteLLMChatMessageTag` | 删除消息标签 |

**注意**：这些都是"消息记录管理"，不是"调大模型"。SDK **没有**直接调大模型生成回复的方法。实际调大模型要自己 HTTP 请求 `/llm-proxy/v1/chat/completions`。

### 2.2 数据问答（Data Asking）

**说白了**：你用自然语言问问题，平台自动查数据库回答你。内部会走 NL2SQL 生成 SQL、执行、再用 AI 总结。

| 方法 | 干什么的 |
|------|----------|
| `AnalyzeDataStream` | 发起数据问答，返回 SSE 流式事件（推理过程 + 最终答案） |
| `CancelAnalyze` | 取消正在进行的问答 |

### 2.3 NL2SQL（自然语言转 SQL）

**说白了**：你给它一句话或一条 SQL，它帮你在 MatrixOne 上执行，或者帮你看表结构。

| 方法 | 干什么的 |
|------|----------|
| `RunNL2SQL` | 执行 SQL 操作，支持多种类型 |
| — `show_databases` | 列出所有数据库 |
| — `show_table` | 列出某个库的所有表 |
| — `desc_table` | 查看表结构（字段、类型、注释） |
| — `show_create_table` | 查看建表语句 |
| — `select_3` | 预览表的前 3 行数据 |
| — `run_sql` | 执行任意 SELECT 语句 |
| `SDKClient.RunSQL` | 高层封装，直接传 SQL 字符串执行 |

### 2.4 NL2SQL 知识库

**说白了**：给 NL2SQL 喂业务知识，让它更懂你的表。比如告诉它"用户"就是 members 表，"日报"就是 daily_entries 表。

| 方法 | 干什么的 |
|------|----------|
| `CreateKnowledge` | 添加一条知识（如"日报=daily_entries 表"） |
| `UpdateKnowledge` | 更新知识 |
| `DeleteKnowledge` | 删除知识 |
| `GetKnowledge` | 获取某条知识 |
| `ListKnowledge` | 列出所有知识 |
| `SearchKnowledge` | 搜索相关知识 |

### 2.5 Catalog 管理（数据资产目录）

**说白了**：MOI 平台有个"数据资产目录"，你要把数据库/表注册进去，平台才知道你有什么数据，Data Asking 才能查。

| 方法 | 干什么的 |
|------|----------|
| `CreateCatalog` | 创建一个目录分类（如"智能日报"） |
| `DeleteCatalog` | 删除目录（连带删里面所有库和表） |
| `UpdateCatalog` | 改目录名称/描述 |
| `GetCatalog` | 获取目录详情 |
| `ListCatalogs` | 列出所有目录 |
| `GetCatalogTree` | 获取完整的目录树结构 |
| `GetCatalogRefList` | 获取目录的引用列表 |
| `DownloadTableData` | 下载表数据为 CSV 文件流 |

### 2.6 Database 管理

**说白了**：在 Catalog 下面注册数据库。

| 方法 | 干什么的 |
|------|----------|
| `CreateDatabase` | 在某个 Catalog 下创建/注册数据库 |
| `DeleteDatabase` | 删除数据库 |
| `UpdateDatabase` | 更新描述 |
| `GetDatabase` | 获取详情 |
| `ListDatabases` | 列出某 Catalog 下所有数据库 |
| `GetDatabaseChildren` | 获取库下面的表和 Volume |
| `GetDatabaseRefList` | 获取引用列表 |

### 2.7 Table 管理

**说白了**：在 Database 下面注册表，定义字段结构。

| 方法 | 干什么的 |
|------|----------|
| `CreateTable` | 创建表（定义字段名、类型、注释） |
| `DeleteTable` | 删除表 |
| `GetTable` | 获取表详情（字段、行数、大小） |
| `GetMultiTable` | 批量获取多张表信息 |
| `GetTableOverview` | 获取所有表的概览 |
| `CheckTableExists` | 检查表是否存在 |
| `PreviewTable` | 预览表的前 N 行数据 |
| `GetTableData` | 分页获取表数据 |
| `LoadTable` | 从文件加载数据到表 |
| `TruncateTable` | 清空表数据 |
| `GetTableDownloadLink` | 获取表数据下载链接 |
| `GetTableFullPath` | 获取表的完整路径 |
| `GetTableRefList` | 获取表的引用列表 |

### 2.8 Volume 管理（数据卷/文件存储）

**说白了**：类似一个文件夹，可以往里面上传文件（CSV、PDF 等），平台会帮你解析和索引。

| 方法 | 干什么的 |
|------|----------|
| `CreateVolume` | 创建一个数据卷 |
| `DeleteVolume` | 删除数据卷 |
| `UpdateVolume` | 更新描述 |
| `GetVolume` | 获取详情 |
| `GetVolumeRefList` | 获取引用列表 |
| `GetVolumeFullPath` | 获取完整路径 |
| `AddVolumeWorkflowRef` | 关联工作流 |
| `RemoveVolumeWorkflowRef` | 取消关联工作流 |

### 2.9 File 管理

**说白了**：Volume 里面的具体文件操作。

| 方法 | 干什么的 |
|------|----------|
| `UploadFile` | 上传文件到 Volume |
| `CreateFile` | 创建文件记录 |
| `UpdateFile` | 更新文件信息 |
| `DeleteFile` | 删除文件 |
| `GetFile` | 获取文件详情 |
| `ListFiles` | 列出 Volume 下所有文件 |
| `GetFileDownloadLink` | 获取下载链接 |
| `GetFilePreviewLink` | 获取预览链接 |
| `GetFilePreviewStream` | 获取预览流 |
| `DeleteFileRef` | 删除文件引用 |

### 2.10 Connector（文件上传/导入）

**说白了**：把本地文件上传到平台，可以导入到表或 Volume。

| 方法 | 干什么的 |
|------|----------|
| `UploadLocalFile` | 上传单个本地文件 |
| `UploadLocalFiles` | 批量上传本地文件 |
| `UploadLocalFileFromPath` | 通过文件路径上传 |
| `UploadConnectorFile` | 上传文件到 Connector |
| `DownloadConnectorFile` | 下载 Connector 文件 |
| `DeleteConnectorFile` | 删除 Connector 文件 |
| `FilePreview` | 预览上传的文件内容 |
| `SDKClient.ImportLocalFileToTable` | **高层封装**：本地文件 → 上传 → 导入到指定表 |
| `SDKClient.ImportLocalFileToVolume` | **高层封装**：本地文件 → 上传 → 导入到 Volume |
| `SDKClient.ImportLocalFilesToVolume` | **高层封装**：批量文件 → Volume |

### 2.11 GenAI 工作流（文档处理）

**说白了**：上传文档（PDF 等），平台自动解析、提取、索引，可以用于 RAG。

| 方法 | 干什么的 |
|------|----------|
| `CreateGenAIPipeline` | 创建文档处理流水线 |
| `GetGenAIJob` | 查看处理任务状态 |
| `DownloadGenAIResult` | 下载处理结果 |
| `SDKClient.CreateDocumentProcessingWorkflow` | **高层封装**：创建文档处理工作流 |
| `SDKClient.GetWorkflowJob` | 获取工作流任务 |
| `SDKClient.WaitForWorkflowJob` | 等待工作流完成 |

### 2.12 用户管理

| 方法 | 干什么的 |
|------|----------|
| `CreateUser / DeleteUser / ListUsers` | 用户 CRUD |
| `GetUserDetail / UpdateUserInfo` | 用户详情/更新 |
| `UpdateUserPassword / UpdateUserStatus` | 改密码/改状态 |
| `UpdateUserRoles` | 分配角色 |
| `GetMyAPIKey / RefreshMyAPIKey` | 管理自己的 API Key |
| `GetMyInfo / UpdateMyInfo / UpdateMyPassword` | 管理自己的信息 |

### 2.13 角色权限

| 方法 | 干什么的 |
|------|----------|
| `CreateRole / DeleteRole / GetRole / ListRoles` | 角色 CRUD |
| `UpdateRoleInfo / UpdateRoleStatus` | 更新角色 |
| `UpdateRoleCodeList` | 设置角色权限码 |
| `UpdateRolesByObject` | 按对象更新角色 |
| `ListRolesByCategoryAndObject` | 按分类查角色 |
| `SDKClient.CreateTableRole / UpdateTableRole` | **高层封装**：创建/更新表级别权限角色 |

### 2.14 其他

| 方法 | 干什么的 |
|------|----------|
| `HealthCheck` | 检查平台是否正常 |
| `GetTask` | 查看异步任务状态 |
| `ListUserLogs / ListRoleLogs` | 查看操作日志 |
| `ListObjectsByCategory` | 按分类列出权限对象 |
| `SDKClient.FindFilesByName` | 按文件名搜索文件 |

---

## 三、我们项目的使用情况

### 3.1 已使用 ✅

| MOI 能力 | 怎么用的 | 状态 |
|----------|----------|------|
| LLM Chat Completions | 手动 HTTP 调 `/llm-proxy/v1/chat/completions`（SDK 没封装这个） | ✅ 核心能力 |
| LLM Streaming | 同上 `stream: true` | ✅ 所有 AI 输出 |
| MatrixOne 数据库 | `mysql.NewConnector` 直连 | ✅ 稳定 |
| Catalog 注册 | `CreateCatalog / CreateDatabase / CreateTable` | ✅ 注册成功 |

### 3.2 试了但有问题 ⚠️

| MOI 能力 | 问题 |
|----------|------|
| Data Asking (`AnalyzeDataStream`) | Catalog 注册后能看到 schema，但查不到通过直连写入的数据。原因是 NL2SQL 执行 SQL 的用户上下文和我们直连 MO 的账号不同 |
| NL2SQL `run_sql` | `show_databases`/`desc_table` 正常，但 `SELECT` 返回空行；且只支持 SELECT，不能 INSERT |

### 3.3 没用但可以用的

| MOI 能力 | 可以干什么 |
|----------|-----------|
| LLM Session 管理 | 保存对话历史，替代我们内存中的 pending 状态 |
| NL2SQL Knowledge | 教 NL2SQL 认识我们的表，提升查询准确率 |
| ImportLocalFileToTable | 把文件导入 Catalog 表，让 Data Asking 能查到数据 |
| Volume + File | 存储用户上传的 xlsx 和生成的周报 .md |
| GenAI 工作流 | 上传 PDF 日报自动解析提取 |
| 用户管理 | 用 MOI 的用户体系替代我们自建的 members 表 |

---

## 四、fin-data-agent 参考项目分析

> 路径：`/Users/wkkuai/matrix/fin-data-agent`
> 这是 MOI 团队自己的项目，金融数据智能体，用于验证 SDK 在真实场景中的使用方式。

### 4.1 项目架构

Go 后端，`pkg/service` + `pkg/handlers` + `pkg/agent` 分层，用 Gin + GORM，依赖 `moi-go-sdk`。

### 4.2 两套 AI 调用模式

**模式一：Data Asking（MOI SDK）— 主要的"问数"功能**

```
用户提问 → CreateLLMSession → AnalyzeDataStream → SSE 流式返回
```

- 调 SDK 的 `RawClient.AnalyzeDataStream()`，走 `/byoa/api/v1/data_asking/analyze`
- 平台内部自动完成：NL2SQL 生成 SQL → 执行 → AI 总结答案
- 需要完整的 `DataAnalysisConfig`（数据源类型、表列表、权限过滤、MCP 端点等）
- 用 `WithSpecialUser(apiKey)` 实现多用户隔离（每个用户有自己的 MOI API Key）
- 用 `WithDirectLLMProxy()` 选项控制是否直连 LLM Proxy
- Session 管理：创建/列表/删除/置顶，配置持久化到 `user_session_config` 表
- 消息历史由 MOI 平台管理，支持同步到 AI Portal

**模式二：langchaingo + 阿里云 DashScope — "材料搜索"功能**

```
用户提问 → RewriteAgent(改写问题) → QueryAgent(NL→SQL) → 直连 MO 执行 SQL
```

- 用 `langchaingo/llms/openai` 第三方库，直接调阿里云 DashScope API
- 地址：`https://dashscope.aliyuncs.com/compatible-mode/v1`，模型：`qwen2.5-14b-instruct-1m`
- **完全没走 MOI SDK，也没走 MOI 的 llm-proxy**
- 用于需要自定义 prompt 的场景：问题改写（RewriteAgent）、SQL 生成（QueryAgent）、文件类型提取
- SQL 生成后用 GORM 直连 MO 执行，不走 NL2SQL

### 4.3 关键发现

| 发现 | 说明 |
|------|------|
| SDK 没有通用 LLM 调用 | fin-data-agent 也没用 SDK 调大模型做自由对话，需要自定义 prompt 时直接用了第三方 LLM |
| Data Asking 是核心 AI 能力 | 但它是完整的 NL2SQL 管道，不是通用 chat completion，不适合日报总结/风险检测 |
| 我们的方式更"MOI 原生" | 我们调 `/llm-proxy/v1/chat/completions` 至少用的是 MOI 的 LLM Proxy；fin-data-agent 的自定义 LLM 调用完全走外部 DashScope |
| Data Asking 需要完整配置 | DataAnalysisConfig 包含数据源、表权限、MCP 端点、上下文配置等，门槛较高 |
| 多用户隔离靠 API Key | 每个用户在 MOI 平台有独立 API Key，通过 `WithSpecialUser()` 切换 |

### 4.4 对我们项目的启示

- **日报总结/风险检测**：继续用 `/llm-proxy/v1/chat/completions`，这是合理的，fin-data-agent 的替代方案是用第三方库
- **查询模式**：可以考虑改成 Data Asking，但需要先解决数据互通问题（Catalog 注册的库查不到直连写入的数据）
- **Session 管理**：fin-data-agent 大量使用 LLM Session，我们可以用来替代内存中的 pending 状态

---

## 五、SDK 缺失与改进建议

### 5.1 没有"调大模型"的方法（最大的缺失）

SDK 有 17 个 LLM 相关方法，但全是"管理对话记录"的（创建会话、存消息、改消息）。真正"发消息给大模型、拿回回复"的方法一个都没有。

我们只能自己写 HTTP 请求调 `/llm-proxy/v1/chat/completions`，SDK 的 auth header、错误处理、baseURL 拼接这些基础能力都用不上（因为都是小写未导出的）。

**建议加**：
```go
resp, err := client.ChatCompletions(ctx, &sdk.ChatRequest{
    Model:    "qwen-plus",
    Messages: []sdk.Message{{Role: "user", Content: "hello"}},
})
// 以及流式版本
stream, err := client.ChatCompletionsStream(ctx, &sdk.ChatRequest{...})
```

### 5.2 没有流式 LLM 调用

SDK 的 `AnalyzeDataStream` 已经有 SSE 流式解析能力，说明技术上没问题。但 LLM Proxy 侧没有对应的流式方法。

### 5.3 Data Asking 必须传 DataAnalysisConfig

不传 config 时 Data Asking 不知道查哪个库，会返回"未查询到相关数据"。必须指定 `DatabaseID` 和 `DbName`：

```go
config := &sdk.DataAnalysisConfig{
    DataSource: &sdk.DataSource{
        Type: "specified",
        Tables: &sdk.DataAskingTableConfig{
            Type: "all", DbName: "smart_daily", DatabaseID: &dbID,
        },
    },
    DataScope: &sdk.DataScope{Type: "all"},
}
```

这个行为没有文档说明，是踩坑后才发现的。建议：无 config 时自动查用户有权限的所有 Catalog 数据。

### 5.4 Catalog 表与用户 MO 实例的数据隔离问题

Catalog 的存储后端是 MOI 平台自己的 MO 实例，用户直连的是另一个 MO 实例（如 freetier-01）。这是两套独立的数据库服务，不是同一个实例的不同 schema。

因此：
- `CreateDatabase(name="smart_daily")` 不是注册已有库，而是在平台 MO 内部新建同名库
- 用户通过直连 MO 写入的数据，Catalog/Data Asking 完全看不到
- SDK 没有"关联已有表"或"外部表"的概念

**当前唯一的数据通道**：通过 `ImportLocalFileToTable` 以文件形式导入（内存生成 CSV → HTTP 上传 → 平台导入）。fin-data-agent 也是这么做的。

**可能的改进方向**：
- MO 发布订阅（CDC）：用户实例 publish 变更 → Catalog 实例 subscribe 自动同步，消除双写
- 联邦查询/外部表：Catalog 实例建外部表指向用户实例，Data Asking 直接查用户数据
- SDK 提供 `InsertTableRows` API：至少省掉文件上传的开销，直接 JSON 写入

### 5.5 NL2SQL 只能读不能写

`run_sql` 只支持 SELECT，INSERT/UPDATE/DELETE 都报错。如果 Catalog 表需要有数据才能被 Data Asking 查到，应该提供写入通道。

### 5.6 Catalog 表没有行级数据操作

Catalog 表只支持：
- **写入**：文件导入（`ImportLocalFileToTable`）— 只能追加
- **读取**：`PreviewTable`、`GetTableData`、`DownloadTableData`
- **删除**：`TruncateTable`（清空整张表）、`DeleteTable`（删表）

没有行级的 INSERT、UPDATE、DELETE。无法修改或删除单条记录。

**实际影响**：用户撤回一条日报，Catalog 里删不掉那条脏数据，只能 truncate 整张表再全量重导。对于需要频繁修改的业务场景，Catalog 本质上是个只读数据仓库，不适合当 OLTP 业务库用。

**建议**：提供 `DeleteTableRows(tableID, condition)` 或 `UpdateTableRows` API，支持行级操作。

### 5.7 Data Asking 无意图路由，非数据查询浪费大量时间

Data Asking 的 Agent 对所有输入都走完整推理链（decomposition → exploration → 多轮 agent_reasoning → sql_generation → sql_execution → insight），没有前置的意图分类。

**实际案例**：用户在查询模式输入"你好"，Agent 仍然跑了 5 轮推理、耗时 21 秒，最终返回"未查询到相关数据"。理想行为应该是 1-2 秒内识别出这不是数据查询，直接返回友好回复。

**日志证据**：
```
step 1: 正在分析问题...
step 2: 推理: 用户仅输入"你好"，未提出具体业务问题...根据兜底策略...
step 3: 推理: 上下文中既无 RAG 片段，也无 SQL 或分析结果...
step 4: 推理: original_blocks 中仅包含"未查询到相关数据"...
step 5: 推理: 无任何表格或数值内容...
最终回答: "未查询到相关数据"
```

Agent 在 step 2 就已经判断出"用户没提具体问题"，但仍然继续跑完剩余流程。

**建议**：在 Agent 入口增加意图路由层，非数据查询直接短路返回，避免无意义的推理开销。

### 5.8 SDK 无通用 Chat 接口，基础 AI 能力需自行绕路

SDK 唯一的 AI 能力入口是 `AnalyzeDataStream`（Data Asking），没有通用的 `ChatCompletions` 方法。开发者需要意图分类、闲聊回复、文本摘要等基础 AI 能力时，只能自行发现并调用未封装的 `llm-proxy` HTTP 接口：

```
POST {baseURL}/llm-proxy/v1/chat/completions
Header: moi-key: {apiKey}
```

这个接口在 SDK 中没有任何封装，是通过阅读 fin-data-agent 源码逆向发现的。SDK 应该提供 `ChatCompletions(messages, model, stream)` 方法，让开发者能直接使用平台的 LLM 能力。

---

## 六、待探索方向

1. **NL2SQL 生成 SQL + 自己执行**：让 MOI 生成 SQL，我们用直连 MO 执行，绕过权限问题
2. **ImportLocalFileToTable 双写**：每次写日报时同步导入 Catalog 表
3. **LLM Session 集成**：用 MOI Session 替代内存对话状态
4. **NL2SQL Knowledge 注入**：添加业务知识提升查询准确率
5. **Volume 存储**：用 MOI 存储上传文件和生成的周报
