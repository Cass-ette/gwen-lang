# Gwen 并发

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
