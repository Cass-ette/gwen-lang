# Gwen 变量作用域

Gwen 采用**显式作用域**设计：默认本地，修改外层需显式声明。

## 默认本地 (:=)

函数内使用 `:=` 总是在**当前函数级**创建或更新变量，不影响外层作用域：

```
counter: int := 0  -- 模块级变量

func increment()
  counter := counter + 1  -- 创建本地变量，模块级仍为 0
endfunc
```

## 显式全局 (global)

使用 `global` 关键字强制修改**外层作用域**（模块级或外层函数）已存在的变量：

```
counter: int := 0

func increment()
  global counter := counter + 1  -- 修改模块级变量
endfunc
```

## 嵌套函数

嵌套函数可以使用 `global` 修改外层函数的变量：

```
func outer() -> int
  x: int := 10

  func inner()
    global x := x + 5  -- 修改 outer 的 x
  endfunc

  inner()
  return x  -- 返回 15
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
  result := n * factorial(n - 1)  -- 递归调用独立
  return result
endfunc
```
