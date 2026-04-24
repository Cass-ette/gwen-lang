# Gwen 核心语义规范

> 类型系统与操作行为的权威定义。实现必须与此文档对齐；如发现偏离，是 bug。

---

## 1. 类型分类

### 1.1 标量类型（Scalar）

| 类型 | 内部表示 | 可哈希 | 可比较 | 说明 |
|------|----------|--------|--------|------|
| `int` / `int8/16/32/64` | Python int | ✅ | ✅ | 有符号整数，溢出检测（定宽类型） |
| `uint8/16/32/64` | Python int | ✅ | ✅ | 无符号整数 |
| `float` / `float32/64` | Python float | ✅ | ✅ | IEEE 754，注意精度问题 |
| `bool` | Python bool | ✅ | ✅ | 仅 `true` / `false` |
| `string` | Python str | ✅ | ✅ | 不可变 UTF-8 序列 |
| `money[Tag]` | int64 (×10,000) | ✅ | ✅ | 定点数，币种强隔离 |

### 1.2 复合类型（Composite）

| 类型 | 内部表示 | 可哈希 | 可比较 | 可遍历 | 说明 |
|------|----------|--------|--------|--------|------|
| `list[T]` | Python list | ❌ | ❌ | ✅ | 可变有序序列 |
| `dict[K, V]` | Python dict | ❌ | ❌ | ❌ | 无序键值对（**禁止直接遍历**） |
| `result[T]` | OkValue/ErrValue | ❌ | ❌ | ❌ | 错误处理专用，必须 match |
| 函数类型 | Python callable | ❌ | ❌ | ❌ | 一等函数 |

### 1.3 类型别名

`type Alias = T` 是**透明别名**，不产生新类型。别名继承原类型的所有能力和约束（如 `int8` 的溢出检测）。

---

## 2. 操作符语义

### 2.1 算术运算

| 操作符 | 左操作数 | 右操作数 | 结果 | 特殊行为 |
|--------|----------|----------|------|----------|
| `+` | `int` | `int` | `int` | |
| `+` | `float` | `float` | `float` | |
| `+` | `int` | `float` | `float` | 自动提升 |
| `+` | `string` | `string` | `string` | 拼接 |
| `+` | `list[T]` | `list[T]` | `list[T]` | 拼接（返回新列表） |
| `+` | `money[T]` | `money[T]` | `money[T]` | 同币种相加 |
| `-` | `int` | `int` | `int` | |
| `-` | `float` | `float` | `float` | |
| `*` | `int` | `int` | `int` | |
| `*` | `float` | `float` | `float` | |
| `*` | `int` | `float` | `float` | 自动提升 |
| `*` | `money[T]` | `int` | `money[T]` | 乘整数 |
| `/` | `int` | `int` | `int` | 整数除法，向零取整 |
| `/` | `float` | `float` | `float` | 浮点除法 |
| `/` | `int` | `float` | `float` | 自动提升 |
| `/` | `money[T]` | `int` | `money[T]` | 除整数 |
| `/` | `money[T]` | `float` | `money[T]` | 除浮点（定点→浮点定点转换） |
| `mod` | `int` | `int` | `int` | 取模（同 Python %） |
| `^` | `int` | `int` | `int` | 幂运算 |
| `^` | `float` | `float` | `float` | 幂运算 |

**禁止的运算**（报错）：
- `money[T] + money[U]`（币种不匹配）
- `money[T] * money[T]`（无意义）
- `money[T] * float`（精度损失，需显式 `as float`）
- `string + int`（必须显式 `str(n)`）
- `list + non-list`（类型不匹配）
- 不同**显式精度类型**直接混算（如 `int8 + int16`, `float32 + float64`），需先 `as` 到同一目标类型

### 2.2 比较运算

| 操作符 | 允许的操作数类型组合 | 说明 |
|--------|----------------------|------|
| `=`, `!=` | 任意同类型 | 值相等比较 |
| `<`, `>`, `<=`, `>=` | `int/int`, `float/float`, `int/float`, `string/string` | 全序比较 |

**注意**：
- `string` 比较按**字典序（lexicographical）**，逐字符比较 Unicode 码点值
- 复合类型（list/dict）**不支持** `<` 等比较（无全序定义）
- `money[T]` 仅支持同币种比较

### 2.3 逻辑运算

| 操作符 | 操作数 | 结果 | 说明 |
|--------|--------|------|------|
| `and` | `bool`, `bool` | `bool` | 短路求值，左假不算右 |
| `or` | `bool`, `bool` | `bool` | 短路求值，左真不算右 |
| `not` | `bool` | `bool` | |

**严格类型**：非 `bool` 报错。没有 truthiness。

---

## 3. 控制流条件

### 3.1 条件位置必须是 bool

| 结构 | 条件位置 | 要求 |
|------|----------|------|
| `if` | `if <cond>` | 必须 `bool` |
| `elif` | `elif <cond>` | 必须 `bool` |
| `while` | `while <cond>` | 必须 `bool` |

### 3.2 for 遍历规则

| 语法 | 遍历变量 | 说明 |
|------|----------|------|
| `for i in a to b` | `int` | 整数范围（含两端） |
| `for c in "a" to "z"` | `string` | ASCII 单字符范围 |
| `for x in list` | `T` | 列表元素 |
| `for c in string` | `string` | 1-char 字符串 |
| `for k in keys(d)` | `K` | dict 键（显式） |
| `for v in values(d)` | `V` | dict 值（显式） |
| `for p in items(d)` | `list[K, V]` | dict 条目，`[key, value]` |

**禁止**：
- `for x in dict`（直接遍历 dict 报错）
- `for x in int`（非可遍历类型报错）
- `for x in result`（错误处理类型禁止遍历）

### 3.3 显式循环控制

- `pass` 是显式空操作，不产生运行时效果。
- Gwen 不提供隐式 `break` / `continue`。
- 需要跳出或进入下一轮时，使用命名循环配合 `leave <name>` / `next <name>`：

```
while running do scan
  if bad_line then
    next scan
  endif

  if done then
    leave scan
  endif
endwhile scan
```

- `leave` / `next` 只能指向当前或外层的已命名循环；不能跨函数、方法、构造器或 lambda。

### 3.4 match 模式

| Subject 类型 | 允许的模式 | 强制覆盖 |
|--------------|------------|----------|
| `int` | 字面量（`1`）、范围（`1 to 10`）、else | 否（可选 else） |
| `string` | 字面量（`"hi"`）、else | 否 |
| `result[T]` / `result[T, E...]` | `ok(x)`, `err(e)`, else | **是**（必须 ok+err 或 +else） |
| `list[T]` | ❌ 暂无列表匹配 | — |
| `dict[K,V]` | ❌ 不支持 | — |

---

## 4. 类型转换（as）

| 源类型 | 目标类型 | 行为 | 溢出/失败 |
|--------|----------|------|-----------|
| `int` | `float` | 精确转换 | — |
| `float` | `int` | 截断小数 | — |
| `int` | `int8/16/32/64` | 检查范围 | **溢出报错** |
| `float` | `float32` | 检查范围/精度 | 溢出报错 |
| `string` | `int` | 解析十进制 | **失败报错**（或 future: result） |
| `string` | `float` | 解析浮点 | **失败报错** |
| `bool` | `bool` | 恒等 | — |
| 非 `bool` | `bool` | ❌ **禁止** | 报错，提示显式比较 |

**当前实现**：运行时检查。未来编译型版本应提前到编译期。

---

## 5. 索引与访问

| 表达式 | 适用类型 | 结果类型 | 越界行为 |
|--------|----------|----------|----------|
| `list[i]` | `list[T]` | `T` | **报错**（Index out of range） |
| `string[i]` | `string` | `string` | **报错**（Index out of range） |
| `dict[k]` | `dict[K,V]` | `V` | **报错**（Key not found） |

---

## 6. 内建函数能力

### 6.1 通用函数

| 函数 | 输入类型 | 输出 | 说明 |
|------|----------|------|------|
| `len(x)` | `list`, `string`, `dict` | `int` | 元素/字符/键值对数量 |
| `typeof(x)` | 任意 | `string` | 类型名（含泛型参数） |

### 6.2 列表操作

**拼接**：`list + list` 是主要拼写（返回新列表）。`concat(a, b)` 保留作为函数式备选，两者等价。

| 函数 | 签名 | 副作用 |
|------|------|--------|
| `append(lst, item)` | `(list[T], T) -> void` | ✅ 原地修改 |
| `pop(lst)` | `(list[T]) -> T` | ✅ 原地修改 |
| `removeat(lst, idx)` | `(list[T], int) -> T` | ✅ 原地修改 |
| `insert(lst, idx, item)` | `(list[T], int, T) -> void` | ✅ 原地修改 |
| `sort(lst, cmp)` | `(list[T], (T,T)->bool) -> list[T]` | ❌ 返回新列表 |
| `reversed(lst)` | `(list[T]) -> list[T]` | ❌ 返回新列表 |
| `concat(a, b)` | `(list[T], list[T]) -> list[T]` | ❌ 返回新列表（同 `a + b`） |

### 6.3 字符串操作

| 函数 | 签名 | 说明 |
|------|------|------|
| `split(s, sep)` | `(string, string) -> list[string]` | `sep=""` 按字符拆 |
| `join(parts, sep)` | `(list[string], string) -> string` | |
| `substring(s, start, end)` | `(string, int, int) -> string` | **双闭区间** `[start, end]`，越界报错 |
| `contains(s, substr)` | `(string, string) -> bool` | |
| `trim(s)` | `(string) -> string` | 去首尾空白 |
| `replace(s, old, new)` | `(string, string, string) -> string` | 替换所有 |

### 6.4 字典操作

| 函数 | 签名 | 说明 |
|------|------|------|
| `haskey(d, k)` | `(dict[K,V], K) -> bool` | 存在性检查 |
| `get(d, k, default)` | `(dict[K,V], K, V) -> V` | 带默认值的读取 |
| `keys(d)` | `(dict[K,V]) -> list[K]` | |
| `values(d)` | `(dict[K,V]) -> list[V]` | |
| `items(d)` | `(dict[K,V]) -> list[list[K,V]]` | 键值对列表 |

---

## 7. 错误处理策略

| 场景 | 策略 | 示例 |
|------|------|------|
| 预执行语义错误（当前） | 运行前失败 | `Unknown type: UserIdd`, `Module 'm' does not export 'x'`, `Module 'm' top level only allows ...`, `Cannot import 'x' ...`, `Cyclic module import detected ...` |
| 运行时类型不匹配 | 抛错 | `'if' condition must be bool` |
| 算术溢出（定宽类型） | 抛错 | `int8 overflow` |
| 索引/键不存在 | 抛错 | `index out of range` |
| 可能失败的操作 | `result[T]` | `readfile`, `readdir`, `as`（future） |

**原则**：错误不静默，失败即崩溃（或强制处理的 `result`）。

**当前预执行检查还会覆盖调用签名**：
- 函数 / 方法 / 构造器 / lambda 的参数个数错误会在执行前失败
- 对显然不兼容的实参类型会在执行前失败
- 若签名里用了类型别名，检查按**定义处作用域**解析，而不是调用处作用域
- 因此模块私有类型别名可以继续被导出函数内部使用，调用者不需要先 `use` 这个私有别名
- 模块顶层现在是**声明区**，不允许在 `module ... endmodule` 内写赋值、裸调用或流程控制作为隐式初始化逻辑
- 模块导入不会静默覆盖当前作用域已有名字；重复导出同名符号也会在运行前失败
- 模块内的 `use` 必须位于声明区最前面；循环导入会在加载阶段直接失败

---

## 8. 未定义行为（UB）

目前 Gwen 没有明确定义的 UB——所有非法操作都应抛错。如果发现了
