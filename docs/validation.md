# 验证记录与验收方法

更新：2026-09-13；CI 执行日期：2026-09-12。**尚未声称完成全部 MVP 实机验收。**

## 本次 GUI 与审查改动

- 本地新增 Go/race 测试和前端状态单测通过；受限 PID namespace 的 OS 测试仍显式跳过。未经该跳过运行的原生清理测试在本环境按预期拒绝进程检查，不能记为通过。
- Windows GUI 本地交叉编译与 ZIP 打包通过；GitHub Windows runner 上的真实系统 WebView 和 Go 绑定测试也已通过。
- [最终 CI #34685019364](https://github.com/aromaw/browser-session/actions/runs/34685019364) 全部 13 个任务成功，对应代码提交 `7e1018006010e8b1e5a54bf3ca635261c47a51df`。后续文档提交不改变被测代码。
- CLI 与 GUI 均生成 `darwin-amd64`、`darwin-arm64`、`linux-amd64`、`linux-arm64`、`windows-amd64` 五目标产物。
- Linux amd64/arm64、Windows amd64、macOS arm64 的真实系统 WebView + Wails 绑定 smoke test 均输出 PASS，覆盖创建持久会话（不启动 Chrome）、搜索、详情、名称确认删除、删除和空状态。**这不是通过 GUI 完成 Chrome 登录/退出生命周期的人工验收。**
- darwin-amd64 GUI 在 arm64 macOS runner 交叉构建，不声称 Intel 原生运行。macOS 首轮暴露的 UniformTypeIdentifiers 框架链接问题已修复；最终两架构打包通过，arm64 原生窗口测试通过。
- 当前预览浏览器策略拒绝访问本地文件，未完成截图人工检查。人工桌面验收仍保留在下方清单。
- 专用 smoke binary 的测试代码不会嵌入生产应用；虚构内存预览也不会嵌入生产应用。

## 既有核心验证与本地环境

| 项目 | 结果 |
| --- | --- |
| Go 工具链 | Go 1.27.1 linux/amd64，官方下载 SHA256 已校验 |
| 单元测试、race、文件锁、路径/输入校验 | 通过；与真实进程 namespace 相关的测试显式跳过 |
| 五目标 binary 交叉构建 | 五目标本地交叉构建均通过；无 CGO，单文件约 3.3–3.6 MB |
| Linux 真实 Chrome | Chrome for Testing 153.0.8010.36 下载成功；运行被环境拒绝创建 Unix socket（EPERM），未完成浏览器测试 |
| 本地 `/proc` 进程集成 | 工具环境的 getpid 与 /proc namespace 不一致，测试发现后产品改为拒绝清理；未假装通过 |
| 本地 macOS / Windows 桌面 | 当前没有对应本地桌面；GitHub 原生 GUI smoke test 结果见上节 |
| GitHub 三平台 CI | Linux/Windows 真实 Chrome Storage 测试通过；macOS hosted runner 的 headless display link 不稳定，真实 Storage 测试明确 skip，原生生命周期测试通过；不可将 skip 视为 Storage 已在 macOS 实测 |

受限本地环境使用 `BROWSER_SESSION_SKIP_OS_TESTS=1 go test -race ./...`，只跳过命名明确的 OS 进程测试。CI 不设置这个变量。真实浏览器测试必须显式设置 `BROWSER_SESSION_TEST_CHROME`，否则 Go 输出 SKIP。

## 自动化覆盖

- 单元：参数注入、准确目录匹配、PID 重用、进程子树、互斥文件锁、随机 ID、非法名称、并发创建不丢数据、损坏 JSON 保留、符号链接拒绝、临时身份 reservation。
- 原生 CLI 集成：编译后的测试 binary 模拟多进程浏览器；两个 Persistent 并行、重复打开拒绝、活跃删除拒绝、关闭 A 不影响 B、保留后重新打开、两个 Temporary 并行、父退出而子仍活跃时不删、子退出后删、supervisor 崩溃后活跃目录保留和退出后回收。Windows 使用真正的隐藏测试窗口验证 WM_CLOSE。
- 显式真实浏览器集成：系统 Chrome headless + localhost 测试服务器 + 网页自身执行 JS。两个进程同时运行并等待同一 barrier；验证 Cookie、localStorage、sessionStorage、IndexedDB、CacheStorage、HTTP disk cache、Service Worker、OPFS 的独立性，随后重启检查持久化。只读取测试生成的 synthetic cookie，不读取任何真实身份的数据。
- CI：三 OS 的原生核心测试；Linux/Windows headless Chrome，macOS 此项显式跳过；CLI/GUI 各五目标构建。

## 桌面手工验收（仍须执行）

使用测试账号。每个平台记录 OS、浏览器完整版本、commit SHA 和结果；不要将真实 Cookie/Token 或完整 Profile 上传到 issue。

1. 创建 `a`、`b`；同时 `open a https://目标网站`、`open b https://目标网站`。分别登录两个测试账号，在两边刷新确认各自身份。
2. 各自 `chrome://version` 的 Profile Path 应为不同随机 ID 下的 `browser-data/Default`，不会指向用户已有默认 Chrome 数据目录。
3. 在 A 给测试网站授予一个浏览器网站权限，在 B 检查权限仍是默认。系统层面的摄像头/麦克风权限可能是浏览器应用共享的 OS 授权，不应误认为按 Session 隔离。
4. 在同源页面的 DevTools Application 检查 Cookie、localStorage、IndexedDB、Cache Storage、Service Worker；修改 A 后确认 B 未出现相同数据。sessionStorage 需同标签页观察。
5. `close a`，确认 B 持续在线。重新 `open a`，确认仍为 A（网站可自行过期登录）。
6. 连续执行两次 `temp`；分别登录测试账号。记录各自 `status` 数据路径。关闭第一个后它的数据目录最终消失，第二个保持在线。
7. 关闭全部窗口后若 macOS 应用还在运行，使用该实例的退出操作或 `close`，确认清理只在真正退出后发生。
8. 模拟 browser crash、supervisor crash 和 OS 重启，执行 `cleanup`。必须保留正在使用的临时数据，只移除已停止临时身份；Persistent 一律不自动移除。
9. 测试包含空格和非 ASCII 的 home/executable 路径。Windows 检查 DACL；Linux 检查 XDG_DATA_HOME；macOS 检查派生 cache 也已清除。
10. 同一 Session 重复启动应被本工具拒绝。不同 Session 应为不同 browser root，默认用户 Chrome 完全不受关闭命令影响。

GUI 登录、权限 UI、下载/退出确认、睡眠/恢复、商业网站策略、macOS 应用生命周期仍不能被 headless 或编译测试替代。
