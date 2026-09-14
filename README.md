# browser-session

轻量、跨平台的 Chrome / Chromium 独立身份启动器。使用已安装的浏览器，每个 Session 使用独立的 `--user-data-dir`。Go CLI + Wails 桌面 GUI，不打包 Chromium 引擎。

**MVP / 待实机验收。** 平台测试状态见 [验证记录](docs/validation.md)。账户隔离不代表匿名、指纹隔离或操作系统安全边界。

## 推荐入口：Chrome 一键 New Session

下载新的 CLI 和 `browser-session-chrome-extension` 构建产物。插件需要一次性注册本机组件，日常无需打开 GUI：

1. 将 CLI 可执行文件放到固定目录（例如你自己的工具目录），不要从会自动清理的临时目录运行。
2. 解压扩展，打开 `chrome://extensions`，启用“开发者模式”，点“加载已解压的扩展程序”，选择含 `manifest.json` 的目录。
3. 扩展会打开设置页。复制其中带有你的扩展 ID 的安装命令，在 CLI 所在目录执行：

   ```sh
   ./browser-session install-extension --extension-id 扩展ID
   ```

   Windows 使用 `.\browser-session.exe install-extension --extension-id 扩展ID`。macOS/Linux 如需添加权限，先执行 `chmod +x browser-session`。
4. 将插件固定到 Chrome 工具栏。停留在目标网页，点击 **New Session** 图标，就会在独立临时 Profile 的新窗口打开当前网址。

每次点击创建新身份，不需要命名，不复制 Cookie。退出新 Chrome 实例后自动清理，原页面不受影响。新 Profile 不会自动安装这个扩展；需要继续创建身份时回到原窗口点击。支持 HTTP/HTTPS 网页，不支持 Chrome 内部页面或本地文件。

[插件安装、更新和卸载说明](docs/extension.md)。这不是 Chrome 商店发布版本，需要加载已解压扩展；组织策略可能禁止开发者模式或 Native Messaging。

## 桌面 GUI

现在提供中文桌面管理界面：新建持久/临时会话、打开、关闭、搜索/分类、查看目录与错误、确认后删除、清理遗留临时会话。GUI 与 CLI 共用同一套 Session 数据和进程看护逻辑；关闭管理窗口不会关闭正在运行的浏览器。

**安装无需 Go、Node 或重新编译：** 登录有权访问本私有仓库的 GitHub 账号，进入 [Actions](https://github.com/aromaw/browser-session/actions/workflows/ci.yml)，打开最新成功的 CI，下载相应 `browser-session-gui-<平台>-<架构>` artifact。先解开 artifact 外层 ZIP，再解开里面的 GUI 安装包。

| 系统 | 下载产物 | 安装与启动 |
| --- | --- | --- |
| macOS Apple Silicon | `browser-session-gui-darwin-arm64` | 将 `Browser Sessions.app` 拖入“应用程序”后双击 |
| macOS Intel | `browser-session-gui-darwin-amd64` | 同上 |
| Windows x64 | `browser-session-gui-windows-amd64` | 解压到自己的应用目录，双击 `browser-session-gui.exe`；可创建桌面快捷方式 |
| Linux x64 / ARM64 | `browser-session-gui-linux-amd64` / `linux-arm64` | 安装 GTK 3、WebKitGTK 4.1 后，`chmod +x browser-session-gui`，直接运行 |

GUI 基线：macOS 13+、Windows 10/11（需 Microsoft WebView2 Runtime）、Ubuntu 24.04 或提供兼容 GTK 3/WebKitGTK 4.1 的 Linux。Ubuntu 运行依赖：`sudo apt install libgtk-3-0t64 libwebkit2gtk-4.1-0`。Linux ZIP 附带 `.desktop` 与 SVG 图标；放入自己的 `~/.local/share/applications/` 和 `~/.local/share/icons/`，并将 binary 放入 PATH 后可从应用菜单启动。

当前没有 Developer ID 签名/Apple 公证，也没有 Windows 代码签名，首次运行可能被系统拦截；请核对来源后按系统提供的单个应用允许流程处理，不要关闭系统整体安全保护。macOS 包只有 ad-hoc 签名。构建与真实测试范围见 [验证记录](docs/validation.md)。

更新时下载新的 GUI 包并替换旧应用即可；会话数据在独立的平台数据目录中保留。替换前关闭管理窗口和所有由它启动的会话，避免 Windows 的运行文件锁。私有仓库保持私有即可，安装后的程序离线管理本机数据，不需要 GitHub 登录。尚未设置公开自动更新服务。

[已验证构建及 GUI 下载](https://github.com/aromaw/browser-session/actions/runs/34685019364) · [GUI 源码构建与设计说明](docs/gui.md) · [本次审查记录](docs/review.md)

## CLI 开始使用

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

- Unix 使用 SIGTERM 请求浏览器退出；Windows 持有原始浏览器的独立进程句柄，在确认该进程仍存活后发送窗口 `WM_CLOSE`，避免等待线程释放句柄后的 PID 重用。退出提示或下载可阻止关闭；超时后保留数据，不强杀。
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
npm test # 可选：Node 22+，仅前端状态单测，无 npm 依赖
```

真实 Chromium 测试为显式开启，仅操作测试生成的 localhost 数据，不使用 Playwright、Selenium 或 CDP：

```sh
BROWSER_SESSION_TEST_CHROME=/path/to/chrome go test -v ./internal/browser -run TestChromiumStorageIsolation
```

CI 在 macOS / Linux / Windows 执行原生进程测试，Linux/Windows 执行真实 headless Chrome Storage 测试（macOS hosted runner 的该项明确跳过），并构建 `darwin-amd64`、`darwin-arm64`、`linux-amd64`、`linux-arm64`、`windows-amd64`。标记 `v*` tag 会在原生测试通过后生成压缩包、SHA256SUMS 和 **Draft Release**；未配置代码签名/公证。GUI 也构建五个目标的 ZIP；真实原生窗口测试单独运行。

MIT License，保留本仓库原有许可证。
