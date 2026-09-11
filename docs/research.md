# 方案与现有项目调研

调研时间：2026-09-11。搜索 GitHub 的 `chrome profile manager`、`chromium profile manager`、`browser profile manager temporary`，并检查候选 README、目录与相关启动代码。结论是**当前检查的候选中没有完整符合需求的可直接采用项目**，不是声称互联网上不存在。

## 开源候选

| 项目 | 实现与匹配程度 | 平台、Temporary、维护证据 | 判断 |
| --- | --- | --- | --- |
| [AIMind-ing/ChromeProfileManager](https://github.com/AIMind-ing/ChromeProfileManager) | 启动代码使用 `--profile-directory` 并可附 `--user-data-dir`，macOS `.app` 路径使用 `open -a`；另有备份/恢复与 Wails GUI | README 标明 Win/macOS/Linux，最近提交 2026-07-26；未找到“随机 user-data-dir + 退出清理”承诺 | 最值得进一步参考，但产品重点是现有 Profile/备份 GUI，不能直接满足这个 CLI |
| [rezonanc/chrome-profiler](https://github.com/rezonanc/chrome-profiler) | `chrome-profile.sh` 实际传独立 `--user-data-dir`；核心方向吻合 | Bash + Linux desktop，最近提交 2014-10-21；未实现临时生命周期 | 简单但重写跨平台生命周期比保留脚本框架合理 |
| [jwoyak/plures（README 名 Profilium）](https://github.com/jwoyak/plures) | GNOME Chrome Profile 桌面入口管理 | README 仅记录 Ubuntu 22.10 / Chrome 109 测试；安装与用法占位；最近提交 2023-02-04 | 不具备三平台与临时会话验收基础 |
| [CryptoBusher/Chrome-profiles-manager](https://github.com/CryptoBusher/Chrome-profiles-manager) | Python 菜单式多 Profile，包含 Selenium、扩展与钱包相关脚本 | README 标注 Windows/macOS，最近提交 2025-02-05；未找到完整独立临时会话清理机制 | 功能范围和依赖不合适，也没有 Linux 一等支持证据 |
| [CloakHQ/CloakBrowser-Manager](https://github.com/CloakHQ/CloakBrowser-Manager) | 独立 Profile，但围绕专用指纹修改浏览器，提供代理/自动化与 Web UI | README 称活跃 alpha，桌面/服务器模式；浏览器引擎下载和并发许可证机制 | anti-detect 产品方向不符合；不采用其专用引擎 |

“未找到”表示此次检查未证实，不等于已完整审计全部代码。维护日期来自 GitHub 默认分支最近提交，不代表安全性保证。没有证据足以把任何候选直接作为可靠的 Temporary 生命周期实现，因此不做薄改 Fork；独立小 CLI 的维护面更小。

## 七种方案

| 方案 | 隔离 / 多实例能力 | 三平台与复杂度 | 更新维护 / 安全边界 | 结论 |
| --- | --- | --- | --- | --- |
| Chrome 原生 Profile | Profile 范围的 Cookie/网站存储通常隔离；同一 user-data-dir 共享部分 Local State 与 browser process | 原生支持，管理简单 | 由系统 Chrome 更新；不是完整 user-data-dir 隔离 | 可用于普通多账号，未覆盖此需求的临时身份与进程管理 |
| 独立 `--user-data-dir` | Profile 存储与 Local State 独立；不同目录可独立 browser root，并行运行 | 三平台支持，启动器小 | 使用系统浏览器更新；目录保护/生命周期由本工具负责 | **采用** |
| Chrome Incognito | 同一 Profile 的无痕窗口共享无痕会话；无法逐窗口命名独立身份 | 最简单 | 仍由 Chrome 更新；本身不承诺多个独立临时身份 | 不采用作隔离机制 |
| Electron `session.fromPartition` | 不同 partition 的 session storage/cookies/cache 隔离；`persist:` 决定持久化 | 三平台，但需要自行做浏览器 UI/权限与下载等行为 | 分发并及时更新 Electron/Chromium；Node 集成增加攻击面 | 比需求重 |
| 扩展 Cookie Isolation | Cookie store/API 不等价于完整 IndexedDB、SW、permissions 等独立 browser context | 安装容易，但可靠完整隔离难 | 扩展权限可接触认证数据；MV3 限制与兼容维护 | 不满足 |
| 自嵌 Chromium / CEF | 正确设计独立 RequestContext 可隔离，但仍需自行实现完整浏览器行为 | 三平台工程与发布成本最高 | 自行承担引擎安全更新和 browser UI 安全 | 不值得 |
| Playwright persistent context | 通常使用独立 userDataDir，每目录只能一份实例；可以实现独立存储 | 三平台，自动化依赖和控制协议 | Playwright 与浏览器版本配套；附加调试/自动化接口 | 自动化场景合适，此 CLI 无需引入 |

## 官方机制依据

- [Chromium User Data Directory](https://chromium.googlesource.com/chromium/src/+/main/docs/user_data_dir.md)：用户数据包含 Profile 与 Local State；macOS、Windows、Linux 可用命令行覆盖；Profile 通常是其 `Default` 子目录。文档还说明 macOS/Linux 的 cache 路径推导方式。此工具显式指定 disk cache，并在删除时处理派生 cache。
- [Chromium POSIX process singleton 源码](https://github.com/chromium/chromium/blob/main/chrome/browser/process_singleton_posix.cc)：同目录再次启动可通过 Unix socket 转发给第一份 browser，并退出第二份进程。SingletonLock 使用主机名与 PID；不能只把“有锁文件”当作仍活跃，也不能把第二份进程退出当作整个浏览器退出。
- [Chromium Windows process singleton 源码](https://github.com/chromium/chromium/blob/main/chrome/browser/process_singleton_win.cc)：Windows 采用 mutex/窗口消息与目录相关的进程发现来做单实例协调；并不是依赖 Unix 的 SingletonLock 文件。
- [Electron Session API](https://www.electronjs.org/docs/latest/api/session#sessionfrompartitionpartition-options)：`persist:` partition 为持久化，同 partition 返回同一 Session，无前缀则内存 Session。
- [Playwright BrowserType](https://playwright.dev/docs/api/class-browsertype#browser-type-launch-persistent-context)：persistent context 使用 user data directory；同目录不能同时启动多个 browser 实例。
- [Chrome Incognito 帮助](https://support.google.com/chrome/answer/95464)：无痕会话持续至所有无痕窗口关闭。

这些是技术可行性的文档/源码依据，**不是三平台实际 GUI 测试结果**。网站仍可通过 IP、设备指纹、系统账户单点登录、用户输入或服务器逻辑关联身份。这个工具提供浏览器数据隔离，不提供身份匿名或反检测。
