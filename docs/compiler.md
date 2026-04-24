[English Version](./compiler.en.md)

# Gwen 编译路线

这篇文档写给继续实现 Gwen 的人看。

它只回答三件事：

- Gwen 现在的编译链是什么
- 哪些边界已经成形
- 接下来真正要补的是什么

## 目标先说清楚

Gwen 的目标不是“表面上有个 build 命令”，而是：

- 编译后的 Gwen 程序不要求安装 Go
- 编译后的 Gwen 程序不继续偷偷跑在 Go runtime 上
- Go 现在只是 bootstrap 实现语言，不是最终依赖

因此，下面这类路线不算 Gwen 想要的结果：

- 把 Gwen 先翻成 Go 包装代码
- 再把 Go runtime 一起带进最终产物
- 用户虽然没直接看到 Go，但程序本质上还是依赖 Go 执行

## 当前编译链

今天仓库里的主路径是：

```text
source
  -> frontend
  -> HIR
  -> MIR
  -> C emitter
  -> cc
  -> native executable
```

对应目录：

- `internal/frontend`
- `internal/hir`
- `internal/mir`
- `internal/backend/cgen`
- `cmd/gwen`

CLI 上能直接走这条链：

```bash
go run ./cmd/gwen build examples/hello.gw
go run ./cmd/gwen emit-c examples/hello.gw
```

`build` 现在会生成宿主平台的原生可执行文件：

- macOS: `Mach-O`
- Linux: `ELF`
- Windows: `PE/.exe`

## 当前已经有的东西

不是“打算有”，而是现在仓库里已经在跑的：

- 统一前端入口：读文件、parse、check 不再散在 CLI 里
- HIR：顶层声明、表达式、语句骨架已经从 AST 分离出来
- binding：名字、`global`、`leave/next`、`match` 目标已经开始显式绑定
- MIR：函数体和脚本体已经开始 lower 成 block、slot、value、terminator
- C emitter：已经能覆盖一批真实示例，而不只是玩具表达式

当前持续压编译链的真实示例包括：

- `examples/http_server.gw`
- `examples/docs_site/main.gw`
- `examples/session_notes.gw`
- `examples/sqlite_basics.gw`
- `examples/rules_app/main.gw`
- `examples/ledger_app/main.gw`

也就是说，Gwen 现在已经不处在“只有 IR 设计，没有真实后端压力”的阶段。

## 当前边界

### 前端

前端已经开始承担正式职责：

- 读文件
- parse
- checker
- 模块展开
- 产出后续 lowering 需要的统一输入

这让“解释器前置步骤”和“编译器前端”不再是两套入口。

### HIR

HIR 现在的职责不是做最终优化，而是把 AST 的表面结构整理成编译器能消费的形式。

已经开始稳定的信息：

- 顶层 `use / func / module / object / type`
- 结构化类型标注
- 语句与表达式骨架
- 一层名字绑定
- `global` 外层目标
- `leave/next` 的循环目标
- `match` 的基本 pattern 形状

还没完全冻结的部分：

- 每个表达式的完整已知类型
- 更完整的 definite-assignment 信息
- 更系统的 binding/type 结合层

### MIR

MIR 现在已经不是“只有控制流草图”的状态了。

它已经开始显式承载：

- `Block`
- `Terminator`
- `Slot`
- `Value`
- 一部分真实指令序列

现在的 MIR 已经能把一批后端需要的信息直接写出来：

- 控制流边
- 局部槽位
- 调用结果
- 多返回值
- 一部分成员/索引访问
- 一部分声明与赋值

但它还没完全 lower 到最末端 primitive：

- 某些高层语义还保留 Gwen 自己的结构
- runtime ABI 还没完全冻结
- 某些 value/typing 仍在“够用但不彻底”的阶段

## 为什么先做 runtime / stdlib / 示例

因为这些东西会反过来决定编译器边界。

`http`、`json`、`sqlite`、`state`、教学站、真实后端示例，逼 Gwen 回答的是这些问题：

- 哪些类型要变成官方 runtime handle
- 哪些语义必须静态可判断
- 哪些 helper 只是 bootstrap 方便
- 哪些能力值得进入长期 ABI

如果这些问题没先压出来，后面即使后端写得快，也会反复返工语言表面。

## 当前还没结束的事

这一阶段已经收口，但编译路线还没有结束。接下来真正该补的是：

### 1. runtime ABI 继续收紧

至少要把这些边界越来越明确：

- 基础值表示
- `result[...]` 表示
- 函数调用 ABI
- 模块初始化顺序
- runtime handle 与 Gwen 值的交界

### 2. lowering 继续去高层化

当前 MIR 已经能驱动第一后端，但还保留一部分 Gwen 自己的高层结构。

接下来要继续压平：

- 更多 control flow
- 更多 assignment/value 形状
- 更少“后端再猜一次”的语义

### 3. 编译路径和解释路径继续对齐

最近已经收了一轮：

- `bool` 显示
- 容器显示
- 一部分 I/O 错误文案
- 编译/解释输出差分回归

但还会继续做：

- `json.stringify(dict)` 的细节对齐
- 错误文本与诊断形状继续统一

### 4. 覆盖面继续扩

第一后端已经足够真实，但还没到“语言所有表面都能编”的程度。

真正该继续做的是：

- 用真实示例补 coverage
- 用差分测试抓语义分叉
- 再决定哪些能力值得进入下一轮冻结

## 为什么第一后端是 C

当前选择 C，不是因为 Gwen 的终点是 C，而是因为它适合做第一块跳板。

原因很直接：

- 产物是原生可执行文件
- 工具链成熟
- 输出可审计
- 有助于先把 ABI、lowering、runtime 边界理清

这条路的意义是：先得到一条不经过解释执行的真实后端。

以后要不要继续做：

- LLVM
- Wasm
- 更低层 native codegen

那是下一阶段的问题，不是当前 bootstrap 路线的前提。

## self-host 不在这一阶段

长期目标当然可以是 self-host。

但顺序不能反：

1. 先把 bootstrap 编译器稳定下来
2. 先让 Gwen 产物脱离 Go runtime
3. 再逐步让 Gwen 能编译 Gwen 自己

也就是说，眼下更重要的是“脱离 Go runtime”，不是“马上脱离 Go 实现语言”。

## 这篇文档不负责什么

它不负责：

- 教新用户写 Gwen
- 宣传 Gwen 有多特别
- 记录每次小改动

这些分别应该去看：

- `docs/syntax.md` / `docs/types.md` / `docs/stdlib.md`
- `docs/philosophy.md`
- `docs/tracking.md`
