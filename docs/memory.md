# Gwen 内存管理

> **当前状态**：`arena` 块语法已实现，但解释器中仅为**语法占位**——块内代码正常执行，无实际内存池行为。真正的区域内存管理属于远期目标。

Gwen 采用**渐进式内存管理**：日常用 GC，性能关键处显式区域优化。

## 设计理念

| 场景 | 策略 | 理由 |
|------|------|------|
| 脚本/工具 | 全自动 GC | 快速开发，无需关心内存 |
| 后端服务 | GC + 显式区域 | 请求级批量释放，无碎片，审计友好 |
| 高性能路径 | 纯区域管理 | 零停顿，确定性释放 |

**显式优于隐式**：性能关键处必须显式标记，不能隐藏在后端。

---

## 全自动 GC（默认）

无需显式管理，适合快速开发：

```
func quick_tool()
  data := load_file()       -- 分配
  process(data)
  -- 函数结束，GC 自动回收
endfunc
```

---

## 显式区域管理（arena）

### 基本语法

```
arena block_name do
  -- 所有在此块内分配的对象属于该 arena
  obj := create_large_object()
  buf := allocate(4096)
endarena
-- 块结束，整个 arena 批量释放，无碎片
```

### 后端服务示例

```
func handle_request(req: Request) -> Response
  arena request_arena do
    -- 请求级内存池
    body := read_body(req)
    cache := parse_json(body)

    -- 大数组显式放入 arena
    buffer: byte[8192] in request_arena

    result := process(cache, buffer)
    return result
  endarena
  -- 请求结束，所有内存一次性释放
endfunc
```

### 嵌套区域

区域可以嵌套，内层先释放，外层后释放：

```
arena outer do
  big := allocate(10000)

  arena inner do
    small := allocate(100)
  endarena
  -- small 已释放，big 仍在

endarena
-- big 释放
```

---

## 编译时提示

编译器可以建议何时使用 arena：

```
-- 警告：循环内频繁分配，建议使用 arena
for i in 1 to 1000000 do
  temp := allocate(1024)    -- 提示: 考虑 arena
  process(temp)
endfor
```

---

## 与作用域结合

区域尊重函数作用域，可以跨函数传递 arena 引用（受限）：

```
func helper(data: Data, a: Arena)
  -- 在传入的 arena 中分配
  result := process_in(data, a)
  return result
endfunc

func main()
  arena main_arena do
    data := load()
    result := helper(data, main_arena)
  endarena
endfunc
```

---

## 实现状态

- [x] `arena ... do ... endarena` 语法
- [x] 嵌套区域支持
- [ ] 基础 GC（当前为 Python GC）
- [ ] `in arena` 分配修饰符
- [ ] 编译器分配热点提示
- [ ] 区域引用类型 `Arena`（受限传递）
- [ ] 真正的 arena 内存池（当前仅为语法占位）
