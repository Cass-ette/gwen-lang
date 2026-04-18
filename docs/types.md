# Gwen 类型系统

## 基础类型

```
int, float, string, bool
```

无限精度整数和双精度浮点，适合一般用途。

---

## 显式精度数值类型

Gwen 采用**显式宽度**设计：数值范围一目了然，溢出必须显式处理。

### 整数类型（已实现）

| 类型 | 范围 | 说明 |
|------|------|------|
| `int8` | -128 ~ 127 | 8 位有符号 |
| `int16` | -32,768 ~ 32,767 | 16 位有符号 |
| `int32` | -2,147,483,648 ~ 2,147,483,647 | 32 位有符号 |
| `int64` | 约 ±9.2×10¹⁸ | 64 位有符号 |
| `uint8` | 0 ~ 255 | 8 位无符号 |
| `uint16` | 0 ~ 65,535 | 16 位无符号 |
| `uint32` | 0 ~ 4,294,967,295 | 32 位无符号 |
| `uint64` | 0 ~ 约 1.8×10¹⁹ | 64 位无符号 |

```
x: int8  := 127
y: int16 := 1000
z: int32 := 2000000000
w: int64 := 1000000000000000

u: uint8  := 255
v: uint16 := 65535
```

### 浮点类型（已实现）

| 类型 | 精度 | 说明 |
|------|------|------|
| `float32` | ~7 位十进制有效数字 | IEEE 754 单精度 |
| `float64` | ~15 位十进制有效数字 | IEEE 754 双精度 |

```
pi: float32  := 3.14159        // ~7 位精度
precise: float64 := 3.14159265358979  // ~15 位精度
```

**精度陷阱**：`float32` 在 2²⁴（16,777,216）处开始丢失整数精度，`float32(16777216) = float32(16777217)` 为 `true`。

---

## 溢出处理（已实现）

溢出**运行时报错**，不静默截断：

```
x: int8 := 127
x := x + 1   // 运行时错误：Overflow: 128 out of range for int8 [-128, 127]
```

---

## 类型转换（已实现）

使用 `as` 语法，转换结果以 `result` 类型返回，用 `match` 处理：

```
match 3.7 as int
  when ok(n) then
    write(n)          // 3，截断
  when err(e) then
    write("failed:", e)
endmatch
```

### 常用转换

```
3.7 as int        // ok(3)，截断小数
"hello" as int    // err("Cannot convert...")
5 as float        // ok(5.0)
42 as string      // ok("42")

// 精度转换
100 as int8       // ok(100)，值在范围内
200 as int8       // err(...)，溢出
```

---

## 类型标注

变量声明时可标注类型，解释器会在赋值时执行强制转换和溢出检测：

```
a: int8  := 100       // 赋值时检查范围
b: float32 := 3.14    // 截断为 float32 精度
c: int64 := 10 ^ 15   // 大整数，int64 范围内
```

---

## 泛型类型

```
list<int>              // 整数列表
list<list<int>>        // 整数矩阵
list<string>           // 字符串列表
```

---

## 函数类型

```
(int) -> int           // 接受 int，返回 int
(int, int) -> int      // 接受两个 int，返回 int
() -> string           // 无参数，返回 string
```

---

## 货币类型 `money<Tag>`

带币种 tag 的定点数，专为金额/会计场景设计：

```
price: money<USD> := 19.99
cny:   money<CNY> := 144.0

total := price + price      // ok，同币种
// bad := price + cny       // 报错：Currency mismatch

doubled := price * 2        // money × 标量 → money
ratio   := price / price    // money ÷ money → float（比率）
// p2    := price * price   // 报错：金额乘金额无语义

f := price as float64       // 允许：丢掉币种，拿原始数值
// e := price as money<EUR> // 返回 err：拒绝隐式汇率转换
```

**规则**

| 操作 | 行为 |
|------|------|
| `money<X> + money<X>` | ok |
| `money<X> + money<Y>` | 报错（币种不匹配） |
| `money<X> + scalar` | 报错 |
| `money<X> * int/float` | ok，结果 `money<X>` |
| `money<X> * money<*>` | 报错 |
| `money<X> / int/float` | ok |
| `money<X> / money<X>` | 结果 `float`（比率） |
| 比较 `=` `<` `>` | 同币种才允许 |
| `as money<Y>` | 不同币种时返回 `err`（不隐式换汇） |
| `as float64` / `as int64` | 允许，丢掉币种 |

**实现细节**

- 内部存 `int64`，值 = 实际金额 × 10_000（scale=4）
- 溢出按 int64 范围检测，超出报错
- Tag 是自由字符串（`USD`/`CNY`/`JPY`/`BTC` 都行），无白名单
- 不自带汇率表，跨币种转换请写显式函数

---

## 实现状态

| 特性 | 状态 | 备注 |
|------|------|------|
| `int` / `float` / `string` / `bool` | ✅ 稳定 | 基础类型 |
| `int8/16/32/64` | ✅ 已实现 | 溢出检测 |
| `uint8/16/32/64` | ✅ 已实现 | 溢出检测 |
| `float32` / `float64` | ✅ 已实现 | IEEE 754 精度模拟 |
| 溢出检测与报错 | ✅ 已实现 | 所有精度类型 |
| `as` 类型转换 | ✅ 已实现 | 返回 result 类型 |
| `list<T>` 泛型 | ✅ 已实现 | 嵌套支持 |
| 函数类型 `(T) -> T` | ✅ 已实现 | 匿名函数支持 |
| 类型别名 `type` | ✅ 已实现 | 透明别名，继承精度约束 |
| 货币类型 `money<Tag>` | ✅ 已实现 | int64×10_000，带币种 tag |
| 混合精度运算检查 | 📋 设计阶段 | 目前允许混用 |
