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
