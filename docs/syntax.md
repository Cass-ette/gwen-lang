# Gwen 语法

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

// 倒序（自动识别）
for i in 10 to 1 do
  write(i)
endfor

// 显式方向：order 升序 / reverse 降序（仅范围 for）
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

### match

```
match x
  when 1 then do_a()
  when 2, 3 then do_b()
  when 4 to 10 then do_c()
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
func read_file(path: string) -> string, bool
  ...
  return content, true
endfunc

data, found := read_file("/etc/config")
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

函数返回 `ok(value)` 或 `err(message)`，调用方用 `match` 处理：

```
func read_file(path: string) -> string, bool
  if file_exists(path) then
    return file_content, true
  else
    return "", false
  endif
endfunc

// 或者用 ok/err 包装
func parse_int(s: string) -> int
  ...
  return ok(n)      // 成功
  return err("not a number")  // 失败
endfunc
```

### match 处理

```
match parse_int("42")
  when ok(n) then
    write(n)
  when err(e) then
    write("error: ", e)
endmatch
```

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
  price: money<USD>   // 0.00 USD
endvar
```

零值表：
| 类型 | 零值 |
|------|------|
| `int` / `intN` / `uintN` | `0` |
| `float` / `floatN` | `0.0` |
| `string` | `""` |
| `bool` | `false` |
| `list<T>` | 新的空 list（每个变量独立） |
| `money<Tag>` | `0` + 原币种 tag |

无预设零值的类型（例如函数类型）会在 `var default` 下报错——必须显式赋值。

### var default <expr> 一键赋值

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
