# 嘉阅圈后端服务（Go）

嘉阅圈私域社群平台的后端 API 服务，使用 Go 语言实现，提供内容社区、互动打卡、小组交流、私信通知、知识课程、AI 共读等核心能力。

前端（Taro H5 / 微信小程序）通过 HTTPS 调用本服务暴露的 REST API 完成数据交互。

## 技术栈

| 能力 | 选型 |
| --- | --- |
| 语言 | Go 1.25+ |
| Web 框架 | Gin |
| ORM | GORM |
| 数据库 | PostgreSQL 14+ |
| 数据库驱动 | gorm.io/driver/postgres（基于 pgx v5） |
| 认证 | golang-jwt/v5（JWT Bearer Token） |
| 配置 | godotenv（按 `--env` 加载 `.env.{env}`） |
| 密码哈希 | bcrypt |
| 微信集成 | code2session 登录、getPhoneNumber 获取手机号 |
| AI 共读 | DeepSeek（OpenAI 兼容）/ Coze，可切换 |
| 定时任务 | 标准库 time.Ticker，内置调度器（日/周/月 AI 总结） |

## 目录结构

```
new-server/
├── cmd/
│   └── server/
│       └── main.go              # 程序入口（加载配置、初始化 DB、启动 HTTP 服务、优雅关闭）
├── docs/                        # swag 自动生成的 OpenAPI 文档（swagger.json/yaml）
├── internal/
│   ├── config/                  # 配置加载（.env.{env} + 系统环境变量）
│   ├── database/                # 数据库连接、AutoMigrate、种子数据
│   │   ├── database.go          # GORM 连接初始化与连接池
│   │   └── seed.go              # 表结构自动迁移与种子数据初始化
│   ├── models/                  # 数据模型（对应数据库表）
│   ├── repository/              # 数据访问层（CRUD）
│   ├── service/                 # 业务服务层（事务编排、领域逻辑）
│   ├── middleware/              # 中间件（JWT 认证、用户上下文）
│   ├── handler/                 # HTTP 处理器（请求解析、响应封装）
│   ├── router/                  # 路由装配（含 CORS、静态文件）
│   ├── scheduler/               # 定时任务调度器（AI 总结帖）
│   └── pkg/                     # 工具包
│       ├── ai/                  # AI provider 抽象（DeepSeek / Coze）
│       ├── jwt/                 # JWT 签发与校验
│       ├── password/            # bcrypt 密码哈希
│       ├── response/            # 统一响应格式
│       ├── businessdate/        # Asia/Shanghai 业务日期处理
│       ├── mediaurl/            # 图片 URL 归属校验
│       ├── nickname/            # 随机昵称生成
│       └── wechat/              # 微信小程序 API 集成
├── migrations/
│   └── 001_init.sql             # PostgreSQL 初始化脚本（建表 + 外键 + CHECK + 触发器 + 种子话题）
├── go.mod
├── go.sum
├── .env.development             # 开发环境配置
├── .env.production              # 生产环境配置（敏感信息应通过系统环境变量覆盖）
├── .gitignore
└── README.md
```

## 快速开始

### 1. 准备数据库

创建 PostgreSQL 数据库（推荐 PG 14+，编码 UTF8）：

```sql
CREATE DATABASE yjedu_reading_club ENCODING 'UTF8' LC_COLLATE 'en_US.utf8' LC_CTYPE 'en_US.utf8' TEMPLATE template0;
```

> **可选**：执行 `migrations/001_init.sql` 一次性创建全部表、外键、CHECK 约束、`updated_at` 触发器与系统话题种子数据：
>
> ```bash
> psql -U postgres -d yjedu_reading_club -f migrations/001_init.sql
> ```
>
> 该步骤并非必须——服务启动时会通过 GORM `AutoMigrate` 自动同步表结构（详见下一节「数据库自动初始化」）。但执行 SQL 脚本可获得完整的 CHECK 约束与触发器，推荐在生产环境执行。

### 2. 数据库自动初始化

服务启动时由 [database.Initialize](file:///c:/Users/jamyido/Documents/Coding/YjEduReadingClub/new-server/internal/database/seed.go) 自动完成以下三步，**无需手动建表/写入种子**：

1. **`InitSchema`** — GORM `AutoMigrate` 同步 17 个模型表结构。AutoMigrate 仅加列加表，不会删除列或修改类型，对现有数据安全。
2. **`SeedDatabase`** — 当 `users` 表为空时，在单个事务中写入：
   - 1 个管理员账户（手机号 `13800000000`，昵称 `嘉阅圈管理员`，密码 bcrypt 哈希）
   - 6 个初始圈子（蓝色系主题色，管理员作为圈主加入）
   - 3 个系统话题（本周精读 / 本月精读 / 打卡挑战）
   - 已有数据时自动跳过，幂等。
3. **`EnsureAIAssistantUser`** — 幂等创建 `AI伴读` 系统用户（手机号 `10000000000`），作为日/周/月 AI 总结帖的统一作者身份。

### 3. 配置环境变量

配置通过 `.env.{env}` 文件 + 系统环境变量加载，**文件加载失败不阻断启动**（生产环境可完全依赖系统环境变量）。

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `DATABASE_URL` | PostgreSQL DSN：`host=主机 port=5432 user=用户名 password=密码 dbname=数据库名 sslmode=disable TimeZone=Asia/Shanghai` | 无（必填） |
| `JWT_SECRET` | JWT 签名密钥，生产环境务必使用 32 位以上强随机字符串 | 无（必填） |
| `JWT_ACCESS_TOKEN_EXPIRES_IN` | Token 有效期，支持 `7d`/`24h`/`60m`/`3600s` 后缀 | `7d` |
| `WECHAT_MINI_APP_ID` | 微信小程序 AppID | 空 |
| `WECHAT_MINI_APP_SECRET` | 微信小程序 AppSecret | 空 |
| `PORT` | HTTP 服务监听端口 | `3001`（.env 文件中配置为 `3100`） |
| `UPLOAD_DIR` | 上传文件根目录（相对路径基于运行目录） | `uploads` |
| `MAX_UPLOAD_SIZE_MB` | 单文件上传大小上限（MB） | `10` |
| `AI_PROVIDER` | AI provider 选择：`deepseek` / `coze`，留空时按已配置密钥自动探测（DeepSeek 优先） | 空 |
| `DEEPSEEK_API_KEY` | DeepSeek API Key | 空 |
| `DEEPSEEK_BASE_URL` | DeepSeek API 基地址 | `https://api.deepseek.com` |
| `DEEPSEEK_MODEL` | DeepSeek 模型名 | `deepseek-chat` |
| `COZE_TOKEN` | Coze 访问令牌（备用 provider） | 空 |
| `COZE_BOT_ID` | Coze 机器人 ID | 空 |
| `COZE_BASE_URL` | Coze API 基地址 | `https://api.coze.cn` |

### 4. 安装依赖并运行

```bash
# 下载依赖
go mod tidy

# 开发运行（默认 --env=development，加载 .env.development）
go run ./cmd/server

# 指定环境运行
go run ./cmd/server --env=production

# 编译为二进制
go build -o bin/server ./cmd/server
./bin/server --env=production
```

服务启动后会打印运行环境、AI provider 选择结果、数据库连接状态；监听端口由 `PORT` 决定（默认 `3001`，`.env.*` 中配置为 `3100`）。

### 5. 常用开发命令

```bash
# 编译检查
go build ./...

# 静态检查
go vet ./...

# 格式化代码
go fmt ./...

# 运行测试（如有）
go test ./...
```

## API 概览

所有接口统一前缀 `/api`，统一响应格式：

```json
{
  "success": true,
  "data": {},
  "message": "",
  "error": null
}
```

分页接口响应格式：

```json
{
  "success": true,
  "data": {
    "list": [],
    "total": 0,
    "page": 1,
    "pageSize": 20
  }
}
```

### 认证机制

- 使用 JWT Bearer Token，请求头携带 `Authorization: Bearer <token>`
- 公开路由使用 `OptionalAuth` 中间件（登录用户可获取个性化数据，如点赞态）
- 需登录路由使用 `AuthRequired` 中间件（未登录返回 401）

### 完整路由列表

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/auth/login/phone` | 公开 | 手机号 + 密码登录 |
| POST | `/api/auth/register` | 公开 | 手机号注册 |
| POST | `/api/auth/weapp/login` | 公开 | 微信小程序登录（code 换 openid） |
| POST | `/api/auth/weapp/phone` | 公开 | 微信小程序绑定手机号 |
| GET | `/api/auth/me` | 登录 | 获取当前登录用户信息 |
| POST | `/api/auth/change-password` | 登录 | 修改密码 |
| POST | `/api/auth/change-phone` | 登录 | 修改绑定手机号 |
| POST | `/api/auth/set-password` | 登录 | 设置密码（微信登录用户） |
| GET | `/api/posts` | 可选 | 帖子列表（圈子帖子对非成员限 3 条预览） |
| POST | `/api/posts` | 登录 | 创建帖子（委托 PostService 事务处理） |
| GET | `/api/posts/:id` | 可选 | 帖子详情（圈子帖子需成员身份） |
| DELETE | `/api/posts/:id` | 登录 | 删除帖子（作者或管理员） |
| GET | `/api/posts/:id/comments` | 可选 | 帖子一级评论列表 |
| POST | `/api/posts/:id/comments` | 登录 | 创建评论（支持二级回复） |
| POST | `/api/posts/:id/like` | 登录 | 点赞帖子 |
| DELETE | `/api/posts/:id/like` | 登录 | 取消点赞 |
| POST | `/api/posts/:id/share` | 公开 | 记录转发（统计分享数） |
| POST | `/api/posts/:id/ai-conversations` | 登录 | 基于帖子创建/获取 AI 共读会话 |
| GET | `/api/ai-conversations` | 登录 | 我的 AI 共读会话列表 |
| GET | `/api/ai-conversations/:id` | 登录 | AI 共读会话详情（含历史消息） |
| POST | `/api/ai-conversations/:id/messages` | 登录 | 发送 AI 共读消息（流式返回 AI 回复） |
| GET | `/api/circles` | 公开 | 圈子列表（分页 + 关键词搜索） |
| POST | `/api/circles` | 登录 | 创建圈子（仅管理员） |
| GET | `/api/circles/mine` | 登录 | 我加入的圈子 |
| GET | `/api/circles/:id` | 可选 | 圈子详情（非成员隐藏成员名单） |
| PUT | `/api/circles/:id` | 登录 | 更新圈子（拥有者或管理员） |
| DELETE | `/api/circles/:id` | 登录 | 删除圈子（拥有者或管理员） |
| POST | `/api/circles/:id/join` | 登录 | 加入圈子 |
| POST | `/api/circles/:id/leave` | 登录 | 退出圈子（拥有者不可退出） |
| GET | `/api/circles/:id/post-days-ranking` | 登录 | 累计发帖天数排行（仅成员） |
| PUT | `/api/circles/:id/members/:userId/role` | 登录 | 变更成员角色（拥有者或管理员） |
| GET | `/api/topics` | 公开 | 话题列表 |
| GET | `/api/checkin` | 公开 | 打卡记录列表 |
| POST | `/api/checkin` | 登录 | 创建打卡（委托 PostService） |
| GET | `/api/courses` | 公开 | 课程列表 |
| GET | `/api/courses/:id` | 可选 | 课程详情（登录用户附带进度） |
| POST | `/api/courses/:id/progress` | 登录 | 更新学习进度 |
| GET | `/api/messages` | 登录 | 私信会话列表 |
| GET | `/api/messages/:userId` | 登录 | 与指定用户的对话（拉取后标记已读） |
| POST | `/api/messages/:userId` | 登录 | 发送私信 |
| POST | `/api/messages/read-all` | 登录 | 全部标记已读 |
| GET | `/api/notifications` | 登录 | 通知列表 |
| GET | `/api/notifications/summary` | 登录 | 未读通知汇总 |
| POST | `/api/notifications/read` | 登录 | 标记单条通知已读 |
| POST | `/api/notifications/dispatch` | 登录 | 派发通知（仅管理员，支持 ALL/USERS/CIRCLE 范围） |
| PUT | `/api/users/profile` | 登录 | 更新个人资料 |
| GET | `/api/users/:id` | 公开 | 用户公开主页（含有效连续打卡天数） |
| POST | `/api/users/:id/follow` | 登录 | 关注用户 |
| DELETE | `/api/users/:id/follow` | 登录 | 取消关注 |
| GET | `/api/users/:id/followers` | 公开 | 粉丝列表 |
| GET | `/api/users/:id/following` | 公开 | 关注列表 |
| POST | `/api/upload` | 登录 | 文件上传（MIME 嗅探防伪造） |
| POST | `/api/admin/scheduler/trigger` | 登录（管理员） | 手动触发定时总结任务（type=daily/weekly/monthly） |

### 静态资源路由

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/uploads/*path` | 上传文件访问（防路径穿越） |
| GET | `/assets/*path` | 内置静态资源访问 |
| GET | `/swagger/*` | Swagger UI（OpenAPI 文档） |

## 架构说明

### 分层架构

```
HTTP 请求
   │
   ▼
middleware（认证、CORS）  ← 解析 JWT、注入用户上下文
   │
   ▼
handler（HTTP 层）         ← 请求参数解析、校验、响应封装
   │
   ▼
service（业务编排）         ← 跨仓储事务、领域规则
   │
   ▼
repository（数据访问）      ← 单表 CRUD、复杂查询
   │
   ▼
models（数据结构）          ← GORM 模型与表映射
```

### 关键设计

- **事务封装**：发帖等关键操作由 PostService 在事务中完成，使用 Serializable 隔离级别 + 重试机制，避免打卡记录并发冲突。底层错误码判断基于 PostgreSQL SQLSTATE（23505/40P01/55P03）。
- **业务日期**：打卡连续天数按 Asia/Shanghai 自然日计算，不依赖数据库时区配置。原生 SQL 中通过 `AT TIME ZONE 'Asia/Shanghai'` 显式指定时区。
- **图片安全**：上传时通过文件头字节嗅探真实 MIME（JPG/PNG/GIF/WebP），防止 Content-Type 伪造；帖子/圈子封面 URL 校验归属，防止越权引用他人图片。
- **圈子权限**：非成员仅可查看圈子帖子前 3 条预览（正文截断 160 字符、媒体只留首张）；成员名单仅对成员与管理员开放。
- **公开用户信息**：对外暴露的 `PublicUser` 结构去除 password、openid 等敏感字段。
- **优雅关闭**：监听 SIGINT/SIGTERM，等待处理中的请求完成（10 秒超时）后再关闭数据库连接。

### 错误响应

非成功响应统一格式：

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "可读的错误描述"
  }
}
```

常见 HTTP 状态码：

| 状态码 | 场景 |
| --- | --- |
| 400 | 请求参数错误、业务校验失败 |
| 401 | 未登录或 Token 失效 |
| 403 | 无权限（如非管理员创建圈子） |
| 404 | 资源不存在 |
| 409 | 冲突（如重复点赞、已加入圈子） |
| 500 | 服务器内部错误 |

## 数据库

数据库统一使用 PostgreSQL，编码 `UTF8`，支持 emoji 与多语言。所有时间戳字段使用 `TIMESTAMP(3) WITH TIME ZONE`，`updated_at` 由触发器自动维护（等价于 MySQL 的 `ON UPDATE CURRENT_TIMESTAMP`）。

表结构对应 `migrations/001_init.sql` 与 [models/](file:///c:/Users/jamyido/Documents/Coding/YjEduReadingClub/new-server/internal/models) 下的 GORM 模型，共 17 张表：

| 模块 | 表 |
| --- | --- |
| 用户 | `users` |
| 圈子 | `circles`、`circle_members` |
| 内容 | `topics`、`posts`、`post_medias`、`comments`、`likes` |
| 互动 | `check_ins`、`follows`、`messages`、`notifications` |
| 课程 | `courses`、`course_chapters`、`course_progresses` |
| AI 共读 | `ai_conversations`、`ai_messages` |

ENUM 类型字段在 PostgreSQL 中使用 `VARCHAR + CHECK` 约束实现，避免原生 ENUM 类型带来的迁移负担。

## 定时任务调度器

[scheduler/scheduler.go](file:///c:/Users/jamyido/Documents/Coding/YjEduReadingClub/new-server/internal/scheduler/scheduler.go) 使用标准库 `time.Ticker`（1 分钟轮询）实现三个定时任务，无需外部 cron 依赖：

| 任务 | 触发时间（北京时间） | 关联话题 | 说明 |
| --- | --- | --- | --- |
| 每日总结 | 23:00 | `check-in-challenge` | 汇总当日圈子内帖子，AI 生成日精华帖 |
| 每周总结 | 周日 23:30 | `weekly-reading` | 汇总本周帖子，AI 生成周精华帖 |
| 每月总结 | 月末 23:45 | `monthly-reading` | 汇总本月帖子，AI 生成月精华帖 |

每个任务遍历所有圈子，调用 AI 生成总结，以 `AI伴读` 身份发布并置顶（同圈子同话题仅保留最新一条置顶）。任务通过 `tryAcquire` 防止重复运行；AI 调用失败时使用兜底文案。可通过 `POST /api/admin/scheduler/trigger` 手动触发（需管理员 Token）。

## AI 共读

- **Provider 选择**：启动时根据 `AI_PROVIDER` 选择 DeepSeek 或 Coze；留空时按已配置密钥自动探测（DeepSeek 优先）。
- **会话模型**：一个用户对一篇帖子只会有一个 AI 共读会话（`ai_conversations` 唯一索引 `user_id + post_id`），会话内包含多条 `ai_messages`（用户提问 + AI 回复）。
- **自动开启**：用户发帖后由 `TriggerAutoAIConversation` 异步触发，AI 失败时使用预设开场白兜底。
- **手动入口**：`POST /api/posts/:id/ai-conversations` 创建/获取会话，`POST /api/ai-conversations/:id/messages` 发送消息。

## 微信小程序集成

- **登录流程**：小程序调用 `wx.login()` 获取 `code` → 调用 `/api/auth/weapp/login` → 后端通过 `code2session` 换取 `openid` → 返回临时 Token（10 分钟有效）。
- **绑定手机号**：小程序调用 `getPhoneNumber` 获取 `code` → 调用 `/api/auth/weapp/phone` → 后端通过 `getuserphonenumber` 换取手机号 → 创建/绑定用户账号 → 返回正式登录 Token。
- **access_token 缓存**：后端内置内存缓存，有效期 2 小时，提前 5 分钟刷新。

## 部署

```bash
# 编译（Linux 目标）
CGO_ENABLED=0 GOOS=linux go build -o bin/server ./cmd/server

# 运行（生产环境通过系统环境变量注入敏感配置）
./bin/server --env=production
```

生产环境建议：

- 使用反向代理（Nginx）处理 HTTPS 终止与静态资源缓存
- PostgreSQL 与应用同机房部署，降低网络 RTT（跨地域部署是 API 延迟的主要元凶）
- 数据库连接池参数按负载调整（默认空闲 10、最大 100）
- `JWT_SECRET` 使用 32 位以上强随机字符串
- `UPLOAD_DIR` 配置为持久化存储路径
- 通过系统环境变量注入 `DATABASE_URL` / `JWT_SECRET` / `WECHAT_MINI_APP_SECRET` 等敏感信息，覆盖 `.env.production` 中的占位值
