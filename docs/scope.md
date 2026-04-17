# Gwen 变量作用域

Gwen 采用**显式作用域**设计：默认本地，修改外层需显式声明。

## 默认本地 (:=)

函数内使用 `:=` 总是在**当前函数级**创建或更新变量，不影响外层作用域：

```
counter: int := 0  // 模块级变量

func increment()
  counter := counter + 1  // 创建本地变量，模块级仍为 0
endfunc
```

## 显式全局 (global)

使用 `global` 关键字强制修改**外层作用域**（模块级或外层函数）已存在的变量：

```
counter: int := 0

func increment()
  global counter := counter + 1  // 修改模块级变量
endfunc
```

## 嵌套函数

嵌套函数可以使用 `global` 修改外层函数的变量：

```
func outer() -> int
  x: int := 10

  func inner()
    global x := x + 5  // 修改 outer 的 x
  endfunc

  inner()
  return x  // 返回 15
endfunc
```

## 作用域规则总结

| 语法 | 作用域 | 说明 |
|------|--------|------|
| `x := value` | 当前函数级 | 默认本地，不影响外层 |
| `global x := value` | 外层作用域 | 显式修改，变量必须已在外层声明 |
| `x: int := value` | 当前函数级 | 带类型的显式声明 |

## 显式优于隐式

代码读者无需推断 `:=` 是创建还是修改，一目了然：

- 看到 `:=` → 当前函数级，本地变量
- 看到 `global` → 跨层修改，必须已有声明

## 递归隔离

同一函数的递归调用各自拥有独立的参数和本地变量，互不影响：

```
func factorial(n: int) -> int
  if n <= 1 then
    return 1
  endif
  result := n * factorial(n - 1)  // 递归调用独立
  return result
endfunc
```

## 控制流块不创建新作用域

`if`、`while`、`for`、`match` 等控制流块**共享所在函数的作用域**，不会创建新的局部作用域：

```
func example()
  found := false

  // if 内 := 修改的是同一个 found
  if true then
    found := true
  endif
  write(found)  // true

  // match 同理
  result := 0
  match ok(42)
    when ok(v) then
      result := v  // 修改外部 result
  endmatch
  write(result)  // 42
  write(v)       // 42，模式变量也注入到当前作用域
endfunc
```

### 作用域边界总结

| 结构 | 创建新作用域？ | 说明 |
|------|---------------|------|
| `func` / `endfunc` | 是 | 函数独立作用域 |
| `module` / `endmodule` | 是 | 模块独立作用域 |
| `if` / `endif` | 否 | 共享父函数作用域 |
| `while` / `endwhile` | 否 | 共享父函数作用域 |
| `for` / `endfor` | 否 | 共享父函数作用域（循环变量也在父作用域） |
| `match` / `endmatch` | 否 | 共享父函数作用域，模式变量注入父作用域 |
| `arena` / `endarena` | 否 | 共享父函数作用域 |

### 设计理由

1. **与审计目标一致**：作用域边界少 → 推理简单。审计者只需关注函数边界。
2. **行为统一**：所有控制流块（if/while/for/match）行为一致，没有例外。
3. **match 模式变量外泄是有意为之**：`when ok(v)` 中的 `v` 在 match 之后仍可用。如果需要隔离，请封装为函数。
