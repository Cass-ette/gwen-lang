# 语言设计文档

## 设计理念

- **审查优先** — AI 写代码的时代，可读性和可审计性是第一优先级
- **数学直觉** — 语法贴近数学表达，有数学和英语基础即可入手
- **显式优于隐式** — 错误必须处理，接口必须标记，并行必须声明
- **自然但不冗余** — 比 Pascal 简洁，比 C 自然

## 目标场景

- 后端开发
- 运维自动化
- Vibe coding 审查友好

## 目标用户

- 有数学基础和英语基础的开发者

---

## 基础语法

### 赋值与比较

```
x := 42              -- 赋值
x = 42               -- 相等判断
x != 42              -- 不等
x <= 42              -- 小于等于
x >= 42              -- 大于等于
```

### 类型标注

- 函数参数：必须标注
- 函数返回值：可选（可推导）
- 局部变量：可选（可推导）
- 全局/模块变量：必须标注

```
x: int := 42         -- 显式标注
x := 42              -- 类型推导

name: string := "hello"
name := "hello"
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

data, ok := read_file("/etc/config")
```

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

---

## 错误处理（Result 类型）

### 函数返回 result

```
func read_file(path: string) -> result(string, error)
  if file_exists(path) then
    return ok(file_content)
  else
    return err("file not found")
  endif
endfunc
```

### 用 match 处理

```
match read_file("/etc/config")
  when ok(data) then
    write(data)
  when err(e) then
    write("error: ", e)
endmatch
```

---

## 模块

### 定义模块

```
module math_utils

export func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc

func helper() -> int       -- 私有，外部不可见
  ...
endfunc

endmodule
```

### 导入

```
-- 导入整个模块
use math_utils
result := math_utils.gcd(12, 8)

-- 导入具体函数
use gcd from math_utils
result := gcd(12, 8)
```

### 可见性

- 默认私有
- `export` 标记的函数/变量外部可用

---

## 并发

### 基本并行

```
parallel do
  deploy(server1)
  deploy(server2)
endparallel
```

### 获取结果

```
parallel => results do
  check(server1)
  check(server2)
endparallel
```

### 失败策略

```
-- 默认：一个失败全停
parallel do
  deploy(server1)
  deploy(server2)
endparallel

-- 允许失败，继续跑
parallel allow_fail do
  deploy(server1)
  deploy(server2)
endparallel

-- 组合：拿结果 + 允许失败（结果为 ok/err）
parallel allow_fail => results do
  check(server1)
  check(server2)
endparallel
```

---

## 导航标记（Tag）

语言内置的可选书签，不影响编译，方便审查和导航：

```
func deploy(config: Config)

  @validate
  check_config(config)

  @build
  build_project()

  @push
  push_to_server()

endfunc
```

### 命名 end

长函数的结束标记可以带名字：

```
func handle_request(req: Request) -> Response
  ...
endfunc handle_request
```

---

## 块结构关键字总览

| 结构 | 开始 | 结束 |
|------|------|------|
| 函数 | `func` | `endfunc` |
| 条件 | `if ... then` | `endif` |
| 循环 | `while ... do` | `endwhile` |
| 遍历 | `for ... do` | `endfor` |
| 匹配 | `match` | `endmatch` |
| 模块 | `module` | `endmodule` |
| 并行 | `parallel do` | `endparallel` |
| 匿名函数 | `=> ...` | `end` |

---

## 缩进与格式

- 缩进**不是语法要求**，块结构由 `end*` 关键字界定
- 编译器可出风格警告，但不报错

## 注释

```
-- 单行注释
```

---

## 完整示例

```
module server_ops

use http_client
use logger from logging

export func health_check(servers: list(string)) -> list(result(string, error))
  parallel allow_fail => results do
    for server in servers do
      check_one(server)
    endfor
  endparallel
  return results
endfunc

func check_one(server: string) -> result(string, error)
  @request
  resp, err := http_client.get(server + "/health")

  match err
    when ok(r) then
      if r.status = 200 then
        return ok(server + " is healthy")
      else
        return err(server + " returned " + r.status)
      endif
    when err(e) then
      return err(server + " unreachable: " + e)
  endmatch
endfunc check_one

endmodule
```
