# MOI 智能日报

基于 MatrixOne MOI 平台的 AI 团队日报系统，单 Go 二进制服务前后端。

## 功能

- 日报提交：LLM 流式摘要 + 风险检测 → 确认 → 存库 + Catalog 同步
- 补报历史日报
- 周报生成 + Markdown 下载
- 数据查询：NL2SQL（Data Asking）+ 思考过程展示

## 技术栈

- 前端：React + TypeScript + Vite
- 后端：Go + Gin + GORM v2
- AI：MOI SDK + LLM Proxy（qwen/gpt）
- 数据库：MatrixOne

## 快速开始

### 环境要求

- Go 1.23+
- Node.js 20+
- MatrixOne 数据库

### 本地开发

```bash
# 编辑配置，填入 api_key 和数据库账密
vim server/etc/config-dev.yaml

# 初始化数据库
mysql -h <host> -P 6001 -u <user> -p smart_daily < server/migration/init.sql

# 启动（前端 :9872 + 后端 :9871）
make dev

# 停止
make dev-stop
```

### 生产部署（二进制）

```bash
# 编译 + 后台启动
make run

# 停止
make run-stop

# 查看日志
tail -f server/logs/server.log
```

### Docker 部署

```bash
cp .env.example .env
vim .env  # 填入真实配置

docker compose up -d
docker compose logs -f
```

## 项目结构

```
├── web/                  前端
│   ├── components/       React 组件
│   ├── services/         API 服务
│   └── types.ts          类型定义
├── server/               后端
│   ├── cmd/server/       入口 + 嵌入前端静态文件
│   ├── internal/
│   │   ├── config/       配置加载
│   │   ├── handler/      HTTP 处理
│   │   ├── log/          日志封装
│   │   ├── middleware/   JWT 认证
│   │   ├── model/        数据模型
│   │   └── service/      业务逻辑
│   ├── etc/              配置文件
│   └── migration/        SQL 脚本
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## 配置

配置优先级：`--config` 指定文件 > `etc/config-dev.yaml` > `/etc/smart-daily/config.yaml` > 环境变量覆盖

主要配置项见 `server/etc/config-dev.yaml` 和 `.env.example`。
