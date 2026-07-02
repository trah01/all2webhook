# All2Webhook

All2Webhook 是一个使用 Go 编写的通知集合转发服务。它保留 IMAP 邮件拉取能力，同时提供固定的公开接收 URL，用于接收 GitHub、飞书、Gotify、Bark、Server 酱、自定义 JSON 或其他 HTTP 通知，并根据规则转发到多个 Webhook 渠道。

## 核心能力

- 统一接收入口：每个接收项目生成高强度密钥 URL，例如 `https://all2webhook.example.com/hook/<secret>`。
- 双端口隔离：`8080` 用于 Web 管理界面，`8081` 只用于公开接收通知。
- 多目标转发：一条转发规则可以同时发送到多个飞书、钉钉、企业微信、Slack、Discord 或自定义 Webhook。
- 自动解析：优先解析常见 JSON 与 GitHub Webhook；无法识别时保留完整请求内容，避免漏通知。
- PostgreSQL 存储：服务仅支持 PostgreSQL，Docker Compose 默认启动并配置数据库。
- 邮件兼容：仍支持 IMAP 拉取邮件并复用原有过滤、格式化和转发能力。

## Docker Compose 启动

```bash
cd all2webhook
docker compose up -d --build
```

访问地址：

- 管理界面：`http://localhost:8080`
- 公开接收地址：`https://all2webhook.example.com`

部署前请把 `docker-compose.yml` 中的 `PUBLIC_BASE_URL` 改成自己的实际域名；接收项目生成的 URL 会使用这个域名。

查看状态和日志：

```bash
docker compose ps
docker compose logs -f all2webhook
docker compose logs -f postgres
```

## 接收通知示例

在管理界面的“接收项目”中创建项目后，复制生成的接收 URL：

```bash
curl -X POST 'https://all2webhook.example.com/hook/<secret>' \
  -H 'Content-Type: application/json' \
  -d '{"title":"部署完成","message":"生产环境发布成功"}'
```

然后在“转发规则”中选择该接收项目作为来源，并勾选一个或多个目标渠道。

## 本地开发

需要 Go 1.21 或更新版本，并准备可访问的 PostgreSQL 数据库。

```bash
go mod download
go test ./...
DATABASE_URL='postgres://all2webhook:all2webhook_password@localhost:5432/all2webhook?sslmode=disable' go run .
```

默认本地运行端口：

- 管理端口：`8080`
- 公开接收端口：`8081`

未设置 `DATABASE_URL` 时服务会直接退出。
