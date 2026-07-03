# All2Webhook

All2Webhook 是一个使用 Go 编写的通知集合转发为webhook服务，统一了大部分的推送渠道。

现在的通知推送渠道实在是太多，而且每个应用或者项目都是只支持部分，甚至于有的想要更多推送渠道还得付费?！

所以我开发了这个通知统一接收入口的项目，允许单个接收链接，接收各种不同格式的webhook推送方式然后转化成你想要的。同时允许配置邮箱，把邮件的通知也统一过来。

目前支持 GitHub、飞书、钉钉、企微、discord、Gotify、Bark等等的规则解析，覆盖市面80%的通知，并根据规则转发到多个不同渠道。

实在是不想存和记录那么多的推送链接了，管你有什么推送渠道，有一个我就转成我自己喜欢的渠道！

## 核心能力

- 统一接收入口：每个接收项目生成高强度密钥 URL，例如 `https://all2webhook.example.com/hook/<secret>`。
- 双端口隔离：`8080` 用于 Web 管理界面，`8081` 只用于公开接收通知。
- 多目标转发：一条转发规则可以同时发送到多个飞书、钉钉、企业微信、Slack、Discord 或自定义 Webhook。
- 自动解析：优先解析常见 JSON 与 GitHub Webhook；无法识别时保留完整请求内容，避免漏通知。
- PostgreSQL 存储：服务仅支持 PostgreSQL，Docker Compose 默认启动并配置数据库。
- 邮件兼容：仍支持 IMAP 拉取邮件并复用原有过滤、格式化和转发能力。

## Docker Compose 启动

```bash
wget -O docker-compose.yml https://raw.githubusercontent.com/trah01/all2webhook/refs/heads/main/docker-compose.yml
修改docker-compose.yml里的公开链接
docker compose up -d
```

访问地址：

- 管理界面：`http://localhost:8080`

  **注意！！管理页面没有做任何鉴权和安全防范措施，请勿对外开放此端口及页面，一定要限制**

- 公开接收地址：`https://all2webhook.example.com`

部署前请把 `docker-compose.yml` 中的 `PUBLIC_BASE_URL` 改成自己的实际域名；接收项目生成的 URL 会使用这个域名。Compose 文件默认使用 `trah01/all2webhook:latest` 镜像，不需要在服务器上克隆源码或本地构建镜像。

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

## 邮件文件夹转发

在管理界面的“邮箱账号”中添加 IMAP 账号后，可以点击“获取可用文件夹”读取邮箱服务器返回的文件夹列表，并在“监听文件夹”中填写一个或多个文件夹名称，多个文件夹用英文逗号分隔，例如：

```text
INBOX,OTHER/测试通知
```

邮件拉取逻辑会按账号配置的检查间隔扫描这些文件夹中的未读邮件。每个文件夹独立建立 IMAP 连接，使用 UID 去重，并且每轮最多处理最新 10 封未读邮件，避免一次性转发过多历史邮件。

首次扫描时，只要邮件仍是未读并且没有被系统处理过，就会进入正常转发流程；不会再因为服务启动时间晚于邮件收件时间而直接忽略。转发是否实际发送，还取决于“转发规则”中的来源、目标 Webhook 和过滤规则是否匹配。

排查邮件没有转发时，优先检查：

- 邮箱账号是否启用，IMAP 服务器、账号和授权码是否正确。
- 监听文件夹名称是否与“获取可用文件夹”返回结果完全一致。
- 邮件是否仍为未读；已读邮件不会被拉取。
- 转发规则的来源是否选择了对应邮箱账号或“所有来源”。
- 过滤规则是否把邮件内容或发送人拦截。
- 管理界面日志中是否出现“检查邮箱文件夹”“收到新邮件”“消息被过滤规则拦截”或“转发成功”等记录。

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
