# Gwen 类型系统

## 基础类型

```
int, float, string, bool
```

## 显式精度数值类型

Gwen 采用**显式宽度**设计：数值范围一目了然，溢出必须显式处理。

### 整数类型

```
x: int8  := 127                -- -128 ~ 127
y: int16 := 1000               -- -32,768 ~ 32,767
z: int32 := 2_000_000_000      -- 约 ±21 亿
w: int64 := large_num          -- 约 ±9e18

flags: uint8 := 0b11110000     -- 无符号 0 ~ 255
addr: uint32 := 0x1A2B3C4D     -- 无符号 0 ~ 4,294,967,295
```

### 自定义位宽大整数

```
-- 位宽由尖括号指定，支持任意精度
hash: int<256> := ...          -- 256 位整数（密码学哈希）
random: int<512> := ...        -- 512 位随机数
```

### 浮点类型

```
pi: float32  := 3.14159        -- 约 7 位十进制精度
precise: float64 := ...        -- 约 15 位十进制精度
```

### 自定义精度浮点

```
-- 十进制精度位数由尖括号指定
pi_custom: float<50> := ...    -- 50 位十进制精度
```

### 定点数（金融/会计）

```
money: decimal<19, 4>          -- 总 19 位，其中 4 位小数
  -- 范围: -999,999,999,999,999.9999 ~ +999,999,999,999,999.9999
```

## 语义规则

### 溢出处理

溢出**必须报错**，不能静默截断：

```
x: int8 := 127
x := x + 1           -- 运行时错误：int8 溢出
```

### 混合运算

不同精度运算需显式转换：

```
a: int32 := 100
b: int64 := 200
c := a + b          -- 错误：类型不匹配
c := a as int64 + b -- 正确：显式转换
```

### 类型别名

```
type Hash = int<256>
type Money = decimal<19, 4>
```

## 泛型类型

```
list<int>                    -- 整数列表
dict<string, int>            -- 字符串键，整数值
result<int, string>          -- 成功时 int，错误时 string
```

## 函数类型

```
(int) -> int                 -- 接受 int，返回 int
(int, int) -> int            -- 接受两个 int，返回 int
() -> string                 -- 无参数，返回 string
(int) -> bool                -- 常用于过滤函数
```

## 高阶函数示例

```
func map(f: (int) -> int, arr: list<int>) -> list<int>
  result := []
  for item in arr do
    append(result, f(item))
  endfor
  return result
endfunc

doubled := map((x: int) => x * 2, [1, 2, 3])
-- doubled = [2, 4, 6]
```

---

## 类型转换

使用 `as` 语法，转换结果以 `result` 类型返回：

```
x := "42" as int
match x
  when ok(n) then
    write(n)
  when err(e) then
    write("conversion failed:", e)
endmatch
```

### 转换示例

```
y := 3.7 as int          -- ok(3)，截断
z := "hello" as int      -- err("Cannot convert str to int")
f := 5 as float          -- ok(5.0)
s := 42 as string        -- ok("42")

-- 显式精度转换
big: int64 := small as int64      -- 小精度到大精度，安全
small: int8 := big as int8        -- 大精度到小精度，溢出时 err
```

## 待实现

- [ ] 基础类型 `int8/16/32/64`, `uint8/16/32/64`
- [ ] 自定义位宽 `int<N>`
- [ ] 浮点 `float32/64`
- [ ] 自定义精度 `float<N>`
- [ ] 定点数 `decimal<P, S>`
- [ ] 溢出检测与报错
- [ ] 类型别名 `type`
