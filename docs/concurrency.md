# Gwen 并发

> **当前状态**：Go runtime 中，`parallel` 块里的每条**顶层语句**都会并发执行。
> 每个任务拿到的是外层环境的独立快照，所以不会共享可变局部绑定。
> Python 参考实现仍保持顺序执行。

## 基本并行

```
parallel do
  deploy(server1)
  deploy(server2)
endparallel
```

## 获取结果

```
parallel => results do
  check(server1)
  check(server2)
endparallel
```

`results` 会按源码顺序收集：

- 表达式语句得到 `ok(value)`
- 非表达式语句得到 `ok(None)`
- `allowfail` 下的失败得到 `err(message)`

## 失败策略

### 默认：一个失败全停

```
parallel do
  deploy(server1)
  deploy(server2)
endparallel
```

当前 Go runtime 会先等待所有已启动任务结束，再按源码顺序返回第一条错误；这样结果是稳定可复现的。

### 允许失败，继续跑

```
parallel allowfail do
  deploy(server1)
  deploy(server2)
endparallel
```

### 组合：拿结果 + 允许失败（结果为 ok/err）

```
parallel allowfail => results do
  check(server1)
  check(server2)
endparallel
```

## 作用域边界

`parallel` 里的赋值和局部修改不会回写到块外：

```
x := 1

parallel do
  x := 2
endparallel

write(x)   // 仍然是 1
```

## 显式共享状态

如果确实需要跨任务共享状态，当前唯一推荐的主路径是 `cell[T]`：

```gwen
use state

counter: cell[int] := state.cell(0)

parallel do
  state.update(counter, (n: int) => n + 1)
  state.update(counter, (n: int) => n + 1)
endparallel

write(state.get(counter))   // 2
```

这里有两个关键规则：

- `parallel` 默认仍然是隔离快照；只有 `cell[T]` 这类显式状态值会跨任务共享
- `state.get(...)` 返回的是快照，`state.set(...)` / `state.update(...)` 写入的也是新快照，不把内部可变别名直接漏出去

推荐这样理解：

- 普通变量：默认隔离
- `cell[T]`：显式共享
- `state.update(...)`：显式原子读改写

这样 Gwen 仍然保持“默认不共享”，但给后端/并发代码留出一条可审计的共享状态通道。

## 当前 HTTP 服务端边界

`http.listen(...)` 现在开始对齐 `parallel` 的主语义：**每个请求都在独立快照里执行**。

- 同一个 `HttpServer` 可以并发处理多个请求
- 普通模块级 `dict` / `list` / 对象状态的修改不会跨请求保留，也不会回写到全局 runtime
- 只有显式 `cell[T]` 才会跨请求共享

这样设计的核心还是同一条：

- Gwen 冻结的是 `默认隔离，显式共享`
- `parallel` 里如此，HTTP handler 里也如此
- 如果共享意图很重要，就应该在代码表面写成 `cell[T]`

这意味着：

- 想在**单个请求内部**并发做独立任务，可以继续显式使用 `parallel`
- 想做跨请求计数、session、内存缓存，应该显式收口到 `cell[T]`
- 想靠模块级 `dict` / `list` 暂存服务端状态，现在不会自动累积到下一次请求
- 不要把今天的 HTTP 行为理解成 Gwen 已经支持“任意共享可变值都能安全并发”；当前安全边界仍然是 `cell[T]`
