# AGENTS.md

Mrome：单二进制自托管音乐库。Go net/http + Svelte 5 + SQLite（WAL），无框架、无 DI、依赖仅 4 个直接包。改代码前先读本文件与 README 的「项目结构」「数据库设计」两节。

## 命令

```bash
cd web && npm run build      # 构建前端（产物嵌入二进制，须先于 go build）
go build -o mrome.exe ./cmd/mrome
go test ./... && go vet ./...
make dev                     # vite dev server（代理到 127.0.0.1:4533）
```

运行中的服务若 `web_dir` 指向 `web/dist`（默认如此），改前端只需 `npm run build`，不必重启进程。

## 架构地图

| 包 | 职责 |
| --- | --- |
| `cmd/mrome` | 启动、六协议挂载表、优雅退出 |
| `internal/api` | 网页端 REST（cookie 会话）、上传、SPA 托管、Navidrome 握手（ndcompat.go） |
| `internal/subsonic` / `jellyfin` / `audiostream` / `plex` | 四套第三方客户端协议（jellyfin 同时以 `/emby` 前缀服务 Emby） |
| `internal/store` | SQLite 全部读写；`CoverPath` 是唯一的封面解析链 |
| `internal/scanner` / `media` | 目录扫描、标签解析；无 ffmpeg 的时长/码率探测 |
| `internal/util` | MD5 ID、JWT、`WriteJSON` / `ServeMedia` / `ServeImage`（文件出口的唯一实现） |
| `web/src` | Svelte 5 runes；状态集中在 `lib/*.svelte.js`，无状态管理库 |

## 硬规则

1. **改 `store.Song` 的 JSON 标签会同时波及网页端与四套协议**。任何 store 模型变更后，至少冒烟：`/api/songs`、`/rest/ping.view`、jellyfin `/System/Info/Public`、`/emby` 同路径。
2. **文件与封面出口只用 `util.ServeMedia` / `util.ServeImage` / `store.CoverPath`**，不要再写 open→stat→ServeContent 的副本。
3. **每个决定同时服务 UX、DX、AX，且不破坏现有行为**。加依赖、加配置项、加一层抽象都需要说明三者各自得到什么。
4. **文案规则**：标签重复相邻内容 → 删；纯操作反馈（进度/报错/防误操作警告/确认）→ 留。页面若需要说明文字才能被理解，先修布局。界面只有中文（品牌与协议名除外），没有 eyebrow 和双语标签。
5. **设计 token**：圆角用 `--r-sm/md/lg/xl`（Cover 组件的 `rounded="sm|md|lg|full|none"`），状态色用 `--ok/--danger/--warn`，不硬编码色值；共享控件样式（`.genre-select`/`.search-mini`/`.loading`）在 `app.css`，组件内不复制。

## 已知取舍（不要"顺手修复"）

- `users.password_enc` 是可逆的 AES-GCM 密码副本 —— Subsonic `md5(password+salt)` 认证依赖它，去掉即砍协议。
- 兼容层约 40% 代码量是产品功能本身（六协议客户端直连），不是死重。
- 上传全局互斥锁、扫描全量遍历：单用户实例下够用，复杂化前先问有没有真实瓶颈。
- 播放时间戳：网页发毫秒、协议层发秒，`annotations` 混存但排序自洽（见 README 数据库设计）。
