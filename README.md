# 曦薇空间 · Xiwei Space

田曦薇专属追星 Web App 的 P0 实现。移动端优先，提供艺人档案、作品与角色、场景化内容发现、收藏与兴趣偏好，以及带来源追溯、规则分类和人工审核的后台采集系统。

> 本项目不内置任何未经授权的艺人图片。页面封面由 CSS 抽象排版生成；自动采集默认不引用第三方图片。

## 功能范围（P0）

**访客 / 粉丝端**
- 首页：艺人简介、今日内容、场景入口、推荐内容、编辑精选、最新动态、作品速览
- 发现：关键词搜索、多场景筛选、内容类型 / 来源类型 / 时间范围筛选、最新与推荐排序
- 作品档案：按年份分组的作品列表、作品详情、角色档案、关联内容
- 内容详情：来源、可信等级、发布时间、审核说明、相关推荐
- 收藏：内容 / 作品 / 角色三类收藏，展示时保存快照，目标不可用时给出提示
- 个人中心：兴趣场景、权重（1–5）、默认场景、账号信息
- 登录 / 注册：HttpOnly 会话、CSRF 防护、登录限流；注册默认关闭

**管理端**
- 概览：今日采集与入库、待审核、来源异常、成功率、最近任务
- 数据源：RSS / 人工来源的创建与编辑、独立启停、立即运行
- 采集任务：状态筛选、计数、耗时、错误信息、失败重试、15 秒自动刷新
- 待审核：通过 / 驳回（填写原因），重要内容人工把关
- 内容管理：筛选、手动录入、编辑状态与关联作品 / 角色
- 基础资料：艺人档案、作品、角色、场景、分类规则、推荐权重

**后端能力**
- 单一 Go 服务（API + Worker），无微服务
- PostgreSQL 16，内嵌 SQL 迁移与种子数据，启动时自动迁移
- SourceAdapter 抽象；RSS 适配器 + 人工录入来源
- 规范化、URL 去重、来源条目去重、内容指纹去重
- 关键词规则分类（剧粉 / 颜值粉 / 综艺粉 / 日常粉 / 时尚商务粉 / 事业粉 / 活动粉）
- 可信等级、重要信息识别、社区内容与未分类内容强制人工审核
- 原始抓取数据仅后台留存，公共 API 不返回
- 后台初始化仅允许在用户表为空时执行，无固定默认密码

## 技术栈

| 层 | 选型 |
| --- | --- |
| 前端 | Vue 3、TypeScript、Vite 6、Vue Router 4、Tailwind CSS 3、lucide-vue-next |
| 后端 | Go 1.26、chi v5、pgx v5、gofeed |
| 数据库 | PostgreSQL 16 |
| 部署 | Docker Compose、nginx（SPA + `/api` 反向代理） |

## 快速启动（Docker Compose）

1. 准备环境变量：

   ```bash
   cp .env.example .env
   ```

   修改 `.env` 中的 `POSTGRES_PASSWORD`，并确认 `WEB_PORT`（默认 `8088`）。

2. 构建并启动：

   ```bash
   docker compose up -d --build
   docker compose ps
   ```

   首次启动会自动执行数据库迁移和经过核验的种子数据。

3. 创建首个管理员：

   浏览器打开 `http://localhost:8088/setup`，设置管理员邮箱与至少 10 位密码。初始化接口只在用户表为空时可用；系统不存在任何默认账号或固定密码。

4. 进入管理后台：

   `http://localhost:8088/admin`。建议先在「数据源」检查种子来源，在「基础资料」核对艺人、作品、角色与场景。

> 若需要从宿主机访问数据库，请自行临时映射端口，不要在生产环境把 PostgreSQL 暴露到公网。

### 使用 HTTPS 时

将 nginx 或上层负载均衡配置 TLS 后，在 `.env` 中设置：

```env
COOKIE_SECURE=true
```

同源部署时 `ALLOWED_ORIGINS` 可以留空；前后端分域部署时，填写完整来源（含协议与端口），逗号分隔。

## 本地开发

依赖：Go 1.26、Node.js 22+、Docker（用于 PostgreSQL）。

```bash
# 1. 启动开发数据库（宿主机 5433 端口）
docker run -d --name xiwei-dev-db \
  -e POSTGRES_USER=xiwei -e POSTGRES_PASSWORD=xiwei_dev -e POSTGRES_DB=xiwei \
  -p 5433:5432 postgres:16-alpine

# 2. API（8090）
cd backend
DATABASE_URL='postgres://xiwei:xiwei_dev@localhost:5433/xiwei?sslmode=disable' \
  ADDR=':8090' APP_ENV=development ALLOW_REGISTRATION=true \
  ALLOWED_ORIGINS='http://localhost:5173,http://127.0.0.1:5173' \
  go run ./cmd/api

# 3. Worker（另开终端）
cd backend
DATABASE_URL='postgres://xiwei:xiwei_dev@localhost:5433/xiwei?sslmode=disable' \
  APP_ENV=development go run ./cmd/worker

# 4. 前端（另开终端）
cd frontend
npm ci
VITE_PROXY_TARGET=http://localhost:8090 npm run dev
```

访问 `http://127.0.0.1:5173`。开发环境如需自助注册，设置 `ALLOW_REGISTRATION=true`；生产建议保持关闭。

### 构建与测试

```bash
cd backend && go test ./... && go vet ./...
cd frontend && npm run build

# 可选：针对一次性测试库运行采集入库集成测试（会写入并清理测试数据）
TEST_DATABASE_URL='postgres://xiwei:xiwei_dev@localhost:5433/xiwei_test?sslmode=disable' \
  go test ./internal/store -run TestInsertCollectedIntegration -v
```

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DATABASE_URL` | 本地开发库 | PostgreSQL 连接串；Compose 中由 `POSTGRES_*` 拼装 |
| `ADDR` | `:8080` | API 监听地址 |
| `APP_ENV` | `development` | `production` 时输出 JSON 日志 |
| `COOKIE_SECURE` | `false` | 仅通过 HTTPS 发送会话 Cookie |
| `ALLOWED_ORIGINS` | 本地开发来源 | 额外允许的跨域来源，逗号分隔；同源请求按 Host 校验 |
| `ALLOW_REGISTRATION` | `false` | 是否开放访客注册 |
| `SESSION_TTL_HOURS` | `168` | 会话有效期（小时） |
| `SEED_ON_START` | `true` | 是否应用种子迁移（只执行一次） |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | — | Compose 数据库配置，口令必须修改 |
| `WEB_PORT` | `8088` | Web 入口端口 |

## 数据流

```text
RSS / 人工来源
      │
      ▼
SourceAdapter.Fetch ──► 规范化（HTML 清理、URL 规范化、长度截断）
      │
      ▼
去重（来源条目 ID → 规范化 URL → 内容指纹，14 天窗口）
      │
      ▼
规则分类 → 可信等级 → 重要信息 / 社区 / 未分类审核判定
      │
      ├── 自动发布（review_status=none）
      └── 待审核（review_status=pending）──► 管理端通过 / 驳回
      │
      ▼
公共 API（不含 raw_data）──► 前端展示
```

推荐权重默认：场景 40%、时效 30%、可信 20%、热度 10%。热度在缺少可验证信号时不参与计算，权重可在管理端调整。

## 备份与恢复

```bash
# 备份到 backups/xiwei-YYYYmmdd-HHMMSS.dump
scripts/backup.sh

# 恢复（会二次确认，并自动停止 / 启动 API 与 Worker）
scripts/restore.sh backups/xiwei-YYYYmmdd-HHMMSS.dump
```

脚本在 Git Bash / Linux / macOS 下运行，依赖 `docker compose`。建议配置定时备份并把 `backups/` 复制到异地。

## 目录结构

```text
backend/
  cmd/api/            API 进程
  cmd/worker/         采集与调度进程
  internal/api/       路由、鉴权、公共与管理接口
  internal/auth/      Argon2id、会话与 CSRF
  internal/collector/ 适配器、规范化、分类、审核判定、Worker
  internal/store/     SQL 数据访问
  internal/db/        连接与内嵌迁移
  migrations/         表结构、种子数据
frontend/
  src/views/          公共页面与管理页面
  src/components/     设计系统组件
  src/stores/         登录状态与 Toast
  nginx.conf          静态资源与 /api 反向代理配置
scripts/              备份与恢复
docs/                 使用、管理、采集与版权说明
```

## 文档

- [使用说明](docs/使用说明.md)
- [管理指南](docs/管理指南.md)
- [数据采集与来源说明](docs/数据采集与来源.md)
- [图片与版权说明](docs/图片与版权说明.md)

## 安全与合规要点

- 会话使用 HttpOnly、SameSite=Lax Cookie；写请求校验 `X-CSRF-Token` 与 `Origin`
- 登录按 IP 限流；管理员初始化只允许一次，密码至少 10 位
- 数据库不对公网暴露；原始采集数据不进入公共 API
- 仅接入公开 RSS 或人工核验录入；不实现模拟登录、Cookie 登录或绕过反爬
- 采集源可独立启停；单个来源同时只有一个活动任务
- 重要信息、社区来源、无法分类的内容强制人工审核后才公开
- 不内置未经授权的图片；RSS 图片默认关闭，需管理员在来源配置中显式设置 `use_feed_images: true` 并确认授权

## P0 边界

当前仅服务单一艺人，不实现多艺人管理、P1/P2 功能、LLM 分类、微服务拆分或平台内私信 / 社区。RSS 适配器依赖来源站点可访问性；若种子中的必应新闻 RSS 在部署网络不可用，可在管理端替换为可公开访问的 RSS 地址或使用人工来源。
