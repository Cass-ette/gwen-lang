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
use pop, insert, sort, reverse, map, filter from list

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

1. **阶段 1**（当前）：核心内置稳定
2. **阶段 2**：实现 `list.gw`、`string.gw`
3. **阶段 3**：`math.gw`、`io.gw`
4. **阶段 4**：包管理器支持第三方模块

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
