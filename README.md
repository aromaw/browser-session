# browser-session

轻量、跨平台的 Chrome / Chromium 独立身份启动器。使用已安装的浏览器，每个 Session 使用独立的 `--user-data-dir`。Go 单文件程序，不嵌入 Chromium。

**MVP / 待实机验收。** 平台测试状态见 [验证记录](docs/validation.md)。账户隔离不代表匿名、指纹隔离或操作系统安全边界。

## 开始使用

需要本机已经安装 **Google Chrome 或 Chromium**。MVP 自动发现这两种浏览器；其他 Chromium 浏览器可手动指定 executable，但尚未作为受支持目标验收。

从本仓库 Actions 中下载对应平台构建产物。解压后，将文件重命名为 `browser-session`（Windows 为 `browser-session.exe`）并放到 PATH。macOS/Linux 还需执行 `chmod +x browser-session`。也可使用当前稳定版 Go 编译：

```sh
git clone https://github.com/aromaw/browser-session.git
cd browser-session
go build -trimpath -o browser-session ./cmd/browser-session
```

Windows 使用 `go build -trimpath -o browser-session.exe ./cmd/browser-session`。

```sh
browser-session browsers
browser-session create personal
browser-session create work
browser-session open personal https://example.com
browser-session open work https://example.com
browser-session temp https://example.com
browser-session temp https://example.com
browser-session list
browser-session close work
browser-session delete work --yes
```

每次 `temp` 都创建随机的新身份，可同时运行。`open`/`temp` 返回终端后，独立看护进程继续跟踪浏览器退出。Persistent 关闭后保留数据；Temporary 确认进程退出后删除。临时会话的名字会打印出来，可用于 `close`。

`open` 一个已运行的 Session 会报“already running”；MVP 不向现有进程转发新的 URL。直接在该 Session 的浏览器窗口中开新标签即可。

## 手动选择浏览器

```sh
browser-session create chromium-work --browser chromium
browser-session create custom --browser "/path/to/browser"
browser-session temp --browser "/path/to/browser" https://example.com
```

macOS 可指定 `.app`（读取 XML `Info.plist` 的 `CFBundleExecutable`），或直接指定真正的 executable：

```sh
browser-session create work --browser "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
```

Linux 使用 PATH 中的原生安装。允许发行版提供的正常 exec wrapper，但不支持自行后台化/脱离进程树的启动脚本、Snap/Flatpak。Windows 自动检查 Program Files、Program Files (x86)、LOCALAPPDATA 和 PATH；手动指定必须是 `.exe`。MVP 没有注册表枚举。

## 命令

| 命令 | 行为 |
| --- | --- |
| `browsers` | 列出发现的 Chrome / Chromium executable |
| `create NAME [--browser …]` | 创建持久化身份，记录选定 executable |
| `open NAME [URL …]` | 启动身份；只接受完整 HTTP/HTTPS URL |
| `temp [--browser …] [URL …]` | 创建并启动随机临时身份 |
| `list [--json]` | 列出身份、状态、浏览器；JSON 额外含数据路径 |
| `status NAME` | 数据目录、状态、最近的清理/启动错误 |
| `close NAME` | 请求此 Session 退出，最多等待 15 秒 |
| `delete NAME --yes` | 永久删除已停止的身份与数据，运行中拒绝删除 |
| `cleanup` | 清理没有活跃进程的遗留临时身份 |
| `version` | 显示版本 |

全局 `--data-dir PATH` 必须写在命令前；也可以设置 `BROWSER_SESSION_HOME`。名称为小写字母开头的 1–48 位字母、数字、`-`、`_`；磁盘使用独立随机 ID，避免路径穿越及 Windows 保留文件名问题。

## 数据目录

| 系统 | 默认根目录 |
| --- | --- |
| macOS | `~/Library/Application Support/browser-session/` |
| Linux | `$XDG_DATA_HOME/browser-session/`，未设置或非绝对路径时使用 `~/.local/share/browser-session/` |
| Windows | `%LOCALAPPDATA%\browser-session\` |

```text
browser-session/
  sessions.json                 # 元数据，含 persistent / temporary
  locks/                        # 内核文件锁的固定 inode，不删除活跃锁
  sessions/<random-id>/
    browser-data/               # 每个 Session 独立
    run.json                    # 启动 ID、启动时间、进程身份，不含 Cookie
    close.json                  # 指定启动 ID 的关闭请求
```

磁盘缓存通过 `--disk-cache-dir` 指向自己的数据目录。Chromium 仍可能推导平台缓存路径，删除会同时处理该 Session 对应的派生缓存目录。请保持 XDG 配置/缓存环境变量稳定，并使用本机磁盘，不要放在网络共享盘。

**登录能否长期保持由网站、Cookie 到期时间及浏览器策略决定。** 本工具保留 Persistent 数据，不承诺网站永不要求重新登录。`sessionStorage` 本来就按标签页/页面会话生存，不承诺关闭后保留。

## 生命周期与安全边界

- Unix 使用 SIGTERM 请求浏览器退出；Windows 使用属于该浏览器 PID 的窗口 `WM_CLOSE`。退出提示或下载可阻止关闭；超时后保留数据，不强杀。
- 关闭最后一个窗口不一定意味着 macOS 应用退出。必要时在该 Session 选择退出，或执行 `close NAME`。默认禁用 Chromium background mode。
- 清理检查原始子进程、已知后代、进程启动时间、用户数据目录参数；Unix 额外跟踪独立进程组。连续一秒没有活跃进程后才自动删除临时数据。
- 看护进程崩溃后，浏览器可继续运行。下一次运行任何管理命令会尝试恢复清理；活跃孤儿保留，手动关闭它后再 `cleanup`。管理器不会按过期 PID 杀进程。
- 进程不可见、权限不足、JSON 损坏、路径有符号链接或正在启动时，拒绝冒险删除。开始阶段预留 30 秒避免并发清理竞争。
- 不要绕过工具手动启动这些受管理目录、换用不同品牌浏览器打开同一目录，或修改 `sessions.json`。任意启动脚本自行重挂父进程、修改参数/进程组不在支持范围；扫描不是内核容器。
- Unix 根目录 `0700`、元数据 `0600`；Windows 根目录设置仅当前用户及 SYSTEM 可继承的 DACL。不会抵御同一 OS 用户或管理员读取数据。
- 临时是“退出后删除磁盘数据”，**不是内存无痕或安全擦除**。下载到目录外的文件、操作系统备份/快照、SSD 残留不在删除范围内。
- 无遥测、联网 API、Cloud Sync、Cookie 内容读取、Cookie 导出、代理、指纹修改、自动化或密码管理。启动器禁用 Chrome Sync；系统浏览器自己的网络行为/更新/遥测仍由浏览器决定。

更多：[方案与开源调研](docs/research.md) · [架构](docs/architecture.md) · [验收步骤和真实测试状态](docs/validation.md)

## 开发与发布

```sh
go test -race -count=1 ./...
go vet ./...
```

真实 Chromium 测试为显式开启，仅操作测试生成的 localhost 数据，不使用 Playwright、Selenium 或 CDP：

```sh
BROWSER_SESSION_TEST_CHROME=/path/to/chrome go test -v ./internal/browser -run TestChromiumStorageIsolation
```

CI 在 macOS / Linux / Windows 执行原生进程测试与 headless Chrome Storage 测试，并构建 `darwin-amd64`、`darwin-arm64`、`linux-amd64`、`linux-arm64`、`windows-amd64`。标记 `v*` tag 会在原生测试通过后生成压缩包、SHA256SUMS 和 **Draft Release**；未配置代码签名/公证。GUI 留待 CLI 验收之后。

MIT License，保留本仓库原有许可证。
