# Gwen 语法

## 赋值与比较

```
x := 42              -- 赋值（当前函数级本地）
global x := 42       -- 显式修改外层作用域（见 scope.md）
x = 42               -- 相等判断
x != 42              -- 不等
x <= 42              -- 小于等于
x >= 42              -- 大于等于
```

### 索引赋值

```
arr[i] := value      -- 支持列表元素就地修改
```

### 算术运算

```
3 / 2                -- 整数 ÷ 整数 = 整数，结果 1
3.0 / 2              -- 有浮点参与 = 浮点，结果 1.5
5 + 2.5              -- 混合运算自动提升为浮点，结果 7.5
10 mod 3             -- 取模，结果 1
2 ^ 3                -- 幂运算，结果 8
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
-- 范围遍历（包含两端）
for i in 1 to 10 do
  write(i)
endfor

-- 带步长
for i in 1 to 10 step 2 do
  write(i)
endfor

-- 倒序（自动识别）
for i in 10 to 1 do
  write(i)
endfor

-- 集合遍历
for item in list do
  process(item)
endfor

-- 带下标
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
-- 单行
sort(list, (a, b: int) => a > b)

-- 多行
handler := (x: int) =>
  y := x * 2
  return y + 1
end
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

-- 或者用 ok/err 包装
func parse_int(s: string) -> int
  ...
  return ok(n)      -- 成功
  return err("not a number")  -- 失败
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
x: int := 42         -- 显式标注
x := 42              -- 类型推导

name: string := "hello"
name := "hello"
```
