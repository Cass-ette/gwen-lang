# Gwen 附录

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
| 类型转换 | `as` | — |

---

## 类型关键字

| 关键字 | 说明 |
|--------|------|
| `int8`, `int16`, `int32`, `int64` | 有符号定宽整数 |
| `uint8`, `uint16`, `uint32`, `uint64` | 无符号定宽整数 |
| `int<N>` | 自定义 N 位大整数 |
| `float32`, `float64` | 定宽浮点数 |
| `float<N>` | 自定义 N 位十进制精度浮点 |
| `decimal<P, S>` | 定点数：P 位总长，S 位小数 |
| `type` | 类型别名 |

---

## 运算符

| 优先级 | 运算符 | 说明 |
|--------|--------|------|
| 1 | `^` | 幂运算（右结合） |
| 2 | `*`, `/`, `mod` | 乘除取模 |
| 3 | `+`, `-` | 加减 |
| 4 | `=`, `!=`, `<`, `>`, `<=`, `>=` | 比较 |
| 5 | `and` | 逻辑与 |
| 6 | `or` | 逻辑或 |

---

## 内置函数

```
write(x, ...)        -- 输出到标准输出
read(prompt)         -- 读取一行输入（可选提示语）
len(list)            -- 列表长度
append(list, item)   -- 向列表追加元素
str(x), int(x), float(x), type(x)  -- 类型相关函数
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
