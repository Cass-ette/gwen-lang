# Gwen 语言设计文档

> 审查优先、数学直觉、显式优于隐式

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

## 文档导航

| 文档 | 内容 |
|------|------|
| [syntax.md](./syntax.md) | 基础语法：赋值、控制流、函数、错误处理 |
| [types.md](./types.md) | 类型系统：**显式精度数值**、泛型、类型转换 |
| [scope.md](./scope.md) | 变量作用域：本地 vs 全局、嵌套函数 |
| [modules.md](./modules.md) | 模块系统：定义、导入、可见性 |
| [concurrency.md](./concurrency.md) | 并发：并行块、失败策略 |
| [memory.md](./memory.md) | 内存管理：GC + 显式区域 |
| [appendix.md](./appendix.md) | 附录：关键字、运算符、内置函数、完整示例 |
| [tracking.md](./tracking.md) | **实现跟踪表**：文档与代码对齐状态 |

---

## 快速示例

```
-- Hello World
func main()
  write("Hello, Gwen!")
endfunc
```

```
-- 快速排序
func sort(arr: list<int>) -> list<int>
  if len(arr) <= 1 then
    return arr
  endif
  -- ...
endfunc
```

```
-- 显式作用域
func counter()
  count: int := 0
  func increment()
    global count := count + 1  -- 显式修改外层
  endfunc
  increment()
  return count
endfunc
```

---

## 缩进与格式

- 缩进**不是语法要求**，块结构由 `end*` 关键字界定
- 编译器可出风格警告，但不报错

## 注释

```
-- 单行注释
```
