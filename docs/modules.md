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

func helper() -> int       -- 私有，外部不可见
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

## 可见性

- 默认私有
- `export` 标记的函数/变量外部可用
