# Gwen 标准库设计

> 模块化扩展，保持核心精简

## 设计原则

1. **核心最小化**：解释器只内置最基础功能
2. **显式导入**：用 `use` 明确依赖，便于审计
3. **审计友好**：标准库源码可读，不隐藏复杂逻辑
4. **渐进增强**：按需加载，不学Python"batteries included"

## 当前状态（2026-04-19）

Gwen 现在的标准库还处在**过渡态**：

- 语言已经有一批稳定可用的“标准能力”
- 但其中很多能力当前还是**解释器内建**
- 它们今天**默认可用，不需要 `use`**
- 同时，官方 `list/string/math/dict/io` 模块名现在也已经可导入
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
它们未来应该表现为**官方 stdlib 模块**；短期内可以继续保留 builtin 兼容层。

| 模块 | 能力 | 现状 |
|------|------|------|
| `list` | `append` `pop` `removeat` `insert` `concat` `sort` `reversed` `map` `filter` `range` `enumerate` | 已 builtin，并已支持官方 `list` 模块导入 |
| `string` | `split` `join` `substring` `contains` `trim` `replace` | 已 builtin，并已支持官方 `string` 模块导入 |
| `math` | `abs` `min` `max` `sqrt` `floor` `ceil` | 已 builtin，并已支持官方 `math` 模块导入 |
| `dict` | `haskey` `get` `keys` `values` `items` | 已 builtin，并已支持官方 `dict` 模块导入 |
| `io` | `readfile` `writefile` `appendfile` | 已 builtin，并已支持官方 `io` 模块导入 |

**推荐迁移策略**：

1. `v0.1`：继续允许这些名字默认可用，避免今天的代码全量破坏。
2. 同时把文档正式写成 `use ... from list/string/math/dict/io` 的目标形态。
3. `v0.2+`：可以考虑让 builtin 只保留兼容别名，逐步鼓励显式导入。

### C. 应等编译器 / runtime 阶段再做

这类能力高度依赖真实运行时、系统接口或内存模型。  
在解释器阶段硬做，容易做成“假能力”。

| 模块 / 能力 | 原因 |
|-------------|------|
| `arena` / `in arena` / `Arena` | 真实区域内存必须和运行时/分配器一起设计 |
| `os` | 环境变量、参数、进程控制、退出码等都依赖 runtime 约定 |
| `net` | 套接字 / 超时 / 并发 / 错误模型都不是解释器阶段该先锁死的 |
| `time` | 时钟 / 定时器 / sleep / timezone 都是 runtime 问题 |
| 包管理器 / 第三方模块系统 | 依赖编译器、构建和发布流程一起收口 |

**结论**：这些能力现在可以写设计，不要急着做完整实现。

---

## 推荐导入形态（已支持，未来推荐）

今天大多数 stdlib 能力仍是 builtin，所以**现在不强制导入**。  
但为了让后续模块化迁移平滑，官方文档建议逐步朝下面的形态写；这批导入形态现在已经可用：

```gwen
use append, pop, insert, sort, reversed, map, filter, range, enumerate from list
use split, join, trim from string
use abs, sqrt from math
use haskey, get, keys from dict
use readfile, writefile from io
```

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
use split, join, trim, replace, contains from string

parts := split("a,b,c", ",")
text := join(["Hello", "World"], " ")
```

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
```

| 函数 | 签名 | 行为 |
|------|------|------|
| `readfile` | `readfile(path: string) -> result[string]` | 读全文（UTF-8），失败返回 `err(msg)` |
| `writefile` | `writefile(path: string, content: string) -> result[int]` | 覆盖写，`ok(bytes_written)` |
| `appendfile` | `appendfile(path: string, content: string) -> result[int]` | 追加写，`ok(bytes_written)` |

**设计说明**（为什么没有某些 API）：

- ❌ **没有 `file_exists`**：它会诱导 TOCTOU bug（查过了再读，中间文件被删），而且自身也会失败（权限/IO）。想知道能不能读，就直接 `readfile` 并处理 `err`——这是唯一不骗自己的写法。
- ❌ **没有 `read_lines`**：`split(content, "\n")` 一行替代，不提供"便利糖"。
- `ok` 载荷带信息（字节数 / 内容），不用 `ok(true)` 这种无信息的废话。

### `os.gw` - 系统接口（远期设计，当前未实现）

```gwen
use env, args, exit from os

home := env("HOME")
```

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
| **阶段 2** | ✅ 完成 | 列表+字符串核心 | **列表**: `sort`, `reversed`, `pop`, `insert`, `concat`<br>**字符串**: `split`, `join`, `substring`, `contains`, `trim`, `replace` |
| **阶段 3** | ✅ 完成 | 数学+字典 | **数学**: `abs`, `min`, `max`, `sqrt`, `floor`, `ceil` ✅<br>**字典**: `dict[K,V]`, `haskey`, `get`, `keys`, `values` ✅ |
| **阶段 4** | ✅ 完成 | 文件+高级迭代 | **文件**: `readfile`, `writefile`, `appendfile` ✅<br>**迭代**: `map`, `filter`, `range`, `enumerate` ✅ |
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
| `contains` | `contains(s: string, substr: string) -> bool` | 子串存在检查 | 空串视为包含 |
| `trim` | `trim(s: string) -> string` | 去首尾空白（space/tab/newline） | 返回新字符串 |
| `replace` | `replace(s: string, old: string, new: string) -> string` | 替换所有出现 | 无匹配返回原串（但仍是新字符串对象） |

**字符串函数哲学**：
- 字符串**不可变**，所有函数返回新字符串，无副作用
- `substring` 越界**报错**（不截断），符合"错误不静默"
- `join` 自动 `str()` 转换元素，方便数字拼接

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
