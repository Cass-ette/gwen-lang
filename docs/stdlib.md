# Gwen 标准库设计

> 模块化扩展，保持核心精简

## 设计原则

1. **核心最小化**：解释器只内置最基础功能
2. **显式导入**：用 `use` 明确依赖，便于审计
3. **审计友好**：标准库源码可读，不隐藏复杂逻辑
4. **渐进增强**：按需加载，不学Python"batteries included"

## 当前状态（2026-04-20）

Gwen 现在的标准库还处在**过渡态**：

- 语言已经有一批稳定可用的“标准能力”
- 但其中很多能力当前还是**解释器内建**
- `list/string/math/dict/io` 今天仍默认可用，不需要 `use`
- 同时，官方 `list/string/math/dict/io/path/http/json/state/sqlite` 模块名现在也已经可导入
- Go runtime 现在也已经接通第一批基础官方模块：`os` / `time` / `http` / `json` / `state` / `sqlite`
- `os` / `time` / `http` / `json` / `state` / `sqlite` 已经收口为**模块专属能力**，需要显式 `use`
- 也**不需要**任何 `#include` / 头文件 / C++ 风格声明

也就是说，当前 Gwen 更接近：

```
// 今天就能直接写
nums := [1, 2, 3]
append(nums, 4)
write(len(nums))
```

而不是：

```
// 这不是 Gwen 当前的使用方式
#include <list>
use append from list
```

`use ... from module` 现在主要用于**用户模块**和**项目内多文件模块**。  
官方 stdlib 的模块化边界，现在开始明确冻结，并且第一批显式导入形态已经接通。

---

## v0.1 边界决议

下面这张表定义 Gwen 在当前体系下的标准库边界。

### A. 应继续作为语言内建

这类能力要么是语言启动就离不开的基本能力，要么是“像运算符一样基础”的通用原语。  
它们未来即使在编译器里实现，也仍应表现为**默认可用**。

| 名称 | 为什么保留为内建 |
|------|------------------|
| `write` / `read` | 最基础的终端 I/O；脚本语言不开箱即用就失去意义 |
| `len` | 跨 `list/string/dict` 的通用原语，接近语言级操作 |
| `str` / `int` / `float` | 最基础的显式转换 |
| `typeof` | 调试和审计都需要，且语义接近反射原语 |

**结论**：这批能力未来不要求 `use`，继续默认可用。

### B. 应下放为真正的官方标准库模块

这类能力已经稳定、有明确语义，但不必都继续绑死在语言核心里。  
它们未来应该表现为**官方 stdlib 模块**；其中 `list/string/math/dict/io` 仍保留 builtin 兼容层，`os/time/http/json/state/sqlite` 已经先切到显式导入。

| 模块 | 能力 | 现状 |
|------|------|------|
| `list` | `append` `pop` `removeat` `insert` `concat` `sort` `reversed` `map` `filter` `range` `enumerate` | 已 builtin，并已支持官方 `list` 模块导入 |
| `string` | `split` `join` `substring` `startswith` `endswith` `contains` `trim` `replace` | 核心字符串函数已 builtin，并已支持官方 `string` 模块导入 |
| `math` | `abs` `min` `max` `sqrt` `floor` `ceil` | 已 builtin，并已支持官方 `math` 模块导入 |
| `dict` | `haskey` `get` `keys` `values` `items` | 已 builtin，并已支持官方 `dict` 模块导入 |
| `io` | `readfile` `readdir` `writefile` `appendfile` | 已 builtin，并已支持官方 `io` 模块导入 |
| `path` | `basename` `dirname` `joinpath` | 官方模块；提供纯字符串路径辅助，不碰文件系统状态 |
| `os` | `args` `cwd` `getenv` | Go runtime 官方模块；已要求显式 `use` |
| `time` | `sleep` `nowunix` `nowunixms` `nowrfc3339` | Go runtime 官方模块；已要求显式 `use` |
| `http` | `get` `request` `listen` `addr` `wait` `close` `method` `path` `requestbody` `requestheader` `requestcookie` `status` `responsebody` `responseheader` `query` `route` `text` `html` `json` `redirect` `withheader` `withcookie` `static` | Go runtime 官方模块；现在同时提供最小 HTTP client 与 bootstrap 级服务端能力 |
| `json` | `parseobject` `parsearray` `stringify` `objectof` `arrayof` `null` `isnull` | Go runtime 官方模块；当前提供最小 JSON 解析/构造能力 |
| `state` | `cell` `get` `set` `update` | Go runtime 官方模块；提供显式共享状态单元 `cell[T]`，为并发/后端代码收口共享状态语义 |
| `sqlite` | `open` `close` `exec` `query` | Go runtime 官方模块；提供最小 SQLite 落盘主路径，先覆盖本地后端/原型的持久化需求 |

**推荐迁移策略**：

1. `v0.1`：`list/string/math/dict/io` 继续允许默认可用，避免今天的代码全量破坏。
2. `v0.1`：`os/time/http/json/state/sqlite` 先收口为模块专属，验证运行时模块边界和导入模型。
3. `v0.2+`：可以考虑让更多 builtin 只保留兼容别名，逐步鼓励显式导入。

### C. 应等编译器 / runtime 阶段再做

这类能力高度依赖真实运行时、系统接口或内存模型。  
在解释器阶段硬做，容易做成“假能力”。

| 模块 / 能力 | 原因 |
|-------------|------|
| `arena` / `in arena` / `Arena` | 真实区域内存必须和运行时/分配器一起设计 |
| `net` / 原始 socket / 更重的服务端 runtime | 套接字 / 请求生命周期 / 超时 / 并发 / 错误模型都需要在 runtime 层统一定口径 |
| 包管理器 / 第三方模块系统 | 依赖编译器、构建和发布流程一起收口 |

**结论**：`os` / `time` 已经落地，`http` 也已经有最小 client + bootstrap 级服务端面；更深的 `net` / 原始 socket / 生产级服务端 runtime 仍然不要抢跑。

---

## 推荐导入形态（已支持，未来推荐）

今天大多数 stdlib 能力仍是 builtin，所以**`list/string/math/dict/io` 现在不强制导入**。  
但 `os/time/http/json/state/sqlite` 已经要求显式导入。为了让后续模块化迁移平滑，官方文档建议逐步朝下面的形态写；这批导入形态现在已经可用：

```gwen
use append, pop, insert, sort, reversed, map, filter, range, enumerate from list
use split, join, trim, startswith, endswith from string
use abs, sqrt from math
use haskey, get, keys from dict
use readfile, readdir, writefile from io
use basename, dirname, joinpath from path
use args, cwd, getenv from os
use sleep, nowunix, nowunixms, nowrfc3339 from time
use http
use json
use state
use sqlite
```

`http` 当前推荐直接 `use http` 做命名空间导入。  
原因很简单：`http.get` 和现有 `dict.get` 同名，如果在顶层直接 `use get from http`，会和全局 `get` 发生冲突。

`json` 也推荐直接 `use json`。  
原因类似：这批能力本来就是一组强相关操作，命名空间调用 `json.parseobject(...)` / `json.stringify(...)` 比散落到顶层更易审计。

`sqlite` 同样推荐直接 `use sqlite`。  
原因也很直接：`open` / `close` / `exec` / `query` 这类名字太通用，命名空间调用 `sqlite.open(...)` / `sqlite.query(...)` 更稳，不容易把来源信息抹掉。

这不是 C/C++ 的头文件系统，也不是文本 include。

- Gwen **没有** `#include`
- Gwen **没有**头文件声明阶段
- Gwen 的依赖边界靠 `module` / `use`

---

## 命名约定

> **内置标识符全部 compound，不带下划线，与语言关键字风格一致。**

**依据**：语言关键字就是这样——`endfunc`、`endmatch`、`allowfail`、`endparallel` 都是 compound。stdlib 函数属于"内置标识符"同一族，不该两套风格。

**判据**（按优先级）：

1. **单词本身就显然 → 用单词**
   ```
   len(x)  sort(lst, cmp)  keys(d)  split(s, sep)
   abs(n)  sqrt(n)  pop(lst)  trim(s)
   ```

2. **动词被"更默认"的目标占了 → compound 写成一个词区分**

   | 动词 | 默认目标（单字） | 非默认（compound） |
   |------|-----|------|
   | `read` / `write` | 终端 I/O | `readfile` / `writefile` / `readdir` |
   | `append` | 列表 `append(lst, item)` | `appendfile(path, content)` |

3. **单词本身不够明白 → compound 补信息**
   ```
   haskey(d, k)    // 比 has(d, k) 信息量更大
   ```

**禁止**：
- ❌ 下划线命名：`read_file`、`has_key`（破坏统一）
- ❌ 驼峰命名：`readFile`、`hasKey`（Gwen 没有驼峰传统）
- ❌ dot 命名空间：`io.read`（Gwen 用 `use from` 导入，不做 method 分派）

**副作用说明**：`readfile`、`haskey` 这类 compound 读感不如 `read_file`、`has_key` 顺眼——这是为了**全语言风格一致**付出的代价，哲学上明确选择一致性优先。

## 核心内置（长期边界）

| 函数 | 用途 | 不扩展的理由 |
|------|------|-------------|
| `write(...)` | 输出 | I/O 基础，无法省略 |
| `read(prompt)` | 读取一行输入（可选提示语） | I/O 基础，无法省略 |
| `len(x)` | 长度 | 跨类型通用操作 |
| `str(x)` | 转字符串 | 调试必需 |
| `int(x)` | 转整数 | 类型转换基础 |
| `float(x)` | 转浮点 | 类型转换基础 |
| `typeof(x)` | 类型检查 | 调试必需 |

> 说明：上表是**应长期保留为内建**的最小集合。  
> 当前解释器里已经实现的其它 builtin（如 `sort`、`split`、`readfile`）是过渡期安排，长期边界以上面的 `v0.1 边界决议` 为准。

## 标准库模块（已支持导入，推荐写法）

### `list.gw` - 列表操作

```gwen
use pop, insert, sort, asc, reversed, map, filter, range, enumerate from list

// 弹出末尾
last := pop(items)

// 插入
insert(items, 0, "head")  // 在索引0插入

// 排序（返回新列表）
sorted := sort(nums, asc)

// 高阶函数
doubles := map(nums, (x) => x * 2)
evens := filter(nums, (x) => x mod 2 = 0)

// 辅助迭代
ids := range(1, 5)
pairs := enumerate(["a", "b"])
```

**为什么不内置？**
- 除最小 builtin 外，其余列表工具不必都绑死在语言核心
- 保持核心解释器简单

### `string.gw` - 字符串处理

```gwen
use split, join, trim, replace, contains, startswith, endswith from string

parts := split("a,b,c", ",")
text := join(["Hello", "World"], " ")
ok := startswith("docs/stdlib.md", "docs/")
```

### `path.gw` - 路径辅助

```gwen
use basename, dirname, joinpath from path

file := basename("docs/stdlib.md")
parent := dirname("docs/stdlib.md")
full := joinpath(parent, file)
```

- 这是**纯字符串路径 helper**，不做存在性检查，也不偷偷访问文件系统
- 当前约定使用 Gwen 一致的 `/` 分隔语义，适合仓库路径、URL path、静态资源路径等场景
- 想知道文件是否真的可读，还是应该直接 `readfile` / `readdir` 并处理 `err(...)`

### `math.gw` - 数学函数

```gwen
use abs, sqrt, floor, ceil from math

root := sqrt(2.0)
rounded := floor(2.9)
```

### `io.gw` - 文件 I/O

全部返回 `result[T]`，**必须 match 处理**——错误不静默。

```
match readfile("/etc/hosts")
  when ok(content) => write(content)
  when err(e) => write("failed:", e)
endmatch

match writefile("output.txt", "hello")
  when ok(n) => write("wrote", n, "bytes")
  when err(e) => write("failed:", e)
endmatch

match readdir("docs")
  when ok(entries) => write(entries)
  when err(e) => write("failed:", e)
endmatch
```

| 函数 | 签名 | 行为 |
|------|------|------|
| `readfile` | `readfile(path: string) -> result[string]` | 读全文（UTF-8），失败返回 `err(msg)` |
| `readdir` | `readdir(path: string) -> result[list[string]]` | 读取目录项名称列表，失败返回 `err(msg)` |
| `writefile` | `writefile(path: string, content: string) -> result[int]` | 覆盖写，`ok(bytes_written)` |
| `appendfile` | `appendfile(path: string, content: string) -> result[int]` | 追加写，`ok(bytes_written)` |

**设计说明**（为什么没有某些 API）：

- ❌ **没有 `file_exists`**：它会诱导 TOCTOU bug（查过了再读，中间文件被删），而且自身也会失败（权限/IO）。想知道能不能读，就直接 `readfile` 并处理 `err`——这是唯一不骗自己的写法。
- ❌ **没有 `read_lines`**：`split(content, "\n")` 一行替代，不提供"便利糖"。
- `readdir` 只返回名字，不偷偷拼出绝对路径，也不替你递归遍历目录。
- `ok` 载荷带信息（字节数 / 内容），不用 `ok(true)` 这种无信息的废话。

### `os.gw` - 运行环境（Go runtime 已实现）

```gwen
use args, cwd, getenv from os

argv := args()
base := cwd()

match getenv("PORT")
  when ok(port) => write("PORT =", port)
  when err(e) => write(e)
endmatch
```

| 函数 | 签名 | 行为 |
|------|------|------|
| `args` | `args() -> list[string]` | 返回 Gwen 程序参数列表；`gwen run app.gw a b` 会得到 `["a", "b"]` |
| `cwd` | `cwd() -> string` | 返回当前工作目录 |
| `getenv` | `getenv(name: string) -> result[string]` | 读取环境变量；不存在时返回 `err(msg)` |

**设计说明**：
- 这是服务启动器、批处理脚本、配置注入的基础层
- 当前先收最小能力：参数、当前目录、环境变量
- `exit` / 进程控制 / 子进程管理先不急着锁死

### `time.gw` - 时钟与延迟（Go runtime 已实现）

```gwen
use sleep, nowunix, nowunixms, nowrfc3339 from time

write("unix:", nowunix())
write("unixms:", nowunixms())
write("stamp:", nowrfc3339())
sleep(50)
```

| 函数 | 签名 | 行为 |
|------|------|------|
| `sleep` | `sleep(ms: int) -> void` | 休眠指定毫秒数；负数报错 |
| `nowunix` | `nowunix() -> int` | 当前 Unix 秒级时间戳 |
| `nowunixms` | `nowunixms() -> int` | 当前 Unix 毫秒级时间戳 |
| `nowrfc3339` | `nowrfc3339() -> string` | 当前时间的 RFC 3339 字符串 |

**设计说明**：
- 给日志、超时、重试、轮询、简单守护脚本提供最小时间原语
- 当前先做 wall-clock 和 sleep；timer/channel/event-loop 还不急着冻结

### `http.gw` - HTTP 客户端与服务端（Go runtime 已实现，bootstrap 版）

```gwen
use http

match http.get("https://example.com")
  when ok(resp) =>
    write("status:", http.status(resp))
    write("bytes:", len(http.responsebody(resp)))
  when err(e) => write("http failed:", e)
endmatch
```

```gwen
use http

func main()
  headers := dict[string, string]{"Authorization": "Bearer demo", "Content-Type": "application/json"}
  match http.request("POST", "https://example.com/api", "{\"name\":\"Ada\"}", headers)
    when ok(resp) =>
      write("status:", http.status(resp))
      write("trace:", http.responseheader(resp, "X-Trace", "missing"))
    when err(e) => write("request failed:", e)
  endmatch
endfunc
```

```gwen
use http
use json

func handle(req: HttpRequest) -> result[HttpReply]
  matched, params := http.route(req, "/hello/:name")
  if matched then
    return http.json(200, json.objectof("name", params["name"], "lang", http.query(req, "lang", "en")))
  endif
  return ok(http.text(404, "missing"))
endfunc

match http.listen("127.0.0.1:8080", handle)
  when ok(server) =>
    write("serving on", http.addr(server))
    match http.wait(server)
      when ok(code) => write("stopped:", code)
      when err(e) => write("server failed:", e)
    endmatch
  when err(e) => write("listen failed:", e)
endmatch
```

| 函数 | 签名 | 行为 |
|------|------|------|
| `get` | `http.get(url: string, timeoutms: int = 5000) -> result[HttpResponse]` | 发起 GET 请求；只要成功收到 HTTP 响应就返回 `ok(response)`，网络/协议错误才返回 `err(msg)` |
| `request` | `http.request(method: string, url: string, body: string, headers: dict[string, string], timeoutms: int = 5000) -> result[HttpResponse]` | 发起显式 HTTP 请求；适合 POST / webhook / 带鉴权 header 的外部 API 调用 |
| `listen` | `http.listen(addr: string, handler) -> result[HttpServer]` | 启动 HTTP 服务；绑定端口失败返回 `err(msg)`，成功返回 `ok(server)`；每个请求会在独立快照里执行 handler，可并发处理 |
| `addr` | `http.addr(server: HttpServer) -> string` | 读取实际监听地址；`127.0.0.1:0` 这类动态端口场景靠它回读 |
| `wait` | `http.wait(server: HttpServer) -> result[int]` | 等待服务退出；正常关闭返回 `ok(0)`，异常退出返回 `err(msg)` |
| `close` | `http.close(server: HttpServer) -> result[int]` | 关闭服务；成功返回 `ok(0)` |
| `method` | `http.method(request: HttpRequest) -> string` | 读取请求方法 |
| `path` | `http.path(request: HttpRequest) -> string` | 读取请求路径（不含 query string） |
| `requestbody` | `http.requestbody(request: HttpRequest) -> string` | 读取服务端请求体字符串 |
| `requestheader` | `http.requestheader(request: HttpRequest, key: string, fallback: string) -> string` | 读取服务端请求 header；缺失时返回调用点显式给出的 fallback |
| `requestcookie` | `http.requestcookie(request: HttpRequest, name: string, fallback: string) -> string` | 读取服务端请求 cookie；缺失时返回调用点显式给出的 fallback |
| `status` | `http.status(response: HttpResponse) -> int` | 读取响应状态码 |
| `responsebody` | `http.responsebody(response: HttpResponse) -> string` | 读取客户端响应体字符串 |
| `responseheader` | `http.responseheader(response: HttpResponse, key: string, fallback: string) -> string` | 读取客户端响应 header；缺失时返回调用点显式给出的 fallback |
| `query` | `http.query(request: HttpRequest, key: string, fallback: string) -> string` | 读取 query 参数；缺失时返回调用点显式给出的 fallback |
| `route` | `http.route(request: HttpRequest, pattern: string) -> bool, dict[string, string]` | 按显式 pattern 做路径匹配；`:name` 形式的段会绑定到返回参数字典 |
| `text` | `http.text(status: int, body: string) -> HttpReply` | 构造 `text/plain` 响应 |
| `html` | `http.html(status: int, body: string) -> HttpReply` | 构造 `text/html` 响应 |
| `json` | `http.json(status: int, value) -> result[HttpReply]` | 把 Gwen 值编码成 JSON 响应；编码失败返回 `err(msg)` |
| `redirect` | `http.redirect(status: int, location: string) -> HttpReply` | 构造带 `Location` header 的重定向响应；status 必须在 `300..399` |
| `withheader` | `http.withheader(reply: HttpReply, key: string, value: string) -> HttpReply` | 返回一个带附加 header 的新响应值；不直接暴露可变 reply 字段 |
| `withcookie` | `http.withcookie(reply: HttpReply, name: string, value: string) -> HttpReply` | 返回一个附加 `Set-Cookie` 的新响应值；可连续调用添加多个 cookie |
| `static` | `http.static(request: HttpRequest, prefix: string, root: string) -> bool, result[HttpReply]` | 先返回该请求是否命中 prefix；只有命中时第二个结果才表示静态文件读取成功/失败 |

**设计说明**：
- `HttpResponse`、`HttpRequest`、`HttpReply`、`HttpServer` 当前都是官方 opaque 类型；用户通过模块 helper 显式交互，不直接依赖内部表示
- `http.requestbody(...)` 和 `http.responsebody(...)` 故意拆开，避免 `http.body(...)` 这种靠实参类型猜上下文的多态入口
- `http.requestheader(...)` 和 `http.responseheader(...)` 也故意拆开；Gwen 不鼓励再回到一个多态 `header(...)` 入口
- `http.requestcookie(...)` 与 `http.withcookie(...)` 提供浏览器后端最小 cookie 面；cookie 仍然通过显式 helper 访问，不暴露 request/reply 内部字段
- reply 内部 header 现在支持多值，因此 `http.withcookie(...)` 可以连续叠加多个 `Set-Cookie`
- `http.withheader(...)` 返回新 `HttpReply`，而不是暴露 reply 内部可变字段
- `http.redirect(...)` 只接受 `300..399`，避免把普通响应误写成“带 Location 的非重定向”
- `http.request(...)` 用一条显式主路径承载 POST / PUT / PATCH / 自定义 method；当前不再额外引入 `post(...)` 这类同义 helper
- `http.query(...)` 不再偷偷默认 `""`；如果缺参数时该回退成什么，必须在调用点写出来
- HTTP `404/409/500` 这类**已收到响应**的情况不再混进 `err(...)`；是否成功由业务代码显式检查状态码
- 服务端 handler 当前推荐写成 `(HttpRequest) -> HttpReply` 或 `(HttpRequest) -> result[HttpReply]`
- `http.listen(...)` 现在采用“每请求独立快照”模型：普通模块级可变状态不跨请求保留，显式 `cell[T]` 才跨请求共享
- 如果 handler 内只是请求内 fan-out，可以继续显式用 `parallel`；它和请求级并发并不冲突
- 路由当前故意不做“隐式魔法路由器”；推荐先用 `http.route(...)` / `http.path(...)` 配合 Gwen 自己的 `if` / `match`
- `http.static(...)` 先返回 prefix 是否命中，再返回命中后的读取结果；这样路由分发不必再把“没命中”塞进 `err(...)`
- `http.static(...)` 现在只是一层很薄的静态文件 helper，不试图变成全功能 web framework
- 当前刻意不急着冻结 `post` / 请求对象字段暴露 / 流式 body / middleware 这些更重的设计
- 当前推荐用命名空间调用 `http.get(...)`，避免和 `dict.get(...)` 顶层导入冲突

### `state.gw` - 显式共享状态（Go runtime 已实现）

```gwen
use state

counter: cell[int] := state.cell(0)

parallel do
  state.update(counter, (n: int) => n + 1)
  state.update(counter, (n: int) => n + 1)
endparallel

write(state.get(counter))
```

| 函数 | 签名 | 行为 |
|------|------|------|
| `cell` | `state.cell(value) -> cell[T]` | 创建一个显式共享状态单元；初始值会按快照存入 |
| `get` | `state.get(cell: cell[T]) -> T` | 读取当前快照；返回值不会和 cell 内部共享可变别名 |
| `set` | `state.set(cell: cell[T], value: T) -> T` | 写入一个新快照，并返回写入后的快照 |
| `update` | `state.update(cell: cell[T], f: (T) -> T) -> T` | 以原子方式执行“读快照 -> 计算新值 -> 提交新快照” |

**设计说明**：
- `cell[T]` 是当前 Gwen 里**唯一推荐的显式共享状态主路径**
- 默认变量仍然不是共享引用模型；只有你显式写出 `cell[T]`，共享意图才出现在代码表面
- `state.get(...)` / `state.set(...)` / `state.update(...)` 都以快照边界工作，避免把内部可变别名偷偷漏到调用点
- `state.update(...)` 适合计数、会话状态、内存缓存这类需要原子读改写的场景
- 这不是在把 Gwen 推向“哪里都能共享内存”；恰好相反，它是为了把共享状态收口到少数可审计入口

### `sqlite.gw` - 最小 SQLite 持久化（Go runtime 已实现，bootstrap 版）

```gwen
use sqlite
use json

match sqlite.open("/tmp/gwen_notes.db")
  when ok(db) =>
    sqlite.exec(db, "create table if not exists notes(id integer primary key, body text, deleted_at text)", [])
    sqlite.exec(db, "insert into notes(body, deleted_at) values(?, ?)", ["ship it", json.null()])

    match sqlite.query(db, "select body, deleted_at from notes order by id desc limit 1", [])
      when ok(rows) =>
        latest := rows[0]
        write(latest["body"])
        write(json.isnull(latest["deleted_at"]))
      when err(e) =>
        write("query failed:", e)
    endmatch

    sqlite.close(db)
  when err(e) =>
    write("open failed:", e)
endmatch
```

| 函数 | 签名 | 行为 |
|------|------|------|
| `open` | `sqlite.open(path: string) -> result[SqliteDB]` | 打开或创建一个 SQLite 数据库；成功返回 opaque `SqliteDB` |
| `close` | `sqlite.close(db: SqliteDB) -> result[int]` | 关闭数据库；成功返回 `ok(0)` |
| `exec` | `sqlite.exec(db: SqliteDB, sql: string, params: list = []) -> result[int]` | 执行不返回结果集的语句；成功返回受影响行数 |
| `query` | `sqlite.query(db: SqliteDB, sql: string, params: list = []) -> result[list[dict]]` | 执行查询语句；成功返回按列名组织的行列表 |

**设计说明**：
- `sqlite` 先只提供一条很薄的落盘主路径，不引入 ORM、模型层或 SQL 生成器
- SQL 参数现在显式写成 `list`，按 `?` 的顺序绑定；避免再引入一套隐式展开规则
- 参数当前只接受 `int` / `float` / `string` / `bool` / `json.null()`
- 查询结果里的列值会尽量保留 Gwen 基础类型；SQL `NULL` 当前复用 `json.null()` / `JsonNull`
- `exec(...)` 只返回受影响行数；如果你要取自增 id，当前推荐显式再查一次 `last_insert_rowid()`
- `SqliteDB` 是官方 opaque runtime handle；它服务于本地后端、demo、教学站和原型，不试图假装成通用分布式数据库抽象

### `json.gw` - JSON 解析与构造（Go runtime 已实现，bootstrap 版）

```gwen
use json

payload := json.objectof("name", "Ada", "roles", json.arrayof("admin", "ops"), "deleted_at", json.null())

match json.stringify(payload)
  when ok(text) => write(text)
  when err(e) => write("json failed:", e)
endmatch
```

| 函数 | 签名 | 行为 |
|------|------|------|
| `parseobject` | `json.parseobject(text: string) -> result[dict]` | 解析顶层 JSON object；顶层不是 object 或 JSON 非法则返回 `err(msg)` |
| `parsearray` | `json.parsearray(text: string) -> result[list]` | 解析顶层 JSON array；顶层不是 array 或 JSON 非法则返回 `err(msg)` |
| `stringify` | `json.stringify(value) -> result[string]` | 把 JSON 形状的 Gwen 值编码成 JSON 字符串；不支持的值返回 `err(msg)` |
| `objectof` | `json.objectof(k1, v1, k2, v2, ...) -> dict` | 构造异构 JSON object；key 必须是 string |
| `arrayof` | `json.arrayof(v1, v2, ...) -> list` | 构造异构 JSON array |
| `null` | `json.null() -> JsonNull` | 返回 JSON null 标记值 |
| `isnull` | `json.isnull(value) -> bool` | 判断值是否为 JSON null |

**设计说明**：
- 这里刻意没有含糊的 `json.parse(...)`；调用点必须显式声明你期待顶层是 object 还是 array
- `JsonNull` 是官方 opaque 类型，用来承载 JSON 的 `null`，而不是把 Gwen 重新拖回“可空文化”
- `json.objectof(...)` / `json.arrayof(...)` 先解决后端里最常见的异构 payload 构造需求
- 当前不急着冻结 schema 校验、typed decode、streaming parser 这些更重的设计

## 对比：内置 vs 标准库 vs 第三方

| 层级 | 来源 | 稳定性 | 审计要求 |
|------|------|--------|----------|
| 核心内置 | 解释器 | 极稳定 | 必审 |
| 标准库 | 官方模块 | 稳定 | 推荐审 |
| 第三方 | 社区 | 不确定 | 必须审 |

## 实现计划

| 阶段 | 状态 | 内容 | 具体函数 |
|------|------|------|----------|
| **阶段 1** | ✅ 完成 | 核心内置 | `write/read/len/str/int/float/typeof` |
| **阶段 2** | ✅ 完成 | 列表+字符串核心 | **列表**: `sort`, `reversed`, `pop`, `insert`, `concat`<br>**字符串**: `split`, `join`, `substring`, `startswith`, `endswith`, `contains`, `trim`, `replace` |
| **阶段 3** | ✅ 完成 | 数学+字典 | **数学**: `abs`, `min`, `max`, `sqrt`, `floor`, `ceil` ✅<br>**字典**: `dict[K,V]`, `haskey`, `get`, `keys`, `values` ✅ |
| **阶段 4** | ✅ 完成 | 文件+高级迭代+路径辅助 | **文件**: `readfile`, `readdir`, `writefile`, `appendfile` ✅<br>**迭代**: `map`, `filter`, `range`, `enumerate` ✅<br>**路径**: `basename`, `dirname`, `joinpath` ✅ |
| **阶段 5** | 📋 远期 | 包管理器 | 第三方模块支持 |

### 当前 API 细节（已实现）

#### 列表函数（`use from list`）

| 函数 | 签名 | 行为 | 复杂度 |
|------|------|------|--------|
| `append` | `append(lst: list[T], item: T) -> list[T]` | **原地修改**，在末尾追加元素；当前仍保留 builtin 兼容 | 均摊 O(1) |
| `sort` | `sort(lst: list[T], cmp: (T,T)->bool) -> list[T]` | **稳定排序**，返回新列表，原列表不变，**必须显式比较器** | O(n log n) |
| `asc` | 比较器 | 预定义 `(a, b) => a < b` | O(1) |
| `desc` | 比较器 | 预定义 `(a, b) => a > b` | O(1) |
| `reversed` | `reversed(lst: list[T]) -> list[T]` | 返回逆序新列表（名称为 `reversed`，`reverse` 是 for 循环关键字） | O(n) |
| `pop` | `pop(lst: list[T]) -> T` | **原地修改**，移除并返回末尾元素 | O(1) |
| `removeat` | `removeat(lst: list[T], idx: int) -> T` | **原地修改**，移除并返回指定索引元素 | O(n) |
| `insert` | `insert(lst: list[T], idx: int, item: T) -> void` | **原地修改**，在索引处插入 | O(n) |
| `concat` | `concat(a: list[T], b: list[T]) -> list[T]` | 返回新列表（**不修改**输入） | O(a+b) |
| `map` | `map(lst: list[T], f: (T) -> U) -> list[U]` | 对每个元素应用回调并返回新列表 | O(n) + 回调成本 |
| `filter` | `filter(lst: list[T], pred: (T) -> bool) -> list[T]` | 保留谓词为真的元素并返回新列表 | O(n) + 回调成本 |
| `range` | `range(start: int, end: int, step: int = auto) -> list[int]` | 生成**包含 end** 的整数序列；默认方向自动判断 | O(n) |
| `enumerate` | `enumerate(lst: list[T]) -> list` | 返回 `[[index, item], ...]` 结构 | O(n) |

**列表函数哲学**：
- `append`/`pop`/`removeat`/`insert`：**原地修改**，副作用明确
- `sort`/`reversed`/`concat`/`map`/`filter`/`range`/`enumerate`：**返回新列表**，原数据不变
- 审计时区分这两类：看函数名+文档，知道是否修改输入

**sort 设计哲学**：
- ✅ **必须显式比较器**：无默认排序规则，不写 `cmp` 报错（审计友好）
- ✅ **返回新列表**：原列表不变，数据流可追踪
- ✅ **稳定排序**：相等元素相对顺序保持（Timsort）
- ✅ **预定义比较器**：`asc`/`desc` 显式引入，减少重复但意图明确

```gwen
use sort, asc, desc from list

sorted := sort(nums, asc)                    // 升序
sorted := sort(nums, desc)                   // 降序
sorted := sort(users, (u1, u2) => u1.score < u2.score)  // 自定义字段
```

#### 字符串函数（`use from string`）

| 函数 | 签名 | 行为 | 边界 |
|------|------|------|------|
| `split` | `split(s: string, sep: string) -> list[string]` | 按分隔符拆分 | `sep` 为空时按字符拆 |
| `join` | `join(parts: list[string], sep: string) -> string` | 用分隔符连接 | 空列表返回空串 |
| `substring` | `substring(s: string, start: int, end: int) -> string` | 提取子串 [start, end] **双闭区间** | 越界报错 |
| `startswith` | `startswith(s: string, prefix: string) -> bool` | 前缀检查 | 空前缀返回 `true` |
| `endswith` | `endswith(s: string, suffix: string) -> bool` | 后缀检查 | 空后缀返回 `true` |
| `contains` | `contains(s: string, substr: string) -> bool` | 子串存在检查 | 空串视为包含 |
| `trim` | `trim(s: string) -> string` | 去首尾空白（space/tab/newline） | 返回新字符串 |
| `replace` | `replace(s: string, old: string, new: string) -> string` | 替换所有出现 | 无匹配返回原串（但仍是新字符串对象） |

**字符串函数哲学**：
- 字符串**不可变**，所有函数返回新字符串，无副作用
- `substring` 越界**报错**（不截断），符合"错误不静默"
- `join` 自动 `str()` 转换元素，方便数字拼接

#### 路径函数（`use from path`）

| 函数 | 签名 | 行为 | 边界 |
|------|------|------|------|
| `basename` | `basename(path: string) -> string` | 取最后一个路径段 | 空串返回空串 |
| `dirname` | `dirname(path: string) -> string` | 取父路径字符串 | 单段路径返回 `.` |
| `joinpath` | `joinpath(left: string, right: string) -> string` | 连接两个路径段 | 自动清理多余 `/` |

**路径函数哲学**：
- 只处理**路径字符串拼装与切分**
- 不做 `exists()`、`isdir()` 这类会诱导 TOCTOU 的“先查再做”接口
- 这套 helper 先服务于仓库路径、静态资源路径和最小后端场景；更重的文件系统抽象不急着冻

**待实现**：`split`/`join` 是否提供 `limit: int` 参数（限制分割次数）？暂不实现，按需再加。

## 与 OOP 的关系

Gwen **不采用 OOP**（类、继承、方法调用），标准库保持**函数式**接口：

```
// Gwen 风格（函数式）
append(lst, item)
sort(lst, asc)

// 不是 OOP 风格
lst.append(item)
lst.sort()
```

理由：
1. 函数调用显式传参，审计时一目了然
2. 没有隐式的 `this` 状态修改
3. 数据和行为分离，符合 "显式优于隐式" 原则
