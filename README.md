# 曦薇空间 · Xiwei Space

田曦薇专属追星 Web App 的 P0 实现。移动端优先，提供艺人档案、作品与角色、场景化内容发现、收藏与兴趣偏好，以及带来源追溯、规则分类和人工审核的后台采集系统。

> 图片以准确、清晰、尺寸合适为准：优先使用与人物 / 作品 / 角色严格对应的真实图片，不把授权确认作为上传或引用的前置条件；没有合格图片或加载失败时回退到 CSS 抽象排版。图片规格见 [图片使用与质量说明](docs/图片使用与质量说明.md)。

## 功能范围（P0）

**访客 / 粉丝端**
- 首页：艺人简介、场景入口、本周精选、推荐内容、最新动态、历史回顾、作品速览；无数据模块自动隐藏
- 发现：按有效时间倒序的主时间线、类型分类、来源平台 / 来源类型 / 时间范围 / 关联作品 / 关联活动 / 场景筛选、游标分页
- 作品档案：按年份分组的作品列表、作品详情、角色档案、关联内容
- 内容详情：来源与来源平台、发布时间及精确度、发现时间、事件关联、审核说明、相关推荐
- 事件页：聚合同一公开活动 / 节点的相关内容，不改变主时间线排序
- 收藏：内容 / 作品 / 角色三类收藏，展示时保存快照，目标不可用时给出提示
- 个人中心：兴趣场景、权重（1–5）、默认场景、账号信息
- 登录 / 注册：HttpOnly 会话、CSRF 防护、登录限流；注册默认关闭

**管理端**
- 概览：今日采集与入库、待审核、来源异常、成功率、最近任务
- 数据源：RSS / 人工来源的创建与编辑、独立启停、立即运行
- 内容发现：搜索服务（必应 RSS / SearXNG）与搜索任务管理、连接测试、立即搜索、运行记录
- 事件管理：事件创建、编辑、公开状态与内容关联
- 编辑精选：从已发布内容中挑选首页条目，配置标题 / 描述 / 封面、顺序、展示窗口与启停
- 采集任务：状态筛选、计数、耗时、错误信息、失败重试、15 秒自动刷新
- 待审核：通过 / 驳回（填写原因），重要内容人工把关
- 内容管理：候选筛选、解析状态、发布时间修正 / 清空、类型与来源、事件 / 作品 / 角色关联
- 基础资料：艺人档案、作品、角色、场景、分类规则、推荐权重；艺人头像 / 封面、作品封面、角色头像支持地址填写与本地上传
- 资料完善：统计头像 / 海报 / 封面的缺失数量与覆盖率，按缺图片、缺摘要、缺时间、缺关联、缺来源筛选缺失清单，并可直接进入对应编辑入口
- 内容库：30 条站内整理的公开资料条目，覆盖剧集、颜值、综艺、日常、时尚商务、事业与活动七类场景，保留公开引用链接
- 图片维护：内容、事件、编辑精选封面统一使用图片选择器，可预览、替换、清除；加载失败自动回退到站内抽象封面

**后端能力**
- 单一 Go 服务（API + Worker），无微服务
- PostgreSQL 16，内嵌 SQL 迁移与种子数据，启动时自动迁移
- SourceAdapter 抽象；RSS 适配器 + 人工录入来源
- SearchProvider 抽象；必应网页搜索 RSS + 可选 SearXNG，搜索任务持久化与运行记录
- Web Metadata Adapter：解析标题、描述、OG、作者、发布时间，失败保留搜索摘要与原始链接；固定 4 路并发、单页 7 秒超时、按搜索顺序确定性处理
- 搜索来源按结果域名独立识别；已验证账号精确匹配；明确无关与上下文不足分别处理，后者进入低置信人工审核
- 任务调度：采集与搜索交替优先领取，最大并发 2；任务心跳与按类型区分的超时回收（采集 8 分钟 / 搜索 10 分钟）
- 任务终态包含 `success` / `partial` / `failed`，超时、panic、关联数据缺失均写入明确错误信息
- 结构化 Worker 日志包含 `worker_id`、`task_type`、任务 ID、耗时、处理计数与错误
- 规范化、URL 去重、来源条目去重、内容指纹去重
- 发布时间 / 发现时间 / 入库时间分离，发布时间精度为 `verified` / `estimated` / `unknown`
- 主时间线按 `COALESCE(published_at, discovered_at)` 倒序，游标分页稳定翻页
- 关键词规则分类（剧粉 / 颜值粉 / 综艺粉 / 日常粉 / 时尚商务粉 / 事业粉 / 活动粉）与独立内容类型
- 可信等级、重要信息识别、社区内容与未分类内容强制人工审核
- 首页编辑精选一次批量加载关联内容，不读取 `raw_data`；公共最多 8 条，后台列表上限 200 条
- 搜索元数据解析到公开 `og:image` / `twitter:image` 时写入候选封面；不下载图片，重复结果与人工封面不会被覆盖
- 图片上传仅限管理员 / 编辑：10MB 上限、服务端类型识别、随机文件名、`MEDIA_DIR` 持久化；公开只读访问不要求登录
- 资料完善接口复用同一份缺失字段统计与清单，计数、覆盖率和筛选结果不会互相漂移；已发布内容与事件才纳入统计
- 未登录访客不展示「为你挑选」，避免与「最新动态」重复
- 原始抓取数据仅后台留存，公共 API 不返回
- 后台初始化仅允许在用户表为空时执行，无固定默认密码
- API Key 只引用服务端环境变量名，不写入数据库、不返回前端

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

   > `worker` 服务通过 `command: ["/app/worker"]` 选择 Worker 二进制。镜像默认命令必须是 `CMD ["/app/api"]`；如果改成 `ENTRYPOINT ["/app/api"]`，worker 容器会实际启动 API，采集与搜索任务将一直停留在排队中。修改镜像或升级代码后请执行 `docker compose up -d --build`，确保 API 与 Worker 使用同一镜像版本。

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

# 可选：针对一次性测试库运行采集、时间线与事件集成测试（会写入并清理测试数据）
TEST_DATABASE_URL='postgres://xiwei:xiwei_dev@localhost:5433/xiwei_test?sslmode=disable' \
  go test ./internal/store -run 'TestInsertCollectedIntegration|TestDiscoveryTimelineIntegration|TestJobLifecycleReliabilityIntegration' -v

# 可选：真实必应搜索连通性测试（会访问公网）
TEST_LIVE_SEARCH=1 go test ./internal/discovery -run TestBingLive -v
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
| `SEARCH_OVERLAP_DAYS` | `7` | 搜索任务基于上次完整成功向前回溯的天数（1–90） |
| `MEDIA_DIR` | `data/media` | 上传图片存储目录；Compose 固定为 `/app/data/media` 并挂载命名卷 `xiwei_media` |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | — | Compose 数据库配置，口令必须修改 |
| `WEB_PORT` | `8088` | Web 入口端口 |

## 数据流

```text
RSS / 人工来源                        搜索任务（Bing RSS / SearXNG）
      │                                        │
      ▼                                        ▼
SourceAdapter.Fetch                    SearchProvider.Search
      │                                        │
      │                                 Web Metadata Adapter（SSRF 防护）
      │                                        │
      ▼                                        ▼
规范化（HTML 清理、URL 规范化、长度截断）  搜索候选（保留摘要 / 原始链接）
      │                                        │
      └──────────────┬─────────────────────────┘
                     ▼
去重（RSS：条目 ID → URL → 内容指纹；搜索：规范化 URL）
                     ▼
规则分类 → 可信等级 / 发布时间精度 → 审核判定
                     │
                     ├── 自动发布（RSS 分类明确且来源可信）
                     └── 待审核 ──► 管理端核验、修正时间 / 分类、关联事件 ──► 通过
                     ▼
公共 API（不含 raw_data）──► 主时间线 / 事件页
```

推荐权重默认：场景 40%、时效 30%、可信 20%、热度 10%。热度在缺少可验证信号时不参与计算，权重可在管理端调整。发现页固定按有效时间排序，不提供热度排序。

### 内容发现与搜索服务

- 搜索服务与搜索任务分开管理，任务可配置关键词、限定站点、分类提示、来源类型、可信等级、间隔与单次上限。
- 默认种子包含 Bing 网页搜索 RSS 与两个搜索任务；可在管理端停用、编辑或添加自建 SearXNG。
- 搜索结果全部进入待审核候选池，不会自动公开；候选保留搜索标题 / 摘要、解析状态和原始链接。
- 发布时间优先级：页面明确给出 → `verified`；仅搜索引擎摘要给出 → `estimated`；都没有 → `unknown`，保留 `published_at = NULL` 并按发现时间排序。
- 必应 RSS 结果受 Microsoft 服务条款限制，仅用于个人非商业 RSS 聚合；正式商用前需确认授权。社交平台页面常要求登录或验证码，此时候选会标记为 `blocked`，不会绕过访问控制。

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
docs/                 使用、管理、采集与图片说明
```

## 文档

- [使用说明](docs/使用说明.md)
- [管理指南](docs/管理指南.md)
- [数据采集与来源说明](docs/数据采集与来源.md)
- [图片使用与质量说明](docs/图片使用与质量说明.md)

## 安全与合规要点

- 会话使用 HttpOnly、SameSite=Lax Cookie；写请求校验 `X-CSRF-Token` 与 `Origin`
- 登录按 IP 限流；管理员初始化只允许一次，密码至少 10 位
- 数据库不对公网暴露；原始采集数据不进入公共 API
- 仅接入公开 RSS、公开搜索结果或人工核验录入；不实现模拟登录、Cookie 注入、签名破解、验证码或反爬绕过
- 页面抓取执行 SSRF 防护：仅 http/https、拒绝内网 / 回环 / 链路本地 / 云元数据地址、限制端口、重定向、响应大小与超时
- 搜索服务 API Key 只保存环境变量名；前端与管理 API 不返回密钥值，配置中出现密钥字段会被拒绝
- 采集源与搜索任务可独立启停；同一来源 / 任务同时只有一个活动运行
- 重要信息、社区来源、无法分类的内容与全部搜索候选强制人工审核后才公开
- 图片不设版权审核门槛；发布前核对图片与人物 / 作品 / 角色严格对应，优先官方高分辨率素材，低质量或无关图片留空并回退抽象封面
- 随包图片位于 `frontend/public/media/`，由 `0010_media_assets.sql` 在字段为空时写入；管理员上传或修改过的封面不会被覆盖
- RSS 图片默认不采集，可按来源设置 `use_feed_images: true` 开启；建议只对图片质量稳定的来源开启
- 搜索候选封面只是原文图片地址的引用，不下载、不转存；管理员可在后台替换或清除，重复采集不会覆盖人工封面
- 上传图片按字节识别类型、使用随机文件名、限制 10MB；图片目录需持久化，避免容器重建后封面失效

## P0 边界

当前仅服务单一艺人，不实现多艺人管理、实时热榜、跨平台互动数据统计、社区 / 评论 / 私信、AI 生成内容、LLM 驱动采集决策、微服务拆分、强制 Redis 或独立消息队列。RSS 适配器依赖来源站点可访问性；若种子中的必应新闻 RSS 在部署网络不可用，可在管理端替换为可公开访问的 RSS 地址或使用人工来源。搜索能力取决于所配置的搜索服务与目标站点可访问性，社交平台覆盖有限，详见 [数据采集与来源说明](docs/数据采集与来源.md)。
