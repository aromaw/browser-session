# Chrome 一键独立会话 · v0.3

工具栏图标的标题是 **New Session**。点击立即获取获授权的当前标签页 URL，向本机 Native Messaging host 发出一次 `new-session` 请求。Go 组件创建新的 temporary 身份，用独立 `--user-data-dir` 启动 Chrome 并看护退出清理。GUI 不参与，也不需要常驻。

## 安装

从同一次成功的 CI 下载你的平台 CLI 和 `browser-session-chrome-extension` artifact，解压扩展，按 README 的四步安装。**以前只有 GUI 的 .app 不包含 CLI host，需要新的 CLI binary。**

本机注册命令：

```sh
./browser-session install-extension --extension-id aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
```

将示例 ID 替换为设置页给出的真实 ID。ID 必须是 32 位 a-p 字母，不接受通配符。该命令使用当前 CLI 的绝对路径注册，只授权这一扩展；不要求管理员权限。安装命令必须在将来使用插件的同一个 OS 用户下执行。

可选参数：

- `--browser "/完整路径/Chrome executable"`：指定要启动的浏览器；默认自动寻找 Google Chrome，不假设源浏览器的 executable。
- `--chrome-data-dir "/当前 Chrome 的 user-data-dir"`：macOS/Linux 上，如果装插件的 Chrome 本身使用自定义数据目录，需要指定该目录。这里不是 `Default` 或 `Profile 1` 子目录。Windows 使用每用户 Chrome 注册表项。
- 全局 `--data-dir PATH`（放在 install-extension 前）：选择管理器的会话目录。安装配置保存在平台默认 browser-session 数据目录；记录自定义会话根目录。

注册位置按 Chrome 官方 [Native Messaging 文档](https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging) 实现：macOS/Linux 为当前 Chrome 数据目录下 `NativeMessagingHosts`；Windows 为 HKCU 的 Chrome NativeMessagingHosts 注册项。Go 直接写 Windows 注册表，无需 PowerShell/Bash 启动脚本。

## 更新与卸载

更新前关闭相关会话，替换固定位置的 CLI，再在 chrome://extensions 点击扩展的“重新加载”。如果移动了 CLI 或扩展目录、扩展 ID 变化，重新执行注册命令。源码更新后 `go build -o browser-session ./cmd/browser-session` 即可，Windows 输出名加 `.exe`。无需重新构建 Wails GUI。

卸载时先在 Chrome 删除扩展；本机的 registration manifest 路径由安装命令输出，可以删除该文件。Windows 还可删除 `HKCU\Software\Google\Chrome\NativeMessagingHosts\io.github.aromaw.browser_session` 注册项。删除原生注册不会删除已有 Session；会话数据需用 CLI/GUI 管理。不要直接删除正在使用的 Profile。

## 生命周期与边界

- 每个点击都是新的临时身份，不复制账号状态、表单内容或页面内存。只有当前 URL 被传给本机并作为 Chrome 启动参数；URL 可能带查询参数，因此不会记录到管理器日志或配置。
- Native Messaging 消息进程处理完一次请求后退出；相同 CLI executable 的 detached supervisor 继续照看浏览器。Windows/macOS/Linux 共用已有的进程锁、身份检查和清理逻辑。
- 新 Profile 默认无扩展。没有强行自动安装插件、改变扩展策略或使用 `--load-extension` 给真实用户窗口注入代码。从原浏览器再次点击即可创建更多身份。
- macOS 关闭最后一个窗口不一定退出 Chrome 实例。必要时在新实例中选择退出，或用 `browser-session close NAME`。当前源浏览器可以继续使用。
- 如果源网页登录地址会重定向，目标依然从新身份访问同一 URL；不保证所有网站允许同一设备多账号。
- 第一版只有 Chrome 注册目标。Chromium/其他品牌的 Native Messaging 注册、企业策略、Chrome Web Store 分发需要后续单独验证。

## 权限与测试

扩展只有 `activeTab`、`nativeMessaging` 权限；没有全站 host permissions、Cookie、webRequest、debugger、内容脚本、外部消息监听或远程服务。消息 schema 只允许 op + URL，后端再次校验；限制输入大小，检查调用方 origin，不接受路径、浏览器参数或 executable 等远程配置。

自动测试覆盖非法 origin、畸形/过大/截断消息、未知字段、URL 注入、响应字节长度、不泄漏错误中的敏感内容，以及原生管道请求后的两个临时会话独立生命周期。**原生管道测试使用编译的模拟浏览器；不等于从真实 Chrome 工具栏加载扩展并点击的端到端测试。** 现有真实 Chrome Storage 隔离测试继续运行。

人工验收：在真实 Chrome 加载扩展并注册；打开测试网站，连续创建两个新身份分别登录；关闭源浏览器后确认新实例仍可用；退出其中一个确认其目录删除、另一个不受影响；重复验证三平台。当前未声称这一人工链路全部实测通过。

## 本次验证记录

代码提交 `9fdc01e` 的 [CI #8](https://github.com/aromaw/browser-session/actions/runs/34814697385) 全部 14 个任务通过：macOS/Linux/Windows 原生进程与消息管道测试、真实 Chrome 无头 Storage 隔离测试；五个 CLI 目标编译；扩展测试与打包；五个 GUI 构建回归。以上不代替真实 Chrome 工具栏的人工端到端验收。
