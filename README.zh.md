[English README](README.md)

# Gwen

Gwen 是一门面向后端与自动化场景的语言。

这个仓库里现在保留两条实现线：

- 一套可运行的 Go bootstrap 实现
- 一条能产出原生可执行文件的编译路径

## 现在的状态

- `cmd/gwen` 是当前主 CLI
- `gwen run/check/repl/build/emit-c` 可用
- 解释器、checker、前端、C emitter 都在仓库内
- `examples/` 里已经有 HTTP、SQLite、docs site、rules app、ledger app 等真实示例
- 旧的 Python 参考实现仍保留在仓库里，但当前主线是 Go 实现

## 版本线

| 版本线 | 实现 | 含义 |
|--------|------|------|
| `v0.1.0` | Python | 参考实现，保留用于历史对照。 |
| `v0.2.x` | Go | 当前主线实现，同时有解释路径和编译路径。 |

## 给 AI Agent

如果你是刚进入这个仓库的 AI coding agent，先读 [AGENTS.md](AGENTS.md)。

默认判断是：

- 当前主线工作优先看 Go 实现
- Python 实现是 `v0.1.0` 参考线
- 改语言行为前先读 [docs/philosophy.md](docs/philosophy.md)
- 声称完成前先跑最小相关验证

## 快速开始

```bash
go run ./cmd/gwen --version
go run ./cmd/gwen run examples/hello.gw
go run ./cmd/gwen check examples/hello.gw
go run ./cmd/gwen repl
```

如果你想直接走编译链，需要本机有 `cc`。例如：

```bash
go run ./cmd/gwen build examples/hello.gw -o /tmp/gwen-hello
/tmp/gwen-hello
```

也可以先只看 C 输出：

```bash
go run ./cmd/gwen emit-c examples/hello.gw
```

## 一个最小例子

```gwen
func gcd(a: int, b: int) -> int
  while b != 0 do
    a, b := b, a mod b
  endwhile
  return a
endfunc

func main()
  write(gcd(48, 18))
endfunc
```

Gwen 的几个基本表面：

- 块用 `endif/endwhile/endfor/endfunc` 显式闭合
- 错误处理走 `result[...]` + `match ok/err`
- 作用域默认本地，修改外层要显式写 `global`
- 并发必须显式写成 `parallel`

## 先试什么

### Hello

```bash
go run ./cmd/gwen run examples/hello.gw
```

### HTTP 示例

```bash
go run ./cmd/gwen run examples/http_server.gw
```

然后打开：

- `http://127.0.0.1:8080/`
- `http://127.0.0.1:8080/api/hello/Ada?lang=zh`
- `http://127.0.0.1:8080/assets/app.css`

### Session Notes

```bash
go run ./cmd/gwen run examples/session_notes.gw
```

然后打开：

- `http://127.0.0.1:8082/`
- `http://127.0.0.1:8082/login/Ada`
- `http://127.0.0.1:8082/api/me`

### 教学站原型

```bash
go run ./cmd/gwen run examples/docs_site/main.gw
```

然后打开：

- `http://127.0.0.1:8090/`
- `http://127.0.0.1:8090/api/health`
- `http://127.0.0.1:8090/api/site/zh`

这个站点直接读取仓库里的 Markdown 和 Gwen 示例。

## VSCode

仓库里有一个最小 VSCode 扩展，提供：

- `.gw` 语法高亮
- 基础 snippets
- 块结构相关缩进/注释配置

安装方式见 [vscode-extension/README.md](vscode-extension/README.md)。

## 仓库怎么读

- [docs/README.md](docs/README.md)
  语言文档入口
- [docs/syntax.md](docs/syntax.md)
  语法表面
- [docs/types.md](docs/types.md)
  类型系统
- [docs/stdlib.md](docs/stdlib.md)
  标准库边界
- [docs/compiler.md](docs/compiler.md)
  编译路线和当前后端边界
- [docs/philosophy.md](docs/philosophy.md)
  Gwen 接受设计时用的判断尺
- [docs/tracking.md](docs/tracking.md)
  文档与实现的对齐记录

## 目录

```text
gwen-lang/
├── cmd/gwen/           # CLI
├── internal/           # Go 主实现
├── gwen/               # 旧 Python 参考实现
├── docs/               # 语言文档
├── examples/           # 示例程序
├── tests/              # Python 侧测试与自检
└── vscode-extension/   # VSCode 扩展
```

## 测试

```bash
go test ./...
pytest
```
