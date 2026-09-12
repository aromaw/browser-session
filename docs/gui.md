# GUI 开发与安装

GUI 使用 Wails v2.15.0 稳定版 + Go + 原生 HTML/CSS/JavaScript。没有 React、npm 运行时依赖、Electron、Rust sidecar 或内嵌 Chromium。系统 WebView 只加载打包的本地管理界面；用户的网站仍由系统 Chrome/Chromium 加独立 user-data-dir 打开。

选择 Wails 而不是 Tauri，是因为已有 Go 核心可以直接复用，无需再维护 Rust 与 IPC sidecar。Wails v3 在本次开发时仍为 beta。参考官方 [平台依赖](https://wails.io/docs/gettingstarted/installation/)、[绑定与窗口配置](https://wails.io/docs/reference/options/)、[原生文件选择器](https://wails.io/docs/reference/runtime/dialog/)。

## 自己编译

使用当前稳定版 Go（模块最低 Go 1.25）。GUI 不需要安装 Node、npm 包或 Wails CLI；前端静态文件直接 embed。构建辅助程序直接调用 Go 编译器并生成可分发 ZIP：

```sh
git pull --ff-only
go run ./scripts/build-gui.go
```

在项目根目录执行。输出在 `dist/`。`-version v0.2.0` 可设置展示版本。`-target darwin-amd64` 等可指定目标；Linux/macOS 使用 CGO，需要相应系统的 SDK，不能像 CLI 一样从 Linux 直接交叉构建所有平台。Windows GUI 可以关闭 CGO 构建。

- macOS：先安装 Xcode Command Line Tools：`xcode-select --install`。生成 `.app`，本地 ad-hoc 签名后打包；未做 Apple 公证。
- Ubuntu 24.04：`sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev`。构建自动使用 `webkit2_41` tag。
- Windows：Go 即可构建；运行需 [Microsoft WebView2 Runtime](https://developer.microsoft.com/en-us/microsoft-edge/webview2/)。可从系统设置检查是否已经安装。

GUI 独立入口：`cmd/browser-session-gui`，需要 `gui,desktop,production` build tags。CLI 原来的构建命令不变，也不需要系统 WebView 开发包。macOS 使用 CLI 时仍是单个无 CGO binary。

## 数据与进程

`internal/desktop.App` 仅绑定会话操作，不提供任意文件读写、Cookie、Shell 或浏览器自动化 API。创建网址在后端再次校验。删除要求填写准确会话名称，后端仍检查内核锁和真实进程。

GUI 启动时异步尝试清理遗留临时会话。状态每三秒刷新，隐藏窗口时暂停轮询；同一轮列表共用进程快照。打开/关闭等待放在 Go 方法调用中，前端保持可操作并显示进行状态，禁止重复点击同一会话。

两种入口都必须先执行 `session.DispatchSupervisor`，再初始化各自的 CLI/UI。GUI 启动的看护子进程再次执行 GUI binary 的 `__supervise` 分支，**不初始化 WebView、不需要额外 CLI 文件**。关掉 GUI 后看护进程仍存在，继续处理临时退出清理。Windows WebView 的 UI 缓存独立存放，不使用任何受管理的浏览器目录。

GUI 默认与 CLI 共用平台数据根目录。需要另外的根目录时，可从终端使用 `browser-session-gui --data-dir <路径>`，或设置 `BROWSER_SESSION_HOME`。从系统应用菜单启动一般不会继承交互式 Shell 配置，请使用默认根目录以保持两者一致。

## 测试

```sh
go test -race ./...
go vet ./...
npm test
```

`npm test` 仅使用 Node 自带测试框架，无需 `npm install`。

CI 的 `python scripts/test-gui.py` 构建带 `guitest` 的专用测试 binary，使用新建临时根目录启动真实系统 WebView，通过真实 Wails 绑定完成新建持久会话（不启动浏览器）、搜索、详情、名称确认删除及空状态检查。Linux 使用 Xvfb。测试代码不进入发布包，不接触真实身份。跨会话 Chrome 生命周期继续由原生 CLI/核心测试验证。

`python scripts/preview-ui.py` 可以生成 `dist/ui-preview/index.html`，供本地浏览器人工检查界面布局。里面的会话全是内存虚构数据，刷新可重置；它没有连接真实后端，也不进入应用。不能把这个预览当成 GUI 原生运行测试。
