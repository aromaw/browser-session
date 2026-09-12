# 最小架构与进程策略

## 取舍

Go 足够：标准库提供 CLI、JSON、随机 ID、进程创建和文件操作，核心依赖 `golang.org/x/sys` 用于内核文件锁、macOS sysctl 与 Windows API。CLI 无 CGO，交叉构建无需 C 工具链。GUI 额外使用 Wails v2.15.0 与系统 WebView，Linux/macOS 原生构建需要 C 工具链。Rust 也能实现，但本项目没有值得引入额外抽象、FFI 工具链和编译负担的性能需求。

| 模块 | 职责 |
| --- | --- |
| `cmd/browser-session` | CLI 输入校验、输出 |
| `cmd/browser-session-gui` | Wails 原生窗口；初始化前分派 supervisor |
| `internal/desktop` | 静态管理界面、绑定到同一 Session Manager |
| `internal/browser` | 自动发现、解析 executable、构造参数、直接 `exec.Command` 启动 |
| `internal/session` | 配置、锁、原子写、身份生命周期和清理 |
| `internal/platform` | 三平台路径、锁、原生进程快照、分离控制台、退出请求 |

`Session` 保存随机 ID、可读名字、类型、浏览器 ID/executable、创建时间。数据目录由根目录与 ID 推导；不接受配置中任意 `dataDir`，防止删除被篡改后的外部路径。Persistent 与 Temporary 共用配置结构，Temporary 在清理成功后删除记录。

## 打开

1. 校验 HTTP/HTTPS URL，浏览器参数不可由 URL 注入；不调用 shell。
2. 短暂获取 store 锁，读取配置，检查该 Session 的 lifetime 锁与遗留进程。
3. 原子写入带随机 run ID 的 starting 状态，分离启动同一 binary 的内部 supervisor。
4. supervisor 持有 lifetime 锁直到所有浏览器进程退出，再次检查 run ID、配置和活跃进程。
5. 直接启动指定 executable，分配独立 Unix 进程组；不使用 `open -na`/PowerShell/Bash 启动器。
6. `open` 等待 supervisor 确认浏览器仍在运行后返回，15 秒无法确认则报告错误并保留目录。

Supervisor 不开放 TCP、HTTP 或调试端口。一个 Session 一个看护进程，不引入常驻全局服务。

## 关闭、监视与异常恢复

原始 `cmd.Wait()` 只说明原始 child 退出，不能说明 Chrome 的全部进程退出。因此每 250ms 读取当前用户的原生进程快照：

- Linux：`/proc/*/{stat,cmdline}`，PID、PPID、PGID、启动 tick；排除 zombie。
- macOS：`sysctl kern.proc.uid` 和 `kern.procargs2`，PID、PPID、PGID、启动时间；排除 zombie。
- Windows：Toolhelp32、进程 token SID、GetProcessTimes、NtQueryInformationProcess 的 ProcessCommandLineInformation（Windows 10/11 目标）。Boot identity 由 SystemBootEnvironmentInformation 获取。目标浏览器/已知后代不可读时保留数据；不会因为不相关的受保护系统服务不可读就阻止全部 Session。

所有参数仅用于本机内存中的所属判断，不持久化完整 argv，也不记录浏览器标准输出/错误。记录 PID 与启动标识，避免 PID 重用造成错误归属；重启后不再信任旧启动周期的进程组和进程列表。

所属判断取以下并集并扩展子树：精确 `--user-data-dir`、该目录下 crashpad `--database`、同启动标识的已知进程、Unix 独立 PGID。这支持常规 Chromium 派生进程和正常发行版 exec wrapper。不是任意恶意/自重挂程序的严格进程容器。不得把“轮询看不到”推广成操作系统安全边界。

发现进程快照与本进程 PID/argv 不一致（例如 `/proc` 来自另一 namespace）时，整体失败，不使用空列表执行清理。

关闭请求为带 run ID 的 `close.json`。只有持有真实子进程对象的 supervisor 发出关闭请求；恢复后的孤儿不会按旧 PID 被强杀。Windows 在 Wait 之前捕获并保留独立进程句柄，通过该句柄确认原进程未退出后对其顶层窗口发 WM_CLOSE，Unix 对原始 browser child 发 SIGTERM。确认对话框可阻止退出，15 秒后 CLI 返回提示。

## 文件与锁

- 稳态看护只在进程身份/状态变化后持久化，不把每次轮询当作写盘心跳；临时写盘失败继续持有 lifetime 锁并重试。
- store 锁保护 JSON read-modify-write；临时文件写入、fsync，再 rename/MoveFileEx 替换。
- 每 Session lifetime 锁由内核持有，程序崩溃时释放。锁文件不删除，避免旧 inode 和新 inode 分裂锁。
- 移除时持有 lifetime 锁，再获取 store 锁；Open 在持有 store 锁时只非阻塞尝试 lifetime 锁，避免锁循环。
- 目录使用随机固定格式 ID；删除拒绝符号链接/异常元数据，限制在本工具管理目录以及可推导的平台缓存目录内。
- 创建和启动之间有 30 秒 reservation；无人启动的临时身份之后可回收。
- 浏览器崩溃后照常检查存活后代；看护进程崩溃后由下一次 CLI 的 cleanup 检查；系统重启后对旧 PID 信息作废，重新扫描当前进程。
- 清理失败保留配置，后续 `cleanup` 可重试；没有强制删除活跃 Profile 的逃生开关。

## 后续可加入而不影响核心模型

浏览器发现新增静态注册项/Windows 注册表适配、应用包二进制 plist 支持、经过实测的更多 Chromium 品牌。没有嵌入浏览器或指纹/代理方向。
