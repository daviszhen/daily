# MOI 智能日报 — 核心技术方案

本文档聚焦 MatrixOne / MOI 平台集成和大模型相关的技术方案。

## 一、MOI 平台集成

### 1.1 LLM Proxy — 多步 AI Pipeline

日报提交不是一次 LLM 调用，而是一条多步 pipeline，每步有明确职责：

```
用户输入 → ExtractWorkContent（从多轮对话合并完整内容）
         → ValidateWorkContent（是否为有效工作内容）
         → AssessCompleteness（充分性检查，不足则追问）
         → StreamSummarize（流式生成摘要）
         → DetectRisks（风险项检测）
         → 用户确认 → 入库 + Catalog 同步
```

每步用独立 prompt，职责单一，避免一个大 prompt 做所有事导致质量下降。流式输出通过 SSE 推送 `token` 事件，前端逐字渲染。

**双模型策略**：系统配置两个模型 — `model`（主模型，Qwen3-Max）用于摘要生成、周报生成等需要高质量输出的场景；`fastModel`（快速模型）用于意图分类、日期解析、内容验证、Topic 提取等结构化判断场景。快速模型响应更快、成本更低，且这类任务不需要最强的生成能力。

### 1.2 Data Asking — 自然语言查数据

用户用自然语言提问，MOI Data Asking Agent 自动完成 NL2SQL 全流程：分析问题 → 探索表结构 → 生成 SQL → 执行 → 返回结果。

**关键优化：**

- **"我"→ 真名替换**：查询前 `injectUserIdentity()` 自动将"我"替换为当前登录用户的真名，并追加"（注：提问者是XXX）"，让 Agent 能准确定位数据，而不是生成 `WHERE name = '我'`。
- **思考过程透传**：Agent 的推理步骤（decomposition → exploration → agent_reasoning → sql_generation → sql_execution → insight）通过 SSE `thinking` 事件实时推送到前端，用户能看到中间过程。
- **空结果兜底**：Data Asking 返回空结果时，`StreamEmptyQueryFallback` 把思考过程的最后几步作为上下文，让 LLM 生成友好的"未查到数据"回复，而不是直接显示空白。
- **Insight 渲染**：Data Asking 返回的 insight blocks 包含 text 和 tables，`flushInsightBlocks` 将其转为 Markdown（≤2列用 bullet list，>2列用 Markdown table），通过 SSE 流式推送。

### 1.3 Catalog 数据同步

日报数据通过 MOI SDK 同步至 Catalog，供 Data Asking 查询。

**同步的 6 张表**：members、teams、daily_entries、daily_summaries、topics、topic_activities。

**表结构同步**（`SyncSchemaFromDB`）：
- 启动时从实际 DB 读 `SHOW COLUMNS`，对比 Catalog 已有表
- 不存在的表自动创建，已存在的不动
- 列的语义描述来自 `columnComments` map — 这是表结构的唯一配置源
- 独立初始化工具 `cmd/catalog_init/` 可手动执行首次建库建表

**数据同步策略**：
- 启动时全量同步 6 张表（异步，不阻塞启动）
- 每次日报提交/导入后增量同步
- 冲突策略 `ConflictPolicyReplace`，主键冲突则替换，保证数据最新

**实现细节**：
- 数据以 CSV 格式上传，触发异步 Task 导入
- Task 完成状态通过轮询检测。绕过了 SDK 的 Task 状态解析（SDK 有 bug：不处理 `{"task":{...}}` 包装层，且 status 是 int 不是 string），直接用 HTTP 调用 Task API 获取原始状态

### 1.4 NL2SQL 语义配置（Knowledge）

裸表结构（列名+类型）不够让 Data Asking Agent 理解业务语义。通过 NL2SQL Knowledge API 注入 4 类语义配置：

| 类型 | 用途 | 示例 |
|------|------|------|
| `glossary` | 术语解释 | "日报"= daily_entries 表中的一条记录 |
| `synonyms` | 同义词→表.列映射 | "姓名/名字/谁" → members.name |
| `logic` | 业务逻辑规则 | 判断谁没交日报：LEFT JOIN + IS NULL |
| `case_library` | 问答样例（问题→SQL） | "今天谁没交日报" → 完整 SQL |

**配置管理**：
- 所有条目定义在 `catalog_sync.go` 的 `KnowledgeEntries()` 函数中，是唯一配置源
- `cmd/catalog_init/` 首次部署时创建，按 key 去重（已存在跳过）
- 服务启动时 `SeedKnowledge()` 增量补缺（list 已有 key → 只创建缺失的）
- 两个入口共用同一份定义，不会不一致

### 1.5 Session / Message 持久化

会话和消息不存本地数据库，而是通过 MOI LLM Proxy Session API 持久化在 MOI 平台。好处是会话数据与 LLM 上下文天然关联，且不增加本地存储负担。消息写入是异步的，不阻塞用户交互。

---

## 二、Prompt 工程实践

### 2.1 日期上下文注入

所有时间敏感的 prompt 统一通过 `todayContext()` 注入当天日期：

```
今天是 2026-03-05（星期三）。
```

LLM 没有实时时间感知，不注入日期会导致"本周""上周"等相对时间推算错误。

### 2.2 周报日期范围解析

用户说"帮我生成本周周报"，需要解析出精确的起止日期。

**踩过的坑**：让 LLM 自己算"本周一是几号"，经常算错（尤其跨月时）。

**解决方案**：prompt 中直接给出本周一的具体日期，LLM 只需要理解用户意图（本周/上周/最近一周），不需要做日期推算：

```
今天是 2026-03-05（星期三），本周一是 2026-03-02。
请从用户输入中提取日期范围...
```

**教训**：不要让 LLM 做它不擅长的事（精确计算）。能预计算的就预计算好喂给它。

### 2.3 意图分类与模式验证

系统有 4 种模式（汇报/补填/查询/周报），用户可以显式选择，也可以不选让系统自动识别。

**关键设计**：模式 = 硬约束。选了"汇报"模式后，如果用户输入的是一个问题（如"谁提交了日报？"），系统不会默默当成汇报处理，而是用 `ClassifyIntent`（不带历史）验证输入是否匹配当前模式，不匹配则引导用户切换。

**分类规则**：
- 有疑问词或疑问语气 → query
- 纯陈述（主语是"我"且过去时）→ report
- 其他 → chat

这避免了"用户选错模式 → 系统强行处理 → 结果离谱"的问题。

### 2.4 对话历史管理

不同模式的对话历史互相隔离：

- 前端每条消息标记 `mode`（report/query/summary/supplement）
- 后端 `buildHistoryFiltered(mode)` 按模式过滤，只传同模式的历史给 LLM
- 滑动窗口：最近 5 轮
- `ExtractWorkContent` 只取用户消息（过滤掉 assistant），避免把 AI 编造的内容当成用户说的

**为什么隔离**：如果把查询模式的对话（"谁没交日报？"）混入汇报模式的上下文，LLM 会把查询内容当成工作内容提取，产生错误摘要。

### 2.5 充分性检查的双层策略

判断用户输入是否足够详细，采用程序化兜底 + LLM 判断：

1. **程序化兜底**：去掉标点后不足 6 字 → 直接追问（不调 LLM，省一次请求）
2. **LLM 判断**：prompt 明确定义通过/不通过的边界 — "登录模块"不通过（没说做了什么），"修复了登录验证码的bug"通过（有动作+有对象）

**设计思路**：简单情况程序化处理，复杂判断交给 LLM。

### 2.6 风险检测的严格约束

`DetectRisks` prompt 同时定义了"什么算风险"和"什么不算风险"：

- ✓ 明确提到阻塞、延期、线上故障未解决、等待外部支持
- ✗ 修复了 bug（正常成果）、任务进行中（正常进展）、计划明天做（正常排期）

**为什么要定义反面**：不加反面约束，LLM 倾向于把所有提到"bug""问题"的内容都标为风险，导致误报率极高。

---

## 三、LLM 批量处理

### 3.1 历史日报导入 — 两级提取策略

导入大文件（693 sections，2-3 年日报）需要兼顾速度和准确性。采用程序化提取优先、LLM 兜底的两级策略：

```
上传文件 → Python 解析 docx → 尝试程序化提取
  ├─ 成功（tab 分隔表格）→ 按列拆解，~3s 完成
  └─ 失败（非表格格式）→ LLM 并行提取（semaphore=50），~30s
```

**程序化提取（快速路径）**：
- 检测 tab 分隔格式 → 定位"成员\t"表头行 → 只解析其后的数据行
- 跳过迭代表（V0.8/V1.0 等版本信息行）和表头行
- 第一列=人名（去空格归一化），其余非空列用 "; " 拼接=内容
- 格式不符则 fallback 到 LLM

**LLM 提取（兜底路径）**：
- 每个 section 独立调用 LLM 提取（无上下文依赖）
- goroutine + semaphore（并发上限 50）并行处理
- prompt 注入已知成员列表，帮助 LLM 区分人名和项目名

**性能对比**（693 sections，1MB 文件）：

| 方案 | 耗时 | 准确率 |
|------|------|--------|
| LLM + 成员列表注入 | ~84s | 准确 |
| 程序化提取 | ~3s | 准确 |

**设计思路**：能确定性解决的不用概率模型。tab 分隔表格的结构是确定的，程序解析比 LLM 更快更准。LLM 留给真正需要"理解"的非结构化文档。

### 3.2 人名后处理归一化

LLM 从文档提取的人名可能不一致（如"马建强"和"马 建强"，原始文档表格对齐导致的空格）。

**方案**：不靠 prompt 约束（不可靠），而是在 LLM 输出后做确定性后处理 — 去除中文名字中的所有空格。各司其职：LLM 负责理解文档结构，后处理负责归一化。

### 3.3 两步导入 + 成员复核

导入流程分两步，中间加人工复核：

```
上传文件 → Python 解析 docx → 并行 LLM 提取
         → Preview（展示提取结果 + 未匹配成员列表）
         → 用户复核未匹配成员（创建/关联已有/忽略）
         → Confirm（按决策处理成员 → 批量写入 → Catalog 同步 → Topic 提取）
```

**为什么需要复核**：LLM 可能把项目名、技术术语误识别为人名。自动创建会产生脏数据，复核让用户决定哪些是真人、哪些该忽略。

### 3.4 Topic 自动提取

日报提交/导入时自动提取研发主题（Topic），用于按 Topic 聚合分析。

**批量提取**（`ExtractTopicsBatch`）：
- 20 条日报打包成一次 LLM 调用，输入格式 `[ID] 内容`，输出 JSON `{"1":["topicA"],"2":["topicB"]}`
- 注入已有 Topic 列表到 prompt，让 LLM 优先匹配已有名称，避免同一项目出现多个叫法（如"MOI"和"MOI平台"）
- prompt 明确排除规则：不把动作（开发/测试）、人名、issue 编号、版本号当 Topic

**启动时全量提取**：
- 首次部署时检测 `topic_activities` 数量，不足则触发全量提取
- goroutine + semaphore（并发 10）并行处理所有 batches
- 进度日志：每 20 批输出一次进度

### 3.5 日报合并

同一天多次提交时，`MergeDailySummary` 用 LLM 合并新旧摘要：
- 后面的记录修正了前面的 → 以最新为准
- 去重，合并相同事项
- 用 fastModel 处理（结构化任务，不需要主模型）

### 3.6 周报生成

`StreamWeeklySummary` 流式生成 Markdown 周报：
- 输入：按日期排列的日报数据（`[日期] 工作内容`）
- 输出：本周重点 → 进展详情（按日期分组）→ 风险与阻塞 → 下周计划
- prompt 严格约束：风险和下周计划仅来自日报原文，不编造；每个有日报的日期都必须列出，不得合并或遗漏

---

## 四、SSE 事件协议

前后端通过 SSE 事件协议通信，事件类型明确分工：

| 事件 | 用途 |
|------|------|
| `token` | 流式文本（逐字输出） |
| `result` | 结构化数据（日报摘要确认卡片，含 summary + risks） |
| `thinking` | Data Asking 推理过程（分步展示） |
| `meta` | 附加信息（周报下载链接） |
| `mode_switch` | 自动切换前端模式 |
| `done` | 流结束 |

这样前端可以根据事件类型做不同渲染，而不是把所有内容混在一个文本流里解析。

---

## 五、踩坑记录

### 5.1 docx 单元格内换行导致人名误识别

**现象**：导入历史日报时，"金盘""安利"等项目名被 LLM 识别为人名。

**根因**：Python docx 解析器用 `'\n'.join(parts)` 连接单元格内段落，与行间的 `\n` 混淆，导致单元格内容被拆成独立行，LLM 把第二行内容当成新的人名。

**解决**：连接符从 `\n` 改为 `; `。

**教训**：LLM 提取质量的上限取决于输入数据的质量。在怀疑 LLM 能力之前，先检查喂给它的数据是否准确。

### 5.2 人名空格不一致

**现象**：同一个人在不同日期被识别为两个不同的成员（"马建强" vs "马 建强"）。

**解决**：LLM 输出后做确定性后处理，去除中文名字中的所有空格。不靠 prompt 约束做归一化 — LLM 是概率模型，不保证 100% 执行。

### 5.3 周报日期范围 LLM 推算错误

**现象**：用户说"生成本周周报"，LLM 返回的日期范围经常算错（尤其跨月时）。

**解决**：prompt 中直接给出本周一的具体日期，LLM 只需理解用户意图，不需要做日期推算。

### 5.4 从 LLM 提取到程序化提取的演进

**过程**：
1. 纯 LLM 并行提取：~84s，项目名被误识别为人名
2. 加成员列表注入：准确率提升，速度无改善
3. 反思：文档是 tab 分隔表格，结构完全确定，为什么要让 LLM 来"理解"？
4. 最终：程序化提取 3s，LLM 只做兜底

**教训**：能确定性解决的问题不要用概率模型。LLM 的价值在于处理模糊、非结构化的输入。

### 5.5 LLM 不适合做节假日数据源

**测试**：用 Qwen3-max 生成中国法定节假日数据 — 放假日基本对，但调休上班日错误率高（6 个只对 3 个）。MOI LLM Proxy 不支持 `enable_search` 和 `tools: web_search`，无法联网查询。

**结论**：需要精确数据的场景必须用确定性数据源（API/静态数据），不能依赖 LLM。

### 5.6 NL2SQL Knowledge 的 type 值

**现象**：用 `term_explanation`、`sql_example` 作为 knowledge_type 创建语义配置，API 返回 `invalid sql knowledge type`。

**根因**：MOI 平台 NL2SQL Knowledge API 支持的 type 是 `glossary`、`synonyms`、`logic`、`case_library`，不是 SDK 文档注释中暗示的 `term_explanation`。

**教训**：SDK 的注释和示例不一定准确，以实际 API 行为为准。

---

## 六、待做方案（TODO）

### 6.1 导入日报 LLM 摘要（可选方案）

**现状**：导入历史日报时，提取出的 `Content` 是原文，直接存入 `daily_summaries.summary`，没有经过 LLM 做摘要提炼。

**影响**：
- 导入的"摘要"实际是原始文本，格式不统一
- 与 Chat 提交的日报（经过 `StreamSummarize` 生成结构化摘要）质量不一致
- 周报生成、NL2SQL 查询时，导入数据的可读性较低

**可选方案**：

| 方案 | 说明 | 耗时 | 适用场景 |
|------|------|------|---------|
| A. 保持原文 | 不做摘要，原文直接存 | 0 | 原文已经比较规整的团队 |
| B. 导入时批量摘要 | 20条/批打包调 LLM，类似 `ExtractTopicsBatch` | ~3-5分钟/4000条 | 追求数据质量一致性 |
| C. 懒加载摘要 | 首次被周报/查询引用时才触发 LLM 摘要，结果缓存 | 按需 | 折中方案 |

**建议**：导入时提供选项让用户选择"快速导入（原文）"或"智能导入（LLM 摘要）"，默认快速导入。

## 7. MergeDailySummary 改造（2026-03-06）

### 问题
同一天多次提交日报时，MergeDailySummary 只传两段提取后的干净摘要给 LLM 合并。如果用户第二次说"前面说错了，作废"，这个指令在内容提取阶段被丢弃，merge 时 LLM 看不到，导致被否定的内容仍被保留。

### 方案
merge 时从 daily_entries 查当天该成员所有提交记录（按 created_at 排序），带时间戳和原始内容一起传给 LLM：

```
[10:30] 完成了A模块的重构
[14:20] 前面说错了，作废，以这次为准。今天做的是B模块的性能优化
```

prompt 明确告知 LLM：如果用户说了"作废/不算/重新提交"等，丢弃被否定的内容。

### 改动
- `repository/daily.go` — 新增 `GetDayEntries(memberID, date)` 按 created_at 排序
- `service/daily.go` — 新增 `GetDayEntries` 透传
- `service/ai.go` — `MergeDailySummary` 参数从 `[]string` 改为 `[]model.DailyEntry`，拼接 `[HH:MM] content` 格式
- `handler/chat.go` — confirm 时查全部 entries 传给 merge，不再只传 existing.Summary + new.Summary

### 效果（benchmark 2026-03-06）
| 场景 | 结果 | 耗时 |
|------|------|------|
| 正常合并（登录+导出） | ✅ 两条都保留 | 23s |
| 作废覆盖（A模块→B模块） | ✅ A模块被丢弃，只保留B模块 | 20s |
