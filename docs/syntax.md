# Gwen 语法

## 设计原则

> **Gwen 是全能语言，但拒绝魔法。**
>
> 借鉴早期语言的"显然性"（Pascal/C 的直白），现代化其类型系统和错误处理。
> 不做能力限制，只限制**隐藏行为**——代码所见即所得，没有运行时悄悄发生的事。

### 0. 为什么这样设计（动机）

Gwen 的目标读者是**人**，不是 AI。

- ❌ **不是为了让 AI vibecoding 时省字**：简短不等于清晰，AI 写起来爽 ≠ 人读起来懂。
- ✅ **是为了让代码写完后，人能快速看明白发生了什么**：当 AI（或他人）写错了，差错应当**显然**——不需要跑起来才暴露，不需要翻三层调用才理解。
- ❌ **也不是无脑 if/else 或仪式感堆叠**：不要让"显式"变成给人看的压力（比如每行都要写一遍类型、每个字段都要 getter/setter）。
- ✅ **而是：在该显式的地方显式（行为、错误、转换），在不该啰嗦的地方放过你**（类型推导、批量声明、零值默认）。

一句话：**让机器产出的代码可被人快速审计，同时不让人为机器写废话。**

### 1. 显然性优先（No Magic）

| 原则 | 含义 | 反例（魔法语言） |
|------|------|----------------|
| **一行一行读，就知道发生了什么** | 没有隐式转换、没有操作符重载、没有隐式 this | `if (obj)` 自动转 bool；`a + b` 触发 5 个方法调用 |
| **行为在调用点可见** | 函数副作用从名字能猜，不会悄悄改全局状态 |  getter 看起来像读属性，实际触发数据库查询 |
| **错误不静默** | 失败抛错，不返回 nil 或默认值 | `dict["missing"]` 返回 nil，后面代码崩 |

**Gwen 的立场**：
- ✅ 能实现 OOP、多态、接口等复杂系统
- ❌ 但这些机制的**代价**必须显式支付（写更多字、显式检查、显式转换）

### 2. 符号 vs 关键字的分工（借鉴 Pascal/Ada）

| 层级 | 负责 | 例子 |
|------|------|------|
| **符号** | 原子操作（无歧义） | `:=` 赋值、`=` 比较、`+` 加法、`[]` 索引 |
| **关键字** | 结构控制 | `if/then/endif`、`while/do/endwhile` |
| **endxxx** | 块结束标记（显式闭合） | `endfunc`、`endmatch` |

**不用 `{}`**：视觉噪声大，嵌套时 `}}}}` 难追踪。

### 3. `=` 家族：形态区分语义

| 符号 | 含义 | 来源 |
|------|------|------|
| `=`  | 相等比较 | 数学传统 |
| `:=` | 赋值（ Algol/Pascal ） | `:` 表示"类型"，`= `表示"值" |
| `=>` | 映射（lambda/match） | "变成"的视觉隐喻 |
| `->` | 类型流向（返回） | 从输入到输出 |

**互不重叠**：没有上下文决定符号含义的情况。

### 4. 括号哲学：一个括号一个域

| 括号 | 用途 | 不做的事 |
|------|------|---------|
| `()` | 调用、分组、形参 | 不做元组（Gwen 用列表 `[a,b]`） |
| `[]` | 容器（字面量、索引、泛型） | 避免 `<>` 与比较混淆 |
| `<>` | **仅比较** | 不做泛型（转到 `[]`） |
| `{}` | 键值对字面量（dict；未来记录类型） | 不做块结构（Gwen 用 `endxxx`） |

### 4.1 保留符号（不做同义词，留给未来）

| 符号 | 当前 | 保留用途（候选，未实现） |
|------|------|----------|
| `%` | 未使用 | 候选：**百分比字面量**（`10%` = 0.10，配合 `money` 类型）。**不做 `mod` 同义词**——取模只有一种拼写 `mod`。 |
| `&` `\|` `~` | 未使用 | 位运算或集合操作关键字化（`band`/`bor`/`bnot`），符号保留。 |
| `?` | 未使用 | 不做可空标记（`result[T]` 已覆盖错误，不需要 null 文化）。保留给未来条件性语法。 |

**原则**：Gwen 不做同义词——**同一操作只有一种拼写**。已有关键字/符号表达的语义（`mod`、`^`、`and`/`or`/`not`），不再提供符号版本。没有 `%`、`**`、`&&`、`\|\|`。

### 5. 全能但显式：复杂能力的实现方式

| 想要的能力 | 其他语言的做法 | Gwen 的做法 |
|------------|---------------|-------------|
| **OOP** | 隐式 this、继承、虚表 | 显式 self 参数、无继承、match 分派（代码可见） |
| **多态** | 接口自动派发、运行时类型擦除 | 显式 `match` 分支或函数指针（静态可知） |
| **泛型** | 类型擦除、自动特化 | 透明别名 + 显式类型参数（`list[int]` 就是 `list`，检查元素） |
| **错误处理** | 异常自动传播 | `result/ok/err` + 显式 `match`（不悄悄忽略） |

**核心区别**：不是"不能做"，而是"做的时候看得见代价"。

### 6. 文档元语法

本文档用 `<placeholder>` 表示占位符，**不是 Gwen 代码**（旧版用 `{}`，现冲突 dict 字面量，已改为 `<>`——`<>` 在文档注释中无歧义，Gwen 源码里 `<>` 只做比较）。

```
// 文档写法
var default <expr>

// 实际代码
var default 0
var default x + 1
```

---

## 注释

```
// 单行注释
x := 10  // 行尾也可以

/*
  块注释
  可跨多行，不支持嵌套
*/
y := /* 行内 */ 20
```

## 赋值与比较

```
x := 42              // 赋值（当前函数级本地）
global x := 42       // 显式修改外层作用域（见 scope.md）
const PI := 3.14     // 不可变绑定，再赋值会报错
const MAX: int32 := 1000  // 支持类型标注 + 溢出检测
x = 42               // 相等判断
x != 42              // 不等
x <= 42              // 小于等于
x >= 42              // 大于等于
```

### 索引赋值

```
arr[i] := value              // 单个元素就地修改
arr[i], arr[j] := arr[j], arr[i]  // 多目标：一次交换两个位置
```

### 列表字面量

```
nums := [1, 2, 3]

// 多行风格，末尾允许逗号
matrix := [
  [1, 2, 3],
  [4, 5, 6],
  [7, 8, 9],
]
```

### 字典字面量

```
// 必须显式给出键值类型，避免歧义
scores := dict[string, int]{"alice": 90, "bob": 85}

// 多行风格，末尾允许逗号
users := dict[int, string]{
  1: "alice",
  2: "bob",
  3: "carol",
}

// 空字典
empty := dict[string, int]{}
```

**读写规则（拒绝魔法）：**

```
scores["alice"]            // 读：键存在 → 值；不存在 → 运行期报错 'Key not found'
scores["alice"] := 95      // 写：存在就覆盖，不存在就新增
haskey(scores, "zoe")     // 想静默判断 → 用 haskey
get(scores, "zoe", 0)      // 想带默认值 → 用 get（显式提供 fallback）
keys(scores)               // 所有键（list）
values(scores)             // 所有值（list）
len(scores)                // 键值对数量
```

> `d["missing"]` 不会悄悄返回 `nil` 或零值——要么用 `haskey` 先问，要么用 `get(d, k, default)` 显式声明 fallback。错误不静默。

### 算术运算

```
3 / 2                // 整数 ÷ 整数 = 整数，结果 1
3.0 / 2              // 有浮点参与 = 浮点，结果 1.5
5 + 2.5              // 混合运算自动提升为浮点，结果 7.5
10 mod 3             // 取模，结果 1
2 ^ 3                // 幂运算，结果 8
```

---

## 控制流

### 条件必须是 bool（严格类型，无 truthiness）

Gwen 跟 Go / Rust / Swift 一样：**`if` / `elif` / `while` 的条件必须是 `bool`**，其他类型一律报错。

```
// ❌ 报错：'if' condition must be bool, got int
x := 1
if x then ... endif

// ❌ 报错：'while' condition must be bool, got int
n := 3
while n do ... endwhile

// ✅ 显式比较
if x != 0 then ... endif
while n > 0 do ... endwhile
if len(lst) > 0 then ... endif
if s != "" then ... endif
```

**`and` / `or` / `not` 同样严格**：操作数必须是 `bool`，结果是 `bool`。

```
// ❌ 报错：left side of 'and' must be bool
if x and y > 0 then ... endif

// ✅ 显式
if x != 0 and y > 0 then ... endif
```

**`and` / `or` 短路求值**：右侧只在必要时求值，可安全用于守卫。

```
if x != 0 and 100 / x > 5 then ...   // x = 0 时右侧不算，安全
if cache_hit or expensive_lookup() then ...  // 命中缓存就跳过查询
```

**为什么这样设计**：
- `if x` 在不同语言里有 5 种以上语义（C 的 0/非 0、Python 的 truthiness、Ruby 的 nil/false 等），读者无法直接看出
- 显式比较 `if x != 0` 多写几个字，但意图毫不含糊——这是 Gwen "拒绝魔法" 的硬性要求
- 已有 `result[T]` 强制 match 覆盖错误；条件位置严格 bool 是同一哲学的另一条腿

> **当前实现**：错误在**运行时**报。Gwen 现阶段是动态类型解释器，无静态类型推导。
> **未来**：转编译型语言后，这类错误必须**编译期**就报，类似 Go。

### if

```
if x > 0 then
  do_a()
endif

if x > 0 then
  do_a()
elif x = 0 then
  do_b()
else
  do_c()
endif
```

### while

```
while b != 0 do
  a, b := b, a mod b
endwhile
```

### for

```
// 范围遍历（包含两端）
for i in 1 to 10 do
  write(i)
endfor

// 带步长
for i in 1 to 10 step 2 do
  write(i)
endfor

// 倒序（自动识别：end < start 时反向）
for i in 10 to 1 do
  write(i)
endfor

// 显式方向：order 强制升序 / reverse 强制降序（仅范围 for）
for i in 1 to 10 order do
  write(i)
endfor

for i in 1 to 10 reverse do
  write(i)
endfor

// 集合遍历
for item in list do
  process(item)
endfor

// 带下标
for item in list with index i do
  write(i, item)
endfor
```

**`order` / `reverse` 什么时候必须写？**

当边界是**变量**时，**强烈推荐显式写方向**——这是 Gwen 鼓励的防御性写法：

```
// 不推荐：a 和 b 谁大不确定，循环方向依赖运行期值（读代码时不显然）
for i in a to b do ... endfor

// 推荐：声明意图——"无论 a、b 谁大，我都要从小到大遍历"
for i in a to b order do ... endfor

// 推荐：声明意图——"无论 a、b 谁大，我都要从大到小遍历"
for i in a to b reverse do ... endfor
```

**语义定义**：

- `order` = 升序意图声明：等价于 `for i in min(a,b) to max(a,b) do`
- `reverse` = 降序意图声明：等价于 `for i in max(a,b) to min(a,b) do`
- 字面量场景（`1 to 10 order`）方向天然一致，行为不变

**为什么不报错也不"循环 0 次"？**

`order`/`reverse` 是**意图声明**，不是断言。它的语义是"我要的方向，请帮我处理边界顺序"——这是显式表达的便利，不是隐式魔法。读到 `a to b order` 的人立刻知道："这一定是从小遍历到大"。

不写 `order`/`reverse` 时是**自动模式**（按 start/end 大小推方向），那才是依赖运行期值的写法。所以：变量边界 → 写方向，是 Gwen 推荐的代码风格。

### match

```
match x
  when 1 => do_a()
  when 2, 3 => do_b()
  when 4 to 10 => do_c()
  else do_d()
endmatch
```

**强制解构规则**：`match` 作用于 `result` 类型（`ok(x)`/`err(e)`）时，必须同时覆盖 `ok` 和 `err` 两个分支，或显式提供 `else`，否则编译期/运行期报错。这避免了悄悄丢掉错误分支的情况。

---

## 函数

### 基本定义

```
func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc
```

### 多返回值

```
func readfile(path: string) -> string, bool
  ...
  return content, true
endfunc

data, found := readfile("/etc/config")
```

> 注意：`ok` 和 `err` 是保留关键字，不能用作变量名。

### 默认参数

```
func connect(host: string, port: int = 3306, timeout: int = 30)
  ...
endfunc

connect("localhost")
connect("localhost", 5432)
connect("localhost", 5432, 60)
```

### 匿名函数

```
// 单行
sort(list, (a, b: int) => a > b)

// 多行
handler := (x: int) =>
  y := x * 2
  return y + 1
endfunc
```

### 函数是一等公民

```
handler := (x: int) => x * 2
apply(list, handler)
```

### 命名结束标记

长函数的结束标记可以带名字：

```
func handle_request(req: Request) -> Response
  ...
endfunc handle_request
```

---

## 错误处理

### Result 类型

可能失败的函数返回 `result[T]`，通过 `ok(value)` / `err(message)` 构造，调用方用 `match` 强制处理两边。

```
func parse_int(s: string) -> result[int]
  ...
  return ok(n)                // 成功
  return err("not a number")  // 失败
endfunc
```

> **主干风格是 `result[T]`**，不是"多返回值 + bool"。原因：`result` 上的 `match` 强制覆盖 ok/err，`bool` 判断可以忘写。错误必须被看见。

> **`ok` / `err` 是语法，不是函数**。`ok(5)` 看起来像函数调用，实际上是 Gwen 的 `OkExpr` / `ErrExpr` 语法糖。因此：
> - 不能 `f := ok` 把它当函数传递（不是一等值）
> - `ok` 和 `err` 是保留关键字，不能用作变量名
> - 括号形式（`ok(x)` / `err(e)`）是语法规定，不是"函数调用括号"

### match 处理

```
match parse_int("42")
  when ok(n) =>
    write(n)
  when err(e) =>
    write("error: ", e)
endmatch
```

**强制覆盖**：`match` 作用于 `result` 时必须同时覆盖 `ok` 和 `err`（或显式 `else`），否则报错。这是"错误不静默"的硬保证。

### 实战：文件 I/O

内置 `readfile` / `writefile` / `appendfile` 都返回 `result`，下面是典型写法：

```
match readfile("/etc/hosts")
  when ok(content) => write(content)
  when err(e) => write("failed:", e)
endmatch
```

详细 API 见 `stdlib.md` → `io.gw`。

---

## 类型标注

- 函数参数：推荐标注（当前不强制）
- 函数返回值：可选
- 局部变量：可选（推导）

```
```

---

## 类型别名

```
type UserId = int64
type Score  = int8

id: UserId := 42     // 等价于 id: int64 := 42
s: Score := 100      // 等价于 s: int8 := 100，溢出检测有效

// 别名可以链式定义
type Id = int32
type UserId = Id
```

- 别名是**透明**的（transparent alias）：只是新名字，不产生新类型
- 别名指向精度类型时，溢出检测照常生效
- `type` 是上下文关键字，仅在语句开头 + 后跟标识符时触发

---

## 变量初始化（var / default / uninit）

Gwen 要求变量有明确的类型与初始值；若声明了类型但不赋值，**读取前必须先赋值**，否则运行期抛 `'x' read before assignment`。

### 单个声明

```
x: int := 10        // 声明并赋值
y: int              // 未初始化；读取前必须 y := ... 否则报错
y := 5
write(y)
```

### var ... endvar 批量声明

```
var
  a: int := 1
  b: string := "hi"
  c: bool              // 未初始化
endvar
```

- 推荐写在函数 / 模块开头，但**不强制限制位置**
- 块内每条和单独 `x: T := v` 等价

### var default 一键零值

```
var default
  n: int          // 0
  s: string       // ""
  b: bool         // false
  price: money[USD]   // 0.00 USD
endvar
```

零值表：
| 类型 | 零值 |
|------|------|
| `int` / `intN` / `uintN` | `0` |
| `float` / `floatN` | `0.0` |
| `string` | `""` |
| `bool` | `false` |
| `list[T]` | 新的空 list（每个变量独立） |
| `money[Tag]` | `0` + 原币种 tag |

无预设零值的类型（例如函数类型）会在 `var default` 下报错——必须显式赋值。

### var default &lt;expr&gt; 一键赋值

```
var default 1
  a: int      // 1
  b: int := 99 // 覆盖块级默认 -> 99
  c: int      // 1
endvar
```

- `<expr>` 只求值一次，副作用不重复执行
- 单项 `:= v` 优先级高于块级默认
- 表达式类型需要能赋给每个变量类型；不匹配（`var default "hi"` + `a: int`）立即报错

---
