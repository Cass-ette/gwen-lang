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
| 显式共享状态 `cell[T]` | ✅ | ✅ | ✅ | ✅ | 已实现；配合 `state` 模块使用，默认变量模型仍保持隔离/本地 |

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
| 官方 stdlib 模块导入 `list/string/math/dict/io/http/json/state/sqlite` | ✅ | ✅ | ✅ | ✅ | 已实现（兼容 builtin 直用，支持 `use module` 与 `use ... from module`；`http/json/state/sqlite` 当前推荐命名空间导入） |
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

2026-04-22 - 第一版 C emitter 继续前推：函数类型标注这层现在已和多返回语义对齐，显式函数类型如 `(int) -> int, int` 可以稳定穿过 parser / checker / HIR / compiled function-value 路径；同时 `list` 高阶 helper 的 compiled subset 也再补一截，`map/filter/range/enumerate` 现在都能走 `gwen emit-c -> cc` 这条真实编译路径。当前 `map/filter` 先诚实收口到 typed `list[T]` + 非捕获 callable，`enumerate` 先返回 bare list 的 `[index, item]` 对结构
2026-04-22 - 第一版 C emitter 继续前推：非闭包函数值现在也开始能走 `gwen emit-c -> cc` 这条真实编译路径；当前先覆盖顶层函数值、对象类型静态方法值、高阶函数参数、函数槽位调用，以及推断得到的多返回函数值槽位。后端会为每种 Gwen 函数签名生成稳定的 C function-pointer typedef，并补上 tuple-return callable 的 C struct forward；同步把绑定实例方法值明确收口为暂不支持，避免在 compiled 路径里假装闭包已存在
2026-04-22 - 第一版 C emitter 继续前推：受限对象系统现在也开始能走 `gwen emit-c -> cc` 这条真实编译路径；当前已接通对象实例的堆上表示、`Object.new(...)`、实例/静态方法调度，以及方法内 `self.field` 读取和写回，确保 `self.n := self.n + 1` 这类修改不会在 compiled 路径里因按值传递丢失。同步补上编译后端与 HIR binding 回归，真实输出已覆盖 `1 2` 与模块导出对象样例 `7`
2026-04-22 - 第一版 C emitter 继续前推：显式共享状态 `cell[T]` 现在也开始能走 `gwen emit-c -> cc` 这条真实编译路径；当前已接通 `state.cell/get/set`，并先把 `state.update(...)` 收口到“命名函数引用”这条诚实可编译表面。同步补上编译后端烟雾与快照语义回归，真实输出已覆盖 `1 2 3 3` 与 `1 1 / 1 7`
2026-04-22 - 第一版 C emitter 继续前推：局部未初始化变量现在不再依赖 C 的未定义行为，编译后二进制会继续保留 Gwen 自己的 `'x' read before assignment` 运行时语义；同时显式 `var ... endvar` 变量块也开始与已存在的 MIR 指令流对齐，至少基础声明/后续赋值路径已经能真实编译运行。同步补上 emitter 回归测试
2026-04-21 - 第一版 C emitter 继续前推：前端现在会把非 stdlib 的文件模块一起带进 lowering，C 后端也开始支持“入口文件 + 用户模块函数”这层真实跨文件编译路径；当前已验证 `use helper` / `use triple from helper` / `helper.triple(...)` 这几类表面都能走 `gwen emit-c -> cc -> run`。同步补上前端与 emitter 单测，并用真实烟雾验证得到输出 `21 24`
2026-04-21 - 第一版 C emitter 继续前推：`io.readfile/readdir/writefile/appendfile` 这层无 handle、直接返回 `result[...]` 的文件 I/O 现在也能走 `gwen emit-c -> cc` 这条真实编译路径；`readdir` 当前在后端里会显式过滤 `.` / `..` 并按名字排序，尽量保持与现有 Go bootstrap 语义一致。同步补上 Go 单测与真实烟雾验证，编译后的文件 I/O 样例实际输出 `write 6`、`append 5`、`content alpha beta` 与 `/tmp` 目录项计数
2026-04-21 - 第一版 C emitter 继续前推：字符串这层纯 helper 又扩了一步，`split/join/substring` 与字符串 `+` 现在也能走 `gwen emit-c -> cc` 这条真实编译路径；当前 `join` 先覆盖编译后端已诚实支持的基础标量列表显示面（如 `list[string/int/float/bool]`）。同步补上 Go 单测与真实烟雾验证，组合样例编译运行输出 `3 b a-b-c we`
2026-04-21 - 第一版 C emitter 继续前推：`string/math/path` 这批纯 helper 现在也能走 `gwen emit-c -> cc` 这条真实编译路径；当前先覆盖 `startswith/endswith/contains/trim/replace`、`abs/min/max/sqrt/floor/ceil`、`path.basename/dirname/joinpath`，并同时支持 builtin 直用、`use ... from module` 和 `use module` + `module.name(...)` 这几类现有 stdlib 表面。同步补上 Go 单测与真实烟雾验证，组合样例编译运行输出 `hi hi`、`true true true hi`、`3 3 5 apple banana 2 2 3`、`stdlib.md docs docs/stdlib.md`
2026-04-21 - 第一版 C emitter 继续前推：显式 stdlib 导入开始和 compiled subset 对齐，顶层 `use` 不再默认把当前已支持的 stdlib 调用挡死；至少 `use ... from dict` 现在已经能和 `dict` 的已编译 helper 一起走通真实编译路径。同步补上 Go 单测与真实烟雾验证，`use keys, get from dict` 的求和样例编译运行输出 `175`
2026-04-21 - 第一版 C emitter 继续前推：`dict` 的迭代 helper 继续接通，`keys(...)` 与 `values(...)` 现在也能走真实编译路径，并且能和既有 `for each` / `dict index` 拼起来工作；后端会为每种 dict 形状额外生成复制型 list helper。同步补上 Go 单测与真实烟雾验证，`keys(scores)` + `values(scores)` 的求和样例编译运行输出 `350`
2026-04-21 - 第一版 C emitter 继续前推：`PlaceIndex` 的 store 现在也开始能走真实编译路径，当前先覆盖 `list[i] := v` 与基于基础命名键值类型的 `dict[k] := v`；dict 写入语义保持“命中更新、缺 key 插入”，后端会为每种 dict 形状生成最小 `set` helper。同步补上 Go 单测与真实烟雾验证，组合样例编译运行输出 `7 95 85 2`
2026-04-21 - 第一版 C emitter 继续前推：基于基础命名键值类型的 `dict[K, V]` 现在也开始能走 `gwen emit-c -> cc` 这条真实编译路径；后端当前先覆盖 dict 字面量、`len(dict)`、`d[k]`、`haskey(...)` 与 `get(...)`，并为每种 dict 形状生成最小 C struct + 线性查找 helper。同步补上 Go 单测与真实烟雾验证，`dict[string, int]` / `dict[int, string]` 组合样例编译运行输出 `90 2 false 0` 与 `alice false none`
2026-04-21 - 第一版 C emitter 继续前推：基础标量上的 `match value` 现在也能走 `gwen emit-c -> cc` 这条真实编译路径；后端当前先覆盖 `int/float/bool/string` 的字面量模式、capture 模式，以及 `int` 的 `a to b` 区间模式。同步补上 Go 单测与真实烟雾验证，组合样例编译运行输出 `mid / f / p / ok`
2026-04-21 - 第一版 C emitter 继续前推：`result[T, E]` 和 capture-style `match result` 现在也能走 `gwen emit-c -> cc` 这条真实编译路径；后端为 `result` 生成最小 C struct，并把 `match` 先收口到 `ok(name)` / `err(name)` 这类显式解构分支。同步补上 Go 单测与真实烟雾验证，`safe_div` 的 `ok` 路径输出 `5`，`err` 路径输出 `division by zero`
2026-04-21 - 第一版 C emitter 继续前推：`len(list|string)` 与 `list/string` 索引读取现在也能走 `gwen emit-c -> cc` 这条真实编译路径；后端补上了最小字符串 helper、列表边界检查和读取型索引 lowering。同步补上 Go 单测与真实烟雾验证，`[4,5,6]` 和 `"go"` 的组合样例编译运行输出 `3 5 2 o`
2026-04-21 - 第一版 C emitter 继续前推：基于基础标量元素的 `list[T]` 字面量与 `for each` 现在也能走 `gwen emit-c -> cc` 这条真实编译路径；后端会为列表生成最小 C struct，并把 `for each` lower 成按索引推进的循环头。同步补上 Go 单测与真实烟雾验证，`[10, 20, 30] with index idx` 求和程序编译运行输出 `63`
2026-04-21 - 第一版 C emitter 继续前推：数值 `for i in a to b [step n]` 现在也能走 `gwen emit-c -> cc` 这条真实编译路径；同时修正 HIR binder 在子块里把 `:=` 误绑定成新局部的问题，让循环/条件里的外层变量更新不再在后端里悄悄丢义。真实烟雾验证：`1..5` 求和程序编译运行输出 `15`
2026-04-21 - MIR loop 输入求值位置修正：`for range` 的 `start/end/step` 和 `for each` 的 `iterable` 现在会在 preheader 先求值，再进入循环头；不再把这些输入值放在 header block 里每轮重算。这让 Gwen 的 MIR 在循环语义上终于和解释器对齐，后端也不用再猜“边界是不是每轮都该重新求值”
2026-04-21 - 第一版 C emitter 起步：新增 `internal/backend/cgen` 与 CLI `gwen emit-c <path>`，开始把当前 MIR 的 primitive 子集直接发成 C 源码；当前先覆盖 `int/float/bool/string`、局部槽位、算术/比较、`if/while` 已 lower 的 CFG、单返回/多返回顶层函数，以及最小 builtin `write`。同时补上 Go 单测，并用真实 `emit-c -> cc -> 运行` 烟雾验证得到 `42`
2026-04-21 - MIR instruction 层接通：`internal/mir` 的 `Block` 现在除了旧的 metadata `Op`，也开始生成真实执行顺序的 `Inst`，先覆盖 `ComputeInst/CallInst/StoreInst/DeclareInst`；`Assign/Var/Return/ExprStmt/Global` 和 `if/while/for/match` 的关键输入值，都会在 terminator/写入前先显式发出计算。这让 Gwen 的 MIR 第一次开始像真正后端可消费的 block-local 指令流
2026-04-21 - MIR 赋值目标开始显式化：在前一层 `Value` 表之外，`internal/mir` 现在也开始生成稳定 `Place` 表，先覆盖 `slot / index / field` 三类真实写目标；`AssignOp` 会挂 `TargetPlaceID`，`VarOp` 会挂声明目标 place，`GlobalOp` 也会显式指向捕获槽位。这让 Gwen 的 MIR 不再只是“值图 + 原始 target 表达式”，而开始拥有后端可直接消费的写入目标骨架
2026-04-21 - MIR 显式值层起步：`internal/mir` 的每个 `Body` 现在除了 `Slot` 表，也开始生成稳定的 `Value` 表；`AssignOp` / `VarOp` / `ExprOp` / `GlobalOp` / `ReturnTerm` / `CondTerm` / `MatchTerm` / `for` 头部这些节点会同步挂上 `ValueID`。当前已能显式 lower 常量、slot/binding 引用、调用、多返回 `call_result`、成员访问、索引、cast、list/dict/object literal、以及常见 unary/binary 表达式。这样 Gwen 的 MIR 开始真正拥有“值图骨架”，后面可以继续往更稳定的 temp / instruction 形式收口
2026-04-21 - MIR 可靠类型传播继续前推：`internal/mir` 现在不只会给参数/局部槽位回填第一批类型，还开始理解更多“不会把 checker 再抄一遍”的稳定来源，包括本地函数签名、对象构造器/方法/字段、模块函数、常见二元/一元表达式、索引取值、以及 `http.route/http.static/state.cell/state.get/state.set` 这类真实后端调用。多返回调用现在也会把类型拆到对应赋值 target 上，typed MIR 不再只停在 slot 轮廓
2026-04-21 - MIR slot typing 起步：`internal/mir` 现在不只生成 `Slot` 表，还会开始给 slot 回填第一批可靠类型信息，优先覆盖参数、显式局部声明、外层捕获、简单赋值推断、`for each` 项类型、以及 `match result` 的 `ok/err` 捕获类型。这样 Gwen 现在已经不只是“有局部槽位”，而是开始拥有一层很薄但真实可用的 typed MIR 落脚点
2026-04-21 - MIR 开始拥有稳定槽位：`NameBinding` 现在带稳定 `BindingID`，`Param` / `Var` / 循环变量 / `parallel => results` 等定义点也会显式挂上 binding；`internal/mir` 的每个 `Body` 现在会生成自己的 `Slot` 表，先区分 `param/local/capture` 三类，并用 `BindingID` 把同名遮蔽和外层捕获稳定分开。这让 Gwen 的 MIR 不再只是控制流骨架，而开始拥有后端可直接依赖的局部存储形状
2026-04-20 - HIR/MIR target binding 继续前推：`Ident` 绑定现在带词法 `ScopeDepth`，`global` 会显式绑定到外层 target，`match` 会区分 `value/result` 形状并记录 `ok/err/capture/range/value` pattern kind；`internal/mir` 现在会把这层 metadata 一并带进 `GlobalOp` / `MatchTerm`，减少后端继续回头猜作用域和分支形状
2026-04-20 - MIR 起步：新增 `internal/mir`，开始把函数体、对象构造器/方法体和顶层脚本语句块 lower 成显式 `Block + Terminator` 结构；当前 `if/while/for/match/return/leave/next` 已开始收口到控制流骨架，`parallel` 也显式成“每条顶层语句一个独立 branch body”。前端 `internal/frontend` 现在在产出 HIR 后会继续产出第一版 MIR，但表达式和值类型仍复用 HIR，typed MIR 与 runtime ABI 还没开始冻结
2026-04-20 - HIR 第一层名字绑定起步：新增 `internal/hir` binder，`Ident` 现在会绑定到 `local/param/func/module/imported/object_type/builtin`，`Member` 会绑定到 `module_value/object_method/object_constructor/object_field`；前端 `internal/frontend` 现在在产出 HIR 后会同步做这一步 binding。当前仍未做类型化 binding 和分支/循环后的 definite-assignment 合流，但 Gwen 已经不再只是“结构化 AST”
2026-04-20 - HIR 扩到表达式层：`internal/hir` 现在会把 `call/member/index/as/list/dict/object literal/lambda/ok/err` 等表达式也 lower 成独立 HIR 节点，`return` 也改成显式多值切片；这让 Gwen 的前端输出不再只是“语句壳子 + 原 AST 表达式”，而开始真正成为后续 binding / MIR 可消费的结构层输入
2026-04-20 - HIR 开始显式绑定命名循环目标：`while/for` 现在在 `internal/hir` 中拥有稳定 `LoopID`，`leave/next` lowering 时会直接解析到对应 target，而不再只是保留源码里的名字字符串；这为后续 CFG/MIR 收口 Gwen 的显式循环控制流打下第一层绑定基础
2026-04-20 - HIR 扩到语句骨架层：`internal/hir` 现在不只 lower 顶层声明，也会把函数体/脚本里的 `var`、`if`、`while`、`for`、`match`、`parallel`、`return`、`leave/next` 等语句降成独立 HIR 节点；表达式当前仍暂保留原 AST，先把控制流和块结构从解释器形状里拆出来
2026-04-20 - 声明级 HIR 起步：新增 `internal/hir`，把顶层 `use / func / module / object / type` 与结构化类型标注从 AST 降成第一版声明/签名 HIR；前端 `internal/frontend` 现在在 `parse -> check` 之后同步产出 HIR，函数体与脚本执行语句暂继续保留原 AST，避免在 statement lowering 尚未设计好前把解释器细节硬塞进后端
2026-04-20 - 编译路线起步：新增 `docs/compiler.md`，明确 Gwen 的硬约束是“已编译程序不依赖 Go runtime，Go 只作为当前 bootstrap 实现语言”；同时新增 `internal/frontend`，把“读文件 -> parse -> check”从 CLI 胶水里独立成真正的编译前端入口，为后续 typed HIR / lowering / 后端选型开始清理架构边界
2026-04-20 - Go `sqlite` bootstrap 起步：新增官方 `sqlite` 模块，提供 `sqlite.open/close/exec/query` 与 opaque `SqliteDB`；先用一条很薄的 SQLite 落盘主路径覆盖本地后端/原型的持久化需求，不引入 ORM 或 SQL 生成器。查询结果当前返回 `list[dict]`，SQL `NULL` 复用 `json.null()` / `JsonNull`。同步补齐 Go checker / interpreter 回归测试、最小示例与 stdlib 文档
2026-04-20 - 显式共享状态 `cell[T]` 落地：新增官方 `state` 模块，提供 `state.cell/get/set/update`，并把 `cell[T]` 作为 Gwen 当前唯一推荐的共享状态主路径冻结下来。`parallel` 仍默认隔离快照，但现在可以通过 `cell[T]` 显式共享且用 `state.update(...)` 做原子读改写。同步补齐 Go checker / interpreter 回归测试、并发/类型/stdlib 文档与最小示例
2026-04-20 - HTTP 并发语义对齐：`http.listen(...)` 现在按“每请求独立快照”执行 handler，可并发处理请求；普通模块级 `dict` / `list` / 对象状态不再跨请求保留，跨请求共享必须显式写成 `cell[T]`。同步补上 Go 解释器回归测试与并发/stdlib 文档，把服务端运行时正式收口到“默认隔离，显式共享”
2026-04-20 - Go `http` client 补上通用请求主路径：新增 `http.request(method, url, body, headers, timeoutms=5000) -> result[HttpResponse]`，用于 POST / webhook / 带鉴权 header 的外部 API 调用；`http.get(...)` 保留为最常见 GET 快捷路径，但不再继续扩散 `post/put/...` 同义 helper。同步更新 checker / interpreter 回归测试、示例与 stdlib 文档
2026-04-20 - Go `http` 增加浏览器后端必需的最小跳转/cookie 面：新增 `http.requestcookie(...)` / `http.redirect(...)` / `http.withcookie(...)`，并把 reply/header 内部表示升级为多值 header，允许连续叠加多个 `Set-Cookie`。同步更新 checker / interpreter 回归测试、示例与 stdlib 文档
2026-04-20 - `docs/philosophy.md` 补上点号表面规则：正式约束 `module.name`、`obj.method()`、opaque runtime helper 这几类 `.` 的合法边界，明确反对同一能力长期双轨写法以及链式调用成为主风格，避免后续标准库/OOP 表面继续漂移
2026-04-20 - Go `http` 补上最小 header 面：新增 `http.requestheader(...)` / `http.responseheader(...)` / `http.withheader(...)`，服务端可显式读取请求头并返回带自定义 header 的新 `HttpReply`，客户端也能显式读取响应头。保持 request/response helper 分拆，不引入多态 `header(...)` 入口。同步更新 checker / interpreter 回归测试与 stdlib 文档
2026-04-20 - Go `http.query(...)` 去掉隐式空字符串默认值：现在 fallback 必须由调用点显式提供，避免“缺 query 参数就悄悄变成 `\"\"`”这种默认语义继续扩散。同步更新 checker 回归测试与 stdlib 文档
2026-04-20 - Go `http.static(...)` 语义拆分：现在返回 `bool, result[HttpReply]`，把“prefix 没命中”和“命中了但文件读取失败”显式分开，避免服务端路由再把静态资源 miss 当作 `err(...)` 流程控制。同步更新示例、checker / interpreter 回归测试与 stdlib 文档
2026-04-20 - Go `http` API 继续向显式语义收口：原先多态的 `http.body(...)` 已拆成 `http.requestbody(request)` 与 `http.responsebody(response)`，避免调用点再靠实参类型脑补上下文。同步更新 checker / interpreter / 示例与 stdlib 文档，为后续继续打磨 `http` 表面时先守住“单一主写法”
2026-04-20 - 混合精度运算检查落地：不同显式精度类型现在不能直接混算，必须先 `as` 到同一目标类型，例如 `int8 + int16`、`float32 + float64` 会在 checker 阶段直接报错；同型显式精度和“显式精度 + 普通字面量/默认 int,float”仍保持可写。同步更新 Go/Python checker、CLI `run` 前置语义检查、测试与类型文档
2026-04-20 - 新增 `docs/philosophy.md`，把 Gwen 的设计判据显式化：新特性默认要回答“是否把隐藏上下文变成显式语法”“是否减少补丁式代码”“是否利于 checker/compiler”这类问题；同时把命名循环 `leave/next` 作为代表性案例固定下来，避免后续设计再次滑回传统隐式控制流
2026-04-20 - Gwen docs-site dogfooding 原型落地：新增 `examples/docs_site/`，由 Gwen 后端直接提供双语页面、示例源码读取、搜索 API 与静态前端壳子；站点内容先以结构化 Gwen 模块承载，避免现在就维护第二套独立文档系统。已通过 `gwen check examples/docs_site/main.gw` 与真实 HTTP smoke 验证，为后续继续把官网/教学站迁到 Gwen 本身提供了第一块脚手架
2026-04-20 - Go 服务端 `http` bootstrap 起步：新增 `HttpRequest` / `HttpReply` / `HttpServer` opaque 类型，以及 `http.listen/addr/wait/close/method/path/query/route/text/html/json/static`；handler 现在可以显式返回 `HttpReply` 或 `result[HttpReply]`，运行时会把 handler 内部失败收口到 HTTP 500。同步补齐 checker / interpreter 回归测试、`examples/http_server.gw` 与 stdlib 文档，为后续用 Gwen 自己承载教学站/后端服务打基础
2026-04-20 - Go `json` bootstrap 起步：新增官方 `json` 模块，提供 `parseobject` / `parsearray` / `stringify` / `objectof` / `arrayof` / `null` / `isnull`；顶层 JSON 形状在调用点显式声明，`JsonNull` 作为 JSON `null` 的 opaque 类型落地。同步补齐 checker / interpreter 回归测试、示例与 stdlib 文档，为后续服务端 HTTP/Request/Response 打基础
2026-04-20 - Go `http` 语义收口到更显式的响应模型：`http.get(url, timeoutms=5000)` 现在返回 `result[HttpResponse]`，新增 `http.status(response)` / `http.responsebody(response)` 访问器；只要成功收到 HTTP 响应就返回 `ok(response)`，非 2xx 不再挤进 `err(...)`，只有真正的网络/协议失败才走错误分支。同步更新 checker / interpreter 回归测试、示例与 stdlib 文档
2026-04-23 - C emitter 的 `parallel` 不再是顺序模拟：compiled 路径现在会为每个分支生成 pthread entry/context，先启动全部任务再按源码顺序 join 与收集 `ok/err` 结果；运行时错误栈改为 TLS，避免跨线程 longjmp/error frame 互相污染。同步把 compiled `state.cell` 改为共享指针 + mutex，并让 `state.update` 在锁内完成 read/modify/write，保持显式共享状态的原子语义；补齐 `time.sleep` compiled helper 和并发耗时回归。验证：`go test ./internal/backend/cgen`、`go test ./...`
2026-04-23 - CLI 编译链再补一截：新增 `gwen build <path> [-o output]`，把 `Analyze -> Emit C -> cc -pthread -> executable` 收口成正式命令，不再要求用户手动接 `emit-c` 后半段；默认输出与源文件同目录同名可执行文件，也支持 `-o` 指定路径。当前 `build` 成功后还会识别并打印宿主二进制格式（`Mach-O` / `ELF` / `PE`），同时为 Windows 默认产物名补 `.exe`。同步补齐 CLI 集成回归，验证能真实生成并运行编译产物
2026-04-19 - Go `http` bootstrap 起步：stdlib 模块现在可以拥有不依赖全局 builtin 名字的模块专属导出，先用这条路径接入 `http.get(url, timeoutms=5000) -> result[string]`；运行时已支持本地 HTTP client 调用、超时参数和非 2xx 转 `err(...)`，并补齐 checker / interpreter 回归测试与示例，为后续后端基础设施继续铺路
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
