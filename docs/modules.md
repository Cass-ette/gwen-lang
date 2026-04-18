# Gwen 模块系统

## 定义模块

```
module math_utils

export func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc

export object Accumulator
  total: int

  new(initial: int) -> Accumulator
    return Accumulator{total := initial}
  endnew

  func add(self: Accumulator, value: int) -> int
    self.total := self.total + value
    return self.total
  endfunc
endobject

export type Count = int

func helper() -> int       // 私有，外部不可见
  ...
endfunc

endmodule
```

## 导入

### 导入整个模块

```
use math_utils
result := math_utils.gcd(12, 8)
```

### 导入具体函数

```
use gcd from math_utils
result := gcd(12, 8)
```

### 导入对象与类型别名

```
use Accumulator, Count from math_utils

acc := Accumulator.new(10)
current: Count := acc.add(5)
```

类型别名不是运行时值，因此不能通过 `use math_utils` 后写 `math_utils.Count` 使用；
需要显式 `use Count from math_utils` 导入到当前作用域。

## 可见性

- 默认私有
- `export func`、`export object`、`export type` 才会暴露给模块外部
- `use name from module` 只能导入导出符号；私有函数 / 对象 / 类型别名都会报错
- `use module` 导入的命名空间只包含导出的运行时符号（函数、对象），不会带出私有成员
- 模块内私有对象不会再因为运行时全局注册表泄漏到模块外部
