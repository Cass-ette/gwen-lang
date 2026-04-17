# Gwen 并发

> **当前状态**：`parallel` 块语法已实现，但解释器中为**顺序执行**。真正的并行执行需要异步运行时，属于远期目标。

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

## 失败策略

### 默认：一个失败全停

```
parallel do
  deploy(server1)
  deploy(server2)
endparallel
```

### 允许失败，继续跑

```
parallel allow_fail do
  deploy(server1)
  deploy(server2)
endparallel
```

### 组合：拿结果 + 允许失败（结果为 ok/err）

```
parallel allow_fail => results do
  check(server1)
  check(server2)
endparallel
```
