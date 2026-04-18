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
| 内存域 | `arena` | `endarena` |
| 匿名函数 | `=> ...` | `endfunc`（块体）/ 无（单行） |
| 类型转换 | `as` | — |

---

## 类型关键字

| 关键字 | 说明 | 状态 |
|--------|------|------|
| `int8`, `int16`, `int32`, `int64` | 有符号定宽整数 | 已实现 |
| `uint8`, `uint16`, `uint32`, `uint64` | 无符号定宽整数 | 已实现 |
| `float32`, `float64` | 定宽浮点数（IEEE 754） | 已实现 |
| `money<Tag>` | 带币种 tag 的定点数 | 已实现 |
| `type` | 类型别名 | 已实现 |

---

## 其他关键字

| 关键字 | 用途 | 状态 |
|--------|------|------|
| `func`, `endfunc` | 函数定义 | 已实现 |
| `if`, `then`, `elif`, `else`, `endif` | 条件分支 | 已实现 |
| `while`, `do`, `endwhile` | 循环 | 已实现 |
| `for`, `in`, `to`, `step`, `endfor` | 遍历 | 已实现 |
| `order`, `reverse` | for 循环方向控制 | 已实现 |
| `with`, `index` | for 循环辅助 | 已实现 |
| `match`, `when`, `endmatch` | 模式匹配 | 已实现 |
| `ok`, `err` | Result 类型构造 | 已实现 |
| `as` | 类型转换 | 已实现 |
| `return` | 函数返回 | 已实现 |
| `and`, `or`, `not` | 逻辑运算 | 已实现 |
| `true`, `false` | 布尔字面量 | 已实现 |
| `mod` | 取模运算 | 已实现 |
| `global` | 全局变量声明 | 已实现 |
| `const` | 不可变绑定 | 已实现 |
| `module`, `endmodule` | 模块定义 | 已实现 |
| `export` | 导出函数 | 已实现 |
| `use`, `from` | 模块导入 | 已实现 |
| `parallel`, `endparallel` | 并行块（当前顺序执行） | 已实现（语法） |
| `allowfail` | 并行容错标记 | 已实现（语法） |
| `arena`, `endarena` | 内存域（当前无操作） | 已实现（语法） |

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
write(x, ...)        // 输出到标准输出
read(prompt)         // 读取一行输入（可选提示语）
len(list)            // 列表长度
append(list, item)   // 向列表追加元素
str(x), int(x), float(x), type(x)  // 类型相关函数
```

---

## 完整示例

```
module server_ops

use http_client
use logger from logging

export func health_check(servers: list<string>) -> list<string>
  results := []
  parallel allowfail => par_results do
    for server in servers do
      check_one(server)
    endfor
  endparallel
  return par_results
endfunc

func check_one(server: string) -> string
  @request
  status, success := http_client.get(server + "/health")

  if success then
    if status = 200 then
      return ok(server + " is healthy")
    else
      return err(server + " returned non-200")
    endif
  else
    return err(server + " unreachable")
  endif
endfunc

endmodule
```
