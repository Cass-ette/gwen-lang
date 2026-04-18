# Gwen 标准库设计

> 模块化扩展，保持核心精简

## 设计原则

1. **核心最小化**：解释器只内置最基础功能
2. **显式导入**：用 `use` 明确依赖，便于审计
3. **审计友好**：标准库源码可读，不隐藏复杂逻辑
4. **渐进增强**：按需加载，不学Python"batteries included"

## 核心内置（解释器自带）

| 函数 | 用途 | 不扩展的理由 |
|------|------|-------------|
| `write(...)` | 输出 | I/O 基础，无法省略 |
| `read(prompt)` | 读取一行输入（可选提示语） | I/O 基础，无法省略 |
| `len(x)` | 长度 | 跨类型通用操作 |
| `append(lst, item)` | 列表追加 | 最常用列表操作 |
| `str(x)` | 转字符串 | 调试必需 |
| `int(x)` | 转整数 | 类型转换基础 |
| `float(x)` | 转浮点 | 类型转换基础 |
| `type(x)` | 类型检查 | 调试必需 |

## 标准库模块（计划）

### `list.gw` - 列表操作

```
use pop, insert, sort, reversed, map, filter from list

// 弹出末尾
last := pop(items)

// 插入
insert(items, 0, "head")  // 在索引0插入

// 排序（返回新列表）
sorted := sort(nums)

// 高阶函数
doubles := map(nums, (x) => x * 2)
evens := filter(nums, (x) => x mod 2 = 0)
```

**为什么不内置？**
- pop/insert/sort 可以用基础操作组合实现
- 保持核心解释器简单

### `string.gw` - 字符串处理

```
use split, join, trim, replace, contains from string

parts := split("a,b,c", ",")
text := join(["Hello", "World"], " ")
```

### `math.gw` - 数学函数

```
use sqrt, pow, sin, cos, floor, ceil from math

root := sqrt(2.0)
```

### `io.gw` - 文件 I/O

```
use read_file, write_file, append_file from io

content := read_file("/etc/hosts")
write_file("output.txt", content)
```

### `os.gw` - 系统接口

```
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
| **阶段 1** | ✅ 完成 | 核心内置 | `write/read/len/append/str/int/float/type` |
| **阶段 2** | ✅ 完成 | 列表+字符串核心 | **列表**: `sort`, `reversed`, `pop`, `insert`, `concat`<br>**字符串**: `split`, `join`, `substring`, `contains`, `trim`, `replace` |
| **阶段 3** | 🚧 进行中 | 数学+字典 | **数学**: `abs`, `min`, `max`, `sqrt`, `floor`, `ceil` ✅<br>**字典**: `dict[K,V]`, `has_key`, `keys`, `values` |
| **阶段 4** | 📋 远期 | 文件+高级迭代 | **文件**: `read_file`, `write_file`<br>**迭代**: `map`, `filter`, `range`, `enumerate` |
| **阶段 5** | 📋 远期 | 包管理器 | 第三方模块支持 |

### 阶段 2 详细设计（实现中）

#### 列表函数（`use from list`）

| 函数 | 签名 | 行为 | 复杂度 |
|------|------|------|--------|
| `sort` | `sort(lst: list[T], cmp: (T,T)->bool) -> list[T]` | **稳定排序**，返回新列表，原列表不变，**必须显式比较器** | O(n log n) |
| `asc` | 比较器 | 预定义 `(a, b) => a < b` | O(1) |
| `desc` | 比较器 | 预定义 `(a, b) => a > b` | O(1) |
| `reversed` | `reversed(lst: list[T]) -> list[T]` | 返回逆序新列表（名称为 `reversed`，`reverse` 是 for 循环关键字） | O(n) |
| `pop` | `pop(lst: list[T]) -> T` | **原地修改**，移除并返回末尾元素 | O(1) |
| `insert` | `insert(lst: list[T], idx: int, item: T) -> void` | **原地修改**，在索引处插入 | O(n) |
| `concat` | `concat(a: list[T], b: list[T]) -> list[T]` | 返回新列表（**不修改**输入） | O(a+b) |

**列表函数哲学**：
- `append`/`pop`/`insert`：**原地修改**，副作用明确
- `sort`/`reversed`/`concat`：**返回新列表**，原数据不变
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
| `substring` | `substring(s: string, start: int, end: int) -> string` | 提取子串 | 越界按实际长度截断 |
| `contains` | `contains(s: string, substr: string) -> bool` | 子串存在检查 | 空串视为包含 |
| `trim` | `trim(s: string) -> string` | 去首尾空白（space/tab/newline） | 返回新字符串 |
| `replace` | `replace(s: string, old: string, new: string) -> string` | 替换所有出现 | 无匹配返回原串（但仍是新字符串对象） |

**字符串函数哲学**：
- 字符串**不可变**，所有函数返回新字符串，无副作用
- `substring` 越界自动截断（不报错），审计友好
- `join` 自动 `str()` 转换元素，方便数字拼接

**待实现**：`split`/`join` 是否提供 `limit: int` 参数（限制分割次数）？暂不实现，按需再加。

## 与 OOP 的关系

Gwen **不采用 OOP**（类、继承、方法调用），标准库保持**函数式**接口：

```
// Gwen 风格（函数式）
append(lst, item)
sort(lst)

// 不是 OOP 风格
lst.append(item)
lst.sort()
```

理由：
1. 函数调用显式传参，审计时一目了然
2. 没有隐式的 `this` 状态修改
3. 数据和行为分离，符合 "显式优于隐式" 原则
