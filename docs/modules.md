# Gwen 模块系统

## 定义模块

```gwen
module math_utils

use dep1 from helpers
use dep2 from other

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

```gwen
use gcd from math_utils
result := gcd(12, 8)
```

### 导入对象与类型别名

```gwen
use Accumulator, Count from math_utils

acc := Accumulator.new(10)
current: Count := acc.add(5)
```

类型别名不是运行时值，因此不能通过 `use math_utils` 后写 `math_utils.Count` 使用；
需要显式 `use Count from math_utils` 导入到当前作用域。

### 导入别名

Gwen `v0.1` **不支持导入别名**。

也就是说，下面这些写法当前都不允许：

```gwen
use gcd as g from math_utils
use math_utils as m
```

如果遇到重名或想保留来源信息，当前推荐做法是优先使用：

```gwen
use math_utils
result := math_utils.gcd(12, 8)
```

## 可见性

- 默认私有
- `export func`、`export object`、`export type` 才会暴露给模块外部
- `use name from module` 只能导入导出符号；私有函数 / 对象 / 类型别名都会报错
- 模块里的 `use` 也属于声明的一部分，必须放在模块体最前面；不要把 `use` 穿插在 `func/object/type` 后面
- `use module` 导入的命名空间只包含导出的运行时符号（函数、对象），不会带出私有成员
- Gwen `v0.1` 不支持 `use ... as ...` 形式的导入别名
- 模块内私有对象不会再因为运行时全局注册表泄漏到模块外部
- 从文件加载模块时，模块文件必须是**纯模块文件**：顶层只能有一个匹配的 `module ... endmodule` 定义，不允许额外顶层语句或旁路副作用
- 模块体本身也是**声明区**：顶层只允许 `use`、`func`、`object`、`type`（含 `export` 变体），不允许赋值、裸调用、`if/for/match`、`parallel` 等执行型语句
- 同一模块不能重复导出同一个运行时名或类型名
- `use` 不会静默覆盖当前作用域里的同名绑定；遇到冲突会直接报错

## 预执行检查

- Gwen 现在会在运行前先检查 `use` / `export`、类型名、对象成员和模块可见性
- `math_utils.Count` 这类把类型别名当运行时成员用的写法，会在执行前直接报错
- 模块文件如果夹带顶层 `write(...)`、赋值、函数定义等杂质，也会在加载前直接拒绝
- 模块体如果夹带初始化逻辑或副作用语句，也会在执行前直接拒绝；初始化请写成显式函数并由调用方主动调用
- 模块里如果用了 `use`，它必须出现在所有 `func/object/type` 之前；导入顺序不会靠“后置 use”隐式修补
- CLI 也支持单独检查：`python -m gwen check path/to/file.gw`
