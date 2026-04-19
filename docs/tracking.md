# Gwen 功能实现跟踪表

> 文档与代码实现对齐状态，确保 design 和 implementation 同步。

## 图例

| 符号 | 含义 |
|------|------|
| ✅ | 已实现 + 文档同步 |
| 🚧 | 部分实现 / 文档占位 |
| ❌ | 尚未实现 |
| 📋 | 设计阶段，未编码 |

---

## 核心语法

| 功能 | 文档 | 代码 | 测试 | 示例 | 状态 |
|------|------|------|------|------|------|
| 变量赋值 `:=` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 不可变绑定 `const` | ✅ | ✅ | ✅ | ✅ | 已实现（支持类型标注、溢出检测） |
| 显式作用域 `global` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 多返回值 | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 类型标注 `: int` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 索引赋值 `arr[i] :=` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 索引多赋值 `arr[i], arr[j] := ...` | ✅ | ✅ | ✅ | ✅ | 已实现（parser 支持） |
| for 循环 | ✅ | ✅ | ✅ | ✅ | 稳定 |
| for `order`/`reverse` | ✅ | ✅ | ✅ | ✅ | 已实现 |
| while 循环 | ✅ | ✅ | ✅ | ✅ | 稳定 |
| if/elif/else | ✅ | ✅ | ✅ | ✅ | 稳定 |
| match 模式匹配 | ✅ | ✅ | ✅ | ✅ | 已实现（强制 ok/err + else） |
| match 作用域对齐 | ✅ | ✅ | ✅ | ✅ | match 与 if/while/for 共享父作用域 |
| match 强制解构 | ✅ | ✅ | ✅ | ✅ | 已实现（Result 类型强制解构） |
| 多行 list 字面量 | ✅ | ✅ | ✅ | ✅ | 已实现（支持换行、末尾逗号、嵌套） |
| 未初始化声明 `x: T` | ✅ | ✅ | ✅ | ✅ | 读前未赋值运行期报错 |
| `var ... endvar` 变量块 | ✅ | ✅ | ✅ | ✅ | 批量声明，推荐置顶 |
| `var default` 零值 | ✅ | ✅ | ✅ | ✅ | 零值表已实现 |
| `var default {expr}` 一键赋值 | ✅ | ✅ | ✅ | ✅ | 支持单项 `:=` 覆盖 |

---

## 类型系统

| 功能 | 文档 | 代码 | 测试 | 示例 | 状态 |
|------|------|------|------|------|------|
| 基础类型 `int/float/string/bool` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 泛型 `list[int]` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 函数类型 `(int) -> int` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 显式精度 `int8/16/32/64` | ✅ | ✅ | ✅ | ✅ | 已实现，文档已对齐 |
| 显式浮点 `float32/64` | ✅ | ✅ | ✅ | ✅ | 已实现，IEEE 754，文档已对齐 |
| 溢出检测 | ✅ | ✅ | ✅ | ✅ | int8/16/32/64 已实现 |
| 类型别名 `type` | ✅ | ✅ | ✅ | ✅ | 透明别名，已实现 |
| 货币类型 `money[Tag]` | ✅ | ✅ | ✅ | ✅ | int64×10_000，带币种 tag |
| 类型转换 `as` | ✅ | ✅ | ✅ | ✅ | 稳定 |

---

## 函数

| 功能 | 文档 | 代码 | 测试 | 示例 | 状态 |
|------|------|------|------|------|------|
| 函数定义 `func` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 多返回值 `-> int, bool` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 默认参数 | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 嵌套函数 | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 匿名函数 `=>` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 命名结束 `endfunc name` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 递归调用 | ✅ | ✅ | ✅ | ✅ | 稳定 |

---

## 模块与并发

| 功能 | 文档 | 代码 | 测试 | 示例 | 状态 |
|------|------|------|------|------|------|
| 模块定义 `module` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 导入 `use` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 导出 `export` | ✅ | ✅ | ✅ | ✅ | 稳定（函数 / 对象 / 类型别名） |
| 预执行语义检查 | ✅ | ✅ | ✅ | ✅ | 已实现（坏导入、坏类型名、坏对象成员、坏调用签名、`self` 约束） |
| 并行 `parallel` | ✅ | ✅ | ✅ | ✅ | Go runtime 已实现真正并发；每条顶层语句独立快照执行 |
| `allowfail` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 获取结果 `=> results` | ✅ | ✅ | ✅ | ✅ | 稳定（按源码顺序收集；表达式语句返回值可见） |

---

## 内存管理

| 功能 | 文档 | 代码 | 测试 | 示例 | 状态 |
|------|------|------|------|------|------|
| 区域语法 `arena` | ✅ | ✅ | ✅ | ✅ | 语法占位（实际无操作，依赖 Python GC；未来编译型版本实现真正内存池） |
| 嵌套区域 | ✅ | ✅ | ✅ | ✅ | 已实现 |
| `in arena` 分配修饰 | 🚧 | ❌ | ❌ | ❌ | 设计冻结，待编译型后端实现 |
| 区域引用 `Arena` | 📋 | ❌ | ❌ | ❌ | 设计阶段（待编译型后端实现） |
| 编译器热点提示 | 📋 | ❌ | ❌ | ❌ | 远期功能 |
| 真正内存池 | 📋 | ❌ | ❌ | ❌ | 依赖运行时架构 |

---

## 其他特性

| 功能 | 文档 | 代码 | 测试 | 示例 | 状态 |
|------|------|------|------|------|------|
| 官方 stdlib 模块导入 `list/string/math/dict/io` | ✅ | ✅ | ✅ | ✅ | 已实现（兼容 builtin 直用，支持 `use module` 与 `use ... from module`） |
| `map/filter/range/enumerate` | ✅ | ✅ | ✅ | ✅ | 已实现（`range` 为闭区间；`enumerate` 返回 `[index, value]` 对） |
| 导航标记 `@tag` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 错误处理 `result/ok/err` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| match 错误处理 | ✅ | ✅ | ✅ | ✅ | 已实现 |
| 单行注释 `//` | ✅ | ✅ | ✅ | ✅ | 稳定 |
| 块注释 `/* */` | ✅ | ✅ | ✅ | ✅ | 稳定（不支持嵌套） |
| **受限对象系统** | ✅ | ✅ | ✅ | ✅ | 已实现（无继承、私有字段、显式 self、`obj.method()` 与 `Object.method(obj)` 等价、构造器 `Object.new(...)`） |

### 受限对象系统规则

- 无继承
- 字段默认私有
- self 显式参数
- 方法调用可还原为函数调用

---

## 近期优先事项

1. **低优先级**：`in arena` 分配修饰
2. **远期**：真正内存池

## 记账：将来可能调整（TODO）

| 项 | 现状 | 未来考虑 |
|---|---|---|
| `readfile` / `writefile` 等命名 | 当前终端 I/O 占着 `read`/`write` | 若终端 I/O 未来移入 `io` 模块，文件版可升格为 `read`/`write` |
| `%` 符号 | 保留，未使用 | 候选用途：百分比字面量（`10%` = 0.10，配合 money 类型） |
| `ok` / `err` 是语法不是函数 | 现状：特殊表达式 | 若升级为一等构造函数需改解释器；暂不动，文档已明示 |
| `type` 上下文关键字 | `type Alias = int` 仍保留 `type` 关键字；`typeof(x)` 为函数 | 观察：若歧义压力大可考虑改为 `alias` 关键字 |

---

## 上次更新

2026-04-19 - Go stdlib 模块边界继续收紧：`os` / `time` 现已从全局 builtin 作用域移除，必须通过 `use os`、`use time` 或 `use ... from os/time` 访问；同时补齐 checker / interpreter 对“未导入直接访问报错”与命名空间导入的回归测试，为后续 `http` 模块接入先把模块边界钉死
2026-04-19 - Go runtime 基础模块起步：新增官方 `os` / `time` 模块导入面，`os.args/cwd/getenv`、`time.sleep/nowunix/nowunixms/nowrfc3339` 已在 Go checker/runtime 落地；CLI 现在支持 `gwen run app.gw arg1 arg2` 透传程序参数。新增 Go checker / interpreter / CLI 回归测试
2026-04-19 - Go 示例入口 smoke 固化：`examples/*.gw` 与各示例应用 `main.gw` 现在纳入 checker 自动回归，避免示例和语义实现继续漂移；巡检中顺手修复 `examples/fibonacci.gw` 的缓存递归签名与缓存初始化
2026-04-19 - Go `result` match 绑定类型前移：checker 现在会把 `when ok(v)` / `when err(e)` 的载荷类型写入分支作用域，并提前拦截 `ok("x")` 这类与已知结果载荷不兼容的模式。新增 3 个 Go checker 回归测试
2026-04-19 - Go `result`-match 语义补齐：checker/runtime 现在都拒绝对 `result` 使用字面量、范围或裸标识符模式，必须写 `ok(...)` / `err(...)`；运行期未匹配且缺少 `else` 的报错也补齐为 exhaustive 提示。新增 1 个 Go checker + 4 个 Go interpreter 回归测试，并验证 `examples/match_strict.gw`
2026-04-19 - Go `result` 泛型语义对齐：checker 现在区分 `ok(...)` / `err(...)` 的载荷类型，支持 `result[T]` 与显式 `result[T, E...]`（其中 `result[T]` 视为 `result[T, string]`）；同时补齐分支合并里的 `ok/err` 结果面，修复 `examples/match_strict.gw` 在 `check` 阶段误报。新增 5 个 Go checker 回归测试，并验证 `examples/match_strict.gw`、`examples/segment_tree.gw`、`examples/file_io.gw`
2026-04-19 - Go definite assignment（第二步）：同一套规则已扩到 `while` / `for`，循环体的新绑定只有在 checker 能证明“至少执行一次”时才会带到块外；当前支持静态识别 `while true`、非空字面量 foreach、以及无冲突步长的范围循环。新增 5 个 Go checker 回归测试，并补 scope 文档中的循环说明
2026-04-19 - Go definite assignment（第一步）：`if` / `match` 的新绑定现在只会在所有继续执行的可达分支都定义时才泄漏到块外；`if` 同时加入轻量常量布尔识别，保留 `if true then ... endif` 这类显然成立的写法。新增 3 个 Go checker 回归测试，并补 scope 文档说明“共享作用域”与“确定赋值”的区别
2026-04-19 - Go checker 分支类型合并收紧：`if` / `match` 对同名变量做分支合并时，已知类型不再静默退化为 unknown；现在会在兼容时求共同类型（如 `int` + `float` -> `float`），在不兼容时直接报错。新增 3 个 Go checker 回归测试，并验证冲突/数值合并两个定向样例
2026-04-19 - Go checker 作用域对齐：`if/while/for/match/arena` 现在按语言文档与 runtime 语义共享父函数作用域，块内新绑定在 `check` 阶段可继续在块外使用；新增 4 个 Go checker 回归测试与 2 个定向 `go run ./cmd/gwen check` 验证样例
2026-04-19 - Go 作用域语义修复：补齐 `global` 的运行时外层赋值逻辑，禁止误改 builtin，并为缺失外层绑定与类型失配补充 checker 拦截；同时修复 `parallel => results` 在 checker 中不可见的问题。新增 8 个 Go 回归测试，并验证 `examples/global_scope.gw` 与定向 `parallel => results` 样例
2026-04-19 - Go `parallel` 运行时落地：每条顶层语句现在会并发执行，并在各自的外层环境快照里运行，避免共享可变局部状态；`=> results` 改为按源码顺序收集真实表达式值，`allowfail` 保留 `ok/err` 结果面；新增 5 个 Go 解释器回归测试覆盖真并发起跑、结果顺序、容错和环境隔离
2026-04-19 - Python 发布收口：补 `pyproject.toml`、包版本号与 `gwen` console script；CLI 升级为 `run/check/repl --version --help`，同时保留 `python -m gwen file.gw` 兼容路径；README 对齐当前语义边界并注明 Python 版 `parallel` 仍为顺序执行；新增 6 个 CLI 回归测试，全套 284 通过
2026-04-19 - 模块口径冻结：明确 Gwen `v0.1` 不支持导入别名（`use foo as bar from mod` / `use mod as m` 均不在当前设计内）；遇到重名时推荐改用 `use module` 保留来源信息
2026-04-19 - 模块导入顺序与循环依赖收口：模块内 `use` 现在必须位于声明区最前面，禁止后置导入修补声明；文件模块循环导入补齐显式回归测试，checker/runtime 两侧都能直接报错；新增 4 个回归测试，全套 278 通过
2026-04-19 - 模块错误模型继续收口：禁止同一模块重复导出同一个运行时名或类型名；`use` 不再静默覆盖当前作用域已有绑定，导入冲突现在直接报错；新增 5 个回归测试，全套 274 通过
2026-04-19 - 模块体收紧为声明区：`module ... endmodule` 顶层现在只允许 `use`、`func`、`object`、`type`（含 `export` 变体），赋值、裸调用和流程控制等执行型语句会在 checker/runtime 两侧同时拒绝；新增 3 个回归测试，全套 269 通过
2026-04-19 - 文件模块边界收紧：`use` 从磁盘加载模块时，模块文件现在必须只包含一个匹配的顶层 `module ... endmodule` 定义；额外顶层语句、函数或副作用会在 checker 与 runtime 两侧同时拒绝，新增 2 个多文件模块回归测试
2026-04-19 - `list` 高阶迭代函数落地：实现 `map/filter/range/enumerate`，同时接入 checker 的回调签名与返回类型检查；更新 stdlib 文档与示例，新增 4 个定向测试和 1 个示例 smoke test，全套 264 通过
2026-04-19 - 官方 stdlib 模块接通：`list/string/math/dict/io` 现在可通过 `use ... from module` 或 `use module` 导入，同时继续保留现有 builtin 直用兼容；新增 3 个测试验证显式导入与命名空间导入，全套 259 通过
2026-04-19 - 标准库边界冻结（v0.1）：明确区分“长期保留内建”的最小集合（`write/read/len/str/int/float/typeof`）、“应下放为官方 stdlib 模块”的集合（`list/string/math/dict/io`），以及必须等编译器/runtime 阶段再做的能力（真实 `parallel`、`arena`、`os`、`net`、`time`、包管理）；文档同时明确当前 Gwen 没有头文件 / `#include`，大多数 stdlib 能力仍默认 builtin 可用
2026-04-19 - 容器语义检查前移：dict 字面量键值类型、显式 `list[T]` 赋值、以及 `append` / `insert` / `get` 等常用 builtin 的容器元素类型错误现在会在运行前拦截；泛型容器兼容性改为“已知参数严格匹配，裸容器保守放行”；新增 5 个测试，全套 256 通过
2026-04-19 - 赋值语义检查前移：变量声明、局部再赋值、函数类型变量赋值、多返回解构赋值现在会在运行前检查数量与明显类型不一致；收紧函数类型兼容性为签名严格匹配；新增 5 个测试，全套 251 通过
2026-04-19 - 返回值语义检查前移：函数 / 方法 / 构造器返回值现在会在运行前检查单返回类型、多返回数量和多返回项类型的一致性；新增 3 个测试，全套 246 通过
2026-04-19 - 函数值语义收口：函数 / lambda 的调用签名现在会在赋值、返回值和高阶函数参数流转中保真；运行前可继续检查经变量中转后的函数调用；lambda 运行时参数也补齐显式类型约束；新增 3 个测试，全套 243 通过
2026-04-19 - 调用签名语义检查补齐：执行前检查函数 / 方法 / 构造器 / lambda 的参数个数与显然类型失配；签名类型按定义处作用域解析，修复模块私有类型别名被导出函数使用时在调用点误报的问题；新增 5 个测试，全套 240 通过
2026-04-19 - 预执行语义检查：新增 name/type/member/module checker；运行前检查 `use`/导出、未知类型、对象成员、方法 `self: ObjectName` 约束；CLI 增加 `python -m gwen check <file>`
2026-04-19 - 模块导出边界补齐：支持 `export object` / `export type`；类型别名改为按环境链解析；`use name from module` 可导入导出对象与类型别名；修复模块私有对象经全局注册表泄漏的问题
2026-04-18 - 受限对象系统实现：`object/endobject` 定义、`new(...)/endnew` 构造器、`func ... endfunc` 方法、`Account{field := value}` 字面量；`obj.method()` 等价于 `Object.method(obj)`；私有字段（仅对象方法绑定的 `self.field` 可读写）；新增 13 个测试，全套 206 通过
2026-04-18 - 语义审计修复：禁止 list/dict 的 < > <= >= 比较（无定义）；验证 money[T] / float 允许；docs/semantics.md 与代码对齐；加 3 个测试，全套 150 通过
2026-04-18 - 严格 bool 条件（Go 风格）：`if`/`elif`/`while`/`and`/`or`/`not`/`as bool` 不再接受 truthiness，必须是 `bool`；同时修复 `and`/`or` 短路求值（之前是 bug，两边都求值）；新增 13 个测试，全套 122 通过；文档说明未来转编译语言后要移到编译期检查
2026-04-18 - `order`/`reverse` 重新定位为"意图声明"（不是断言）：变量边界场景推荐写法，文档移除"小魔法"警告

2026-04-18 - 文件 I/O (`readfile`/`writefile`/`appendfile`)，全部 `result[T]` 风格；命名统一为 compound（`haskey`/`readfile`...），`type(x)` → `typeof(x)`；`{}` 规则更新为 dict 字面量；syntax.md 补 `ok`/`err` 是语法说明、`order`/`reverse` 使用场景；保留 `%` 符号（候选百分比字面量）
2026-04-18 - 实现 `dict[K, V]` 字典类型（`{}` 字面量、缺键报错、`haskey`/`get`/`keys`/`values`）
2026-04-18 - 实现 `var / default / endvar` 变量初始化机制（未初始化读前报错、零值表、一键赋值、单项覆盖、基础类型严格类型校验）
2026-04-18 - 实现 `money[Tag]` 货币类型（int64 定点，4 位小数，币种强隔离，无隐式换汇）
2026-04-18 - 文档补齐：syntax.md 加 const / 索引多赋值 / list 字面量 / order & reverse / match 强制解构；oop.md 将 `decimal` 替换为 `float64`
2026-04-18 - 实现类型别名 `type`（透明别名，继承精度约束）
